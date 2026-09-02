// 覆盖目标：resourcecatalog handler 的 service 错误路径（表删除）与
// 无 scope 请求、版本/冲突列表的存储错误分支。
package resourcecatalog

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_List_StoreError(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Migrator().DropTable("resource_capabilities"))
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestHandler_ListSemanticVersions_StoreError(t *testing.T) {
	db := setupTestDB(t)
	seedCatalogPlayer(t, db)
	require.NoError(t, db.Migrator().DropTable("capability_semantics"))
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player/semantics/versions", "")
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_ListConflicts_StoreError(t *testing.T) {
	db := setupTestDB(t)
	seedCatalogPlayer(t, db)
	require.NoError(t, db.Migrator().DropTable("capability_semantics"))
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player/conflicts", "")
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Detail_UnknownResource(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/ghost", "")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestHandler_ListSemanticVersions_InvalidLimitQuery(t *testing.T) {
	db := setupTestDB(t)
	seedCatalogPlayer(t, db)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player/semantics/versions?limit=abc", "")
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}
