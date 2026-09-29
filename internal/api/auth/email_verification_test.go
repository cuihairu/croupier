package auth

// 注册邮箱验证令牌链路测试（OPEN-ISSUES #51c 第二批）：required 必填、
// 注册签发（哈希落库 + 明文经缝隙外发）、登录 403 拦截与验证放行、
// VerifyEmailToken 一次性与重放拒绝、ResendVerification 防枚举静默 +
// 频控 + 旧令牌作废。发信经 sendVerificationEmailFn 缝隙捕获（同包测试
// 串行执行，替换/恢复安全）。

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

// hookVerificationSender 替换发信缝隙并捕获入参；返回恢复函数。
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

// verificationFixture 播种 required=true 并构造带验证模型的注册服务。
// admins 与 email_verifications 必须同一个 :memory: 库（每次 setupTestDB
// 都是独立实例——分开建库会让 Consume 的 admins 更新落空）。
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

	// required 开 + 邮箱缺 → 400（errorx.NewBadRequest）
	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "noemail1", Password: "Str0ngPass!x",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必须填写邮箱")

	// required 开 + 邮箱填 → 成功且 email_verified=false
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

	// 库里只存哈希：按明文 token 能查到行；行内无明文
	row := findTokenRow(t, m, cap.token)
	require.NotNil(t, row, "SHA-256(token) 应落库")
	assert.Equal(t, admin.ID, row.AdminID)
	assert.Equal(t, "register", row.Purpose)
	assert.True(t, row.ExpiresAt.After(time.Now().Add(23*time.Hour)), "有效期约 24h")
	assert.NotContains(t, row.TokenHash, cap.token)

	// 发信失败不影响注册成功（仅日志）
	sent2, restore2 := hookVerificationSender(t, errors.New("smtp down"))
	defer restore2()
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "mailuser2", Password: "Str0ngPass!x", Email: "mailuser2@example.com",
	})
	require.NoError(t, err)
	assert.Empty(t, *sent2)
}

func TestRegister_NoVerificationWhenDisabled(t *testing.T) {
	// 不播种 required：开关默认关
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

	// 未验证登录 → ErrEmailNotVerified（403 + email_not_verified）
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "blockme1", Password: "Str0ngPass!x"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEmailNotVerified)
	var ce interface{ GetStableCode() string }
	if errors.As(err, &ce) {
		assert.Equal(t, "email_not_verified", ce.GetStableCode())
	}

	// 用注册时缝隙捕获的明文 token 验证 → 登录放行
	token := (*sent)[0].token
	require.NoError(t, svc.VerifyEmailToken(context.Background(), token))
	_, err = svc.Login(context.Background(), &LoginRequest{Username: "blockme1", Password: "Str0ngPass!x"})
	require.NoError(t, err, "验证后应放行")

	// 存量无邮箱账号不受拦（无可验证之物）
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

	// 空 token → 400
	require.Error(t, svc.VerifyEmailToken(context.Background(), ""))
	// 伪 token → 400 同文案（不区分原因防探测）
	err = svc.VerifyEmailToken(context.Background(), "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效或已过期")

	// 有效 → 成功 + email_verified 落真
	require.NoError(t, svc.VerifyEmailToken(context.Background(), token))

	// 重放（已用）→ 拒绝
	err = svc.VerifyEmailToken(context.Background(), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效或已过期")

	// 过期令牌直接落库 → 拒绝
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

	// 防枚举：用户名/邮箱不匹配 → 静默成功（nil）且不发
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "wrong@example.com"))
	require.NoError(t, svc.ResendVerification(context.Background(), "ghost", "resendme1@example.com"))
	assert.Len(t, *sent, 1)

	// 频控内 → 静默不发
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "resendme1@example.com"))
	assert.Len(t, *sent, 1, "最小间隔内重发不应发信")

	// 参数缺失 → 400
	require.Error(t, svc.ResendVerification(context.Background(), "", ""))

	// 绕过频控（直接回拨最近令牌时间）后重发：新 token + 旧 token 作废
	require.NoError(t, db.Exec("UPDATE email_verifications SET created_at = ?", time.Now().Add(-2*time.Minute)).Error)
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "resendme1@example.com"))
	require.Len(t, *sent, 2)
	newToken := (*sent)[1].token
	assert.NotEqual(t, oldToken, newToken)
	assert.Nil(t, findTokenRow(t, m, oldToken), "重发后旧令牌应作废")
	require.NotNil(t, findTokenRow(t, m, newToken), "新令牌应可用")

	// 已验证账号重发 → 静默不发
	require.NoError(t, svc.VerifyEmailToken(context.Background(), newToken))
	require.NoError(t, svc.ResendVerification(context.Background(), "resendme1", "resendme1@example.com"))
	assert.Len(t, *sent, 2)
}

// GetStableCode 接口断言辅助：errorx.CodeError 的稳定码经 errors.As 读取。
func TestErrEmailNotVerified_Shape(t *testing.T) {
	assert.Equal(t, 403, ErrEmailNotVerified.Code)
	assert.Equal(t, "email_not_verified", ErrEmailNotVerified.StableCode)
}
