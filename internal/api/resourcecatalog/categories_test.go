package resourcecatalog

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression for the Categories aggregation endpoint (OPEN-ISSUES #14): the filter
// dropdown must receive the full set of distinct categories; it must not be derived
// by the frontend from the (already category-filtered) list, otherwise picking a
// single item collapses the options down to one.
func TestService_Categories(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	capModel := model.NewResourceCapabilityModel(db)
	seed := []model.ResourceCapability{
		// No review category: derived from the resource key prefix -> player / mail
		{GameID: "demo-game", Env: "development", ResourceKey: "player"},
		{GameID: "demo-game", Env: "development", ResourceKey: "player.item"},
		{GameID: "demo-game", Env: "development", ResourceKey: "mail.template"},
		// The review category takes precedence over prefix derivation
		{GameID: "demo-game", Env: "development", ResourceKey: "legacy.thing", CategoryKey: "archived"},
		// Rows from other scopes must not be counted
		{GameID: "other-game", Env: "development", ResourceKey: "order"},
	}
	for i := range seed {
		require.NoError(t, capModel.UpsertCapability(ctx, &seed[i]))
	}

	resp, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)

	require.Len(t, resp.Items, 3)
	assert.Equal(t, []CategoryOption{
		{CategoryKey: "archived", Count: 1},
		{CategoryKey: "mail", Count: 1},
		{CategoryKey: "player", Count: 2},
	}, resp.Items)

	// Scope isolation: the other scope has only the single "order" item
	other, err := service.Categories(ctx, "other-game", "development")
	require.NoError(t, err)
	require.Len(t, other.Items, 1)
	assert.Equal(t, "order", other.Items[0].CategoryKey)
}

// Cache behavior: rows added while the entry is valid are not re-queried (the stale
// set is returned), and a re-query after invalidate picks up the new categories.
// This also pins down the "write path invalidates proactively" semantics -- after
// UpdateSemantics changes a category, the dropdown no longer serves a 30s stale
// option list.
func TestService_Categories_CacheAndInvalidation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	capModel := model.NewResourceCapabilityModel(db)
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "demo-game", Env: "development", ResourceKey: "player",
	}))

	first, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)
	require.Len(t, first.Items, 1)

	// Rows added inside the cache validity window stay invisible (TTL not elapsed)
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "demo-game", Env: "development", ResourceKey: "mail.template",
	}))
	cached, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, cached.Items, 1, "cache hit should serve the stale set until invalidated")

	// Re-query after invalidation -> the full set
	service.invalidateCategoryCache("demo-game", "development")
	fresh, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, fresh.Items, 2)
}
