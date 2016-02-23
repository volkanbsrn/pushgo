package ios

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"time"

	"github.com/RobotsAndPencils/buford/certificate"
	"github.com/RobotsAndPencils/buford/payload"
	"github.com/RobotsAndPencils/buford/push"
	"github.com/omerkirk/pushgo/core"
)

const (
	// Maximum number of messages to be queued
	maxNumberOfMessages = 100000000

	// Response channel buffer size
	responseChannelBufferSize = 100000
)

type Service struct {
	certificate tls.Certificate
	bundleID    string

	senderCount int

	isProduction bool

	respCh chan *core.Response

	msgQueue chan *msgResponse
}

func New(certName, passwd string, bundleID string, senderCount int, isProduction bool) *Service {
	cert, key, err := certificate.Load(certName, passwd)
	if err != nil {
		log.Fatal(err)
	}
	s := &Service{
		certificate:  certificate.TLS(cert, key),
		bundleID:     bundleID,
		isProduction: isProduction,

		senderCount: senderCount,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *msgResponse, maxNumberOfMessages)}

	for i := 0; i < senderCount; i++ {
		go s.sender()
	}
	return s
}

func (s *Service) Queue(msg *core.Message) {
	p := payload.APS{
		Alert: payload.Alert{Body: msg.Alert},
		Sound: msg.Sound}

	pm := p.Map()
	for k, v := range msg.Custom {
		pm[k] = v
	}
	b, err := json.Marshal(pm)
	if err != nil {
		log.Printf("pushgo: ios queue error: cannot convert msg to json %v\n", pm)
		return
	}
	msg.Bytes = b

	go s.msgDistributor(msg)
}

func (s *Service) Listen() chan *core.Response {
	return s.respCh
}

func (s *Service) msgDistributor(msg *core.Message) {
	respCh := make(chan error, responseChannelBufferSize)
	sr := &core.Response{
		Extra: msg.Extra,
	}
	h := &push.Headers{
		Topic:      s.bundleID,
		Expiration: time.Now().Add(time.Second * time.Duration(msg.Expiration))}
	if msg.Priority == core.PriorityNormal {
		h.LowPriority = true
	}
	for i := 0; i < len(msg.Devices); i++ {
		s.msgQueue <- &msgResponse{payload: msg.Bytes, device: msg.Devices[i], headers: h, respCh: respCh}
	}

	for {
		select {
		case err := <-respCh:
			sr.Total++
			if err != nil {
				sr.Failure++
				if err == push.ErrUnregistered {
					sp := core.Result{}
					sp.Type = core.ResponseTypeDeviceExpired
					sp.RegistrationID = msg.Device
					sr.Results = append(sr.Results, sp)
				}
			} else {
				sr.Success++
			}
			if sr.Total == len(msg.Devices) {
				s.respCh <- sr
				return
			}
		}
	}

}

type msgResponse struct {
	payload []byte
	device  string
	headers *push.Headers

	respCh chan error
}

func (s *Service) sender() {
	client, err := push.NewClient(s.certificate)
	if err != nil {
		log.Fatal(err)
	}
	apns := push.Service{
		Client: client}
	if s.isProduction {
		apns.Host = push.Production
	} else {
		apns.Host = push.Development
	}
	for {
		select {
		case mr := <-s.msgQueue:
			_, err := apns.PushBytes(mr.device, mr.headers, mr.payload)
			mr.respCh <- err
		}
	}
}
