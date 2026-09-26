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
// otpauth:// URI —— 这是「扫码即绑」能不能成立的关键
// ------------------------------------------------------------------

// MFASetup 必须返回可被验证器 App 解析的 otpauth URI。
//
// 修复前是手写串 `otpauth://totp/Croupier:%s?secret=%s&issuer=Croupier`：
// 账号名完全没转义，缺 algorithm/digits/period。含 '@'、'/' 或非 ASCII 的
// 用户名会让 Google/Microsoft Authenticator 解析失败或条目被丢弃。
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
	// issuer 必须同时出现在 label 前缀与 query 里，两者不一致时
	// Google Authenticator 会静默丢弃该条目
	assert.Equal(t, otp.DefaultIssuer, q.Get("issuer"))
	// 显式声明算法/位数/步长，不依赖接收方默认值
	assert.Equal(t, otp.AlgorithmSHA1, q.Get("algorithm"))
	assert.Equal(t, "6", q.Get("digits"))
	assert.Equal(t, "30", q.Get("period"))

	label, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/"))
	require.NoError(t, err)
	assert.Equal(t, otp.DefaultIssuer+":scanme", label)
	assert.Equal(t, otp.DefaultIssuer, setup.Issuer)
}

// 含特殊字符的账号名必须被转义，否则 URI 里的 query 会被吃掉。
func TestMFASetup_EscapesSpecialUsername(t *testing.T) {
	const username = "user@example.com"
	svc, ctx := newMfaService(t, username)
	setup, err := svc.MFASetup(ctx, username)
	require.NoError(t, err)

	u, err := url.Parse(setup.OtpauthURL)
	require.NoError(t, err)
	// 未转义时 '@' 之后会被当作 host 信息，secret 解析不到
	assert.Equal(t, setup.Secret, u.Query().Get("secret"),
		"账号名未转义导致 secret 丢失: %s", setup.OtpauthURL)
}

// ------------------------------------------------------------------
// 备用恢复码
// ------------------------------------------------------------------

func TestMFAConfirm_IssuesRecoveryCodes(t *testing.T) {
	svc, ctx := newMfaService(t, "recover")
	admin := mustFindAdmin(t, "recover", svc)

	setup, err := svc.MFASetup(ctx, "recover")
	require.NoError(t, err)

	resp, err := svc.MFAConfirm(ctx, "recover", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	require.Len(t, resp.RecoveryCodes, otp.RecoveryCodeCount)

	// 每个码都必须能通过格式校验
	for _, c := range resp.RecoveryCodes {
		assert.True(t, otp.IsWellFormedRecoveryCode(c), "恢复码 %q 格式非法", c)
	}

	// 库里只存哈希，绝不存明文；且每个哈希恰好对应一个明文码
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

// 恢复码一次性：消费后不可再用。
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

	// 剩余数量相应减少
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

	// 未签发的合法格式码
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, "ZZZZZZZZZZ")
	require.NoError(t, err)
	assert.False(t, ok)

	// 格式非法（位数不对 / 含字母表外字符）
	for _, bad := range []string{"", "SHORT", "TOOLONGCODE12", "ABCD2345E0", "!!!!!!!!!!"} {
		ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, bad)
		require.NoError(t, err)
		assert.False(t, ok, "格式非法的码 %q 不应被接受", bad)
	}
}

// 用户按 4 位一组抄写恢复码时带空格，登录应照样认。
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

	// 关闭后旧恢复码不得再能消费
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, resp.RecoveryCodes[0])
	require.NoError(t, err)
	assert.False(t, ok, "关闭两步验证后旧恢复码必须失效")

	again, err := svc.MFASetup(ctx, "clearme")
	require.NoError(t, err)
	require.False(t, again.AlreadyDone)
}

// 重新绑定必须换掉整批恢复码，否则用户以为作废的旧码仍然有效。
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

	// 旧码全部失效
	for _, old := range first.RecoveryCodes {
		ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, old)
		require.NoError(t, err)
		assert.False(t, ok, "重新绑定后旧恢复码 %q 必须失效", old)
	}
	// 新码可用
	ok, err := svc.ConsumeRecoveryCode(ctx, admin.ID, second.RecoveryCodes[0])
	require.NoError(t, err)
	assert.True(t, ok)
}

// ------------------------------------------------------------------
// 状态
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

// 外部身份源账号：MFA 归 IdP 管，平台侧不签发恢复码。
func TestMFAStatus_ExternalAccountHasNoRecoveryCodes(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionserviceForTest(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()
	// 外部 JIT 影子账号：password_hash 为空
	require.NoError(t, db.Create(&model.Admin{Username: "oidcuser", Status: 1}).Error)

	st := svc.MFAStatus(ctx, "oidcuser")
	assert.False(t, st.Local)
	assert.False(t, st.Enabled)
	assert.Equal(t, 0, st.RecoveryCodesRemaining)

	_, err := svc.MFASetup(ctx, "oidcuser")
	assert.Error(t, err, "外部身份源账号不应允许平台侧绑定 MFA")
}

// ------------------------------------------------------------------
// 登录：TOTP 优先，恢复码兜底
// ------------------------------------------------------------------

func TestLogin_AcceptsRecoveryCodeWhenTOTPMissing(t *testing.T) {
	svc, ctx := newMfaService(t, "loginrecover")
	setup, err := svc.MFASetup(ctx, "loginrecover")
	require.NoError(t, err)
	resp, err := svc.MFAConfirm(ctx, "loginrecover", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	// 不带 totpCode → 要求 MFA
	_, err = svc.Login(ctx, &LoginRequest{Username: "loginrecover", Password: "CorrectPass123"})
	require.ErrorIs(t, err, ErrMFARequired)

	// 带恢复码 → 登录成功
	tok, err := svc.Login(ctx, &LoginRequest{
		Username: "loginrecover",
		Password: "CorrectPass123",
		TOTPCode: resp.RecoveryCodes[0],
	})
	require.NoError(t, err, "恢复码应能替代动态码登录")
	require.NotEmpty(t, tok.Token)

	// 同一码不能再次登录
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
		TOTPCode: "ZZZZZZZZZZ", // 格式合法但未签发
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrMFARequired, "带码但码错时应直接报验证码错误")
}

// 正常动态码路径不受恢复码改动影响。
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

// permissionserviceForTest 构造 MFA 用例所需的权限服务。
func permissionserviceForTest(db *gorm.DB) *permissionservice.PermissionService {
	return permissionservice.NewPermissionService(db)
}

// mustFindAdmin 取出测试账号的 admins 行。
func mustFindAdmin(t *testing.T, username string, svc *Service) *model.Admin {
	t.Helper()
	a, err := svc.adminModel.FindByUsername(context.Background(), username)
	require.NoError(t, err)
	return a
}

// recoveryDBOf 供断言「库里只存哈希」使用。
func recoveryDBOf(svc *Service) *gorm.DB { return svc.recoveryDB }
