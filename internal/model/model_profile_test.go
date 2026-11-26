package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewProfileModel(t *testing.T) {
	db := setupProfileTestDB(t)
	model := NewProfileModel(db)
	assert.NotNil(t, model)
}

func TestProfileModel_ReplacePermissions(t *testing.T) {
	db := setupProfileTestDB(t)
	model := NewProfileModel(db)
	ctx := context.Background()

	perms := []ProfilePermission{
		{Resource: "resource1", GameID: "game1", Env: "prod", Actions: JSON([]byte(`["read","write"]`))},
		{Resource: "resource2", GameID: "game1", Env: "prod", Actions: JSON([]byte(`["read"]`))},
	}

	err := model.ReplacePermissions(ctx, 1, perms)
	require.NoError(t, err)

	// Verify
	list, err := model.ListPermissions(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Replace with different set
	newPerms := []ProfilePermission{
		{Resource: "resource3", GameID: "game2", Env: "test", Actions: JSON([]byte(`["admin"]`))},
	}
	err = model.ReplacePermissions(ctx, 1, newPerms)
	require.NoError(t, err)

	list, err = model.ListPermissions(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "resource3", list[0].Resource)
}

func TestProfileModel_ListPermissions(t *testing.T) {
	db := setupProfileTestDB(t)
	model := NewProfileModel(db)
	ctx := context.Background()

	perms := []ProfilePermission{
		{Resource: "res1", GameID: "game1", Actions: JSON([]byte(`["read"]`))},
		{Resource: "res2", GameID: "game1", Actions: JSON([]byte(`["write"]`))},
	}
	err := model.ReplacePermissions(ctx, 1, perms)
	require.NoError(t, err)

	// List permissions
	list, err := model.ListPermissions(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// List for non-existent admin
	list, err = model.ListPermissions(ctx, 999)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestProfileModel_ReplaceGames(t *testing.T) {
	db := setupProfileTestDB(t)
	model := NewProfileModel(db)
	ctx := context.Background()

	games := []ProfileGame{
		{GameID: "game1", GameName: "Game 1", Color: "red", Envs: JSON([]byte(`["prod","test"]`))},
		{GameID: "game2", GameName: "Game 2", Color: "blue", Envs: JSON([]byte(`["dev"]`))},
	}

	err := model.ReplaceGames(ctx, 1, games)
	require.NoError(t, err)

	// Verify
	list, err := model.ListGames(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestProfileModel_ListGames(t *testing.T) {
	db := setupProfileTestDB(t)
	model := NewProfileModel(db)
	ctx := context.Background()

	games := []ProfileGame{
		{GameID: "game1", GameName: "Game 1", Color: "red"},
		{GameID: "game2", GameName: "Game 2", Color: "blue"},
	}
	err := model.ReplaceGames(ctx, 1, games)
	require.NoError(t, err)

	// List games
	list, err := model.ListGames(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// List for non-existent admin
	list, err = model.ListGames(ctx, 999)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

// ===== RateLimitModel Tests =====
