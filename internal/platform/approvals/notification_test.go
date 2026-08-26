package approvals

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockNotificationStore implements NotificationStore for testing.
type mockNotificationStore struct {
	records []*NotificationRecord
	count   int
	err     error
}

func (m *mockNotificationStore) RecordNotification(recipient string, channel NotificationChannel, event NotificationEvent) error {
	m.records = append(m.records, &NotificationRecord{
		Recipient: recipient,
		Channel:   channel,
		Event:     event,
		CreatedAt: time.Now(),
	})
	return m.err
}

func (m *mockNotificationStore) GetNotificationCount(recipient string, channel NotificationChannel, duration time.Duration) (int, error) {
	return m.count, m.err
}

func (m *mockNotificationStore) GetNotifications(recipient string, limit int) ([]*NotificationRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if limit > len(m.records) {
		limit = len(m.records)
	}
	return m.records[:limit], nil
}

func (m *mockNotificationStore) MarkAsRead(id string) error {
	return m.err
}

// mockNotificationSender implements NotificationSender for testing.
type mockNotificationSender struct {
	channel NotificationChannel
	err     error
	sent    []NotificationEvent
}

func (m *mockNotificationSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	m.sent = append(m.sent, event)
	return m.err
}

func (m *mockNotificationSender) Channel() NotificationChannel {
	return m.channel
}

func TestNewEmailSender(t *testing.T) {
	s := NewEmailSender("smtp.example.com", 587, "user", "pass", "from@example.com")
	if s == nil {
		t.Fatal("expected non-nil sender")
	}
	if s.Channel() != ChannelEmail {
		t.Errorf("expected channel %q, got %q", ChannelEmail, s.Channel())
	}
}

func TestEmailSender_Send(t *testing.T) {
	// When SMTP host is empty, Send must be a no-op so an unwired sender
	// stays safe in dev/test environments.
	s := NewEmailSender("", 0, "", "", "from@example.com")
	if err := s.Send(context.Background(), "to@example.com", NotificationEvent{Title: "Test", Message: "Body"}); err != nil {
		t.Errorf("expected nil error when not configured, got: %v", err)
	}
}

func TestEmailSender_Send_RequiresRecipient(t *testing.T) {
	s := NewEmailSender("smtp.example.com", 587, "user", "pass", "from@example.com")
	if err := s.Send(context.Background(), "", NotificationEvent{Title: "Test"}); err == nil {
		t.Error("expected error for empty recipient")
	}
}

func TestEmailSender_Send_PropagatesSendError(t *testing.T) {
	s := NewEmailSender("smtp.example.com", 587, "user", "pass", "from@example.com")
	wantErr := errors.New("smtp boom")
	s.sendMail = func(ctx context.Context, msg *emailMessage) error {
		return wantErr
	}

	if err := s.Send(context.Background(), "to@example.com", NotificationEvent{Title: "Test"}); err != wantErr {
		t.Errorf("expected propagated error, got: %v", err)
	}
}

func TestEmailSender_Send_RendersMessage(t *testing.T) {
	s := NewEmailSender("smtp.example.com", 587, "user", "pass", "from@example.com")

	var captured *emailMessage
	s.sendMail = func(ctx context.Context, msg *emailMessage) error {
		captured = msg
		return nil
	}

	event := NotificationEvent{
		Title:    "Approval Required",
		Message:  "Player refund waiting",
		Priority: "high",
		Data:     map[string]interface{}{"game_id": "demo"},
	}
	if err := s.Send(context.Background(), "ops@example.com", event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.From != "from@example.com" {
		t.Errorf("From = %q", captured.From)
	}
	if captured.To != "ops@example.com" {
		t.Errorf("To = %q", captured.To)
	}
	if captured.Subject != "Approval Required" {
		t.Errorf("Subject = %q", captured.Subject)
	}
	if !strings.Contains(captured.Body, "Player refund waiting") {
		t.Errorf("Body missing message: %q", captured.Body)
	}
	if !strings.Contains(captured.Body, "game_id") || !strings.Contains(captured.Body, "demo") {
		t.Errorf("Body missing data table: %q", captured.Body)
	}
}

func TestBuildEmailBody_EscapesHTML(t *testing.T) {
	body := buildEmailBody(NotificationEvent{
		Title:   "<script>alert(1)</script>",
		Message: "a < b & c > d",
		Data:    map[string]interface{}{"k<v": "v\"x"},
	})

	if strings.Contains(body, "<script>") {
		t.Errorf("title not escaped: %s", body)
	}
	if strings.Contains(body, "a < b") || strings.Contains(body, "c > d") {
		t.Errorf("message not escaped: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("expected escaped title: %s", body)
	}
}

func TestBuildEmailMessage_HasRFCHeaders(t *testing.T) {
	raw := buildEmailMessage(&emailMessage{
		From:    "from@example.com",
		To:      "to@example.com",
		Subject: "你好",
		Body:    "<html></html>",
	})

	lower := strings.ToLower(string(raw))
	for _, want := range []string{
		"from: from@example.com",
		"to: to@example.com",
		"mime-version: 1.0",
		"content-type: text/html; charset=utf-8",
		"subject: =?utf-8?q?",
	} {
		if !strings.Contains(lower, want) {
			t.Errorf("missing header %q in:\n%s", want, raw)
		}
	}
	if !strings.HasSuffix(string(raw), "<html></html>") {
		t.Errorf("body not appended: %s", raw)
	}
}

func TestNewDingTalkSender(t *testing.T) {
	s := NewDingTalkSender("https://oapi.dingtalk.com/robot/send", "secret")
	if s == nil {
		t.Fatal("expected non-nil sender")
	}
	if s.Channel() != ChannelDingTalk {
		t.Errorf("expected channel %q, got %q", ChannelDingTalk, s.Channel())
	}
}

func TestDingTalkSender_Send(t *testing.T) {
	s := NewDingTalkSender("https://oapi.dingtalk.com/robot/send", "secret")
	err := s.Send(context.Background(), "user1", NotificationEvent{Title: "Test", Message: "Body"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewWebhookSender(t *testing.T) {
	s := NewWebhookSender("https://example.com/hook", "secret", "audience")
	if s == nil {
		t.Fatal("expected non-nil sender")
	}
	if s.Channel() != ChannelWebhook {
		t.Errorf("expected channel %q, got %q", ChannelWebhook, s.Channel())
	}
}

func TestWebhookSender_Send(t *testing.T) {
	// 空配置 no-op（未配置即安全跳过）。
	s := NewWebhookSender("", "secret", "audience")
	if err := s.Send(context.Background(), "user1", NotificationEvent{Title: "Test"}); err != nil {
		t.Errorf("unexpected error for unconfigured sender: %v", err)
	}
}

func TestNewMultiChannelNotifier(t *testing.T) {
	store := &mockNotificationStore{}
	n := NewMultiChannelNotifier(store)
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	if n.senders == nil || n.configs == nil {
		t.Error("expected initialized maps")
	}
}

func TestMultiChannelNotifier_RegisterSender(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)

	n.mu.RLock()
	_, ok := n.senders[ChannelEmail]
	n.mu.RUnlock()
	if !ok {
		t.Error("expected sender to be registered")
	}
}

func TestMultiChannelNotifier_SetConfig(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	config := &NotificationConfig{Channel: ChannelEmail, Enabled: true}
	n.SetConfig(ChannelEmail, config)

	n.mu.RLock()
	got, ok := n.configs[ChannelEmail]
	n.mu.RUnlock()
	if !ok || got != config {
		t.Error("expected config to be set")
	}
}

func TestMultiChannelNotifier_Notify_NoChannels(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	err := n.Notify(context.Background(), []string{"user1"}, NotificationEvent{})
	if err != nil {
		t.Errorf("expected no error when no channels configured, got %v", err)
	}
}

func TestMultiChannelNotifier_Notify_EnabledChannel(t *testing.T) {
	store := &mockNotificationStore{}
	n := NewMultiChannelNotifier(store)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: true})

	err := n.Notify(context.Background(), []string{"user1"}, NotificationEvent{Title: "Test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Errorf("expected 1 sent event, got %d", len(sender.sent))
	}
	if len(store.records) != 1 {
		t.Errorf("expected 1 recorded notification, got %d", len(store.records))
	}
}

func TestMultiChannelNotifier_Notify_DisabledChannel(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: false})

	err := n.Notify(context.Background(), []string{"user1"}, NotificationEvent{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Errorf("expected 0 sent events, got %d", len(sender.sent))
	}
}

func TestMultiChannelNotifier_NotifyWithChannels_SenderError(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	sender := &mockNotificationSender{channel: ChannelEmail, err: errors.New("send failed")}
	n.RegisterSender(sender)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: true})

	err := n.NotifyWithChannels(context.Background(), []string{"user1"}, NotificationEvent{}, []NotificationChannel{ChannelEmail})
	if err == nil {
		t.Error("expected error when sender fails")
	}
}

func TestMultiChannelNotifier_NotifyWithChannels_MissingConfig(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)
	// No config set for ChannelEmail

	err := n.NotifyWithChannels(context.Background(), []string{"user1"}, NotificationEvent{}, []NotificationChannel{ChannelEmail})
	if err != nil {
		t.Errorf("expected no error for missing config, got %v", err)
	}
	if len(sender.sent) != 0 {
		t.Error("expected no events sent for missing config")
	}
}

func TestMultiChannelNotifier_NotifyWithChannels_MissingSender(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: true})
	// No sender registered

	err := n.NotifyWithChannels(context.Background(), []string{"user1"}, NotificationEvent{}, []NotificationChannel{ChannelEmail})
	if err != nil {
		t.Errorf("expected no error for missing sender, got %v", err)
	}
}

func TestMultiChannelNotifier_Notify_RateLimit(t *testing.T) {
	store := &mockNotificationStore{count: 10} // Already at limit
	n := NewMultiChannelNotifier(store)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: true, RateLimit: 5})

	err := n.Notify(context.Background(), []string{"user1"}, NotificationEvent{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Should not send because count (10) >= rateLimit (5)
	if len(sender.sent) != 0 {
		t.Errorf("expected 0 sent events (rate limited), got %d", len(sender.sent))
	}
}

func TestMultiChannelNotifier_Notify_MultipleRecipients(t *testing.T) {
	store := &mockNotificationStore{}
	n := NewMultiChannelNotifier(store)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: true})

	err := n.Notify(context.Background(), []string{"user1", "user2", "user3"}, NotificationEvent{Title: "Test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(sender.sent) != 3 {
		t.Errorf("expected 3 sent events, got %d", len(sender.sent))
	}
	if len(store.records) != 3 {
		t.Errorf("expected 3 recorded notifications, got %d", len(store.records))
	}
}

func TestMultiChannelNotifier_Notify_NilStore(t *testing.T) {
	n := NewMultiChannelNotifier(nil)
	sender := &mockNotificationSender{channel: ChannelEmail}
	n.RegisterSender(sender)
	n.SetConfig(ChannelEmail, &NotificationConfig{Channel: ChannelEmail, Enabled: true})

	err := n.Notify(context.Background(), []string{"user1"}, NotificationEvent{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Should still send even without store
	if len(sender.sent) != 1 {
		t.Errorf("expected 1 sent event, got %d", len(sender.sent))
	}
}

func TestNewInAppNotifier(t *testing.T) {
	store := &mockNotificationStore{}
	hub := NewWebSocketHub()
	n := NewInAppNotifier(store, hub)
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	if n.store != store {
		t.Error("expected store to be set")
	}
	if n.hub != hub {
		t.Error("expected hub to be set")
	}
}

func TestInAppNotifier_Channel(t *testing.T) {
	n := NewInAppNotifier(nil, nil)
	if n.Channel() != ChannelInApp {
		t.Errorf("expected channel %q, got %q", ChannelInApp, n.Channel())
	}
}

func TestInAppNotifier_Send(t *testing.T) {
	store := &mockNotificationStore{}
	hub := NewWebSocketHub()
	go hub.Run()

	n := NewInAppNotifier(store, hub)
	err := n.Send(context.Background(), "user1", NotificationEvent{Title: "Test", Message: "Body"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(store.records) != 1 {
		t.Errorf("expected 1 recorded notification, got %d", len(store.records))
	}
}

func TestNewWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.clients == nil || hub.broadcast == nil || hub.register == nil || hub.unregister == nil {
		t.Error("expected initialized channels and maps")
	}
}

func TestWebSocketHub_RegisterUnregister(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	client := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 10)}
	hub.Register(client)
	time.Sleep(10 * time.Millisecond) // Let Run process

	hub.mu.RLock()
	clients := hub.clients["user1"]
	hub.mu.RUnlock()
	if len(clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(clients))
	}

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	clients = hub.clients["user1"]
	hub.mu.RUnlock()
	if len(clients) != 0 {
		t.Errorf("expected 0 clients after unregister, got %d", len(clients))
	}
}

func TestWebSocketHub_SendToUser(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	client := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 10)}
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.SendToUser("user1", map[string]string{"msg": "hello"})

	select {
	case data := <-client.Send:
		if len(data) == 0 {
			t.Error("expected non-empty data")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestWebSocketHub_BroadcastToMultipleClients(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	c1 := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 10)}
	c2 := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 10)}
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	hub.SendToUser("user1", "test")

	for _, c := range []*WebSocketClient{c1, c2} {
		select {
		case <-c.Send:
			// OK
		case <-time.After(time.Second):
			t.Error("timeout waiting for message")
		}
	}
}

func TestNotificationChannel_Constants(t *testing.T) {
	tests := []struct {
		ch   NotificationChannel
		want string
	}{
		{ChannelEmail, "email"},
		{ChannelSMS, "sms"},
		{ChannelWebhook, "webhook"},
		{ChannelWebSocket, "websocket"},
		{ChannelDingTalk, "dingtalk"},
		{ChannelWeChat, "wechat"},
		{ChannelSlack, "slack"},
		{ChannelInApp, "in_app"},
	}
	for _, tt := range tests {
		if string(tt.ch) != tt.want {
			t.Errorf("Channel %v = %q, want %q", tt.ch, string(tt.ch), tt.want)
		}
	}
}
