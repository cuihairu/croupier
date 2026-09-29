package resourcecatalog

// Categories handler 覆盖（OPEN-ISSUES #14 落地时只补了 service 层用例，
// HTTP handler 整段无测试）：静态段路由可达性、聚合响应体形态、scope 隔离、
// 存储错误路径。
//
// 语义锚点：过滤下拉必须拿到全量 distinct 分类（不能由前端从已过滤列表推导），
// 因此响应是「键 + 计数」而非 key 列表；计数按 scope 聚合。

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

// seedCategoryCapability 直写资源能力行（categoryKey 为空即走 key 前缀推导）。
func seedCategoryCapability(t *testing.T, db *gorm.DB, gameID, env, resourceKey, categoryKey string) {
	t.Helper()
	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(context.Background(), &model.ResourceCapability{
		GameID: gameID, Env: env, ResourceKey: resourceKey, CategoryKey: categoryKey,
	}))
}

func TestHandler_Categories_AggregatesScopedCounts(t *testing.T) {
	db := setupTestDB(t)
	// 前缀推导分类：player（含子资源）/mail；审核分类覆盖推导
	seedCategoryCapability(t, db, "g1", "e1", "player", "")
	seedCategoryCapability(t, db, "g1", "e1", "player.item", "")
	seedCategoryCapability(t, db, "g1", "e1", "mail.template", "")
	seedCategoryCapability(t, db, "g1", "e1", "legacy.thing", "archived")
	// 其他 scope 不得混入
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
	// 下拉消费方按数组渲染：空集合必须是 [] 而非 null
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
