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
	senderCount int
	retryCount  int

	isProduction bool

	respCh   chan *core.Response
	msgQueue chan *gcm.Message
}

func New(apiKey string, senderCount, retryCount int, isProduction bool) *Service {
	s := &Service{
		senderCount: senderCount,
		retryCount:  retryCount,

		isProduction: isProduction,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *gcm.Message, maxNumberOfMessages)}

	for i := 0; i < senderCount; i++ {
		go s.sender(i, apiKey)
	}
	return s
}

func (s *Service) Queue(msg *core.Message) {
	priority := gcm.MessagePriorityHigh
	if msg.Priority == core.PriorityNormal {
		priority = gcm.MessagePriorityNormal
	}
	deviceGroups := core.DeviceList(msg.Devices).Group(1000)
	for i := 0; i < len(deviceGroups); i++ {
		gcmMsg := gcm.NewMessage(deviceGroups[i], msg.Json, priority, msg.Expiration)
		gcmMsg.SetExtra(msg.Extra)
		gcmMsg.DryRun = !s.isProduction
		s.msgQueue <- gcmMsg
	}
}

func (s *Service) Listen() chan *core.Response {
	return s.respCh
}

func (s *Service) sender(senderID int, apiKey string) {
	for {
		select {
		case msg := <-s.msgQueue:
			extra := msg.Extra()
			header := extra["header"]
			thread := extra["thread"]
			log.Printf("pushgo: sender %d received msg from thread %d of %d devices for header %d\n", senderID, thread, len(msg.RegistrationIDs), header)
			go func(m *gcm.Message, thread, header interface{}) {
				c := &gcm.Sender{ApiKey: apiKey}
				resp, err := c.Send(m, s.retryCount)
				log.Printf("pushgo: sender %d received response of thread %d for header %d\n", senderID, thread, header)
				if err != nil {
					log.Println("pushgo error: ", err)
				} else {
					s.respCh <- core.NewResponse(resp, m)
					log.Printf("pushgo: sender %d pushed response of thread %d to response channel for header %d\n", senderID, thread, header)
				}
			}(msg, thread, header)
		}
	}
}
