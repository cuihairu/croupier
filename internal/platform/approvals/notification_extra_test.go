package approvals

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailSender_SendBranches(t *testing.T) {
	t.Run("no host is a no-op", func(t *testing.T) {
		sender := NewEmailSender("", 0, "", "", "")
		require.NoError(t, sender.Send(context.Background(), "user@example.com", NotificationEvent{Title: "hi"}))
	})

	t.Run("empty recipient is rejected", func(t *testing.T) {
		sender := NewEmailSender("smtp.example.com", 587, "", "", "from@example.com")
		err := sender.Send(context.Background(), "", NotificationEvent{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient is required")
	})

	t.Run("injected transport error", func(t *testing.T) {
		sender := NewEmailSender("smtp.example.com", 587, "", "", "from@example.com")
		sender.sendMail = func(ctx context.Context, msg *emailMessage) error { return errors.New("smtp offline") }
		err := sender.Send(context.Background(), "user@example.com", NotificationEvent{Title: "hi"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp offline")
	})

	t.Run("channel is email", func(t *testing.T) {
		assert.Equal(t, ChannelEmail, NewEmailSender("", 0, "", "", "").Channel())
	})
}

func TestEmailSender_DefaultSendMail(t *testing.T) {
	t.Run("cancelled context", func(t *testing.T) {
		sender := NewEmailSender("smtp.example.com", 587, "", "", "from@example.com")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sender.defaultSendMail(ctx, &emailMessage{})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("plain dial failure", func(t *testing.T) {
		// Port 1 on loopback is guaranteed to refuse connections.
		sender := NewEmailSender("127.0.0.1", 1, "", "", "from@example.com")
		err := sender.defaultSendMail(context.Background(), &emailMessage{To: "user@example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dial smtp")
	})

	t.Run("implicit TLS dial failure", func(t *testing.T) {
		sender := NewEmailSender("127.0.0.1", 465, "", "", "from@example.com")
		err := sender.defaultSendMail(context.Background(), &emailMessage{To: "user@example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dial smtp")
	})
}

func TestBuildEmailMessageAndBody(t *testing.T) {
	msg := &emailMessage{From: "from@example.com", To: "to@example.com", Subject: "Approval <needed>", Body: "<b>hi</b>"}
	raw := buildEmailMessage(msg)
	assert.Contains(t, string(raw), "From: from@example.com")
	assert.Contains(t, string(raw), "MIME-Version: 1.0")

	body := buildEmailBody(NotificationEvent{
		Title:   "Payment & Refund",
		Message: "<script>alert(1)</script>",
		Data:    map[string]interface{}{"amount": 42},
	})
	assert.Contains(t, body, "Payment &amp; Refund")
	assert.Contains(t, body, "&lt;script&gt;")
	assert.Contains(t, body, "<td>42</td>")

	assert.Equal(t, `a&quot;b&#39;c&lt;d&gt;&amp;`, htmlEscape(`a"b'c<d>&`))
}

// failingNotificationStore records errors for the in-app notifier.
type failingNotificationStore struct {
	err error
}

func (s *failingNotificationStore) RecordNotification(string, NotificationChannel, NotificationEvent) error {
	return s.err
}
func (s *failingNotificationStore) GetNotificationCount(string, NotificationChannel, time.Duration) (int, error) {
	return 0, nil
}
func (s *failingNotificationStore) GetNotifications(string, int) ([]*NotificationRecord, error) {
	return nil, nil
}
func (s *failingNotificationStore) MarkAsRead(string) error { return nil }

func TestInAppNotifier_SendBranches(t *testing.T) {
	t.Run("store failure surfaces", func(t *testing.T) {
		notifier := NewInAppNotifier(&failingNotificationStore{err: errors.New("disk full")}, nil)
		err := notifier.Send(context.Background(), "user-1", NotificationEvent{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disk full")
	})

	t.Run("nil store and hub succeed", func(t *testing.T) {
		notifier := NewInAppNotifier(nil, nil)
		require.NoError(t, notifier.Send(context.Background(), "user-1", NotificationEvent{}))
		assert.Equal(t, ChannelInApp, notifier.Channel())
	})

	t.Run("pushes via websocket hub", func(t *testing.T) {
		hub := NewWebSocketHub()
		go hub.Run()
		notifier := NewInAppNotifier(nil, hub)

		client := &WebSocketClient{UserID: "user-1", Send: make(chan []byte, 4)}
		hub.Register(client)

		require.NoError(t, notifier.Send(context.Background(), "user-1", NotificationEvent{Title: "hi"}))

		select {
		case raw := <-client.Send:
			assert.Contains(t, string(raw), "notification")
		case <-time.After(time.Second):
			t.Fatal("hub never delivered the notification")
		}
	})
}

func TestWebSocketHub_Run(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	clientA := &WebSocketClient{UserID: "user-a", Send: make(chan []byte, 1)}
	hub.Register(clientA)

	// Marshal failure is skipped without breaking the loop.
	hub.SendToUser("user-a", make(chan int))
	hub.SendToUser("user-a", map[string]string{"ok": "yes"})

	select {
	case raw := <-clientA.Send:
		assert.Contains(t, string(raw), `"ok":"yes"`)
	case <-time.After(time.Second):
		t.Fatal("hub never delivered the message")
	}

	hub.Unregister(clientA)

	// After unregistering, no further delivery happens.
	hub.SendToUser("user-a", map[string]string{"late": "true"})
	select {
	case <-clientA.Send:
		t.Fatal("unregistered client should not receive messages")
	default:
	}
}

// stubNotificationSender is a configurable NotificationSender.
type stubNotificationSender struct {
	channel   NotificationChannel
	err       error
	sentTo    []string
	allowSend func(recipient string) bool
}

func (s *stubNotificationSender) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	if s.allowSend != nil && !s.allowSend(recipient) {
		return nil
	}
	s.sentTo = append(s.sentTo, recipient)
	return s.err
}

func (s *stubNotificationSender) Channel() NotificationChannel { return s.channel }

// countingNotificationStore tracks recorded notifications for rate limiting.
type countingNotificationStore struct {
	count    int
	recorded int
}

func (s *countingNotificationStore) RecordNotification(string, NotificationChannel, NotificationEvent) error {
	s.recorded++
	return nil
}
func (s *countingNotificationStore) GetNotificationCount(string, NotificationChannel, time.Duration) (int, error) {
	return s.count, nil
}
func (s *countingNotificationStore) GetNotifications(string, int) ([]*NotificationRecord, error) {
	return nil, nil
}
func (s *countingNotificationStore) MarkAsRead(string) error { return nil }

func TestMultiChannelNotifier(t *testing.T) {
	t.Run("no enabled channels is a no-op", func(t *testing.T) {
		notifier := NewMultiChannelNotifier(nil)
		require.NoError(t, notifier.Notify(context.Background(), []string{"u"}, NotificationEvent{}))
	})

	t.Run("delivers through enabled sender", func(t *testing.T) {
		notifier := NewMultiChannelNotifier(nil)
		sender := &stubNotificationSender{channel: ChannelEmail}
		notifier.RegisterSender(sender)
		notifier.SetConfig(ChannelEmail, &NotificationConfig{Enabled: true})

		require.NoError(t, notifier.Notify(context.Background(), []string{"a@example.com", "b@example.com"}, NotificationEvent{}))
		assert.Equal(t, []string{"a@example.com", "b@example.com"}, sender.sentTo)
	})

	t.Run("disabled channel is skipped", func(t *testing.T) {
		notifier := NewMultiChannelNotifier(nil)
		sender := &stubNotificationSender{channel: ChannelWebhook}
		notifier.RegisterSender(sender)
		notifier.SetConfig(ChannelWebhook, &NotificationConfig{Enabled: false})

		require.NoError(t, notifier.NotifyWithChannels(context.Background(), []string{"u"}, NotificationEvent{}, []NotificationChannel{ChannelWebhook}))
		assert.Empty(t, sender.sentTo)
	})

	t.Run("missing sender is skipped", func(t *testing.T) {
		notifier := NewMultiChannelNotifier(nil)
		notifier.SetConfig(ChannelSMS, &NotificationConfig{Enabled: true})
		require.NoError(t, notifier.NotifyWithChannels(context.Background(), []string{"u"}, NotificationEvent{}, []NotificationChannel{ChannelSMS}))
	})

	t.Run("send errors are aggregated", func(t *testing.T) {
		notifier := NewMultiChannelNotifier(nil)
		notifier.RegisterSender(&stubNotificationSender{channel: ChannelEmail, err: errors.New("smtp down")})
		notifier.SetConfig(ChannelEmail, &NotificationConfig{Enabled: true})

		err := notifier.Notify(context.Background(), []string{"a@example.com"}, NotificationEvent{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel email")
		assert.Contains(t, err.Error(), "smtp down")
	})

	t.Run("rate limit skips recipient", func(t *testing.T) {
		store := &countingNotificationStore{count: 10}
		notifier := NewMultiChannelNotifier(store)
		sender := &stubNotificationSender{channel: ChannelInApp}
		notifier.RegisterSender(sender)
		notifier.SetConfig(ChannelInApp, &NotificationConfig{Enabled: true, RateLimit: 5})

		require.NoError(t, notifier.Notify(context.Background(), []string{"u"}, NotificationEvent{}))
		assert.Empty(t, sender.sentTo)
		assert.Zero(t, store.recorded)
	})

	t.Run("records successful deliveries", func(t *testing.T) {
		store := &countingNotificationStore{}
		notifier := NewMultiChannelNotifier(store)
		sender := &stubNotificationSender{channel: ChannelInApp}
		notifier.RegisterSender(sender)
		notifier.SetConfig(ChannelInApp, &NotificationConfig{Enabled: true})

		require.NoError(t, notifier.Notify(context.Background(), []string{"u"}, NotificationEvent{}))
		assert.Equal(t, 1, store.recorded)
	})
}

func TestTemplateRenderer_Substitution(t *testing.T) {
	renderer := NewTemplateRenderer()
	renderer.AddTemplate(&NotificationTemplate{
		ID:      "welcome",
		Type:    "info",
		Subject: "Hello {{name}}",
		Body:    "Welcome, {{name}}! Level {{level}}",
	})

	event, err := renderer.Render("welcome", map[string]string{"name": "Ada", "level": "5"})
	require.NoError(t, err)
	assert.Equal(t, "Hello Ada", event.Title)
	assert.Equal(t, "Welcome, Ada! Level 5", event.Message)

	_, err = renderer.Render("missing", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

func TestWebhookAndDingTalkSenders(t *testing.T) {
	var gotPath, gotBody string
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	webhook := NewWebhookSender(srv.URL+"/hook", "secret", "aud")
	require.NoError(t, webhook.Send(context.Background(), "u", NotificationEvent{Title: "t"}))
	assert.Equal(t, ChannelWebhook, webhook.Channel())
	assert.Equal(t, "/hook", gotPath)
	assert.Contains(t, gotBody, `"recipient":"u"`)
	assert.Equal(t, "sha256=", gotHeaders.Get("X-Croupier-Signature")[:7])

	dingtalk := NewDingTalkSender(srv.URL+"/ding?access_token=x", "secret")
	require.NoError(t, dingtalk.Send(context.Background(), "u", NotificationEvent{Title: "t", Message: "m"}))
	assert.Equal(t, ChannelDingTalk, dingtalk.Channel())
	assert.Equal(t, "/ding", gotPath)
	assert.Contains(t, gotBody, `"msgtype":"markdown"`)

	// 未配置（空 URL）：no-op。
	assert.NoError(t, NewWebhookSender("", "s", "").Send(context.Background(), "u", NotificationEvent{}))
	assert.NoError(t, NewDingTalkSender("", "s").Send(context.Background(), "u", NotificationEvent{}))

	// 目标 5xx：报错。
	srv5xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv5xx.Close()
	err := NewWebhookSender(srv5xx.URL, "", "").Send(context.Background(), "u", NotificationEvent{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDingTalkSender_SignedURL(t *testing.T) {
	// 无 secret：原样返回。
	d := NewDingTalkSender("https://oapi.dingtalk.com/robot/send?access_token=t", "")
	assert.Equal(t, "https://oapi.dingtalk.com/robot/send?access_token=t", d.signedURL())

	// 有 secret：追加 timestamp 与 sign。
	d = NewDingTalkSender("https://oapi.dingtalk.com/robot/send?access_token=t", "SECx")
	u := d.signedURL()
	assert.Contains(t, u, "&timestamp=")
	assert.Contains(t, u, "&sign=")

	// URL 无 query 时用 ? 分隔。
	d = NewDingTalkSender("https://oapi.dingtalk.com/robot/send", "SECx")
	assert.Contains(t, d.signedURL(), "?timestamp=")
}

func TestReplaceAll_MultiOccurrence(t *testing.T) {
	assert.Equal(t, "a-b-c", replaceAll("aXbXc", "X", "-"))
}

func TestValidateEmailAddress_Injection(t *testing.T) {
	// 合法地址。
	require.NoError(t, validateEmailAddress("user@example.com"))
	require.NoError(t, validateEmailAddress("a.b+c@sub.example.com"))

	// SMTP 命令注入尝试：CR/LF/空格/列表分隔符一律拒绝。
	for _, bad := range []string{
		"user@example.com\r\nRCPT TO:<attacker@evil.com>",
		"user@example.com\nBCC:attacker@evil.com",
		"user example@example.com",
		"a@b@example.com,c@d@example.com",
		"",
		"@example.com",
		"user@",
	} {
		require.Error(t, validateEmailAddress(bad), bad)
	}
}

func TestEmailSender_SendRejectsInjectedRecipient(t *testing.T) {
	sender := NewEmailSender("smtp.example.com", 587, "", "", "from@example.com")
	err := sender.Send(context.Background(), "user@example.com\r\nRCPT TO:<x@evil.com>", NotificationEvent{Title: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected")
}
