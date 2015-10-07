package android

import "github.com/omerkirk/gcm"

const (
	ResponseTypeDeviceExpired = 1
	ResponseTypeDeviceChanged = 2
)

type ServiceResponse struct {
	Success      int
	Failure      int
	CanonicalIDs int
	Total        int
	Results      []ServiceResult
}

type ServiceResult struct {
	Type              int
	RegistrationID    string
	NewRegistrationID string
}

func NewServiceResponse(resp *gcm.Response, regIDs []string) *ServiceResponse {
	sr := &ServiceResponse{
		Success:      resp.Success,
		Failure:      resp.Failure,
		CanonicalIDs: resp.CanonicalIDs,
		Total:        len(regIDs)}

	if resp.Failure == 0 && resp.CanonicalIDs == 0 {
		return sr
	}

	serviceResults := make([]ServiceResult, 0)
	for i := 0; i < len(regIDs); i++ {
		sp := ServiceResult{}
		result := resp.Results[i]
		if result.MessageID != "" {
			if result.RegistrationID != "" {
				sp.Type = ResponseTypeDeviceChanged
				sp.RegistrationID = regIDs[i]
				sp.NewRegistrationID = result.RegistrationID
				serviceResults = append(serviceResults, sp)
			}
		} else {
			if result.Error == gcm.ResponseErrorInvalidRegistration || result.Error == gcm.ResponseErrorNotRegistered {
				sp.Type = ResponseTypeDeviceExpired
				sp.RegistrationID = regIDs[i]
				serviceResults = append(serviceResults, sp)
			}
		}
	}
	sr.Results = serviceResults
	return sr
}
