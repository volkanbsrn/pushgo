package ios

import (
	"log"
	"net/http"

	"github.com/omerkirk/apns"
)

type Service struct {
	apnsClient  *apns.Client
	senderCount int
	retryCount  int

	isProduction bool

	respCh   chan *ServiceResponse
	msgQueue chan *apns.Notification

	client *http.Client
}

func StartService(apiKey string, senderCount, retryCount int, isProduction bool) *Service {
	apnsService := &Service{

		senderCount: senderCount,
		retryCount:  retryCount,

		isProduction: isProduction,

		respCh: make(chan *ServiceResponse, responseChannelBufferSize),

		msgQueue: make(chan *apns.Notification, maxNumberOfMessages)}

	for i := 0; i < senderCount; i++ {
		go gcmService.sender()
	}
	return gcmService
}

func (s *Service) Queue(msg *apns.Notification) {
	s.msgQueue <- msg
}

func (s *Service) Listen() chan *ServiceResponse {
	return s.respCh
}

func (s *Service) sender() {
	for {
		select {
		case msg := <-s.msgQueue:
			resp, err := s.apnsClient.Send(msg)
			if err != nil {
				log.Println("pushgo error: ", err)
			} else {
				s.respCh <- NewServiceResponse(resp, msg)
			}

		}
	}
}
