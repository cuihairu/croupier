package sitesettings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()
	settings.ResetForTest()
	db := newTestDB(t)
	require.NoError(t, model.AutoMigrate(db))
	store := model.NewPlatformSettingModel(db)
	layered := settings.InitLayered(t.Context(), &settings.ConfigInput{}, store)
	h := NewHandler(layered, store)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	RegisterPublic(api.Group("/public"), h)
	h.RegisterAdmin(api.Group("/"))
	return r
}

func TestPutKey_BoolAndURLValidation(t *testing.T) {
	r := setupRouter(t)

	// features.* 只接受 JSON bool。
	req := httptest.NewRequest(http.MethodPut, "/api/v1/site/features.ops", body(`{"value":"yes"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	req = httptest.NewRequest(http.MethodPut, "/api/v1/site/features.ops", body(`{"value":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// obs.* 校验 URL 形态。
	req = httptest.NewRequest(http.MethodPut, "/api/v1/site/obs.jaegerUrl", body(`{"value":"not-a-url"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	req = httptest.NewRequest(http.MethodPut, "/api/v1/site/obs.jaegerUrl", body(`{"value":"http://jaeger:16686"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// bootstrap 类 key 禁止写入。
	req = httptest.NewRequest(http.MethodPut, "/api/v1/site/database.dataSource", body(`{"value":"mysql://…"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRouteOrder_FeaturesReadEndpoint(t *testing.T) {
	r := setupRouter(t)

	// GET /site/features 命中读端点而非 :key（gin 静态路由优先）。
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/site/features", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"domains"`)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/site/observability", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	// 契约键必须为 lowerCamelCase（前端 services/api/sites.ts 依赖）；
	// 注意不能只断言 "alertmanagerUrl" 子串——sources 的 key 包含它，
	// 会掩盖顶层字段缺 json tag 时输出 PascalCase 的回归。
	body := rec.Body.String()
	assert.Contains(t, body, `"alertmanagerUrl"`)
	assert.Contains(t, body, `"grafanaExploreUrl"`)
	assert.Contains(t, body, `"jaegerUrl"`)
	assert.Contains(t, body, `"sources"`)
	assert.NotContains(t, body, `"AlertmanagerURL"`)
	assert.NotContains(t, body, `"GrafanaExploreURL"`)
	assert.NotContains(t, body, `"JaegerURL"`)
	assert.NotContains(t, body, `"Sources"`)
}
