package ios

import (
	"log"
	"net/http"
	"time"

	"github.com/omerkirk/apns"
)

type Service struct {
	apnsClient *apns.Client

	certFile string
	keyFile  string

	senderCount int
	retryCount  int

	isProduction bool

	respCh   chan *ServiceResponse
	msgQueue chan *apns.Notification

	client *http.Client
}

func StartService(certFile, keyFile string, senderCount, retryCount int, isProduction bool) *Service {
	gw := apns.SandboxGateway
	if isProduction {
		gw = apns.ProductionGateway
	}
	apnsClient := apns.NewClient(gw, certFile, keyFile)
	apnsService := &Service{
		client: apnsClient,

		certFile: certFile,
		keyFile:  keyFile,

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

func (s *Service) listener() {
	for f := range s.client.FailedNotifs {
		log.Printf("Notif", f.Notif.ID, "failed with", f.Err.Error())
	}
}

func (s *Service) feedbackListener() {
	for {
		f, err := apns.NewFeedback(apns.ProductionFeedbackGateway, s.certFile, s.keyFile)
		if err != nil {
			log.Println("Could not create feedback", err.Error())
		} else {
			for ft := range f.Receive() {
				log.Println("Feedback for token:", ft.DeviceToken)
			}
		}
		time.Sleep(1 * time.Hour)
	}
}
