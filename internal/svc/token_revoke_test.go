package svc

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTokenRevokeFixture 构造带真实 AdminModel 的 AuthMiddleware。
func setupTokenRevokeFixture(t *testing.T) (*AuthMiddleware, *model.AdminModel, uint) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))

	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: "revoker", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "Passw0rd!x"))

	// 全局 secret 是 sync.Once：同进程其他测试可能先初始化。用测试专用
	// reset 钩子换入本用例的 secret，结束后还原，避免顺序依赖。
	t.Cleanup(jwtutil.ResetGlobalSecretForTesting("revoke-test-secret"))
	mw := NewAuthMiddlewareImpl(&ServiceContext{AdminModel: adminModel})
	return mw, adminModel, admin.ID
}

func TestAuthMiddleware_TokenRevokedAfterBump(t *testing.T) {
	mw, adminModel, adminID := setupTokenRevokeFixture(t)
	ctx := context.Background()

	// 版本 0 签发的 token 初始可用
	token, err := jwtutil.Sign("revoke-test-secret", "revoker", []string{"admin"}, adminID, 0, time.Now())
	require.NoError(t, err)
	_, _, _, err = mw.authenticate(ctx, token)
	require.NoError(t, err)

	// 模拟撤销（登出/改密/禁用）：版本 +1
	require.NoError(t, adminModel.BumpTokenVersion(ctx, adminID))

	// 缓存 TTL 内旧版本仍被接受（30s 生效窗口是已知设计取舍）；
	// 清缓存模拟 TTL 过期后的行为
	mw.tokenVersions.Delete(adminID)

	_, _, _, err = mw.authenticate(ctx, token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token revoked")

	// 新版本签发的 token 放行
	fresh, err := jwtutil.Sign("revoke-test-secret", "revoker", []string{"admin"}, adminID, 1, time.Now())
	require.NoError(t, err)
	_, _, _, err = mw.authenticate(ctx, fresh)
	require.NoError(t, err)
}

func TestAuthMiddleware_LegacyTokenWithoutVersion(t *testing.T) {
	mw, _, adminID := setupTokenRevokeFixture(t)

	// 旧 token 无 tokenVersion 字段 → 解析为 0，与库中初始版本 0 一致，平滑兼容
	token, err := jwtutil.Sign("revoke-test-secret", "revoker", nil, adminID, 0, time.Now())
	require.NoError(t, err)
	_, _, _, err = mw.authenticate(context.Background(), token)
	require.NoError(t, err)
}

func TestAuthMiddleware_VersionCacheHit(t *testing.T) {
	mw, _, adminID := setupTokenRevokeFixture(t)
	ctx := context.Background()

	token, err := jwtutil.Sign("revoke-test-secret", "revoker", nil, adminID, 0, time.Now())
	require.NoError(t, err)

	// 首次认证填充缓存
	_, _, _, err = mw.authenticate(ctx, token)
	require.NoError(t, err)
	_, ok := mw.tokenVersions.Load(adminID)
	assert.True(t, ok, "first authenticate should populate the version cache")
}

func TestAdminModel_BumpAndGetTokenVersion(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Admin{}))

	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: "v", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "Passw0rd!x"))

	ctx := context.Background()
	v, err := adminModel.GetTokenVersion(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, v)

	require.NoError(t, adminModel.BumpTokenVersion(ctx, admin.ID))
	require.NoError(t, adminModel.BumpTokenVersion(ctx, admin.ID))
	v, err = adminModel.GetTokenVersion(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, v)
}
