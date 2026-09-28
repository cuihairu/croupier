package approvals

// SMTP 传输细节测试（OPEN-ISSUES #55）：AUTH LOGIN 协议时序、加密模式矩阵
// （none 全链路 / starttls 强制未宣告报错 / ssl 对明文服务握手失败）。

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"net/smtp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginAuth_Protocol(t *testing.T) {
	a := &loginAuth{username: "u@example.com", password: "secret"}
	mech, initial, err := a.Start(nil)
	require.NoError(t, err)
	assert.Equal(t, "LOGIN", mech)
	assert.Nil(t, initial)

	resp, err := a.Next([]byte("Username:"), true)
	require.NoError(t, err)
	assert.Equal(t, "u@example.com", string(resp))

	resp, err = a.Next([]byte("Password:"), true)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(resp))

	// 大小写不敏感 + 空白容忍（真实服务器提示形态各异）
	resp, err = a.Next([]byte("  username:"), true)
	require.NoError(t, err)
	assert.Equal(t, "u@example.com", string(resp))

	// 未知提示拒绝（不盲发凭据）
	_, err = a.Next([]byte("Challenge:"), true)
	assert.Error(t, err)

	// more=false 终止
	resp, err = a.Next(nil, false)
	require.NoError(t, err)
	assert.Nil(t, resp)
}

// serveFakeSMTP 明文假 SMTP 服务器：完整走 EHLO/AUTH LOGIN/MAIL/RCPT/DATA/QUIT。
func serveFakeSMTP(t *testing.T, ln net.Listener, advertiseAuth bool) {
	t.Helper()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		br := bufio.NewReader(conn)
		bw := bufio.NewWriter(conn)
		writeLine := func(s string) {
			_, _ = bw.WriteString(s + "\r\n")
			_ = bw.Flush()
		}
		readLine := func() string {
			s, _ := br.ReadString('\n')
			return strings.TrimSpace(s)
		}
		writeLine("220 fake ESMTP")
		for {
			line := readLine()
			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"):
				if advertiseAuth {
					writeLine("250-fake")
					writeLine("250 AUTH PLAIN LOGIN")
				} else {
					writeLine("250 fake")
				}
			case strings.HasPrefix(upper, "HELO"):
				writeLine("250 fake")
			case strings.HasPrefix(upper, "AUTH"):
				writeLine("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
				user := readLine()
				writeLine("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
				pass := readLine()
				if user == base64.StdEncoding.EncodeToString([]byte("u@example.com")) &&
					pass == base64.StdEncoding.EncodeToString([]byte("secret")) {
					writeLine("235 authenticated")
				} else {
					writeLine("535 bad credentials")
					return
				}
			case strings.HasPrefix(upper, "MAIL"), strings.HasPrefix(upper, "RCPT"):
				writeLine("250 ok")
			case strings.HasPrefix(upper, "DATA"):
				writeLine("354 go")
				for {
					if readLine() == "." {
						break
					}
				}
				writeLine("250 accepted")
			case strings.HasPrefix(upper, "QUIT"):
				writeLine("221 bye")
				return
			default:
				writeLine("500 unknown")
			}
		}
	}()
}

func TestEmailSender_PlaintextWithAuthLogin(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	serveFakeSMTP(t, ln, true)

	host, portRaw, _ := net.SplitHostPort(ln.Addr().String())
	sender := NewEmailSender(host, atoi(t, portRaw), "u@example.com", "secret", "from@example.com").
		WithTransport("none", "login", false)

	err = sender.Send(context.Background(), "to@example.com", NotificationEvent{Title: "t", Message: "m"})
	require.NoError(t, err)
}

func TestEmailSender_StarttlsMandatoryFailsWhenNotAdvertised(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	serveFakeSMTP(t, ln, false)

	host, portRaw, _ := net.SplitHostPort(ln.Addr().String())
	sender := NewEmailSender(host, atoi(t, portRaw), "", "", "from@example.com").
		WithTransport("starttls", "plain", false)

	err = sender.Send(context.Background(), "to@example.com", NotificationEvent{Title: "t", Message: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STARTTLS")
}

func TestEmailSender_SslAgainstPlaintextServerFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	serveFakeSMTP(t, ln, false)

	host, portRaw, _ := net.SplitHostPort(ln.Addr().String())
	sender := NewEmailSender(host, atoi(t, portRaw), "", "", "from@example.com").
		WithTransport("ssl", "plain", false)

	err = sender.Send(context.Background(), "to@example.com", NotificationEvent{Title: "t", Message: "m"})
	require.Error(t, err, "TLS 握手对明文服务器必失败")
}

func TestEmailSender_WithTransport(t *testing.T) {
	e := NewEmailSender("h", 25, "u", "p", "f")
	assert.Equal(t, "", e.smtpEncryption)
	e.WithTransport("ssl", "login", true)
	assert.Equal(t, "ssl", e.smtpEncryption)
	assert.Equal(t, "login", e.smtpAuthType)
	assert.True(t, e.insecureSkipVerify)
	// smtp.Auth 断言：接口兼容
	var _ smtp.Auth = &loginAuth{}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		require.True(t, c >= '0' && c <= '9', s)
		n = n*10 + int(c-'0')
	}
	return n
}
