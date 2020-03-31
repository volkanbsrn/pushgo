package core

import (
	"encoding/json"
	"log"

	"github.com/omerkirk/go-fcm"
	"github.com/omerkirk/go-hcm"
)

const (
	ResponseTypeDeviceExpired = 1
	ResponseTypeDeviceChanged = 2
)

type Response struct {
	Success      int
	Failure      int
	ReasonMap    map[error]int
	CanonicalIDs int
	Total        int
	Extra        map[string]interface{}
	Results      []Result
}

type Result struct {
	Type              int
	RegistrationID    string
	NewRegistrationID string
}

func NewResponse(resp *fcm.Response, msg *fcm.Message) *Response {
	regIDs := msg.RegistrationIDs
	sr := &Response{
		Success:      resp.Success,
		Failure:      resp.Failure,
		CanonicalIDs: resp.CanonicalIDs,
		Extra:        msg.Extra(),
		Total:        len(regIDs)}

	if resp.Failure == 0 && resp.CanonicalIDs == 0 {
		return sr
	}

	serviceResults := make([]Result, 0)
	for i := 0; i < len(regIDs); i++ {
		result := resp.Results[i]
		if result.MessageID != "" {
			if result.RegistrationID != "" {
				sp := Result{}
				sp.Type = ResponseTypeDeviceChanged
				sp.RegistrationID = regIDs[i]
				sp.NewRegistrationID = result.RegistrationID
				serviceResults = append(serviceResults, sp)
			}
		} else {
			if result.Error == fcm.ErrInvalidRegistration || result.Error == fcm.ErrNotRegistered {
				sp := Result{}
				sp.Type = ResponseTypeDeviceExpired
				sp.RegistrationID = regIDs[i]
				serviceResults = append(serviceResults, sp)
			}
		}
	}
	sr.Results = serviceResults
	return sr
}

func NewHCMResponse(resp *hcm.Response, msg *hcm.Message) *Response {
	regIDs := msg.Message.Token
	sr := &Response{
		Extra: msg.Extra(),
		Total: len(regIDs)}

	if resp.Code == hcm.RespCodeSuccess {
		sr.Success = len(regIDs)
		sr.Failure = 0
	} else if resp.Code == hcm.RespCodePartialSuccess {
		var respMsg map[string]interface{}
		err := json.Unmarshal([]byte(resp.Message), &respMsg)
		if err != nil {
			log.Printf("cannot parse hcm msg %s: %+v", resp.Message, err)
		} else {
			sr.Success = int(respMsg["success"].(float64))
			sr.Failure = int(respMsg["failure"].(float64))
			illegalTokens := respMsg["illegal_tokens"].([]string)
			serviceResults := make([]Result, 0)
			for i := 0; i < len(illegalTokens); i++ {
				sp := Result{}
				sp.Type = ResponseTypeDeviceExpired
				sp.RegistrationID = illegalTokens[i]
				serviceResults = append(serviceResults, sp)
			}
			sr.Results = serviceResults
		}
	}

	return sr
}
