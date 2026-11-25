package auth

// Email verification token chain tests (OPEN-ISSUES #51c, second batch):
// required-mandatory enforcement, registration issue (hash landed in the
// database + plaintext routed out through the seam), login 403 blocking and
// release after verification, VerifyEmailToken single-use and replay
// rejection, ResendVerification anti-enumeration silence + rate limiting +
// old token invalidation. Mail-out is captured through the
// sendVerificationEmailFn seam (tests in this package run serially, so
// swap/restore is safe).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedEmail struct {
	to       string
	username string
	token    string
}

// hookVerificationSender replaces the mail seam and captures its arguments;
// returns a restore function.
func hookVerificationSender(t *testing.T, fail error) (*[]capturedEmail, func()) {
	t.Helper()
	sent := &[]capturedEmail{}
	orig := sendVerificationEmailFn
	sendVerificationEmailFn = func(_ context.Context, _ string, to, username, token string) error {
		if fail != nil {
			return fail
		}
		*sent = append(*sent, capturedEmail{to: to, username: username, token: token})
		return nil
	}
	return sent, func() { sendVerificationEmailFn = orig }
}

// verificationFixture seeds required=true and builds a registration service
// carrying the verification model. admins and email_verifications must share
// one :memory: database (every setupTestDB call is an independent instance —
// separate databases would make Consume's admins update miss).
func verificationFixture(t *testing.T) (*Service, *model.EmailVerificationModel, *gorm.DB) {
	t.Helper()
	seedEmailPolicy(t, map[string]string{settings.KeyAuthEmailVerificationRequired: `true`})
	db := setupTestDB(t)
	verifModel := model.NewEmailVerificationModel(db)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRecoveryDB(db).
		WithRoleModel(model.NewRoleModel(db)).
		WithVerificationModel(verifModel)
	ip, err := BuildIdentityProviders(config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})
	require.NoError(t, err)
	return ip.Attach(svc), verifModel, db
}

func findTokenRow(t *testing.T, m *model.EmailVerificationModel, token string) *model.EmailVerification {
	t.Helper()
	sum := sha256.Sum256([]byte(token))
	row, err := m.FindValidByTokenHash(context.Background(), hex.EncodeToString(sum[:]))
	require.NoError(t, err)
	return row
}

func TestRegister_VerificationRequiredEmailMandatory(t *testing.T) {
	svc, _, _ := verificationFixture(t)

	// required on + missing email → 400 (errorx.NewBadRequest)
	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "noemail1", Password: "Str0ngPass!x",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必须填写邮箱")

	// required on + email provided → success with email_verified=false
	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "withemail1", Password: "Str0ngPass!x", Email: "withemail1@example.com",
	})
	require.NoError(t, err)
	assert.False(t, admin.EmailVerified)
}

func TestRegister_IssuesVerificationEmail(t *testing.T) {
	svc, m, _ := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	admin, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "mailuser1", Password: "Str0ngPass!x", Email: "mailuser1@example.com",
	})
	require.NoError(t, err)

	require.Len(t, *sent, 1)
	cap := (*sent)[0]
	assert.Equal(t, "mailuser1@example.com", cap.to)
	assert.Equal(t, "mailuser1", cap.username)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), cap.token, "令牌应为 64 位 hex 明文")

	// only the hash is stored: the row is found via the plaintext token, and
	// the row itself holds no plaintext
	row := findTokenRow(t, m, cap.token)
	require.NotNil(t, row, "SHA-256(token) 应落库")
	assert.Equal(t, admin.ID, row.AdminID)
	assert.Equal(t, "register", row.Purpose)
	assert.True(t, row.ExpiresAt.After(time.Now().Add(23*time.Hour)), "有效期约 24h")
	assert.NotContains(t, row.TokenHash, cap.token)

	// a mail-out failure does not affect registration success (logged only)
	sent2, restore2 := hookVerificationSender(t, errors.New("smtp down"))
	defer restore2()
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "mailuser2", Password: "Str0ngPass!x", Email: "mailuser2@example.com",
	})
	require.NoError(t, err)
	assert.Empty(t, *sent2)
}

func TestRegister_NoVerificationWhenDisabled(t *testing.T) {
	// do not seed required: the switch defaults to off
	seedEmailPolicy(t, nil)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()
	svc := registerFixture(t, config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "quietuser1", Password: "Str0ngPass!x", Email: "quietuser1@example.com",
	})
	require.NoError(t, err)
	assert.Empty(t, *sent, "开关关闭时不应发验证邮件")
}

func TestLogin_BlockedUntilVerified(t *testing.T) {
	svc, _, _ := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "blockme1", Password: "Str0ngPass!x", Email: "blockme1@example.com",
	})
	require.NoError(t, err)
	require.Len(t, *sent, 1)

	// login while unverified → ErrEmailNotVerified (403 + email_not_verified)
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "blockme1", Password: "Str0ngPass!x"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEmailNotVerified)
	var ce interface{ GetStableCode() string }
	if errors.As(err, &ce) {
		assert.Equal(t, "email_not_verified", ce.GetStableCode())
	}

	// verify with the plaintext token captured through the seam at register
	// time → login is released
	token := (*sent)[0].token
	require.NoError(t, svc.VerifyEmailToken(context.Background(), token))
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "blockme1", Password: "Str0ngPass!x"})
	require.NoError(t, err, "验证后应放行")

	// a pre-existing account without an email is not gated (nothing to verify)
	admin := &model.Admin{Username: "legacyuser1", Nickname: "Legacy", Email: "", Status: 1}
	require.NoError(t, svc.adminModel.Create(context.Background(), admin, "Str0ngPass!x"))
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "legacyuser1", Password: "Str0ngPass!x"})
	require.NoError(t, err)
}

func TestVerifyEmailToken_Matrices(t *testing.T) {
	svc, m, _ := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "verifyme1", Password: "Str0ngPass!x", Email: "verifyme1@example.com",
	})
	require.NoError(t, err)
	require.Len(t, *sent, 1)
	token := (*sent)[0].token

	// empty token → 400
	require.Error(t, svc.VerifyEmailToken(context.Background(), ""))
	// bogus token → 400 with the same wording (reason not distinguished, to
	// prevent probing)
	err = svc.VerifyEmailToken(context.Background(), "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效或已过期")

	// valid token → success, and email_verified lands as true
	require.NoError(t, svc.VerifyEmailToken(context.Background(), token))

	// replay (already used) → rejected
	err = svc.VerifyEmailToken(context.Background(), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效或已过期")

	// an already-expired token inserted directly → rejected
	sum := sha256.Sum256([]byte("expiredtoken"))
	require.NoError(t, m.CreateInvalidatingActive(context.Background(), &model.EmailVerification{
		AdminID:   999,
		TokenHash: hex.EncodeToString(sum[:]),
		Purpose:   "register",
		ExpiresAt: time.Now().Add(-time.Hour),
	}))
	err = svc.VerifyEmailToken(context.Background(), "expiredtoken")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效或已过期")
}

func TestResendVerification_Matrices(t *testing.T) {
	svc, m, db := verificationFixture(t)
	sent, restore := hookVerificationSender(t, nil)
	defer restore()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "resendme1", Password: "Str0ngPass!x", Email: "resendme1@example.com",
	})
	require.NoError(t, err)
	require.Len(t, *sent, 1)
	oldToken := (*sent)[0].token

	// anti-enumeration: username/email mismatch → silent success (nil) and no
	// mail sent
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "wrong@example.com"))
	require.NoError(t, svc.ResendVerification(context.Background(), "ghost", "resendme1@example.com"))
	assert.Len(t, *sent, 1)

	// within the rate limit → silently no mail
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "resendme1@example.com"))
	assert.Len(t, *sent, 1, "最小间隔内重发不应发信")

	// missing parameters → 400
	require.Error(t, svc.ResendVerification(context.Background(), "", ""))

	// bypass the rate limit (rewind the latest token time directly), then
	// resend: new token + old token invalidated
	require.NoError(t, db.Exec("UPDATE email_verifications SET created_at = ?", time.Now().Add(-2*time.Minute)).Error)
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "resendme1@example.com"))
	require.Len(t, *sent, 2)
	newToken := (*sent)[1].token
	assert.NotEqual(t, oldToken, newToken)
	assert.Nil(t, findTokenRow(t, m, oldToken), "重发后旧令牌应作废")
	require.NotNil(t, findTokenRow(t, m, newToken), "新令牌应可用")

	// resend for an already-verified account → silently no mail
	require.NoError(t, svc.VerifyEmailToken(context.Background(), newToken))
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "resendme1@example.com"))
	assert.Len(t, *sent, 2)
}

// GetStableCode interface assertion helper: the stable code of
// errorx.CodeError is read via errors.As.
func TestErrEmailNotVerified_Shape(t *testing.T) {
	assert.Equal(t, 403, ErrEmailNotVerified.Code)
	assert.Equal(t, "email_not_verified", ErrEmailNotVerified.StableCode)
}
