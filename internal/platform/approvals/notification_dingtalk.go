package approvals

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// DingTalkSender sends DingTalk group-bot notifications.
//
// When webhookURL is empty the sender is treated as not-configured and Send is
// a no-op (returns nil) — same philosophy as EmailSender. With secret set the
// official HMAC-SHA256 timestamp signature is appended to the webhook URL.
type DingTalkSender struct {
	webhookURL string
	secret     string

	// postJSON is an injection point for tests in the same package. When nil,
	// the default net/http implementation is used.
	postJSON func(ctx context.Context, url string, payload []byte) error
}

// NewDingTalkSender creates a new DingTalk sender. webhookURL is the group
// robot webhook (https://oapi.dingtalk.com/robot/send?access_token=…);
// secret is the optional 加签 key (SEC…).
func NewDingTalkSender(webhookURL, secret string) *DingTalkSender {
	return &DingTalkSender{webhookURL: webhookURL, secret: secret}
}

// Send posts a markdown message to the group bot.
// recipient is unused (group bots address the whole group).
func (d *DingTalkSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	if d.webhookURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": event.Title,
			"text":  fmt.Sprintf("### %s\n\n%s", event.Title, event.Message),
		},
	}
	// payload 为字面量构造的纯 string 嵌套 map，Marshal 恒成功，err 分支为死代码已删。
	body, _ := json.Marshal(payload)
	postJSON := d.postJSON
	if postJSON == nil {
		postJSON = defaultPostJSON
	}
	return postJSON(ctx, d.signedURL(), body)
}

// Channel returns the channel type
func (d *DingTalkSender) Channel() NotificationChannel {
	return ChannelDingTalk
}

// signedURL appends the official &timestamp=…&sign=… query when a secret is
// configured; otherwise the raw webhook URL is returned.
func (d *DingTalkSender) signedURL() string {
	if d.secret == "" {
		return d.webhookURL
	}
	ts := time.Now().UnixMilli()
	mac := hmac.New(sha256.New, []byte(d.secret))
	_, _ = fmt.Fprintf(mac, "%d", ts)
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sep := "&"
	if !strings.Contains(d.webhookURL, "?") {
		sep = "?"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", d.webhookURL, sep, ts, url.QueryEscape(sign))
}
