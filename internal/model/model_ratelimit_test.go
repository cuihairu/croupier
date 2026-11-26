package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewRateLimitModel(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	assert.NotNil(t, model)
}

func TestRateLimitModel_Upsert(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	ctx := context.Background()

	rl := &RateLimit{
		RateLimitID: "limit-001",
		Name:        "API Rate Limit",
		Resource:    "/api/v1",
		Limit:       100,
		Window:      60,
		Action:      "throttle",
		Status:      1,
	}

	err := model.Upsert(ctx, rl)
	require.NoError(t, err)
	assert.NotZero(t, rl.ID)

	// Update via upsert
	rl.Limit = 200
	err = model.Upsert(ctx, rl)
	require.NoError(t, err)
}

func TestRateLimitModel_FindByID(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	ctx := context.Background()

	rl := &RateLimit{
		RateLimitID: "findbyid-001",
		Name:        "Find Test",
		Resource:    "/api/test",
		Limit:       50,
		Window:      60,
		Action:      "block",
		Status:      1,
	}
	err := model.Upsert(ctx, rl)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindByID(ctx, rl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Find Test", found.Name)

	// Find non-existent
	_, err = model.FindByID(ctx, 99999)
	assert.Error(t, err)
}

func TestRateLimitModel_FindByKey(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	ctx := context.Background()

	rl := &RateLimit{
		RateLimitID: "findbykey-001",
		Name:        "Find By Key Test",
		Resource:    "/api/key",
		Limit:       30,
		Window:      60,
		Action:      "throttle",
		Status:      1,
	}
	err := model.Upsert(ctx, rl)
	require.NoError(t, err)

	// Find by key
	found, err := model.FindByKey(ctx, "findbykey-001")
	require.NoError(t, err)
	assert.Equal(t, "Find By Key Test", found.Name)

	// Find non-existent key
	_, err = model.FindByKey(ctx, "non-existent")
	assert.Error(t, err)
}

func TestRateLimitModel_Delete(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	ctx := context.Background()

	rl := &RateLimit{
		RateLimitID: "delete-001",
		Name:        "Delete Test",
		Resource:    "/api/delete",
		Limit:       10,
		Window:      60,
		Action:      "block",
		Status:      1,
	}
	err := model.Upsert(ctx, rl)
	require.NoError(t, err)

	// Delete
	err = model.Delete(ctx, rl.ID)
	require.NoError(t, err)

	// Verify
	_, err = model.FindByID(ctx, rl.ID)
	assert.Error(t, err)
}

func TestRateLimitModel_DeleteByKey(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	ctx := context.Background()

	rl := &RateLimit{
		RateLimitID: "deletebykey-001",
		Name:        "Delete By Key Test",
		Resource:    "/api/deletekey",
		Limit:       20,
		Window:      60,
		Action:      "block",
		Status:      1,
	}
	err := model.Upsert(ctx, rl)
	require.NoError(t, err)

	// Delete by key
	err = model.DeleteByKey(ctx, "deletebykey-001")
	require.NoError(t, err)

	// Verify
	_, err = model.FindByKey(ctx, "deletebykey-001")
	assert.Error(t, err)

	// Delete non-existent key
	err = model.DeleteByKey(ctx, "non-existent")
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestRateLimitModel_List(t *testing.T) {
	db := setupRateLimitTestDB(t)
	model := NewRateLimitModel(db)
	ctx := context.Background()

	limits := []*RateLimit{
		{RateLimitID: "list-001", Name: "Limit 1", Resource: "/api/v1", Limit: 100, Window: 60, Action: "throttle", Status: 1},
		{RateLimitID: "list-002", Name: "Limit 2", Resource: "/api/v2", Limit: 50, Window: 60, Action: "block", Status: 1},
		{RateLimitID: "list-003", Name: "Limit 3", Resource: "/api/v1", Limit: 200, Window: 120, Action: "throttle", Status: 0},
	}
	for _, rl := range limits {
		err := model.Upsert(ctx, rl)
		require.NoError(t, err)
	}

	// List all
	all, err := model.List(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 3)

	// Filter by resource
	filtered, err := model.List(ctx, "/api/v1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(filtered), 2)
}

// ===== BackupModel Tests =====
