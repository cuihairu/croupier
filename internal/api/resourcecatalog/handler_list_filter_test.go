package resourcecatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression (OPEN-ISSUES #5): the frontend sends lowercase query keys
// (category=/query=). ListRequest had no form tags before, so gin's exact field-name
// match never bound the filter params and the list always returned everything. Two
// resources in different categories prove the filter really narrows the result (the
// old implementation had total==2).
func TestHandler_List_CategoryAndQueryFiltersBindLowercaseParams(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	ctx := context.Background()
	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	}))
	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "mail.template",
		Labels: map[string]interface{}{"zh-CN": "邮件模板"},
	}))

	// sanity: without a filter both rows come back
	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var all ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &all))
	assert.Equal(t, 2, all.Total)

	// category filter (old implementation: category never bound, total always 2)
	rec = doCatalogRequest(router, http.MethodGet, "/api/resource-catalog?category=mail", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var byCategory ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &byCategory))
	require.Len(t, byCategory.Items, 1)
	assert.Equal(t, 1, byCategory.Total)
	assert.Equal(t, "mail.template", byCategory.Items[0].ResourceKey)

	// query filter (same disease: the old implementation always returned everything)
	rec = doCatalogRequest(router, http.MethodGet, "/api/resource-catalog?query=player", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var byQuery ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &byQuery))
	assert.Equal(t, 1, byQuery.Total)
	assert.Equal(t, "player", byQuery.Items[0].ResourceKey)
}
