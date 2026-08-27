package approvals

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NotificationEvent represents a notification event
type NotificationEvent struct {
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	InstanceID string                 `json:"instanceId,omitempty"`
	ApprovalID string                 `json:"approvalId,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Priority   string                 `json:"priority"` // low, normal, high, urgent
}

// NotificationChannel represents a notification delivery channel
type NotificationChannel string

const (
	ChannelEmail     NotificationChannel = "email"
	ChannelSMS       NotificationChannel = "sms"
	ChannelWebhook   NotificationChannel = "webhook"
	ChannelWebSocket NotificationChannel = "websocket"
	ChannelDingTalk  NotificationChannel = "dingtalk"
	ChannelWeChat    NotificationChannel = "wechat"
	ChannelSlack     NotificationChannel = "slack"
	ChannelInApp     NotificationChannel = "in_app"
)

// NotificationConfig represents configuration for a notification channel
type NotificationConfig struct {
	Channel   NotificationChannel `json:"channel"`
	Enabled   bool                `json:"enabled"`
	Config    map[string]string   `json:"config"`
	RateLimit int                 `json:"rateLimit"` // Max notifications per hour
}

// Notifier interface for sending notifications
type Notifier interface {
	Notify(ctx context.Context, recipients []string, event NotificationEvent) error
	NotifyWithChannels(ctx context.Context, recipients []string, event NotificationEvent, channels []NotificationChannel) error
}

// NotificationSender sends notifications via a specific channel
type NotificationSender interface {
	Send(ctx context.Context, recipient string, event NotificationEvent) error
	Channel() NotificationChannel
}

// MultiChannelNotifier implements Notifier with multiple channels
type MultiChannelNotifier struct {
	senders map[NotificationChannel]NotificationSender
	configs map[NotificationChannel]*NotificationConfig
	store   NotificationStore
	mu      sync.RWMutex
}

// NewMultiChannelNotifier creates a new multi-channel notifier
func NewMultiChannelNotifier(store NotificationStore) *MultiChannelNotifier {
	return &MultiChannelNotifier{
		senders: make(map[NotificationChannel]NotificationSender),
		configs: make(map[NotificationChannel]*NotificationConfig),
		store:   store,
	}
}

// RegisterSender registers a notification sender for a channel
func (n *MultiChannelNotifier) RegisterSender(sender NotificationSender) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.senders[sender.Channel()] = sender
}

// SetConfig sets configuration for a channel
func (n *MultiChannelNotifier) SetConfig(channel NotificationChannel, config *NotificationConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.configs[channel] = config
}

// Notify sends notifications through all enabled channels
func (n *MultiChannelNotifier) Notify(ctx context.Context, recipients []string, event NotificationEvent) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var channels []NotificationChannel
	for ch, config := range n.configs {
		if config.Enabled {
			channels = append(channels, ch)
		}
	}

	if len(channels) == 0 {
		return nil
	}

	return n.NotifyWithChannels(ctx, recipients, event, channels)
}

// NotifyWithChannels sends notifications through specified channels
func (n *MultiChannelNotifier) NotifyWithChannels(ctx context.Context, recipients []string, event NotificationEvent, channels []NotificationChannel) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var errors []error

	for _, channel := range channels {
		config, exists := n.configs[channel]
		if !exists || !config.Enabled {
			continue
		}

		sender, exists := n.senders[channel]
		if !exists {
			continue
		}

		for _, recipient := range recipients {
			// Check rate limit
			if config.RateLimit > 0 && n.store != nil {
				count, err := n.store.GetNotificationCount(recipient, channel, time.Hour)
				if err == nil && count >= config.RateLimit {
					continue
				}
			}

			// Send notification
			if err := sender.Send(ctx, recipient, event); err != nil {
				errors = append(errors, fmt.Errorf("channel %s: %w", channel, err))
				continue
			}

			// Record notification
			if n.store != nil {
				n.store.RecordNotification(recipient, channel, event)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %v", errors)
	}
	return nil
}

// NotificationStore interface for notification persistence
type NotificationStore interface {
	RecordNotification(recipient string, channel NotificationChannel, event NotificationEvent) error
	GetNotificationCount(recipient string, channel NotificationChannel, duration time.Duration) (int, error)
	GetNotifications(recipient string, limit int) ([]*NotificationRecord, error)
	MarkAsRead(id string) error
}

// NotificationRecord represents a stored notification
type NotificationRecord struct {
	ID        string              `json:"id"`
	Recipient string              `json:"recipient"`
	Channel   NotificationChannel `json:"channel"`
	Event     NotificationEvent   `json:"event"`
	Read      bool                `json:"read"`
	ReadAt    *time.Time          `json:"readAt,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
}

// InAppNotifier manages in-app notifications
type InAppNotifier struct {
	store NotificationStore
	hub   *WebSocketHub
}

// NewInAppNotifier creates a new in-app notifier
func NewInAppNotifier(store NotificationStore, hub *WebSocketHub) *InAppNotifier {
	return &InAppNotifier{
		store: store,
		hub:   hub,
	}
}

// Send sends an in-app notification
func (n *InAppNotifier) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	// Record the notification
	record := &NotificationRecord{
		ID:        generateID("notif"),
		Recipient: recipient,
		Channel:   ChannelInApp,
		Event:     event,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if n.store != nil {
		if err := n.store.RecordNotification(recipient, ChannelInApp, event); err != nil {
			return err
		}
	}

	// Push via WebSocket if connected
	if n.hub != nil {
		n.hub.SendToUser(recipient, map[string]interface{}{
			"type":       "notification",
			"id":         record.ID,
			"event":      event,
			"created_at": record.CreatedAt,
		})
	}

	return nil
}

// Channel returns the channel type
func (n *InAppNotifier) Channel() NotificationChannel {
	return ChannelInApp
}

// WebSocketHub manages WebSocket connections for real-time notifications
type WebSocketHub struct {
	clients    map[string][]*WebSocketClient
	broadcast  chan *BroadcastMessage
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	UserID string
	Send   chan []byte
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	UserID string
	Data   interface{}
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[string][]*WebSocketClient),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Run starts the hub
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = append(h.clients[client.UserID], client)
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			clients := h.clients[client.UserID]
			for i, c := range clients {
				if c == client {
					h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
					close(client.Send)
					break
				}
			}
			if len(h.clients[client.UserID]) == 0 {
				delete(h.clients, client.UserID)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg.Data)
			if err != nil {
				continue
			}
			h.mu.RLock()
			clients := h.clients[msg.UserID]
			for _, client := range clients {
				select {
				case client.Send <- data:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendToUser sends a message to all connections of a user
func (h *WebSocketHub) SendToUser(userID string, data interface{}) {
	h.broadcast <- &BroadcastMessage{
		UserID: userID,
		Data:   data,
	}
}

// Register registers a new WebSocket client
func (h *WebSocketHub) Register(client *WebSocketClient) {
	h.register <- client
}

// Unregister unregisters a WebSocket client
func (h *WebSocketHub) Unregister(client *WebSocketClient) {
	h.unregister <- client
}

// EmailSender sends email notifications.
//
// When smtpHost is empty the sender is treated as not-configured and Send is a
// no-op (returns nil) so callers can wire an EmailSender unconditionally
// without forcing SMTP configuration in dev/test environments. Send only ever
// contacts a real SMTP server when smtpHost is set.
type EmailSender struct {
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	fromAddress  string

	// sendMail is an injection point for tests in the same package. When nil,
	// the default net/smtp-based implementation (defaultSendMail) is used.
	sendMail func(ctx context.Context, msg *emailMessage) error
}

// emailMessage carries the rendered fields needed to produce an RFC 5322
// message via buildMessage.
type emailMessage struct {
	From    string
	To      string
	Subject string
	Body    string // HTML body
}

// NewEmailSender creates a new email sender
func NewEmailSender(host string, port int, user, password, from string) *EmailSender {
	return &EmailSender{
		smtpHost:     host,
		smtpPort:     port,
		smtpUser:     user,
		smtpPassword: password,
		fromAddress:  from,
	}
}

// Send sends an email notification.
//
// Returns nil without doing anything when the sender was constructed without
// an SMTP host, so an unwired EmailSender remains a safe no-op. Any failure
// while talking to the SMTP server is returned wrapped so callers can surface
// it via MultiChannelNotifier's aggregated error.
func (e *EmailSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	if e.smtpHost == "" {
		return nil
	}
	if err := validateEmailAddress(recipient); err != nil {
		return err
	}

	msg := &emailMessage{
		From:    e.fromAddress,
		To:      recipient,
		Subject: event.Title,
		Body:    buildEmailBody(event),
	}

	sendMail := e.sendMail
	if sendMail == nil {
		sendMail = e.defaultSendMail
	}
	return sendMail(ctx, msg)
}

// Channel returns the channel type
func (e *EmailSender) Channel() NotificationChannel {
	return ChannelEmail
}

// defaultSendMail connects to the configured SMTP server and delivers a single
// message. Port 465 uses implicit TLS; every other port uses a plain TCP
// connection and upgrades via STARTTLS when the server advertises it.
func (e *EmailSender) defaultSendMail(ctx context.Context, msg *emailMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	addr := net.JoinHostPort(e.smtpHost, strconv.Itoa(e.smtpPort))
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	var conn net.Conn
	var err error
	if e.smtpPort == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: e.smtpHost})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial smtp %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.smtpHost)
	if err != nil {
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer func() { _ = client.Quit() }()

	if e.smtpPort != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: e.smtpHost}); err != nil {
				return fmt.Errorf("smtp STARTTLS: %w", err)
			}
		}
	}

	if e.smtpUser != "" {
		if err := client.Auth(smtp.PlainAuth("", e.smtpUser, e.smtpPassword, e.smtpHost)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(e.fromAddress); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	// 邮件注入防线：RCPT 前校验收件人不含 CR/LF 等控制字符，
	// 防止恶意收件人字符串操纵 SMTP 命令序列。
	if err := validateEmailAddress(msg.To); err != nil {
		return err
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := writer.Write(buildEmailMessage(msg)); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp DATA close: %w", err)
	}

	return nil
}

// buildEmailMessage renders an RFC 5322 message with UTF-8 HTML body.
func buildEmailMessage(msg *emailMessage) []byte {
	headers := map[string]string{
		"From":         msg.From,
		"To":           msg.To,
		"Subject":      mime.QEncoding.Encode("utf-8", msg.Subject),
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	var buf bytes.Buffer
	for k, v := range headers {
		buf.WriteString(k)
		buf.WriteString(": ")
		buf.WriteString(v)
		buf.WriteString("\r\n")
	}
	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)
	return buf.Bytes()
}

// buildEmailBody renders an HTML body from the event fields, escaping user
// controlled text to avoid injection in HTML-literal mail clients.
func buildEmailBody(event NotificationEvent) string {
	var sb strings.Builder
	sb.WriteString("<html><body>\n")

	if event.Title != "" {
		sb.WriteString("<h2>")
		sb.WriteString(htmlEscape(event.Title))
		sb.WriteString("</h2>\n")
	}
	if event.Message != "" {
		sb.WriteString("<p>")
		sb.WriteString(htmlEscape(event.Message))
		sb.WriteString("</p>\n")
	}
	if len(event.Data) > 0 {
		sb.WriteString("<table border=\"1\" cellpadding=\"4\" cellspacing=\"0\">\n")
		for k, v := range event.Data {
			sb.WriteString("<tr><td><strong>")
			sb.WriteString(htmlEscape(k))
			sb.WriteString("</strong></td><td>")
			sb.WriteString(htmlEscape(fmt.Sprint(v)))
			sb.WriteString("</td></tr>\n")
		}
		sb.WriteString("</table>\n")
	}

	sb.WriteString("</body></html>")
	return sb.String()
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

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
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
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
	mac.Write([]byte(fmt.Sprintf("%d", ts)))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sep := "&"
	if !strings.Contains(d.webhookURL, "?") {
		sep = "?"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", d.webhookURL, sep, ts, url.QueryEscape(sign))
}

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
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook responded %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// NotificationTemplate represents a notification template
type NotificationTemplate struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Type      string                `json:"type"`
	Subject   string                `json:"subject"`
	Body      string                `json:"body"`
	Variables []string              `json:"variables"`
	Channels  []NotificationChannel `json:"channels"`
}

// TemplateRenderer renders notification templates
type TemplateRenderer struct {
	templates map[string]*NotificationTemplate
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		templates: make(map[string]*NotificationTemplate),
	}
}

// AddTemplate adds a notification template
func (r *TemplateRenderer) AddTemplate(template *NotificationTemplate) {
	r.templates[template.ID] = template
}

// Render renders a notification from a template
func (r *TemplateRenderer) Render(templateID string, variables map[string]string) (*NotificationEvent, error) {
	template, exists := r.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	// Simple variable substitution
	subject := template.Subject
	body := template.Body

	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		subject = replaceAll(subject, placeholder, value)
		body = replaceAll(body, placeholder, value)
	}

	return &NotificationEvent{
		Type:    template.Type,
		Title:   subject,
		Message: body,
	}, nil
}

// Helper function to replace all occurrences
func replaceAll(s, old, new string) string {
	result := ""
	for {
		idx := findSubstring(s, old)
		if idx == -1 {
			return result + s
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
}

func findSubstring(s, substr string) int {
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// validateEmailAddress 阻止 SMTP 命令注入：地址不得含 CR/LF/空格等
// 控制字符与分隔符（email-injection 防线），并要求 user@domain 基本形态。
func validateEmailAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("email recipient is required")
	}
	// net/mail.ParseAddress 是 CodeQL email-injection 认可的 sanitizer：
	// 拒绝含 CR/LF 等控制字符的地址，阻断 SMTP 命令注入。
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		return fmt.Errorf("invalid email recipient: %w", err)
	}
	// 只接受裸地址，不允许 "Display Name <addr>" 形态——
	// 显示名可能含注入载荷（如 Name <real@x>\r\nRCPT TO:<evil>）。
	if parsed.Address != addr {
		return fmt.Errorf("invalid email recipient: display name not allowed")
	}
	if strings.ContainsAny(addr, "\r\n \t;,") {
		return fmt.Errorf("invalid email recipient: control characters rejected")
	}
	return nil
}
