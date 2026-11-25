package resourcecatalog

// Categories handler coverage (when OPEN-ISSUES #14 landed, only service-layer
// cases were added and the HTTP handler was left entirely untested): static
// segment route reachability, aggregated response body shape, scope isolation,
// and storage error paths.
//
// Semantic anchor: the filter dropdown must obtain the full set of distinct
// categories (it must not be derived by the frontend from an already-filtered
// list), so the response is "key + count" rather than a key list; counts are
// aggregated per scope.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedCategoryCapability writes a resource capability row directly (an empty
// categoryKey falls back to key-prefix derivation).
func seedCategoryCapability(t *testing.T, db *gorm.DB, gameID, env, resourceKey, categoryKey string) {
	t.Helper()
	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(context.Background(), &model.ResourceCapability{
		GameID: gameID, Env: env, ResourceKey: resourceKey, CategoryKey: categoryKey,
	}))
}

func TestHandler_Categories_AggregatesScopedCounts(t *testing.T) {
	db := setupTestDB(t)
	// Prefix-derived categories: player (with sub-resources) / mail; an
	// explicit category key overrides the derivation
	seedCategoryCapability(t, db, "g1", "e1", "player", "")
	seedCategoryCapability(t, db, "g1", "e1", "player.item", "")
	seedCategoryCapability(t, db, "g1", "e1", "mail.template", "")
	seedCategoryCapability(t, db, "g1", "e1", "legacy.thing", "archived")
	// Other scopes must not bleed in
	seedCategoryCapability(t, db, "g2", "e1", "order", "")

	router := newResourceCatalogRouter(t, db)
	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/categories", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp CategoriesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []CategoryOption{
		{CategoryKey: "archived", Count: 1},
		{CategoryKey: "mail", Count: 1},
		{CategoryKey: "player", Count: 2},
	}, resp.Items, "counts aggregate per scope, keys sorted ascending")
}

func TestHandler_Categories_EmptyScopeReturnsEmptyArray(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/categories", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Dropdown consumers render an array: an empty collection must be []
	// rather than null
	assert.Contains(t, rec.Body.String(), `"items":[]`)

	var resp CategoriesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Items)
	assert.Empty(t, resp.Items)
}

func TestHandler_Categories_StoreError(t *testing.T) {
	db := setupTestDB(t)
	seedCategoryCapability(t, db, "g1", "e1", "player", "")
	require.NoError(t, db.Migrator().DropTable("resource_capabilities"))
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/categories", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}
