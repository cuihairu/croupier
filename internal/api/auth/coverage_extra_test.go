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

// TestIssueLogin_RoleLoadFailure covers the role-load failure branch:
// the admin exists and authentication passes, but the admin_roles table was
// removed so GetAdminRoles errors.
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

// TestProvisionShadowAdmin_CreateFailure covers the JIT provisioning failure
// branch: the external identity authenticates, but the admins table is
// unavailable so both provisioning and the follow-up lookup fail.
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

// TestLogin_ExistingExternalUser covers the path where an external identity
// hits an existing local record.
func TestLogin_ExistingExternalUser(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(
		adminModel,
		permissionservice.NewPermissionService(db),
		"test-secret",
	)

	// Pre-seed an account with the same name; the same user logs in via LDAP:
	// resolve hits the existing record.
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
