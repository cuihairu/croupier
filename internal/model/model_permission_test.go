package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewPermissionModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewPermissionModel(db)
	assert.NotNil(t, model)
}

func TestPermissionModel_List(t *testing.T) {
	db := setupTestDB(t)
	model := NewPermissionModel(db)
	ctx := context.Background()

	// Create test permissions
	for i := 0; i < 5; i++ {
		perm := &Permission{
			ID:       "testperm" + string(rune('a'+i)),
			Name:     "Test Permission " + string(rune('a'+i)),
			Resource: "resource",
			Action:   "action",
			Category: "test",
		}
		err := db.Create(perm).Error
		require.NoError(t, err)
	}

	perms, total, err := model.List(ctx, ListPermissionsOptions{
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.GreaterOrEqual(t, len(perms), 5)
}

func TestPermissionModel_List_CategoryFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewPermissionModel(db)
	ctx := context.Background()

	perm1 := &Permission{ID: "catperm1", Name: "Cat 1", Resource: "r", Action: "a", Category: "admin"}
	perm2 := &Permission{ID: "catperm2", Name: "Cat 2", Resource: "r", Action: "a", Category: "user"}
	err := db.Create(perm1).Error
	require.NoError(t, err)
	err = db.Create(perm2).Error
	require.NoError(t, err)

	perms, total, err := model.List(ctx, ListPermissionsOptions{
		Page:     1,
		PageSize: 10,
		Category: "admin",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, p := range perms {
		assert.Equal(t, "admin", p.Category)
	}
}

func TestPermissionModel_List_ResourceFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewPermissionModel(db)
	ctx := context.Background()

	perm1 := &Permission{ID: "resperm1", Name: "Res 1", Resource: "games", Action: "read", Category: "test"}
	perm2 := &Permission{ID: "resperm2", Name: "Res 2", Resource: "players", Action: "read", Category: "test"}
	err := db.Create(perm1).Error
	require.NoError(t, err)
	err = db.Create(perm2).Error
	require.NoError(t, err)

	perms, total, err := model.List(ctx, ListPermissionsOptions{
		Page:     1,
		PageSize: 10,
		Resource: "games",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, p := range perms {
		assert.Equal(t, "games", p.Resource)
	}
}

func TestPermissionModel_FindOne(t *testing.T) {
	db := setupTestDB(t)
	model := NewPermissionModel(db)
	ctx := context.Background()

	perm := &Permission{
		ID:       "findoneperm",
		Name:     "Find One Perm",
		Resource: "resource",
		Action:   "action",
		Category: "test",
	}
	err := db.Create(perm).Error
	require.NoError(t, err)

	found, err := model.FindOne(ctx, "findoneperm")
	require.NoError(t, err)
	assert.Equal(t, perm.Name, found.Name)

	_, err = model.FindOne(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission not found")
}

// ===== PlayerModel Tests =====
