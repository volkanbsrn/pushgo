package huawei

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/omerkirk/go-hcm"
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
	msgQueue chan *hcm.Message

	atLock      sync.RWMutex
	accessToken string
}

func New(appID int, appSecret string, senderCount, retryCount int, isProduction bool) *Service {
	s := &Service{
		appID:     appID,
		appSecret: appSecret,

		senderCount: senderCount,
		retryCount:  retryCount,

		isProduction: isProduction,

		respCh: make(chan *core.Response, responseChannelBufferSize),

		msgQueue: make(chan *hcm.Message, maxNumberOfMessages)}
	go s.refreshAccessToken()
	for i := 0; i < senderCount; i++ {
		go s.sender(i)
	}
	return s
}

func (s *Service) refreshAccessToken() {
	for {
		at, expires, err := s.getAccessToken()
		if err != nil {
			log.Printf("pushgo huawei - cannot get access token: %+v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if at == "" || expires == 0 {
			log.Printf("pushgo huawei - invalid access token response received: %s - %d", at, expires)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("pushgo huawei - access token received: %s", at)
		s.atLock.Lock()
		s.accessToken = at
		s.atLock.Unlock()
		if expires > 600 {
			time.Sleep(time.Duration(expires-600) * time.Second)
		} else {
			time.Sleep(time.Duration(expires) * time.Second)
		}

	}
}

func (s *Service) getAccessToken() (string, int, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	data := fmt.Sprintf("grant_type=client_credentials&client_id=%d&client_secret=%s", s.appID, s.appSecret)
	req, err := http.NewRequest("POST", oAuthURL, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return "", 0, err
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
		return "", 0, err
	}
	defer resp.Body.Close()

	// check response status
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("%d error: %s", resp.StatusCode, resp.Status)
	}

	// build return
	response := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", 0, err
	}

	return response["access_token"].(string), int(response["expires_in"].(float64)), nil
}

func (s *Service) Queue(msg *core.Message) {
	deviceGroups := core.DeviceList(msg.Devices).Group(1000)
	data, err := json.Marshal(msg.Json)
	if err != nil {
		log.Printf("pushgo huawei - cannot marshal msj json: %+v", err)
		return
	}

	for i := 0; i < len(deviceGroups); i++ {
		hcmMsg := hcm.NewMessage(deviceGroups[i], string(data), strconv.Itoa(msg.Expiration), s.isProduction, msg.Extra)
		log.Printf("pushgo huawei - message: %+v", hcmMsg)
		s.msgQueue <- hcmMsg
	}
}

func (s *Service) Listen() chan *core.Response {
	return s.respCh
}

func (s *Service) sender(senderID int) {
	for {
		select {
		case msg := <-s.msgQueue:
			log.Printf("pushgo: sender %d received msg with extra %+v of %d devices\n", senderID, msg.Extra(), len(msg.Message.Token))
			go func(m *hcm.Message) {
				c, err := hcm.NewClient(s.appID)
				if err != nil {
					log.Fatalln(err)
				}
				s.atLock.RLock()
				resp, err := c.SendWithRetry(m, s.accessToken, s.retryCount)
				s.atLock.RUnlock()
				log.Printf("pushgo: sender %d received response of message with extra %+v\n", senderID, msg.Extra())
				if err != nil {
					log.Println("pushgo error: ", err)
				} else {
					s.respCh <- core.NewHCMResponse(resp, m)
					log.Printf("pushgo: sender %d pushed response of msg with extra %+v to response channel\n", senderID, msg.Extra())
				}
			}(msg)
		}
	}
}
