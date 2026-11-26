package permission

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func requireTableDropV9(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table).Error)
}

func TestV9_CheckPermission_LoadRolesFails(t *testing.T) {
	db := setupTestDB(t)
	requireTableDropV9(t, db, "roles")

	allowed, err := NewPermissionService(db).CheckPermission(context.Background(), 1, "game", "read")
	require.Error(t, err)
	assert.False(t, allowed)
}

func TestV9_CheckPermission_PluckPermissionIDsFails(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "pluckadmin"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "test_role"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)
	requireTableDropV9(t, db, "role_permissions")

	allowed, err := NewPermissionService(db).CheckPermission(context.Background(), admin.ID, "game", "read")
	require.Error(t, err)
	assert.False(t, allowed)
}

// TestV9_CheckPermission_EnforceErrorViaSeam 经 enforceAnyPermission 接缝注入
// 失败，驱动 CheckPermission 的 casbin 错误透传分支（真实路径恒 nil，见
// service.go 接缝处不可达论证）。
func TestV9_CheckPermission_EnforceErrorViaSeam(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "enforceadmin"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "test_role"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)

	orig := enforceAnyPermission
	enforceAnyPermission = func(sub string, perms []string, required ...string) (bool, error) {
		return false, errors.New("casbin exploded")
	}
	t.Cleanup(func() { enforceAnyPermission = orig })

	allowed, err := NewPermissionService(db).CheckPermission(context.Background(), admin.ID, "game", "read")
	require.Error(t, err)
	assert.False(t, allowed)
	assert.Contains(t, err.Error(), "failed to enforce permission with casbin")
}
