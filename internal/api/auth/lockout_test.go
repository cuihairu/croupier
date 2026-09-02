package auth

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newLockoutService 构造带锁定策略的 Service（阈值 3，锁定 1 分钟）。
func newLockoutService(t *testing.T) (*Service, *model.AdminModel, *gorm.DB) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").
		WithLoginLockout(config.LoginLockoutConfig{Threshold: 3, LockMinutes: 1})
	return svc, adminModel, db
}

func TestLogin_Lockout_TriggeredAfterThreshold(t *testing.T) {
	svc, adminModel, db := newLockoutService(t)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "lockme", "CorrectPass123", "admin")

	// 前两次失败：提示密码错误
	for i := 0; i < 2; i++ {
		_, err := svc.Login(ctx, &LoginRequest{Username: "lockme", Password: "wrong"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "用户名或密码错误")
	}

	// 第三次失败：达到阈值，触发锁定
	_, err := svc.Login(ctx, &LoginRequest{Username: "lockme", Password: "wrong"})
	require.Error(t, err)

	admin, err := adminModel.FindByUsername(ctx, "lockme")
	require.NoError(t, err)
	require.NotNil(t, admin.LockedUntil, "threshold reached should set LockedUntil")
	assert.Equal(t, 3, admin.FailedAttempts)

	// 锁定期内：正确密码也被拒绝，且错误消息是锁定提示
	_, err = svc.Login(ctx, &LoginRequest{Username: "lockme", Password: "CorrectPass123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "账号已锁定")
}

func TestLogin_Lockout_ExpiresAfterWindow(t *testing.T) {
	svc, adminModel, db := newLockoutService(t)
	ctx := context.Background()
	adminID := createTestAdminWithRole(t, db, "expireme", "CorrectPass123", "admin")

	// 手动设置一个已过期的锁定
	past := time.Now().Add(-time.Minute).UTC()
	require.NoError(t, adminModel.Update(ctx, adminID, map[string]interface{}{
		"failed_attempts": 5,
		"locked_until":    past,
	}))

	// 锁定已过期：正确密码应能登录，且计数清零
	resp, err := svc.Login(ctx, &LoginRequest{Username: "expireme", Password: "CorrectPass123"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)

	admin, err := adminModel.FindByUsername(ctx, "expireme")
	require.NoError(t, err)
	assert.Equal(t, 0, admin.FailedAttempts)
	assert.Nil(t, admin.LockedUntil)
}

func TestLogin_SuccessResetsFailureCount(t *testing.T) {
	svc, adminModel, db := newLockoutService(t)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "resetme", "CorrectPass123", "admin")

	// 两次失败（未达阈值）
	for i := 0; i < 2; i++ {
		_, err := svc.Login(ctx, &LoginRequest{Username: "resetme", Password: "wrong"})
		require.Error(t, err)
	}
	admin, err := adminModel.FindByUsername(ctx, "resetme")
	require.NoError(t, err)
	assert.Equal(t, 2, admin.FailedAttempts)

	// 成功登录后清零
	_, err = svc.Login(ctx, &LoginRequest{Username: "resetme", Password: "CorrectPass123"})
	require.NoError(t, err)
	admin, err = adminModel.FindByUsername(ctx, "resetme")
	require.NoError(t, err)
	assert.Equal(t, 0, admin.FailedAttempts)
}

func TestLogin_UnknownUserNotLocked(t *testing.T) {
	svc, _, _ := newLockoutService(t)
	ctx := context.Background()

	// 不存在的账号连续失败：永远返回通用错误，不触发锁定路径（防账号枚举）
	for i := 0; i < 5; i++ {
		_, err := svc.Login(ctx, &LoginRequest{Username: "ghost", Password: "wrong"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "用户名或密码错误")
	}
}
