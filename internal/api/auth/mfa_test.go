package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// currentTOTP 用与 otp 包相同的算法生成当前窗口验证码（测试夹具）。
func currentTOTP(t *testing.T, secret string) string {
	t.Helper()
	dec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	require.NoError(t, err)
	counter := uint64(time.Now().Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, dec)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := int(sum[len(sum)-1] & 0x0F)
	bin := (int(sum[offset])&0x7f)<<24 | (int(sum[offset+1])&0xff)<<16 | (int(sum[offset+2])&0xff)<<8 | (int(sum[offset+3]) & 0xff)
	return pad6(bin % 1000000)
}

func pad6(v int) string {
	s := ""
	for i := 0; i < 6; i++ {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	return s
}

func TestMFA_SetupConfirmDisable_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
	ctx := context.Background()
	createTestAdminWithRole(t, db, "mfauser", "CorrectPass123", "admin")

	// setup：返回 secret，未启用
	setup, err := svc.MFASetup(ctx, "mfauser")
	require.NoError(t, err)
	require.NotEmpty(t, setup.Secret)
	assert.False(t, setup.AlreadyDone)
	assert.Contains(t, setup.OtpauthURL, "otpauth://totp/")

	// 错误 code 不能 confirm
	err = svc.MFAConfirm(ctx, "mfauser", "000000")
	require.Error(t, err)

	// 正确 code 确认启用
	err = svc.MFAConfirm(ctx, "mfauser", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	admin, err := adminModel.FindByUsername(ctx, "mfauser")
	require.NoError(t, err)
	assert.True(t, admin.OTPEnabled)

	// 再次 setup 返回 alreadyEnabled
	again, err := svc.MFASetup(ctx, "mfauser")
	require.NoError(t, err)
	assert.True(t, again.AlreadyDone)

	// disable：错误密码拒绝
	err = svc.MFADisable(ctx, "mfauser", currentTOTP(t, setup.Secret), "wrong-pass")
	require.Error(t, err)
	// 正确 code + 密码关闭
	err = svc.MFADisable(ctx, "mfauser", currentTOTP(t, setup.Secret), "CorrectPass123")
	require.NoError(t, err)
	admin, err = adminModel.FindByUsername(ctx, "mfauser")
	require.NoError(t, err)
	assert.False(t, admin.OTPEnabled)
	assert.Empty(t, admin.OTPSecret)
}

func TestLogin_MFA_RequiredAndVerified(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
	ctx := context.Background()
	createTestAdminWithRole(t, db, "guard", "CorrectPass123", "admin")

	// 启用 MFA
	setup, err := svc.MFASetup(ctx, "guard")
	require.NoError(t, err)
	require.NoError(t, svc.MFAConfirm(ctx, "guard", currentTOTP(t, setup.Secret)))

	// 无 totpCode：返回 ErrMFARequired
	_, err = svc.Login(ctx, &LoginRequest{Username: "guard", Password: "CorrectPass123"})
	require.ErrorIs(t, err, ErrMFARequired)

	// 错误 code：拒绝
	_, err = svc.Login(ctx, &LoginRequest{Username: "guard", Password: "CorrectPass123", TOTPCode: "000000"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "二次验证码错误")

	// 正确 code：放行
	resp, err := svc.Login(ctx, &LoginRequest{Username: "guard", Password: "CorrectPass123", TOTPCode: currentTOTP(t, setup.Secret)})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

func TestLogin_MFA_SkippedForExternalProvider(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")
	ctx := context.Background()

	// 影子账号：无 password_hash（外部身份源 JIT 形态），但 OTPEnabled 意外为真
	shadow := &model.Admin{Username: "ext-user", Status: 1, OTPEnabled: true, OTPSecret: "JBSWY3DPEHPK3PXP"}
	require.NoError(t, db.Create(shadow).Error)

	svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "ext-user"})

	// 外部身份源登录不触发 MFA 分支
	resp, err := svc.Login(ctx, &LoginRequest{Username: "ext-user", Password: "whatever"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

// stubPasswordProvider 模拟外部密码身份源（任意密码均通过）。
type stubPasswordProvider struct {
	kind     string
	username string
}

func (p stubPasswordProvider) Kind() string { return p.kind }

func (p stubPasswordProvider) Authenticate(_ context.Context, username, _ string) (*identity.Identity, error) {
	if username != p.username {
		return nil, identity.ErrInvalidCredentials
	}
	return &identity.Identity{Provider: p.kind, Username: username}, nil
}
