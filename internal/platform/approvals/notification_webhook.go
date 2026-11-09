package approvals

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/secguard"
)

// WebhookSender sends generic signed webhook notifications.
//
// When url is empty the sender is treated as not-configured and Send is a
// no-op. When secret is set the request carries an HMAC-SHA256 signature
// header (X-Croupier-Signature) so receivers can verify authenticity.
type WebhookSender struct {
	url      string
	secret   string
	audience string

	// postJSON is an injection point for tests in the same package. When nil,
	// the default net/http implementation is used.
	postJSON func(ctx context.Context, u string, payload []byte, headers map[string]string) error
}

// NewWebhookSender creates a new generic webhook sender.
func NewWebhookSender(url, secret, audience string) *WebhookSender {
	return &WebhookSender{url: url, secret: secret, audience: audience}
}

// Send posts a JSON envelope to the configured endpoint.
// recipient is carried in the payload for receiver-side routing.
func (w *WebhookSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	if w.url == "" {
		return nil
	}
	payload := map[string]interface{}{
		"recipient": recipient,
		"audience":  w.audience,
		"event":     event,
		"timestamp": time.Now().Unix(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{}
	if w.secret != "" {
		mac := hmac.New(sha256.New, []byte(w.secret))
		mac.Write(body)
		headers["X-Croupier-Signature"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	postJSON := w.postJSON
	if postJSON == nil {
		postJSON = func(ctx context.Context, u string, payload []byte, hdrs map[string]string) error {
			return defaultPostJSONWithHeaders(ctx, u, payload, hdrs)
		}
	}
	return postJSON(ctx, w.url, body, headers)
}

// Channel returns the channel type
func (w *WebhookSender) Channel() NotificationChannel {
	return ChannelWebhook
}

// defaultPostJSON posts a JSON body with a 10s timeout.
func defaultPostJSON(ctx context.Context, u string, payload []byte) error {
	return defaultPostJSONWithHeaders(ctx, u, payload, nil)
}

func defaultPostJSONWithHeaders(ctx context.Context, u string, payload []byte, headers map[string]string) error {
	// 出站安全守卫（OPEN-ISSUES #56）：sec.* 四键全关时零开销直通
	guard := secguard.Resolve(settings.Current())
	if err := secguard.CheckURL(ctx, guard, u); err != nil {
		return err
	}
	// 出站调用策略（OPEN-ISSUES #57）：net.requestTimeoutMs 覆盖整体预算
	// （重试共享预算防风暴），net.maxRetries/Backoff 控制重试
	reqCtx, cancel := context.WithTimeout(ctx, guard.TimeoutOrDefault(10*time.Second))
	defer cancel()
	hdrs := map[string]string{"Content-Type": "application/json"}
	for k, v := range headers {
		hdrs[k] = v
	}
	resp, err := secguard.DoWithRetry(reqCtx, secguard.HTTPClient(guard, http.DefaultClient),
		http.MethodPost, u, payload, hdrs, guard.Retries(), guard.Backoff())
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook responded %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
