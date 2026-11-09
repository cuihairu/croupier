package approvals

import (
	"context"
	"encoding/json"
	"fmt"
)

// WecomSender sends 企业微信群机器人 notifications.
//
// webhookURL 为群机器人 webhook（https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…）。
// 企业微信机器人无加签机制，key 由 URL 携带。URL 为空时视为未配置，Send 直接 no-op。
type WecomSender struct {
	webhookURL string

	// postJSON 与 DingTalkSender 同款测试注入口。
	postJSON func(ctx context.Context, url string, payload []byte) error
}

// NewWecomSender creates a new 企业微信 sender.
func NewWecomSender(webhookURL string) *WecomSender {
	return &WecomSender{webhookURL: webhookURL}
}

// Send posts a markdown message to the group bot.
func (w *WecomSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	if w.webhookURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("### %s\n\n%s", event.Title, event.Message),
		},
	}
	// payload 为字面量构造的纯 string 嵌套 map，Marshal 恒成功，err 分支为死代码已删。
	body, _ := json.Marshal(payload)
	postJSON := w.postJSON
	if postJSON == nil {
		postJSON = defaultPostJSON
	}
	return postJSON(ctx, w.webhookURL, body)
}

// Channel returns the channel type.
func (w *WecomSender) Channel() NotificationChannel {
	return ChannelWecom
}
