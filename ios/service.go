package ios

import (
	"fmt"
	"log"
	"time"

	"github.com/kokteyldev/apns2"
	"github.com/kokteyldev/apns2/payload"
	"github.com/kokteyldev/apns2/token"
	"github.com/omerkirk/pushgo/core"
)

const (
	// Maximum number of messages to be queued
	maxNumberOfMessages = 100000000

	// Response channel buffer size
	responseChannelBufferSize = 100000
)

type Service struct {
	client    *apns2.Client
	devClient *apns2.Client
	bundleID  string

	senderCount int

	isProduction bool

	respCh chan *core.Response

	msgQueue chan *message
}

func New(authFile, teamID, keyID string, bundleID string, senderCount int, isProduction bool) *Service {
	authKey, err := token.AuthKeyFromFile(authFile)
	if err != nil {
		log.Fatal("token error:", err)
	}

	token := &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}

	s := &Service{
		bundleID:     bundleID,
		isProduction: isProduction,

		senderCount: senderCount,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *message, maxNumberOfMessages)}

	if isProduction {
		s.client = apns2.NewTokenClient(token).Production()
		s.devClient = apns2.NewTokenClient(token).Development()
	} else {
		s.client = apns2.NewTokenClient(token).Development()
	}

	for i := 0; i < senderCount; i++ {
		go s.sender()
	}
	return s
}

func (s *Service) Queue(msg *core.Message) {
	p := payload.NewPayload().Alert(msg.Alert).Sound(msg.Sound).ThreadID(msg.ThreadID)

	for k, v := range msg.Custom {
		p.Custom(k, v)
	}
	if msg.Title != "" {
		p.AlertBody(msg.Alert)
		p.AlertTitle(msg.Title)
	}
	if msg.Icon != "" {
		p = p.MutableContent()
		p.Custom("media-url", msg.Icon)
	}

	if msg.PushType == string(apns2.PushTypeLiveActivity) {
		if msg.Event == "end" {
			p.DismissalDate(time.Now().Add(time.Second * time.Duration(msg.DismissDuration)).Unix())
		}
		p.Event(msg.Event)
		p.ContentState(msg.ContentState)
		p.Timestamp(time.Now().Unix())
		p.SetAttributesType(msg.AttributesType)
		p.SetAttributes(msg.Attributes)
		log.Printf("live activity: timestamp: %d, push type: %s, event: %s, content state: %v, attribute type: %s, attributes: %v",
			time.Now().Unix(), msg.PushType, msg.Event, msg.ContentState, msg.AttributesType, msg.Attributes)
	}
	b, err := p.MarshalJSON()
	if err != nil {
		log.Printf("pushgo: ios queue error: cannot convert msg to json %+v\n", p)
		return
	}
	msg.Bytes = b

	go s.msgDistributor(msg)
}

func (s *Service) Listen() chan *core.Response {
	return s.respCh
}

func (s *Service) msgDistributor(msg *core.Message) {
	respCh := make(chan *response, responseChannelBufferSize)
	sr := &core.Response{
		Extra:     msg.Extra,
		ReasonMap: make(map[string]int),
	}
	groupSize := (len(msg.Devices) / s.senderCount) + 1
	deviceGroups := core.DeviceList(msg.Devices).Group(groupSize)
	for i := 0; i < len(deviceGroups); i++ {
		for j := 0; j < len(deviceGroups[i]); j++ {
			n := &apns2.Notification{
				DeviceToken: deviceGroups[i][j],
				Topic:       s.bundleID,
				Expiration:  time.Now().Add(time.Second * time.Duration(msg.Expiration)),
				Payload:     msg.Bytes,
			}
			if msg.Priority == core.PriorityNormal {
				n.Priority = apns2.PriorityLow
			} else {
				n.Priority = apns2.PriorityHigh
			}
			if msg.PushType == string(apns2.PushTypeLiveActivity) {
				n.PushType = apns2.PushTypeLiveActivity
				n.Topic = fmt.Sprintf("%s.push-type.liveactivity", s.bundleID)
			}
			s.msgQueue <- &message{n, respCh}
		}
	}

	for {
		select {
		case res := <-respCh:
			resp := res.resp
			sr.Total++
			if resp.Sent() {
				sr.Success++
			} else {
				sr.Failure++
				if count, ok := sr.ReasonMap[resp.Reason]; ok {
					sr.ReasonMap[resp.Reason] = count + 1
				} else {
					sr.ReasonMap[resp.Reason] = 1
				}
				// IOs specific error can be returned even if the returned error is not of type push.Error
				if resp.Reason == apns2.ReasonUnregistered || resp.Reason == apns2.ReasonDeviceTokenNotForTopic {
					sp := core.Result{}
					sp.Type = core.ResponseTypeDeviceExpired
					sp.RegistrationID = res.deviceToken
					sr.Results = append(sr.Results, sp)
				}
			}
			if sr.Total == len(msg.Devices) {
				s.respCh <- sr
				return
			}
		}
	}

}

type message struct {
	notif  *apns2.Notification
	respCh chan *response
}

type response struct {
	resp        *apns2.Response
	deviceToken string
}

func (s *Service) sender() {
	for {
		select {
		case msg := <-s.msgQueue:
			go func(m *message) {
				log.Printf("ios service push: %+v", string(msg.notif.Payload.([]byte)))
				res, err := s.client.Push(msg.notif)
				log.Printf("response: %+v", res)
				if err != nil {
					log.Println("pushgo error: ", err)
				} else {
					msg.respCh <- &response{res, msg.notif.DeviceToken}
				}
				if s.devClient != nil {
					res, err := s.devClient.Push(msg.notif)
					if err != nil {
						log.Println("pushgo error: ", err)
					} else {
						msg.respCh <- &response{res, msg.notif.DeviceToken}
					}
				}

			}(msg)
		}
	}
}
