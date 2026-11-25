// Coverage targets (coverage final):
//  1. handler.MFAStatus: not-logged-in 401 / success 200 branches.
//  2. service.MFAStatus: the three states — missing admin, external shadow
//     account, and local account.
//  3. service.provisionShadowAdmin: falling back to the existing record after
//     a Create conflict.
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
	// Not logged in: mfaUsername empty → 401
	NewHandler(&Service{}).MFAStatus(ginCtx)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Logged in + local account → 200 (enabled=false, local=true)
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "statususer", "pw123456", "admin")
	svcAuth := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret").WithRecoveryDB(db).WithRecoveryDB(db)

	ginCtx2, rec2 := newAuthTestContext(http.MethodGet, "/api/v1/auth/mfa/status", "")
	ginCtx2.Set("username", "statususer")
	NewHandler(svcAuth).MFAStatus(ginCtx2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), `"local":true`)
}

func TestService_MFAStatus_Branches(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	svcAuth := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret").WithRecoveryDB(db).WithRecoveryDB(db)

	// Admin missing → empty response (FindByUsername nil)
	resp := svcAuth.MFAStatus(ctx, "ghost")
	assert.False(t, resp.Local)
	assert.False(t, resp.Enabled)

	// Local account (has password_hash) → local=true
	createTestAdminWithRole(t, db, "localu", "pw123456", "admin")
	resp = svcAuth.MFAStatus(ctx, "localu")
	assert.True(t, resp.Local)
	assert.False(t, resp.Enabled)

	// Shadow account (no password_hash) → local=false
	shadow := &model.Admin{Username: "shadowu", Status: 1, OTPEnabled: true}
	require.NoError(t, db.Create(shadow).Error)
	resp = svcAuth.MFAStatus(ctx, "shadowu")
	assert.False(t, resp.Local)
	assert.False(t, resp.Enabled, "非本地账号 enabled 应为 false")

	// Query error (connection closed) → falls back to the empty response
	db2 := setupTestDB(t)
	createTestAdminWithRole(t, db2, "closedu", "pw123456", "admin")
	svcAuth2 := NewService(model.NewAdminModel(db2), permissionservice.NewPermissionService(db2), "secret").WithRecoveryDB(db2)
	sqlDB, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	resp = svcAuth2.MFAStatus(ctx, "closedu")
	assert.NotNil(t, resp)
}

func TestService_ProvisionShadowAdmin_FallsBackToExisting(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Pre-insert a shadow account with the same name (simulating one already
	// created by a concurrent first login).
	existing := &model.Admin{Username: "dup-shadow", Nickname: "dup", Status: 1}
	require.NoError(t, db.Create(existing).Error)

	svcAuth := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret").WithRecoveryDB(db).WithRecoveryDB(db)

	// Inject an error into Create → the unique-index conflict path: fall back to
	// the existing record.
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
