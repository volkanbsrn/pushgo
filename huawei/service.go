package huawei

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/omerkirk/go-fcm"
	"github.com/omerkirk/pushgo/core"
)

const (
	// Maximum number of messages to be queued
	maxNumberOfMessages = 100000

	// Response channel buffer size
	responseChannelBufferSize = 1000

	oAuthURL = "https://oauth-login.cloud.huawei.com/oauth2/v3/token"
)

type Service struct {
	appID     int
	appSecret string

	senderCount int
	retryCount  int

	isProduction bool

	respCh   chan *core.Response
	msgQueue chan *fcm.Message
}

func New(appID int, appSecret string, senderCount, retryCount int, isProduction bool) *Service {
	s := &Service{
		appID:     appID,
		appSecret: appSecret,

		senderCount: senderCount,
		retryCount:  retryCount,

		isProduction: isProduction,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *fcm.Message, maxNumberOfMessages)}
	go s.refreshAccessToken()
	return s
}

func (s *Service) refreshAccessToken() {
	for {
		at, err := s.getAccessToken()
		if err != nil {
			log.Printf("cannot get access token: %+v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("access token received: %s", at)
		time.Sleep(10 * time.Second)
	}
}

func (s *Service) getAccessToken() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	data := fmt.Sprintf("grant_type=client_credentials&client_id=%d&client_secret=%s", s.appID, s.appSecret)
	req, err := http.NewRequest("POST", oAuthURL, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return "", err
	}

	// request with timeout
	ctx, cancel := context.WithTimeout(context.TODO(), client.Timeout)
	defer cancel()
	req = req.WithContext(ctx)

	// add headers
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%d error: %s", resp.StatusCode, resp.Status)
	}

	// build return
	response := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	log.Printf("access token: %s will expire in %d", response["access_token"], response["expires_in"])
	return response["access_token"].(string), nil
}

func (s *Service) Queue(msg *core.Message) {
	var ttl = uint(msg.Expiration)
	deviceGroups := core.DeviceList(msg.Devices).Group(1000)
	for i := 0; i < len(deviceGroups); i++ {
		fcmMsg := &fcm.Message{
			RegistrationIDs: deviceGroups[i],
			Data:            msg.Json,
			TimeToLive:      &ttl}

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
