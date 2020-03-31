package pushgo

import (
	"github.com/omerkirk/pushgo/android"
	"github.com/omerkirk/pushgo/core"
	"github.com/omerkirk/pushgo/huawei"
	"github.com/omerkirk/pushgo/ios"
)

func NewGCM(apiKey string, senderCount, retryCount int, isProduction bool) core.Pusher {
	return android.New(apiKey, senderCount, retryCount, isProduction)
}

func NewHuawei(appID int, appSecret string, senderCount, retryCount int, isProduction bool) core.Pusher {
	return huawei.New(appID, appSecret, senderCount, retryCount, isProduction)
}

func NewAPNS(certName, passwd string, bundleID string, senderCount int, isProduction bool) core.Pusher {
	return ios.New(certName, passwd, bundleID, senderCount, isProduction)
}
