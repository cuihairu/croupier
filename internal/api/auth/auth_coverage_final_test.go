// 覆盖目标（coverage final）：
//  1. handler.MFAStatus：未登录 401 / 成功 200 分支。
//  2. service.MFAStatus：admin 缺失、外部影子账号、本地账号三态。
//  3. service.provisionShadowAdmin：Create 冲突后回落已存在记录分支。
package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHandler_MFAStatus_Branches(t *testing.T) {
	ginCtx, rec := newAuthTestContext(http.MethodGet, "/api/v1/auth/mfa/status", "")
	// 未登录：mfaUsername 为空 → 401
	NewHandler(&Service{}).MFAStatus(ginCtx)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 已登录 + 本地账号 → 200（enabled=false, local=true）
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "statususer", "pw123456", "admin")
	svcAuth := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret")

	ginCtx2, rec2 := newAuthTestContext(http.MethodGet, "/api/v1/auth/mfa/status", "")
	ginCtx2.Set("username", "statususer")
	NewHandler(svcAuth).MFAStatus(ginCtx2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), `"local":true`)
}

func TestService_MFAStatus_Branches(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	svcAuth := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret")

	// admin 不存在 → 空响应（FindByUsername nil）
	resp, err := svcAuth.MFAStatus(ctx, "ghost")
	require.NoError(t, err)
	assert.False(t, resp.Local)
	assert.False(t, resp.Enabled)

	// 本地账号（有 password_hash）→ local=true
	createTestAdminWithRole(t, db, "localu", "pw123456", "admin")
	resp, err = svcAuth.MFAStatus(ctx, "localu")
	require.NoError(t, err)
	assert.True(t, resp.Local)
	assert.False(t, resp.Enabled)

	// 影子账号（无 password_hash）→ local=false
	shadow := &model.Admin{Username: "shadowu", Status: 1, OTPEnabled: true}
	require.NoError(t, db.Create(shadow).Error)
	resp, err = svcAuth.MFAStatus(ctx, "shadowu")
	require.NoError(t, err)
	assert.False(t, resp.Local)
	assert.False(t, resp.Enabled, "非本地账号 enabled 应为 false")

	// 查询错误（连接关闭）→ 空响应兜底
	db2 := setupTestDB(t)
	createTestAdminWithRole(t, db2, "closedu", "pw123456", "admin")
	svcAuth2 := NewService(model.NewAdminModel(db2), permissionservice.NewPermissionService(db2), "secret")
	sqlDB, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	resp, err = svcAuth2.MFAStatus(ctx, "closedu")
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_ProvisionShadowAdmin_FallsBackToExisting(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// 预插同名影子账号（模拟并发首登已创建）。
	existing := &model.Admin{Username: "dup-shadow", Nickname: "dup", Status: 1}
	require.NoError(t, db.Create(existing).Error)

	svcAuth := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret")

	// Create 注错 → 唯一索引冲突路径：回落到已存在记录。
	require.NoError(t, db.Callback().Create().Before("gorm:create").
		Register("test:auth_fail_create", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("duplicate key"))
		}))

	admin, err := svcAuth.provisionShadowAdmin(ctx, &identity.Identity{
		Provider: identity.KindLDAP,
		Username: "dup-shadow",
		Email:    "d@x.io",
	})
	require.NoError(t, err)
	require.NotNil(t, admin)
	assert.Equal(t, "dup-shadow", admin.Username)
}
