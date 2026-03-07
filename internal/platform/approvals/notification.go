package approvals

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NotificationEvent represents a notification event
type NotificationEvent struct {
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	InstanceID string                 `json:"instance_id,omitempty"`
	ApprovalID string                 `json:"approval_id,omitempty"`
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
	RateLimit int                 `json:"rate_limit"` // Max notifications per hour
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
	ReadAt    *time.Time          `json:"read_at,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
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

// EmailSender sends email notifications
type EmailSender struct {
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	fromAddress  string
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

// Send sends an email notification
func (e *EmailSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	// Email sending logic would be implemented here
	// This is a placeholder that would use net/smtp or a library
	/*
		subject := event.Title
		body := fmt.Sprintf(`
		<html>
		<body>
		<h2>%s</h2>
		<p>%s</p>
		%s
		</body>
		</html>
		`, event.Title, event.Message, e.formatData(event.Data))
	*/
	return nil
}

// Channel returns the channel type
func (e *EmailSender) Channel() NotificationChannel {
	return ChannelEmail
}

// DingTalkSender sends DingTalk notifications
type DingTalkSender struct {
	webhookURL string
	secret     string
}

// NewDingTalkSender creates a new DingTalk sender
func NewDingTalkSender(webhookURL, secret string) *DingTalkSender {
	return &DingTalkSender{
		webhookURL: webhookURL,
		secret:     secret,
	}
}

// Send sends a DingTalk notification
func (d *DingTalkSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	// DingTalk webhook message format
	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": event.Title,
			"text":  fmt.Sprintf("### %s\n\n%s", event.Title, event.Message),
		},
	}

	// Would make HTTP POST to webhook URL
	_ = message
	return nil
}

// Channel returns the channel type
func (d *DingTalkSender) Channel() NotificationChannel {
	return ChannelDingTalk
}

// WebhookSender sends webhook notifications
type WebhookSender struct {
	url      string
	secret   string
	audience string
}

// NewWebhookSender creates a new webhook sender
func NewWebhookSender(url, secret, audience string) *WebhookSender {
	return &WebhookSender{
		url:      url,
		secret:   secret,
		audience: audience,
	}
}

// Send sends a webhook notification
func (w *WebhookSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	payload := map[string]interface{}{
		"recipient": recipient,
		"event":     event,
		"timestamp": time.Now().Unix(),
	}

	// Would make HTTP POST with signature
	_ = payload
	return nil
}

// Channel returns the channel type
func (w *WebhookSender) Channel() NotificationChannel {
	return ChannelWebhook
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
