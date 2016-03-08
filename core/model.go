package core

const (
	PriorityNormal = 0
	PriorityHigh   = 1
)

type Pusher interface {
	Queue(msg *Message)
	Listen() chan *Response
}

type Config struct {
	SenderCount int
	RetryCount  int
}

type Message struct {
	//Common
	Devices    []string
	Priority   int
	Expiration int

	// GCM related fields
	Json map[string]interface{}

	// APNS related fields
	Alert  string
	Sound  string
	Custom map[string]interface{}
	Bytes  []byte

	Extra map[string]interface{}
}

func NewGCMMessage(devices []string, json map[string]interface{}, priority int, expiration int, extra map[string]interface{}) *Message {
	return &Message{
		Json:       json,
		Priority:   priority,
		Expiration: expiration,

		Devices: devices,

		Extra: extra}
}

func NewAPNSMessage(devices []string, alert, sound string, custom map[string]interface{}, priority int, expiration int, extra map[string]interface{}) *Message {
	return &Message{
		Alert:  alert,
		Sound:  sound,
		Custom: custom,

		Priority:   priority,
		Expiration: expiration,

		Devices: devices,

		Extra: extra}
}

type DeviceList []string

func (dl DeviceList) Group(groupSize int) [][]string {
	deviceList := []string(dl)
	groupCount := (len(deviceList) / groupSize) + 1

	groupedDevices := make([][]string, groupCount)
	for i := 0; i < groupCount; i++ {
		lowerbound := groupSize * i
		upperbound := groupSize * (i + 1)
		if upperbound > len(deviceList) {
			upperbound = len(deviceList)
		}
		groupedDevices[i] = deviceList[lowerbound:upperbound]
	}

	return groupedDevices
}
