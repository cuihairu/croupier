package utils

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestParseRoleID_ValidID(t *testing.T) {
	id, err := ParseRoleID("123")
	assert.NoError(t, err)
	assert.Equal(t, uint(123), id)
}

func TestParseRoleID_EmptyID(t *testing.T) {
	_, err := ParseRoleID("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色ID不能为空")
}

func TestParseRoleID_WhitespaceID(t *testing.T) {
	_, err := ParseRoleID("   ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色ID不能为空")
}

func TestParseRoleID_InvalidID(t *testing.T) {
	_, err := ParseRoleID("abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的角色ID")
}

func TestParseRoleID_ZeroID(t *testing.T) {
	_, err := ParseRoleID("0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色ID必须大于0")
}

func TestParseRoleID_NegativeID(t *testing.T) {
	_, err := ParseRoleID("-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的角色ID")
}

func TestBuildRole_ValidRole(t *testing.T) {
	role := &model.Role{
		Name:        "admin",
		Description: "Administrator",
		Category:    "system",
	}
	role.ID = 1

	result := BuildRole(role, []string{"perm1", "perm2"})
	assert.Equal(t, int64(1), result.Id)
	assert.Equal(t, "admin", result.Name)
	assert.Equal(t, "Administrator", result.Description)
	assert.Equal(t, "system", result.Category)
	assert.Equal(t, []string{"perm1", "perm2"}, result.Permissions)
}

func TestBuildRole_EmptyPermissions(t *testing.T) {
	role := &model.Role{
		Name: "user",
	}
	role.ID = 2

	result := BuildRole(role, nil)
	assert.Equal(t, int64(2), result.Id)
	assert.Equal(t, "user", result.Name)
	assert.Nil(t, result.Permissions)
}

func TestEnsurePermissionIDs_NilRoleModel(t *testing.T) {
	_, err := EnsurePermissionIDs(context.Background(), nil, []string{"perm1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role model is not initialized")
}

func TestEnsurePermissionIDs_WithValidRoleModel(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	roleModel := model.NewRoleModel(db)

	// Create a permission first
	perm := &model.Permission{
		ID:       "test:read",
		Name:     "test:read",
		Resource: "test",
		Action:   "read",
	}
	err = db.Create(perm).Error
	require.NoError(t, err)

	result, err := EnsurePermissionIDs(context.Background(), roleModel, []string{"test:read"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"test:read"}, result)
}

func TestEnsurePermissionIDs_WithInvalidPermission(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	roleModel := model.NewRoleModel(db)

	_, err = EnsurePermissionIDs(context.Background(), roleModel, []string{"nonexistent:perm"})
	assert.Error(t, err) // Should return error for non-existent permission
}

func TestRoleNamesFromModels_EmptySlice(t *testing.T) {
	result := RoleNamesFromModels([]model.Role{})
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestRoleNamesFromModels_NilSlice(t *testing.T) {
	result := RoleNamesFromModels(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestRoleNamesFromModels_MultipleRoles(t *testing.T) {
	roles := []model.Role{
		{Name: "admin"},
		{Name: "user"},
		{Name: "guest"},
	}

	result := RoleNamesFromModels(roles)
	assert.Len(t, result, 3)
	assert.Equal(t, []string{"admin", "user", "guest"}, result)
}

func TestHasRole_EmptyRole(t *testing.T) {
	result := HasRole([]string{"admin", "user"}, "")
	assert.False(t, result)
}

func TestHasRole_WhitespaceRole(t *testing.T) {
	result := HasRole([]string{"admin", "user"}, "   ")
	assert.False(t, result)
}

func TestHasRole_ExactMatch(t *testing.T) {
	result := HasRole([]string{"admin", "user"}, "admin")
	assert.True(t, result)
}

func TestHasRole_CaseInsensitive(t *testing.T) {
	result := HasRole([]string{"Admin", "User"}, "admin")
	assert.True(t, result)
}

func TestHasRole_WhitespaceInRoles(t *testing.T) {
	result := HasRole([]string{"  admin  ", "  user  "}, "admin")
	assert.True(t, result)
}

func TestHasRole_NoMatch(t *testing.T) {
	result := HasRole([]string{"admin", "user"}, "guest")
	assert.False(t, result)
}

func TestHasRole_EmptyRolesList(t *testing.T) {
	result := HasRole([]string{}, "admin")
	assert.False(t, result)
}

func TestHasRole_NilRolesList(t *testing.T) {
	result := HasRole(nil, "admin")
	assert.False(t, result)
}
