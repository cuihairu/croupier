package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/otp"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
)

func newMfaService(t *testing.T, username string) (*Service, context.Context) {
	t.Helper()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionserviceForTest(db), "test-secret").WithRecoveryDB(db)
	createTestAdminWithRole(t, db, username, "CorrectPass123", "admin")
	return svc, context.Background()
}

// ------------------------------------------------------------------
// otpauth:// URI — the linchpin of whether "scan to bind" holds up
// ------------------------------------------------------------------

// MFASetup must return an otpauth URI that a verifier app can parse.
//
// Before the fix it was a hand-written string
// `otpauth://totp/Croupier:%s?secret=%s&issuer=Croupier`: the account name was
// not escaped at all and algorithm/digits/period were missing. Usernames
// containing '@', '/', or non-ASCII characters made Google/Microsoft
// Authenticator fail to parse the URI or drop the entry.
func TestMFASetup_ProducesScannableOtpauthURI(t *testing.T) {
	svc, ctx := newMfaService(t, "scanme")
	setup, err := svc.MFASetup(ctx, "scanme")
	require.NoError(t, err)
	require.NotEmpty(t, setup.OtpauthURL)

	u, err := url.Parse(setup.OtpauthURL)
	require.NoError(t, err, "产出的 URI 必须能被解析: %s", setup.OtpauthURL)
	assert.Equal(t, "otpauth", u.Scheme)
	assert.Equal(t, "totp", u.Host)

	q := u.Query()
	assert.Equal(t, setup.Secret, q.Get("secret"))
	// issuer must appear both as the label prefix and in the query; when the
	// two disagree Google Authenticator silently drops the entry
	assert.Equal(t, otp.DefaultIssuer, q.Get("issuer"))
	// declare algorithm/digits/period explicitly rather than relying on the
	// receiver's defaults
	assert.Equal(t, otp.AlgorithmSHA1, q.Get("algorithm"))
	assert.Equal(t, "6", q.Get("digits"))
	assert.Equal(t, "30", q.Get("period"))

	label, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/"))
	require.NoError(t, err)
	assert.Equal(t, otp.DefaultIssuer+":scanme", label)
	assert.Equal(t, otp.DefaultIssuer, setup.Issuer)
}

// An account name with special characters must be escaped, otherwise the query
// in the URI gets eaten.
func TestMFASetup_EscapesSpecialUsername(t *testing.T) {
	const username = "user@example.com"
	svc, ctx := newMfaService(t, username)
	setup, err := svc.MFASetup(ctx, username)
	require.NoError(t, err)

	u, err := url.Parse(setup.OtpauthURL)
	require.NoError(t, err)
	// unescaped, everything after '@' is treated as host information and the
	// secret cannot be parsed
	assert.Equal(t, setup.Secret, u.Query().Get("secret"),
		"账号名未转义导致 secret 丢失: %s", setup.OtpauthURL)
}

// ------------------------------------------------------------------
// backup recovery codes
// ------------------------------------------------------------------

func TestMFAConfirm_IssuesRecoveryCodes(t *testing.T) {
	svc, ctx := newMfaService(t, "recover")
	admin := mustFindAdmin(t, "recover", svc)

	setup, err := svc.MFASetup(ctx, "recover")
	require.NoError(t, err)

	resp, err := svc.MFAConfirm(ctx, "recover", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	require.Len(t, resp.RecoveryCodes, otp.RecoveryCodeCount)

	// every code must pass the well-formedness check
	for _, c := range resp.RecoveryCodes {
		assert.True(t, otp.IsWellFormedRecoveryCode(c), "恢复码 %q 格式非法", c)
	}

	// only hashes are stored, never plaintext; and each hash matches exactly one
	// plaintext code
	rows, err := model.NewAdminOTPRecoveryCodeModel(recoveryDBOf(svc)).ListUsable(ctx, admin.ID)
	require.NoError(t, err)
	require.Len(t, rows, otp.RecoveryCodeCount)
	matched := make(map[string]bool, len(rows))
	for _, row := range rows {
		for _, plain := range resp.RecoveryCodes {
			assert.NotEqual(t, plain, row.CodeHash, "库里不得存明文恢复码")
			if otp.RecoveryCodeMatches(plain, row.CodeHash) {
				assert.False(t, matched[plain], "同一明文码匹配到多条记录: %s", plain)
				matched[plain] = true
			}
		}
	}
	for _, plain := range resp.RecoveryCodes {
		assert.True(t, matched[plain], "恢复码 %s 未能由库中任一哈希验证", plain)
	}
}

// Recovery codes are one-shot: once consumed, they cannot be used again.
func TestRecoveryCode_IsSingleUse(t *testing.T) {
	svc, ctx := newMfaService(t, "singleuse")
	admin := mustFindAdmin(t, "singleuse", svc)

	setup, err := svc.MFASetup(ctx, "singleuse")
	require.NoError(t, err)
	resp, err := svc.MFAConfirm(ctx, "singleuse", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	code := resp.RecoveryCodes[0]
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, code)
	require.NoError(t, err)
	assert.True(t, ok, "首次使用应成功")

	ok, err = svc.ConsumeRecoveryCode(ctx, admin.ID, code)
	require.NoError(t, err)
	assert.False(t, ok, "同一恢复码不得二次使用")

	// the remaining count drops accordingly
	remaining, err := svc.CountRecoveryCodes(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, otp.RecoveryCodeCount-1, remaining)
}

func TestConsumeRecoveryCode_RejectsUnknownAndMalformed(t *testing.T) {
	svc, ctx := newMfaService(t, "rejectme")
	admin := mustFindAdmin(t, "rejectme", svc)

	setup, err := svc.MFASetup(ctx, "rejectme")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "rejectme", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	// a well-formed code that was never issued
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, "ZZZZZZZZZZ")
	require.NoError(t, err)
	assert.False(t, ok)

	// malformed (wrong length / characters outside the alphabet)
	for _, bad := range []string{"", "SHORT", "TOOLONGCODE12", "ABCD2345E0", "!!!!!!!!!!"} {
		ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, bad)
		require.NoError(t, err)
		assert.False(t, ok, "格式非法的码 %q 不应被接受", bad)
	}
}

// Users copying a recovery code in groups of four insert spaces; login should
// still accept it.
func TestRecoveryCode_AcceptsSpacedInput(t *testing.T) {
	svc, ctx := newMfaService(t, "spaced")
	admin := mustFindAdmin(t, "spaced", svc)

	setup, err := svc.MFASetup(ctx, "spaced")
	require.NoError(t, err)
	resp, err := svc.MFAConfirm(ctx, "spaced", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	raw := resp.RecoveryCodes[0]
	spaced := raw[:4] + " " + raw[4:]
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, spaced)
	require.NoError(t, err)
	assert.True(t, ok, "带空格的恢复码应被接受")
}

func TestMFADisable_ClearsRecoveryCodes(t *testing.T) {
	svc, ctx := newMfaService(t, "clearme")
	admin := mustFindAdmin(t, "clearme", svc)

	setup, err := svc.MFASetup(ctx, "clearme")
	require.NoError(t, err)
	resp, err := svc.MFAConfirm(ctx, "clearme", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	require.NotEmpty(t, resp.RecoveryCodes)

	require.NoError(t, svc.MFADisable(ctx, "clearme", currentTOTP(t, setup.Secret), "CorrectPass123"))

	// after disabling, the old recovery codes must no longer be consumable
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, resp.RecoveryCodes[0])
	require.NoError(t, err)
	assert.False(t, ok, "关闭两步验证后旧恢复码必须失效")

	again, err := svc.MFASetup(ctx, "clearme")
	require.NoError(t, err)
	require.False(t, again.AlreadyDone)
}

// Re-binding must replace the whole batch of recovery codes, otherwise old
// codes the user believes are revoked stay valid.
func TestMFAReenroll_ReplacesRecoveryCodes(t *testing.T) {
	svc, ctx := newMfaService(t, "reenroll")
	admin := mustFindAdmin(t, "reenroll", svc)

	setup, err := svc.MFASetup(ctx, "reenroll")
	require.NoError(t, err)
	first, err := svc.MFAConfirm(ctx, "reenroll", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	require.NoError(t, svc.MFADisable(ctx, "reenroll", currentTOTP(t, setup.Secret), "CorrectPass123"))

	setup2, err := svc.MFASetup(ctx, "reenroll")
	require.NoError(t, err)
	second, err := svc.MFAConfirm(ctx, "reenroll", currentTOTP(t, setup2.Secret))
	require.NoError(t, err)

	// every old code is invalidated
	for _, old := range first.RecoveryCodes {
		ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, old)
		require.NoError(t, err)
		assert.False(t, ok, "重新绑定后旧恢复码 %q 必须失效", old)
	}
	// the new codes work
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, second.RecoveryCodes[0])
	require.NoError(t, err)
	assert.True(t, ok)
}

// ------------------------------------------------------------------
// status
// ------------------------------------------------------------------

func TestMFAStatus_ReportsRemainingRecoveryCodes(t *testing.T) {
	svc, ctx := newMfaService(t, "statusme")
	admin := mustFindAdmin(t, "statusme", svc)

	before := svc.MFAStatus(ctx, "statusme")
	assert.True(t, before.Local)
	assert.False(t, before.Enabled)
	assert.Equal(t, 0, before.RecoveryCodesRemaining)

	setup, err := svc.MFASetup(ctx, "statusme")
	require.NoError(t, err)
	resp, err := svc.MFAConfirm(ctx, "statusme", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	after := svc.MFAStatus(ctx, "statusme")
	assert.True(t, after.Enabled)
	assert.Equal(t, otp.RecoveryCodeCount, after.RecoveryCodesRemaining)
	assert.Equal(t, otp.RecoveryCodeCount, after.RecoveryCodeTotal)

	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, resp.RecoveryCodes[0])
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, otp.RecoveryCodeCount-1, svc.MFAStatus(ctx, "statusme").RecoveryCodesRemaining)
}

// External identity provider account: MFA is the IdP's business, and the
// platform does not issue recovery codes.
func TestMFAStatus_ExternalAccountHasNoRecoveryCodes(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionserviceForTest(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()
	// an external JIT shadow account: password_hash is empty
	require.NoError(t, db.Create(&model.Admin{Username: "oidcuser", Status: 1}).Error)

	st := svc.MFAStatus(ctx, "oidcuser")
	assert.False(t, st.Local)
	assert.False(t, st.Enabled)
	assert.Equal(t, 0, st.RecoveryCodesRemaining)

	_, err := svc.MFASetup(ctx, "oidcuser")
	assert.Error(t, err, "外部身份源账号不应允许平台侧绑定 MFA")
}

// ------------------------------------------------------------------
// login: TOTP first, recovery code as the fallback
// ------------------------------------------------------------------

func TestLogin_AcceptsRecoveryCodeWhenTOTPMissing(t *testing.T) {
	svc, ctx := newMfaService(t, "loginrecover")
	setup, err := svc.MFASetup(ctx, "loginrecover")
	require.NoError(t, err)
	resp, err := svc.MFAConfirm(ctx, "loginrecover", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	// no totpCode → MFA is required
	_, err = svc.Login(ctx, &LoginRequest{Username: "loginrecover", Password: "CorrectPass123"})
	require.ErrorIs(t, err, ErrMFARequired)

	// with a recovery code → the login succeeds
	tok, err := svc.Login(ctx, &LoginRequest{
		Username: "loginrecover",
		Password: "CorrectPass123",
		TOTPCode: resp.RecoveryCodes[0],
	})
	require.NoError(t, err, "恢复码应能替代动态码登录")
	require.NotEmpty(t, tok.Token)

	// the same code cannot log in again
	_, err = svc.Login(ctx, &LoginRequest{
		Username: "loginrecover",
		Password: "CorrectPass123",
		TOTPCode: resp.RecoveryCodes[0],
	})
	require.Error(t, err, "恢复码一次性：复用应失败")
}

func TestLogin_InvalidRecoveryCodeRejected(t *testing.T) {
	svc, ctx := newMfaService(t, "badrecover")
	setup, err := svc.MFASetup(ctx, "badrecover")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "badrecover", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	_, err = svc.Login(ctx, &LoginRequest{
		Username: "badrecover",
		Password: "CorrectPass123",
		TOTPCode: "ZZZZZZZZZZ", // well formed but never issued
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrMFARequired, "带码但码错时应直接报验证码错误")
}

// The normal dynamic code path is unaffected by the recovery-code changes.
func TestLogin_StillAcceptsValidTOTP(t *testing.T) {
	svc, ctx := newMfaService(t, "totpok")
	setup, err := svc.MFASetup(ctx, "totpok")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "totpok", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	tok, err := svc.Login(ctx, &LoginRequest{
		Username: "totpok",
		Password: "CorrectPass123",
		TOTPCode: currentTOTP(t, setup.Secret),
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.Token)
}

// permissionserviceForTest builds the permission service needed by the MFA
// cases.
func permissionserviceForTest(db *gorm.DB) *permissionservice.PermissionService {
	return permissionservice.NewPermissionService(db)
}

// mustFindAdmin fetches the admins row of the test account.
func mustFindAdmin(t *testing.T, username string, svc *Service) *model.Admin {
	t.Helper()
	a, err := svc.adminModel.FindByUsername(context.Background(), username)
	require.NoError(t, err)
	return a
}

// recoveryDBOf supports the "only hashes are stored" assertions.
func recoveryDBOf(svc *Service) *gorm.DB { return svc.recoveryDB }
