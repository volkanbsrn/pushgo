package core

import "github.com/omerkirk/gcm"

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

func NewResponse(resp *gcm.Response, msg *gcm.Message) *Response {
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
			if result.Error == gcm.ResponseErrorInvalidRegistration || result.Error == gcm.ResponseErrorNotRegistered {
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
