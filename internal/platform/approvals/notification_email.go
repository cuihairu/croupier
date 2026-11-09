package approvals

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

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

	// 传输细节（OPEN-ISSUES #55）：encryption 取 ""|none|ssl|starttls，
	// 空串 = 自动（465 隐式 TLS，否则 STARTTLS-if-advertised）；
	// authType 取 plain|login（默认 plain）；insecureSkipVerify 供自签证书。
	smtpEncryption     string
	smtpAuthType       string
	insecureSkipVerify bool

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

// WithTransport 设置传输细节（加密方式/认证方式/跳过 TLS 证书校验），
// 返回自身便于链式构造（OPEN-ISSUES #55）。
func (e *EmailSender) WithTransport(encryption, authType string, insecureSkipVerify bool) *EmailSender {
	e.smtpEncryption = encryption
	e.smtpAuthType = authType
	e.insecureSkipVerify = insecureSkipVerify
	return e
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
	// 使用解析后的规范地址作为收件人：validate 返回的是
	// mail.ParseAddress 产出的新字符串，原始请求输入不再直接
	// 进入 SMTP 命令（注入防线 + 污点截断）。
	canonical, err := validateEmailAddress(recipient)
	if err != nil {
		return err
	}

	safeEvent := sanitizeNotificationEventForEmail(event)
	msg := &emailMessage{
		From:    e.fromAddress,
		To:      canonical,
		Subject: safeEvent.Title,
		Body:    buildEmailBody(safeEvent),
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
// message. Transport follows smtpEncryption (#55): ""/auto = port 465 implicit
// TLS, otherwise plain + STARTTLS when advertised; "ssl" = implicit TLS on any
// port; "starttls" = plain dial + mandatory STARTTLS (fails when the server
// does not advertise it); "none" = plaintext. smtpAuthType "login" forces the
// AUTH LOGIN mechanism (net/smtp ships only PLAIN/CRAM-MD5).
func (e *EmailSender) defaultSendMail(ctx context.Context, msg *emailMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	mode := e.smtpEncryption
	if mode == "" {
		if e.smtpPort == 465 {
			mode = "ssl"
		} else {
			mode = "starttls-auto"
		}
	}

	addr := net.JoinHostPort(e.smtpHost, strconv.Itoa(e.smtpPort))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tlsCfg := &tls.Config{ServerName: e.smtpHost, InsecureSkipVerify: e.insecureSkipVerify}

	var conn net.Conn
	var err error
	if mode == "ssl" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial smtp %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, e.smtpHost)
	if err != nil {
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer func() { _ = client.Quit() }()

	if mode == "starttls" || mode == "starttls-auto" {
		ok, _ := client.Extension("STARTTLS")
		if !ok && mode == "starttls" {
			return fmt.Errorf("smtp STARTTLS: 服务器未宣告 STARTTLS（强制加密模式）")
		}
		if ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("smtp STARTTLS: %w", err)
			}
		}
	}

	if e.smtpUser != "" {
		var auth smtp.Auth
		if e.smtpAuthType == "login" {
			auth = &loginAuth{username: e.smtpUser, password: e.smtpPassword}
		} else {
			auth = smtp.PlainAuth("", e.smtpUser, e.smtpPassword, e.smtpHost)
		}
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(e.fromAddress); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	// 邮件注入防线：RCPT 前校验收件人不含 CR/LF 等控制字符，
	// 防止恶意收件人字符串操纵 SMTP 命令序列。
	rcpt, err := validateEmailAddress(msg.To)
	if err != nil {
		return err
	}
	if err := client.Rcpt(rcpt); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	// 抑制理由：该查询的 taint 配置（EmailInjectionCustomizations.qll）只有 source/sink、
	// 不定义任何 sanitizer，因此收件人经 validateEmailAddress（mail.ParseAddress 拒绝 CR/LF，
	// 断 SMTP 头注入）、主题/正文经 sanitizeEmailText 与 htmlEscape 的处理都"看不见"。
	// 实际风险已在上游消除：To 由 ParseAddress 白名单解析，Subject/Body 为 RFC5322 编码后的
	// 静态渲染（buildEmailMessage 用 \r\n 定界且动态段已净化），攻击者无法注入头部或命令。
	// 这是 GitHub 扫描器的建模盲区而非代码缺陷（业界同规则普遍采用带理由抑制，如
	// infiniflow/ragflow 的 smtp.go），故按 CodeQL 指令抑制本行结果。
	// 注意：codeql 指令必须紧贴被抑制语句的上一行才生效，理由写在指令上方。
	// codeql[go/email-injection]
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
	safeSubject := sanitizeEmailText(msg.Subject)
	safeBody := sanitizeEmailText(msg.Body)
	headers := map[string]string{
		"From":         msg.From,
		"To":           msg.To,
		"Subject":      mime.QEncoding.Encode("utf-8", safeSubject),
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
	buf.WriteString(safeBody)
	return buf.Bytes()
}

func sanitizeNotificationEventForEmail(event NotificationEvent) NotificationEvent {
	event.Title = sanitizeEmailText(event.Title)
	event.Message = sanitizeEmailText(event.Message)
	if len(event.Data) > 0 {
		clean := make(map[string]interface{}, len(event.Data))
		for k, v := range event.Data {
			safeKey := sanitizeEmailText(k)
			switch vv := v.(type) {
			case string:
				clean[safeKey] = sanitizeEmailText(vv)
			default:
				clean[safeKey] = sanitizeEmailText(fmt.Sprint(v))
			}
		}
		event.Data = clean
	}
	return event
}

func sanitizeEmailText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\r' || r == '\n' {
			continue
		}
		if r < 0x20 && r != '\t' {
			continue
		}
		if r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
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
	// stdlib html.EscapeString 把双引号转成 &#34;，与历史输出 &quot; 等价；
	// 这里归一回 &quot; 保持邮件 HTML 输出不变，同时保留 stdlib 调用本身
	// （code scanning 识别 html.EscapeString 为 email-injection sanitizer）。
	return strings.ReplaceAll(html.EscapeString(s), "&#34;", "&quot;")
}

// parseEmailAddress 是 net/mail.ParseAddress 的包级接缝：生产恒为真实实现，
// 测试注入「解析成功但携带控制字符」的返回值以驱动下方纵深防御分支（当前
// net/mail 实现下该分支不可达，见分支处注释）。
var parseEmailAddress = mail.ParseAddress

// validateEmailAddress 阻止 SMTP 命令注入：返回 mail.ParseAddress 解析
// 出的规范地址（拒绝 Display Name 形态与控制字符），调用方必须使用该
// 返回值而非原始输入。
func validateEmailAddress(addr string) (string, error) {
	if addr == "" {
		return "", fmt.Errorf("email recipient is required")
	}
	// net/mail.ParseAddress 拒绝含 CR/LF 等控制字符的地址，阻断 SMTP
	// 命令注入；其解析输出是独立的规范字符串。
	parsed, err := parseEmailAddress(addr)
	if err != nil {
		return "", fmt.Errorf("invalid email recipient: %w", err)
	}
	// 只接受裸地址，不允许 "Display Name <addr>" 形态——
	// 显示名可能含注入载荷（如 Name <real@x>\r\nRCPT TO:<evil>）。
	if parsed.Address != addr {
		return "", fmt.Errorf("invalid email recipient: display name not allowed")
	}
	// 纵深防御冗余防线：在当前 net/mail 实现下 ParseAddress 拒绝 CR/LF
	// （上一分支返回 err），quoted-local-part 形态（"a b"@x.com 等）的输出
	// 会剥掉引号导致 parsed.Address != addr 被上方分支拦截，裸地址无法携带
	// 空格/tab/;/,——生产路径恒不可达，仅当 parseEmailAddress 接缝被注入
	// 「解析成功但保留控制字符」的返回值时可驱动。保留原因：安全防线不应
	// 依赖单一解析器行为，若未来 Go 的 mail 包输出形态变化（如保留引号），
	// 此处仍是最后兜底，删除会把安全性押在标准库实现细节上。
	if strings.ContainsAny(addr, "\r\n \t;,") {
		return "", fmt.Errorf("invalid email recipient: control characters rejected")
	}
	return parsed.Address, nil
}

// loginAuth 实现 AUTH LOGIN 机制（net/smtp 仅内置 PLAIN/CRAM-MD5；部分
// 服务商——如强制 AUTH LOGIN 的 Exchange/旧版邮服——不支持 PLAIN）。
// 协议时序：服务器依次下发 "Username:"/"Password:" 提示，客户端回明文。
type loginAuth struct {
	username string
	password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(strings.TrimSpace(string(fromServer)))
	switch {
	case strings.Contains(prompt, "username"):
		return []byte(a.username), nil
	case strings.Contains(prompt, "password"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("smtp AUTH LOGIN: unexpected server prompt %q", string(fromServer))
	}
}
