package auth

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIssueLogin_RoleLoadFailure 覆盖角色加载失败分支：
// admin 存在且认证通过，但 admin_roles 表被移除导致 GetAdminRoles 报错。
func TestIssueLogin_RoleLoadFailure(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "erin", "pw", "ops")
	require.NoError(t, db.Migrator().DropTable("admin_roles"))

	svc := NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		"test-secret",
	)
	_, err := svc.Login(context.Background(), &LoginRequest{Username: "erin", Password: "pw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "获取用户角色失败")
}

// TestProvisionShadowAdmin_CreateFailure 覆盖 JIT 建号失败分支：
// 外部身份认证通过，但 admins 表不可用导致建号与二次查找均失败。
func TestProvisionShadowAdmin_CreateFailure(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Migrator().DropTable("admins"))

	svc := NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		"test-secret",
	)
	svc.WithPasswordProvider(&fakePasswordProvider{
		kind:       identity.KindLDAP,
		validUsers: map[string]string{"ghost": "ldappw"},
	})

	_, err := svc.Login(context.Background(), &LoginRequest{Username: "ghost", Password: "ldappw"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "登录失败")
}

// TestLogin_ExistingExternalUser 覆盖外部身份命中已存在本地记录的路径。
func TestLogin_ExistingExternalUser(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(
		adminModel,
		permissionservice.NewPermissionService(db),
		"test-secret",
	)

	// 预置同名账号，LDAP 侧同一用户登录：resolve 命中已存在记录。
	admin := &model.Admin{Username: "frank", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "localpw"))

	svc.WithPasswordProvider(&fakePasswordProvider{
		kind:       identity.KindLDAP,
		validUsers: map[string]string{"frank": "ldappw"},
	})

	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "frank", Password: "ldappw"})
	require.NoError(t, err)
	assert.Equal(t, "frank", resp.User.Username)
}
