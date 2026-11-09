package approvals

import (
	"context"
	"fmt"
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
	ChannelWecom     NotificationChannel = "wecom"
	ChannelFeishu    NotificationChannel = "feishu"
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
				_ = n.store.RecordNotification(recipient, channel, event)
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
