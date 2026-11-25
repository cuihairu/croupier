// Email verification token chain for registration (OPEN-ISSUES #51c, second
// batch): the service layer behind registration mail-out, the login gate, and
// the public verify and resend endpoints. Tokens are stored only as SHA-256
// hashes (see model.EmailVerification); every register/resend reads L3 live —
// saving the setting takes effect immediately.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// ErrEmailNotVerified marks the login gate for a local account whose email is
// unverified. The CodeError carries 403 + StableCode=email_not_verified: it is
// emitted through the unified response.Error, and the frontend branches on the
// error code to surface a "resend verification email" entry; message is not read.
var ErrEmailNotVerified = &errorx.CodeError{
	Code:       http.StatusForbidden,
	Message:    "邮箱尚未验证，请查收验证邮件或重新发送后再登录",
	StableCode: "email_not_verified",
}

const (
	emailVerificationPurpose = "register"
	// emailVerificationTTL is the verification token lifetime (honest edge:
	// a constant, not a settings key; promote it to a settings key when
	// site-level configurability is needed).
	emailVerificationTTL = 24 * time.Hour
	// resendVerificationMinInterval is the resend rate limit: the minimum
	// interval between two mail-outs for the same account. It protects the
	// public endpoint from mail abuse (resend cadence that matters to admins
	// is far larger than this).
	resendVerificationMinInterval = time.Minute
)

// sendVerificationEmailFn is an injectable seam (same convention as
// sitesettings.sendTestEmailFn / ops.systemUpdateFetchFn): it builds an
// EmailSender on the fly from the current L3 SMTP config and sends for real.
// When siteURL is empty the body degrades to bare token text (deployments
// that cannot build a frontend link can still relay it manually).
var sendVerificationEmailFn = func(ctx context.Context, siteURL, to, username, token string) error {
	smtpCfg := settings.Current().NotifySMTP()
	sender := approvals.NewEmailSender(smtpCfg.Host, smtpCfg.Port, smtpCfg.User, smtpCfg.Password, smtpCfg.From).
		WithTransport(smtpCfg.Encryption, smtpCfg.AuthType, smtpCfg.InsecureSkipVerify)
	link := fmt.Sprintf("%s/user/verify-email?token=%s", strings.TrimRight(siteURL, "/"), token)
	if siteURL == "" {
		link = token
	}
	return sender.Send(ctx, to, approvals.NotificationEvent{
		Type:  "email_verification",
		Title: "Croupier 邮箱验证",
		Message: fmt.Sprintf("您好 %s，请在 24 小时内访问以下链接完成邮箱验证：%s（若非本人操作请忽略本邮件）。",
			username, link),
		Priority: "high",
	})
}

// emailVerificationRequired reads the L3 switch live (nil-safe against
// settings.Current(): environments whose layered settings were never
// initialized, such as tests, are treated as off).
func emailVerificationRequired() bool {
	cur := settings.Current()
	if cur == nil {
		return false
	}
	return cur.GetBool(settings.KeyAuthEmailVerificationRequired, false)
}

// siteServerURL reads the site's externally reachable address (used to build
// verification email links; empty = not configured).
func siteServerURL() string {
	cur := settings.Current()
	if cur == nil {
		return ""
	}
	return cur.SiteSnapshot().ServerURL
}

// newVerificationToken generates a 64-char hex token and its SHA-256 hash.
func newVerificationToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("生成验证令牌失败: %w", err)
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

// issueEmailVerification issues a fresh verification token for an account
// (invalidating the old ones) and sends the mail. A send failure is only
// logged, never rolled back — the token has already landed in the database and
// the user can use resend; registration must not fail wholesale because of SMTP
// jitter (honest edge: with SMTP unconfigured or broken the mail never arrives,
// so the resend endpoint is the fallback).
func (s *Service) issueEmailVerification(ctx context.Context, admin *model.Admin) error {
	if s.verificationModel == nil {
		return nil
	}
	token, hash, err := newVerificationToken()
	if err != nil {
		return err
	}
	if err := s.verificationModel.CreateInvalidatingActive(ctx, &model.EmailVerification{
		AdminID:   admin.ID,
		TokenHash: hash,
		Purpose:   emailVerificationPurpose,
		ExpiresAt: time.Now().Add(emailVerificationTTL),
	}); err != nil {
		return err
	}
	if err := sendVerificationEmailFn(ctx, siteServerURL(), admin.Email, admin.Username, token); err != nil {
		slog.Default().Warn("send verification email failed",
			"username", admin.Username, "error", err)
	}
	return nil
}

// maybeSendVerificationOnRegister sends the verification email after
// registration when applicable: skip when the switch is off or no email was
// given at registration; required with an empty email is already rejected at
// the Register entry point. Any failure in the mail path leaves the
// registration result untouched (logged only).
func (s *Service) maybeSendVerificationOnRegister(ctx context.Context, admin *model.Admin) {
	if !emailVerificationRequired() || admin.Email == "" {
		return
	}
	if err := s.issueEmailVerification(ctx, admin); err != nil {
		slog.Default().Warn("issue email verification after register failed",
			"username", admin.Username, "error", err)
	}
}

// emailVerificationBlocksLogin decides whether the account is gated by the email
// verification gate: only the local provider counts (email ownership for
// LDAP/OIDC is the IdP's job), the switch is on, and the account has an email
// that is not yet verified. Pre-existing accounts without an email are never
// gated — there is nothing to verify.
func emailVerificationBlocksLogin(admin *model.Admin) bool {
	return admin.Email != "" && !admin.EmailVerified
}

// VerifyEmailToken validates the token and lands email_verified. Invalid,
// expired, and already-used tokens uniformly return errors.New (the handler
// maps it to 400) — the reason is not distinguished, to prevent probing for
// token validity.
func (s *Service) VerifyEmailToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errorx.NewBadRequest("token 不能为空")
	}
	if s.verificationModel == nil {
		return errors.New("验证服务不可用")
	}
	sum := sha256.Sum256([]byte(token))
	row, err := s.verificationModel.FindValidByTokenHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	if row == nil {
		return errorx.NewBadRequest("验证链接无效或已过期，请重新发送验证邮件")
	}
	ok, err := s.verificationModel.Consume(ctx, row)
	if err != nil {
		return err
	}
	if !ok {
		return errorx.NewBadRequest("验证链接无效或已过期，请重新发送验证邮件")
	}
	return nil
}

// ResendVerification resends the verification email by username+email
// (anonymous endpoint). Anti-enumeration: unknown username, email mismatch,
// already verified, or within the rate limit all silently succeed (no account
// state is exposed); mail goes out only on a real match that is still unverified.
func (s *Service) ResendVerification(ctx context.Context, username, email string) error {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if username == "" || email == "" {
		return errorx.NewBadRequest("username 与 email 不能为空")
	}
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || admin == nil || !strings.EqualFold(strings.TrimSpace(admin.Email), email) {
		return nil
	}
	if admin.EmailVerified {
		return nil
	}
	if s.verificationModel != nil {
		last, err := s.verificationModel.LatestCreatedAtFor(ctx, admin.ID, emailVerificationPurpose)
		if err == nil && !last.IsZero() && time.Since(last) < resendVerificationMinInterval {
			return nil // within the rate limit: stay silent, no send and no error
		}
	}
	return s.issueEmailVerification(ctx, admin)
}
