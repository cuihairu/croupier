package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewConfigVersionModel(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	assert.NotNil(t, model)
}

func TestConfigVersionModel_Create(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	ctx := context.Background()

	record, err := model.Create(ctx, "test-key", "test-value", "admin")
	require.NoError(t, err)
	assert.NotZero(t, record.ID)
	assert.Equal(t, "test-key", record.Key)
	assert.Equal(t, 1, record.Version)
	assert.Equal(t, "admin", record.CreatedBy)
}

func TestConfigVersionModel_List(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	ctx := context.Background()

	// Create multiple versions
	for i := 1; i <= 3; i++ {
		_, err := model.Create(ctx, "list-test-key", "value-v"+string(rune('0'+i)), "admin")
		require.NoError(t, err)
	}

	// List versions
	records, err := model.List(ctx, "list-test-key")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 3)

	// Verify order (newest first)
	assert.Greater(t, records[0].Version, records[len(records)-1].Version)

	// Empty key returns empty
	records, err = model.List(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestConfigVersionModel_Find(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	ctx := context.Background()

	// Create test record
	_, err := model.Create(ctx, "find-test-key", "find-test-value", "admin")
	require.NoError(t, err)

	// Find version 1
	record, err := model.Find(ctx, "find-test-key", 1)
	require.NoError(t, err)
	assert.Equal(t, "find-test-key", record.Key)
	assert.Equal(t, 1, record.Version)

	// Find non-existent version
	_, err = model.Find(ctx, "find-test-key", 999)
	assert.Error(t, err)

	// Empty key
	_, err = model.Find(ctx, "", 1)
	assert.Error(t, err)

	// Invalid version
	_, err = model.Find(ctx, "find-test-key", 0)
	assert.Error(t, err)
	_, err = model.Find(ctx, "find-test-key", -1)
	assert.Error(t, err)
}

func TestConfigVersionModel_CreateWithMeta(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	ctx := context.Background()

	payload := ConfigVersionPayload{
		Key:         "meta-key",
		Content:     "meta-value",
		Format:      "yaml",
		GameID:      "game1",
		Env:         "prod",
		Message:     "Initial config",
		BaseVersion: 0,
	}

	record, err := model.CreateWithMeta(ctx, payload, "admin")
	require.NoError(t, err)
	assert.NotZero(t, record.ID)
	assert.Equal(t, "meta-key", record.Key)
	assert.Equal(t, "yaml", record.Format)
	assert.Equal(t, "game1", record.GameID)
	assert.Equal(t, "prod", record.Env)

	// Create with stale base version (conflict with existing version 1)
	payload2 := payload
	payload2.BaseVersion = 999
	_, err = model.CreateWithMeta(ctx, payload2, "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config has been updated by another user")

	// Create another version (should increment)
	record2, err := model.CreateWithMeta(ctx, ConfigVersionPayload{
		Key:     "meta-key",
		Content: "meta-value-v2",
	}, "admin")
	require.NoError(t, err)
	assert.Equal(t, 2, record2.Version)

	// Create with stale base version
	_, err = model.CreateWithMeta(ctx, ConfigVersionPayload{
		Key:         "meta-key",
		Content:     "meta-value-v3-stale",
		BaseVersion: 1, // stale, we're at version 2 now
	}, "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config has been updated by another user")

	// Create with correct base version
	record3, err := model.CreateWithMeta(ctx, ConfigVersionPayload{
		Key:         "meta-key",
		Content:     "meta-value-v3",
		BaseVersion: 2,
	}, "admin")
	require.NoError(t, err)
	assert.Equal(t, 3, record3.Version)

	// Create with stale base version
	_, err = model.CreateWithMeta(ctx, ConfigVersionPayload{
		Key:         "meta-key",
		Content:     "meta-value-v4",
		BaseVersion: 2, // stale
	}, "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config has been updated by another user")
}

func TestConfigVersionModel_ListLatest(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	ctx := context.Background()

	// Create configs with different attributes
	configs := []ConfigVersionPayload{
		{Key: "config1", Content: "value1", GameID: "game1", Env: "prod", Format: "json"},
		{Key: "config2", Content: "value2", GameID: "game1", Env: "test", Format: "yaml"},
		{Key: "config3", Content: "value3", GameID: "game2", Env: "prod", Format: "json"},
	}
	for _, cfg := range configs {
		_, err := model.CreateWithMeta(ctx, cfg, "admin")
		require.NoError(t, err)
	}

	// List all latest
	records, err := model.ListLatest(ctx, ConfigListOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 3)

	// Filter by game ID
	records, err = model.ListLatest(ctx, ConfigListOptions{GameID: "game1"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 2)

	// Filter by env
	records, err = model.ListLatest(ctx, ConfigListOptions{Env: "prod"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 2)

	// Filter by format
	records, err = model.ListLatest(ctx, ConfigListOptions{Format: "json"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 2)

	// Filter by ID like
	records, err = model.ListLatest(ctx, ConfigListOptions{IDLike: "config"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 3)
}

func TestConfigVersionModel_FindLatest(t *testing.T) {
	db := setupConfigVersionTestDB(t)
	model := NewConfigVersionModel(db)
	ctx := context.Background()

	// Create multiple versions
	for i := 1; i <= 3; i++ {
		_, err := model.Create(ctx, "latest-test", "value-v"+string(rune('0'+i)), "admin")
		require.NoError(t, err)
	}

	// Find latest
	record, err := model.FindLatest(ctx, "latest-test")
	require.NoError(t, err)
	assert.Equal(t, 3, record.Version)

	// Non-existent key
	_, err = model.FindLatest(ctx, "non-existent")
	assert.Error(t, err)

	// Empty key
	_, err = model.FindLatest(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config key required")
}

// ===== TermDictionaryModel Tests =====
