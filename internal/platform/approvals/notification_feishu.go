package approvals

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// FeishuSender sends 飞书群机器人 notifications.
//
// webhookURL 为自定义机器人 webhook（https://open.feishu.cn/open-apis/bot/v2/hook/…）。
// secret 为可选的加签密钥：官方校验方式为 sign = base64(hmac_sha256(key=timestamp+"\n"+secret, data=""))，
// payload 顶层附带 timestamp（秒）与 sign。
type FeishuSender struct {
	webhookURL string
	secret     string

	postJSON func(ctx context.Context, url string, payload []byte) error
}

// NewFeishuSender creates a new 飞书 sender.
func NewFeishuSender(webhookURL, secret string) *FeishuSender {
	return &FeishuSender{webhookURL: webhookURL, secret: secret}
}

// Send posts a text message to the group bot.
func (f *FeishuSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	if f.webhookURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": fmt.Sprintf("%s\n%s", event.Title, event.Message),
		},
	}
	if f.secret != "" {
		ts := time.Now().Unix()
		mac := hmac.New(sha256.New, []byte(fmt.Sprintf("%d\n%s", ts, f.secret)))
		payload["timestamp"] = fmt.Sprintf("%d", ts)
		payload["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	// payload 仅含 string 字面量与 HMAC/base64 派生 string，Marshal 恒成功，err 分支为死代码已删。
	body, _ := json.Marshal(payload)
	postJSON := f.postJSON
	if postJSON == nil {
		postJSON = defaultPostJSON
	}
	return postJSON(ctx, f.webhookURL, body)
}

// Channel returns the channel type.
func (f *FeishuSender) Channel() NotificationChannel {
	return ChannelFeishu
}
