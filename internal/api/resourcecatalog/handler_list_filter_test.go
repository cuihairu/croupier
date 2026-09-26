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

// 回归（OPEN-ISSUES #5）：前端发的是小写 query key（category=/query=），
// ListRequest 此前无 form tag，gin 按字段名精确匹配导致过滤参数从未绑定、
// 列表恒返回全量。用两个不同分类的资源断言过滤真的收窄（旧实现下 total==2）。
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

	// sanity: 无过滤时全量两条
	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var all ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &all))
	assert.Equal(t, 2, all.Total)

	// category 过滤（旧实现：category 从不绑定，total 恒为 2）
	rec = doCatalogRequest(router, http.MethodGet, "/api/resource-catalog?category=mail", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var byCategory ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &byCategory))
	require.Len(t, byCategory.Items, 1)
	assert.Equal(t, 1, byCategory.Total)
	assert.Equal(t, "mail.template", byCategory.Items[0].ResourceKey)

	// query 过滤（同病类：旧实现恒全量）
	rec = doCatalogRequest(router, http.MethodGet, "/api/resource-catalog?query=player", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var byQuery ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &byQuery))
	assert.Equal(t, 1, byQuery.Total)
	assert.Equal(t, "player", byQuery.Items[0].ResourceKey)
}
