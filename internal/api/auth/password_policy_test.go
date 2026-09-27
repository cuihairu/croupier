package auth

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OPEN-ISSUES #20 回归：登录响应 mustChangePassword——
// ① 被标记 must_change_password 的账号登录后 flag=true；
// ② 密码已过有效期（password_expires_at）视同必须改密；
// ③ 干净账号恒 false（无标记且未过期）。

func loginWithFlags(t *testing.T, mutate func(*model.Admin)) *LoginResponse {
	t.Helper()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)

	admin := &model.Admin{Username: "policyuser", Nickname: "Policy User", Status: 1}
	if mutate != nil {
		mutate(admin)
	}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	service := NewService(adminModel, permission.NewPermissionService(db), "test-secret-key")
	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "policyuser",
		Password: "password123",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

func TestService_Login_MustChangePassword_Flagged(t *testing.T) {
	t.Parallel()
	resp := loginWithFlags(t, func(a *model.Admin) { a.MustChangePassword = true })
	assert.True(t, resp.MustChangePassword, "被标记账号登录必须返回 mustChangePassword=true")
}

func TestService_Login_MustChangePassword_ExpiredPassword(t *testing.T) {
	t.Parallel()
	resp := loginWithFlags(t, func(a *model.Admin) {
		expired := time.Now().Add(-24 * time.Hour)
		a.PasswordExpiresAt = &expired
	})
	assert.True(t, resp.MustChangePassword, "密码已过期视同必须改密")
}

func TestService_Login_MustChangePassword_CleanAccount(t *testing.T) {
	t.Parallel()
	resp := loginWithFlags(t, nil)
	assert.False(t, resp.MustChangePassword, "干净账号不应触发强制改密")
}

func TestService_Login_MustChangePassword_FutureExpiry(t *testing.T) {
	t.Parallel()
	resp := loginWithFlags(t, func(a *model.Admin) {
		future := time.Now().Add(90 * 24 * time.Hour)
		a.PasswordExpiresAt = &future
	})
	assert.False(t, resp.MustChangePassword, "未过有效期的账号不触发强制改密")
}
