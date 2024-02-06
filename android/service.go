package android

import (
	"log"

	"github.com/omerkirk/go-fcm"
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
	msgQueue chan *fcm.Message
}

func New(apiKey string, senderCount, retryCount int, isProduction bool) *Service {
	s := &Service{
		senderCount: senderCount,
		retryCount:  retryCount,

		isProduction: isProduction,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *fcm.Message, maxNumberOfMessages)}

	for i := 0; i < senderCount; i++ {
		go s.sender(i, apiKey)
	}
	return s
}

func (s *Service) Queue(msg *core.Message) {
	var ttl = uint(msg.Expiration)
	deviceGroups := core.DeviceList(msg.Devices).Group(1000)
	for i := 0; i < len(deviceGroups); i++ {
		fcmMsg := &fcm.Message{
			RegistrationIDs: deviceGroups[i],
			Data:            msg.Json,
			TimeToLive:      &ttl}

		if msg.Title != "" || msg.Text != "" {
			fcmMsg.Notification = &fcm.Notification{Title: msg.Title, Body: msg.Text, Icon: msg.Icon, Tag: msg.ThreadID}
		}
		if msg.Priority == core.PriorityHigh {
			fcmMsg.Priority = "high"
		} else {
			fcmMsg.Priority = "normal"
		}
		fcmMsg.SetExtra(msg.Extra)
		fcmMsg.DryRun = !s.isProduction
		s.msgQueue <- fcmMsg
	}
}

func (s *Service) Listen() chan *core.Response {
	return s.respCh
}

func (s *Service) sender(senderID int, apiKey string) {
	for {
		select {
		case msg := <-s.msgQueue:
			log.Printf("pushgo: sender %d received msg with extra %+v of %d devices\n", senderID, msg.Extra(), len(msg.RegistrationIDs))
			log.Printf("android service push: %+v", msg)
			go func(m *fcm.Message) {
				c, err := fcm.NewClient(apiKey)
				if err != nil {
					log.Fatalln(err)
				}
				resp, err := c.SendWithRetry(m, s.retryCount)
				log.Printf("pushgo: sender %d received response of message with extra %+v\n", senderID, msg.Extra())
				if err != nil {
					log.Println("pushgo error: ", err)
				} else {
					s.respCh <- core.NewResponse(resp, m)
					log.Printf("pushgo: sender %d pushed response of msg with extra %+v to response channel\n", senderID, msg.Extra())
				}
			}(msg)
		}
	}
}
