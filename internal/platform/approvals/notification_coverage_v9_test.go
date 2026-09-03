package approvals

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSMTPConfigV9 controls the behavior of the minimal fake SMTP server.
type fakeSMTPConfigV9 struct {
	immediateClose    bool // close the connection before the greeting
	advertiseSTARTTLS bool // advertise STARTTLS in EHLO, then refuse the upgrade
	advertiseAuth     bool // advertise AUTH PLAIN in EHLO
	authErr           bool // reject AUTH with 535
	mailErr           bool // reject MAIL FROM with 550
	rcptErr           bool // reject RCPT TO with 550
	dataErr           bool // reject DATA with 500
	dropDuringData    bool // close the connection right after the 354 reply
	afterDataErr      bool // accept the message body but reply 451
}

func startFakeSMTPV9(t *testing.T, cfg fakeSMTPConfigV9) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeSMTPConnV9(conn, cfg)
		}
	}()
	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	p := 0
	for _, c := range portStr {
		if c < '0' || c > '9' {
			t.Fatalf("unexpected port %q", portStr)
		}
		p = p*10 + int(c-'0')
	}
	return host, p
}

func handleFakeSMTPConnV9(conn net.Conn, cfg fakeSMTPConfigV9) {
	defer conn.Close()
	if cfg.immediateClose {
		return
	}
	if _, err := conn.Write([]byte("220 fake esmtp\r\n")); err != nil {
		return
	}
	reader := bufio.NewReader(conn)
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if inData {
			if strings.TrimSpace(line) == "." {
				inData = false
				if cfg.afterDataErr {
					_, _ = conn.Write([]byte("451 temp unavailable\r\n"))
				} else {
					_, _ = conn.Write([]byte("250 ok\r\n"))
				}
			}
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			switch {
			case cfg.advertiseSTARTTLS:
				_, _ = conn.Write([]byte("250-fake\r\n250 STARTTLS\r\n"))
			case cfg.advertiseAuth:
				_, _ = conn.Write([]byte("250-fake\r\n250 AUTH PLAIN\r\n"))
			default:
				_, _ = conn.Write([]byte("250 ok\r\n"))
			}
		case strings.HasPrefix(upper, "STARTTLS"):
			_, _ = conn.Write([]byte("454 tls unavailable\r\n"))
		case strings.HasPrefix(upper, "AUTH"):
			if cfg.authErr {
				_, _ = conn.Write([]byte("535 auth failed\r\n"))
			} else {
				_, _ = conn.Write([]byte("235 ok\r\n"))
			}
		case strings.HasPrefix(upper, "MAIL"):
			if cfg.mailErr {
				_, _ = conn.Write([]byte("550 mailbox unavailable\r\n"))
			} else {
				_, _ = conn.Write([]byte("250 ok\r\n"))
			}
		case strings.HasPrefix(upper, "RCPT"):
			if cfg.rcptErr {
				_, _ = conn.Write([]byte("550 rcpt refused\r\n"))
			} else {
				_, _ = conn.Write([]byte("250 ok\r\n"))
			}
		case strings.HasPrefix(upper, "DATA"):
			if cfg.dataErr {
				_, _ = conn.Write([]byte("500 data refused\r\n"))
				continue
			}
			_, _ = conn.Write([]byte("354 end with .\r\n"))
			inData = true
			if cfg.dropDuringData {
				return
			}
		case strings.HasPrefix(upper, "QUIT"):
			_, _ = conn.Write([]byte("221 bye\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("250 ok\r\n"))
		}
	}
}

func TestEmailSenderDefaultSendMailHappyPathV9(t *testing.T) {
	host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{})
	sender := NewEmailSender(host, port, "", "", "from@example.com")
	err := sender.defaultSendMail(context.Background(), &emailMessage{
		From:    "from@example.com",
		To:      "to@example.com",
		Subject: "hello",
		Body:    "<html><body>hi</body></html>",
	})
	require.NoError(t, err)
}

func TestEmailSenderDefaultSendMailAuthV9(t *testing.T) {
	t.Run("auth accepted", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{advertiseAuth: true})
		sender := NewEmailSender(host, port, "user", "pass", "from@example.com")
		err := sender.defaultSendMail(context.Background(), &emailMessage{To: "to@example.com"})
		require.NoError(t, err)
	})

	t.Run("auth rejected", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{advertiseAuth: true, authErr: true})
		sender := NewEmailSender(host, port, "user", "pass", "from@example.com")
		err := sender.defaultSendMail(context.Background(), &emailMessage{To: "to@example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp auth")
	})
}

func TestEmailSenderDefaultSendMailStartTLSRefusedV9(t *testing.T) {
	host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{advertiseSTARTTLS: true})
	sender := NewEmailSender(host, port, "", "", "from@example.com")
	err := sender.defaultSendMail(context.Background(), &emailMessage{To: "to@example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smtp STARTTLS")
}

func TestEmailSenderDefaultSendMailCommandErrorsV9(t *testing.T) {
	msg := &emailMessage{From: "from@example.com", To: "to@example.com", Subject: "s", Body: "b"}

	t.Run("mail from rejected", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{mailErr: true})
		err := NewEmailSender(host, port, "", "", "from@example.com").defaultSendMail(context.Background(), msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp MAIL FROM")
	})

	t.Run("rcpt rejected", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{rcptErr: true})
		err := NewEmailSender(host, port, "", "", "from@example.com").defaultSendMail(context.Background(), msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp RCPT TO")
	})

	t.Run("data rejected", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{dataErr: true})
		err := NewEmailSender(host, port, "", "", "from@example.com").defaultSendMail(context.Background(), msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp DATA")
	})

	t.Run("injected rcpt rejected before smtp", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{})
		bad := &emailMessage{From: "from@example.com", To: "to@example.com\r\nRCPT TO:<x@evil.com>"}
		err := NewEmailSender(host, port, "", "", "from@example.com").defaultSendMail(context.Background(), bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email recipient")
	})
}

func TestEmailSenderDefaultSendMailTransferErrorsV9(t *testing.T) {
	t.Run("connection dropped during data", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{dropDuringData: true})
		big := &emailMessage{
			From:    "from@example.com",
			To:      "to@example.com",
			Subject: "big",
			Body:    "<html><body>" + strings.Repeat("x", 256*1024) + "</body></html>",
		}
		err := NewEmailSender(host, port, "", "", "from@example.com").defaultSendMail(context.Background(), big)
		require.Error(t, err)
	})

	t.Run("final dot rejected", func(t *testing.T) {
		host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{afterDataErr: true})
		err := NewEmailSender(host, port, "", "", "from@example.com").defaultSendMail(context.Background(),
			&emailMessage{From: "from@example.com", To: "to@example.com", Subject: "s", Body: "b"})
		require.Error(t, err)
	})
}

func TestEmailSenderDefaultSendMailHandshakeFailureV9(t *testing.T) {
	host, port := startFakeSMTPV9(t, fakeSMTPConfigV9{immediateClose: true})
	sender := NewEmailSender(host, port, "", "", "from@example.com")
	err := sender.defaultSendMail(context.Background(), &emailMessage{To: "to@example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smtp handshake")
}

func TestEmailSenderSendFallsBackToDefaultSendMailV9(t *testing.T) {
	// Host is configured but sendMail is not injected: Send must fall back to
	// defaultSendMail, which immediately observes the cancelled context.
	sender := NewEmailSender("127.0.0.1", 587, "", "", "from@example.com")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sender.Send(ctx, "to@example.com", NotificationEvent{Title: "hi"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSanitizeEmailTextV9(t *testing.T) {
	assert.Equal(t, "abb", sanitizeEmailText("a\rb\nb"))
	assert.Equal(t, "ab", sanitizeEmailText("a\x00b"))
	assert.Equal(t, "a\tb", sanitizeEmailText("a\tb"))
	assert.Equal(t, "ab", sanitizeEmailText("a\x7fb"))
	assert.Equal(t, "trimmed", sanitizeEmailText("  trimmed \t"))
	assert.Equal(t, "héllo", sanitizeEmailText("héllo"))
}

func TestSanitizeNotificationEventForEmailV9(t *testing.T) {
	event := NotificationEvent{
		Title:   "line1\r\nline2",
		Message: "tab\tvalue\x01",
		Data: map[string]interface{}{
			"count": 42,
			"name":  "a\r\nb",
		},
	}
	got := sanitizeNotificationEventForEmail(event)
	assert.Equal(t, "line1line2", got.Title)
	assert.Equal(t, "tab\tvalue", got.Message)
	assert.Equal(t, "42", got.Data["count"])
	assert.Equal(t, "ab", got.Data["name"])
}

func TestValidateEmailAddressDisplayNameV9(t *testing.T) {
	_, err := validateEmailAddress("Display Name <user@example.com>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "display name not allowed")

	_, err = validateEmailAddress("<user@example.com>")
	require.Error(t, err)
}

func TestDefaultPostJSONErrorsV9(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		err := defaultPostJSON(context.Background(), "http://bad host/path", []byte("{}"))
		require.Error(t, err)
	})

	t.Run("cancelled context", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer srv.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := defaultPostJSON(ctx, srv.URL, []byte("{}"))
		require.Error(t, err)
	})
}

func TestWecomSenderRealHTTPV9(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewWecomSender(srv.URL + "/wecom")
	require.NoError(t, sender.Send(context.Background(), "", NotificationEvent{Title: "t", Message: "m"}))
	assert.Equal(t, "/wecom", gotPath)
}

func TestFeishuSenderUnconfiguredAndRealHTTPV9(t *testing.T) {
	t.Run("empty webhook url is a no-op", func(t *testing.T) {
		require.NoError(t, NewFeishuSender("", "secret").Send(context.Background(), "", NotificationEvent{Title: "x"}))
	})

	t.Run("real http post via default transport", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sender := NewFeishuSender(srv.URL+"/feishu", "sec")
		require.NoError(t, sender.Send(context.Background(), "", NotificationEvent{Title: "t", Message: "m"}))
		assert.Equal(t, "/feishu", gotPath)
	})
}
