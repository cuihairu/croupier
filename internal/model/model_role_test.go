package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewRoleModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	assert.NotNil(t, model)
}

func TestRoleModel_Create(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	// Use unique name to avoid conflicts with other tests
	role := &Role{
		Name:        "testrole" + time.Now().Format("150405"),
		Description: "Test Role",
		Category:    "test",
	}

	err := model.Create(ctx, role)
	require.NoError(t, err)
	assert.NotZero(t, role.ID)
}

func TestRoleModel_FindOne(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role := &Role{
		Name:        "findonerole",
		Description: "Find One Role",
		Category:    "test",
	}
	err := model.Create(ctx, role)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, role.Name, found.Name)

	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role not found")
}

func TestRoleModel_Update(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role := &Role{
		Name:        "updaterole",
		Description: "Update Role",
		Category:    "test",
	}
	err := model.Create(ctx, role)
	require.NoError(t, err)

	updates := map[string]interface{}{
		"description": "Updated Description",
	}
	err = model.Update(ctx, role.ID, updates)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Description", found.Description)
}

func TestRoleModel_Update_EmptyUpdates(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role := &Role{
		Name:     "emptyupdaterole",
		Category: "test",
	}
	err := model.Create(ctx, role)
	require.NoError(t, err)

	// Empty updates should not error
	err = model.Update(ctx, role.ID, map[string]interface{}{})
	require.NoError(t, err)
}

func TestRoleModel_Delete(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role := &Role{
		Name:     "deleterole",
		Category: "test",
	}
	err := model.Create(ctx, role)
	require.NoError(t, err)

	err = model.Delete(ctx, role.ID)
	require.NoError(t, err)

	_, err = model.FindOne(ctx, role.ID)
	assert.Error(t, err)
}

func TestRoleModel_List(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		role := &Role{
			Name:     "listrole" + string(rune('a'+i)),
			Category: "test",
		}
		err := model.Create(ctx, role)
		require.NoError(t, err)
	}

	roles, total, err := model.List(ctx, ListRolesOptions{
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.GreaterOrEqual(t, len(roles), 5)
}

func TestRoleModel_List_CategoryFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role1 := &Role{Name: "adminrole", Category: "admin"}
	role2 := &Role{Name: "userrole", Category: "user"}
	err := model.Create(ctx, role1)
	require.NoError(t, err)
	err = model.Create(ctx, role2)
	require.NoError(t, err)

	roles, total, err := model.List(ctx, ListRolesOptions{
		Page:     1,
		PageSize: 10,
		Category: "admin",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, r := range roles {
		assert.Equal(t, "admin", r.Category)
	}
}

func TestRoleModel_List_Search(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role1 := &Role{Name: "searchrole", Description: "Searchable Role", Category: "test"}
	role2 := &Role{Name: "otherrole", Description: "Other Role", Category: "test"}
	err := model.Create(ctx, role1)
	require.NoError(t, err)
	err = model.Create(ctx, role2)
	require.NoError(t, err)

	roles, total, err := model.List(ctx, ListRolesOptions{
		Page:     1,
		PageSize: 10,
		Search:   "search",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	found := false
	for _, r := range roles {
		if r.Name == "searchrole" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestRoleModel_ReplacePermissions(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	// Create permissions
	perm1 := &Permission{
		ID:       "perm1",
		Name:     "Permission 1",
		Resource: "resource1",
		Action:   "action1",
		Category: "test",
	}
	perm2 := &Permission{
		ID:       "perm2",
		Name:     "Permission 2",
		Resource: "resource2",
		Action:   "action2",
		Category: "test",
	}
	err := db.Create(perm1).Error
	require.NoError(t, err)
	err = db.Create(perm2).Error
	require.NoError(t, err)

	// Create role
	role := &Role{Name: "permrole", Category: "test"}
	err = model.Create(ctx, role)
	require.NoError(t, err)

	// Replace permissions
	err = model.ReplacePermissions(ctx, role.ID, []string{"perm1", "perm2"})
	require.NoError(t, err)

	// Verify
	permIDs, err := model.GetRolePermissionIDs(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permIDs, 2)

	// Replace with fewer permissions
	err = model.ReplacePermissions(ctx, role.ID, []string{"perm1"})
	require.NoError(t, err)

	permIDs, err = model.GetRolePermissionIDs(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permIDs, 1)
	assert.Equal(t, "perm1", permIDs[0])
}

func TestRoleModel_ReplacePermissions_Empty(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	// Use unique name to avoid conflicts
	role := &Role{Name: "emptypermrole" + time.Now().Format("150405"), Category: "test"}
	err := model.Create(ctx, role)
	require.NoError(t, err)

	// Add permissions then clear them
	perm1 := &Permission{ID: "perm1empty" + time.Now().Format("150405"), Name: "Perm 1", Resource: "r1", Action: "a1", Category: "test"}
	err = db.Create(perm1).Error
	require.NoError(t, err)

	err = model.ReplacePermissions(ctx, role.ID, []string{perm1.ID})
	require.NoError(t, err)

	// Clear permissions
	err = model.ReplacePermissions(ctx, role.ID, []string{})
	require.NoError(t, err)

	permIDs, err := model.GetRolePermissionIDs(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permIDs, 0)
}

func TestRoleModel_GetRolePermissionIDs(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role := &Role{Name: "getpermrole", Category: "test"}
	err := model.Create(ctx, role)
	require.NoError(t, err)

	// No permissions initially
	permIDs, err := model.GetRolePermissionIDs(ctx, role.ID)
	require.NoError(t, err)
	assert.Empty(t, permIDs)

	// Add permission
	perm := &Permission{ID: "testperm", Name: "Test Perm", Resource: "r", Action: "a", Category: "test"}
	err = db.Create(perm).Error
	require.NoError(t, err)

	err = model.ReplacePermissions(ctx, role.ID, []string{"testperm"})
	require.NoError(t, err)

	permIDs, err = model.GetRolePermissionIDs(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permIDs, 1)
}

func TestRoleModel_GetRolesPermissionIDs(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	role1 := &Role{Name: "multirole1", Category: "test"}
	role2 := &Role{Name: "multirole2", Category: "test"}
	err := model.Create(ctx, role1)
	require.NoError(t, err)
	err = model.Create(ctx, role2)
	require.NoError(t, err)

	perm1 := &Permission{ID: "multi1", Name: "Multi 1", Resource: "r", Action: "a", Category: "test"}
	perm2 := &Permission{ID: "multi2", Name: "Multi 2", Resource: "r", Action: "a", Category: "test"}
	err = db.Create(perm1).Error
	require.NoError(t, err)
	err = db.Create(perm2).Error
	require.NoError(t, err)

	err = model.ReplacePermissions(ctx, role1.ID, []string{"multi1"})
	require.NoError(t, err)
	err = model.ReplacePermissions(ctx, role2.ID, []string{"multi1", "multi2"})
	require.NoError(t, err)

	result, err := model.GetRolesPermissionIDs(ctx, []uint{role1.ID, role2.ID})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result[role1.ID], 1)
	assert.Len(t, result[role2.ID], 2)
}

func TestRoleModel_GetRolesPermissionIDs_Empty(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	result, err := model.GetRolesPermissionIDs(ctx, []uint{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestRoleModel_ValidatePermissionIDs(t *testing.T) {
	db := setupTestDB(t)
	model := NewRoleModel(db)
	ctx := context.Background()

	// Create permissions
	perm1 := &Permission{ID: "validperm1", Name: "Valid 1", Resource: "r", Action: "a", Category: "test"}
	perm2 := &Permission{ID: "validperm2", Name: "Valid 2", Resource: "r", Action: "a", Category: "test"}
	err := db.Create(perm1).Error
	require.NoError(t, err)
	err = db.Create(perm2).Error
	require.NoError(t, err)

	// Test valid IDs
	validated, err := model.ValidatePermissionIDs(ctx, []string{"validperm1", "validperm2"})
	require.NoError(t, err)
	assert.Len(t, validated, 2)

	// Test with invalid ID
	_, err = model.ValidatePermissionIDs(ctx, []string{"validperm1", "invalidperm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test empty input
	validated, err = model.ValidatePermissionIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Nil(t, validated)

	// Test deduplication
	validated, err = model.ValidatePermissionIDs(ctx, []string{"validperm1", "validperm1", "validperm2"})
	require.NoError(t, err)
	assert.Len(t, validated, 2)
}

// ===== PermissionModel Tests =====
