package auth

// mfa.go / service.go 覆盖率补缺（巡检 98.4%→）：
// - With* 构造选项此前 0%（注释口径本就面向测试注入）
// - 经 WithOTPRecoveryModel 注入「关库」恢复码模型，驱动 Consume/Count/
//   DeleteAll 的错误分支与 MFAStatus/登录二段校验的降级路径
// - requireLocalAdmin 缺失账号、OtpauthURI 空账号拒绝等守卫分支
// 全部为可达分支，无凑数用例。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newBrokenRecoveryModel 打开即关的独立库：恢复码模型任何操作都报错，
// 且不影响业务库（业务库保持可用）。
func newBrokenRecoveryModel(t *testing.T) *model.AdminOTPRecoveryCodeModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return model.NewAdminOTPRecoveryCodeModel(db)
}

// enableMFAFor 走完整 Setup+Confirm 启用 TOTP，返回 setup secret 供后续
// 生成当前窗口验证码。
func enableMFAFor(t *testing.T, db *gorm.DB, svc *Service, username string) string {
	t.Helper()
	setup, err := svc.MFASetup(context.Background(), username)
	require.NoError(t, err)
	_, err = svc.MFAConfirm(context.Background(), username, currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	return setup.Secret
}

// With* 构造选项：返回自身并完成字段接线（service.go 此前 0%）。
func TestMFA_ServiceBuilderOptions(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	recoveryModel := model.NewAdminOTPRecoveryCodeModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")

	got := svc.WithAdminModel(adminModel)
	assert.Same(t, svc, got)
	assert.Same(t, adminModel, got.adminModel)

	got = svc.WithOTPRecoveryModel(recoveryModel)
	assert.Same(t, svc, got)
	assert.Same(t, recoveryModel, got.otpRecoveryModel)
}

// MFASetup 对缺失账号透传 requireLocalAdmin 错误。
func TestMFA_Setup_UnknownUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	_, err := svc.MFASetup(context.Background(), "nobody")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户不存在")
}

// 用户名可精确检索但 trim 后为空 → OtpauthURI 拒绝空账号，
// 覆盖「生成绑定链接失败」守卫分支（防止空账号悄悄产出坏链接）。
func TestMFA_Setup_BlankUsername_OtpauthURIError(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, " ", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	_, err := svc.MFASetup(context.Background(), " ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "生成绑定链接失败")
}

// 恢复码清理失败不阻断关闭流程，但必须留下错误日志（残留码是安全隐患）。
func TestMFA_Disable_RecoveryCleanupError_NotBlocking(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "dluser", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	secret := enableMFAFor(t, db, svc, "dluser")

	svc.WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	require.NoError(t, svc.MFADisable(context.Background(), "dluser", currentTOTP(t, secret), "CorrectPass123"))

	admin, err := svc.adminModel.FindByUsername(context.Background(), "dluser")
	require.NoError(t, err)
	assert.False(t, admin.OTPEnabled)
}

// Consume/Count 的存储错误原样透传；非形态码在触库前即拒绝。
func TestMFA_RecoveryCode_ModelErrors_PassThrough(t *testing.T) {
	db := setupTestDB(t)
	id := createTestAdminWithRole(t, db, "rcuser", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").
		WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	ctx := context.Background()

	// ABDC23EFGH：形态合法（10 位、字母表内）但存储失败 → 错误透传
	ok, err := svc.ConsumeRecoveryCode(ctx, id, "ABCD23EFGH")
	require.Error(t, err)
	assert.False(t, ok)

	// 形态非法 → 不触库直接拒绝
	ok, err = svc.ConsumeRecoveryCode(ctx, id, "short")
	require.NoError(t, err)
	assert.False(t, ok)

	n, err := svc.CountRecoveryCodes(ctx, id)
	require.Error(t, err)
	assert.Zero(t, n)
}

// 登录二段校验：恢复码形态合法但存储失败 → 校验失败拒绝登录（不误放行）。
func TestMFA_Login_RecoveryModelError_Denies(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "mfaerr", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	enableMFAFor(t, db, svc, "mfaerr")

	svc.WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	_, err := svc.Login(context.Background(), &LoginRequest{
		Username: "mfaerr",
		Password: "CorrectPass123",
		TOTPCode: "ABCD23EFGH",
	})
	require.Error(t, err)
}

// MFAStatus：剩余统计失败仅降级为 0（提示性数据不阻塞状态查询）。
func TestMFA_Status_RecoveryCountError_Degrades(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "stuser", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	enableMFAFor(t, db, svc, "stuser")

	svc.WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	out := svc.MFAStatus(context.Background(), "stuser")
	assert.True(t, out.Enabled)
	assert.True(t, out.Local)
	assert.Zero(t, out.RecoveryCodesRemaining)
	assert.Zero(t, out.RecoveryCodeTotal)
}
