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

// newLockoutService builds a Service with a lockout policy (threshold 3,
// lock for 1 minute).
func newLockoutService(t *testing.T) (*Service, *model.AdminModel, *gorm.DB) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db).WithRecoveryDB(db).
		WithLoginLockout(config.LoginLockoutConfig{Threshold: 3, LockMinutes: 1})
	return svc, adminModel, db
}

func TestLogin_Lockout_TriggeredAfterThreshold(t *testing.T) {
	svc, adminModel, db := newLockoutService(t)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "lockme", "CorrectPass123", "admin")

	// first two failures: the wrong-password message
	for i := 0; i < 2; i++ {
		_, err := svc.Login(ctx, &LoginRequest{Username: "lockme", Password: "wrong"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "用户名或密码错误")
	}

	// third failure: the threshold is reached and the lockout triggers
	_, err := svc.Login(ctx, &LoginRequest{Username: "lockme", Password: "wrong"})
	require.Error(t, err)

	admin, err := adminModel.FindByUsername(ctx, "lockme")
	require.NoError(t, err)
	require.NotNil(t, admin.LockedUntil, "threshold reached should set LockedUntil")
	assert.Equal(t, 3, admin.FailedAttempts)

	// inside the lockout window: even the right password is rejected, and the
	// error message is the lockout notice
	_, err = svc.Login(ctx, &LoginRequest{Username: "lockme", Password: "CorrectPass123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "账号已锁定")
}

func TestLogin_Lockout_ExpiresAfterWindow(t *testing.T) {
	svc, adminModel, db := newLockoutService(t)
	ctx := context.Background()
	adminID := createTestAdminWithRole(t, db, "expireme", "CorrectPass123", "admin")

	// manually set an already-expired lockout
	past := time.Now().Add(-time.Minute).UTC()
	require.NoError(t, adminModel.Update(ctx, adminID, map[string]interface{}{
		"failed_attempts": 5,
		"locked_until":    past,
	}))

	// lockout expired: the right password should log in, and the counter is
	// cleared
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

	// two failures (below the threshold)
	for i := 0; i < 2; i++ {
		_, err := svc.Login(ctx, &LoginRequest{Username: "resetme", Password: "wrong"})
		require.Error(t, err)
	}
	admin, err := adminModel.FindByUsername(ctx, "resetme")
	require.NoError(t, err)
	assert.Equal(t, 2, admin.FailedAttempts)

	// cleared after a successful login
	_, err = svc.Login(ctx, &LoginRequest{Username: "resetme", Password: "CorrectPass123"})
	require.NoError(t, err)
	admin, err = adminModel.FindByUsername(ctx, "resetme")
	require.NoError(t, err)
	assert.Equal(t, 0, admin.FailedAttempts)
}

func TestLogin_UnknownUserNotLocked(t *testing.T) {
	svc, _, _ := newLockoutService(t)
	ctx := context.Background()

	// repeated failures for a nonexistent account: always the generic error,
	// never the lockout path (guards against account enumeration)
	for i := 0; i < 5; i++ {
		_, err := svc.Login(ctx, &LoginRequest{Username: "ghost", Password: "wrong"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "用户名或密码错误")
	}
}
