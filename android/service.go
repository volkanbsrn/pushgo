package android

import (
	"log"

	"github.com/omerkirk/gcm"
	"github.com/omerkirk/pushgo/core"
)

const (
	// Maximum number of messages to be queued
	maxNumberOfMessages = 100000

	// Response channel buffer size
	responseChannelBufferSize = 1000
)

type Service struct {
	client *gcm.Sender

	senderCount int
	retryCount  int

	isProduction bool

	respCh   chan *core.Response
	msgQueue chan *gcm.Message
}

func New(apiKey string, senderCount, retryCount int, isProduction bool) *Service {
	s := &Service{
		client: &gcm.Sender{ApiKey: apiKey},

		senderCount: senderCount,
		retryCount:  retryCount,

		isProduction: isProduction,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *gcm.Message, maxNumberOfMessages)}

	for i := 0; i < senderCount; i++ {
		go s.sender()
	}
	return s
}

func (s *Service) Queue(msg *core.Message) {
	gcmMsg := gcm.NewMessage(nil, msg.Json, "", msg.Expiration)
	if msg.Priority == core.PriorityNormal {
		gcmMsg.Priority = gcm.MessagePriorityNormal
	} else if msg.Priority == core.PriorityHigh {
		gcmMsg.Priority = gcm.MessagePriorityHigh
	}
	gcmMsg.SetExtra(msg.Extra)
	if s.isProduction {
		gcmMsg.DryRun = false
	} else {
		gcmMsg.DryRun = true
	}
	deviceGroups := core.DeviceList(msg.Devices).Group(1000)
	for i := 0; i < len(deviceGroups); i++ {
		gcmMsg.RegistrationIDs = deviceGroups[i]
		s.msgQueue <- gcmMsg
	}

}

func (s *Service) Listen() chan *core.Response {
	return s.respCh
}

func (s *Service) sender() {
	for {
		select {
		case msg := <-s.msgQueue:
			resp, err := s.client.Send(msg, s.retryCount)
			if err != nil {
				log.Println("pushgo error: ", err)
			} else {
				s.respCh <- core.NewResponse(resp, msg)
			}

		}
	}
}
