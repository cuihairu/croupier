package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RuntimeSources 守卫与过滤补充：无权限拒绝、缺 scope 拒绝、非
// provider: 前缀会话不展示、RegistryStore 未注入时空列表。
func TestRuntimeSourcesServiceGuardsAndFilters(t *testing.T) {
	// 无权限
	noPerm, ctx := setupOpenAPITestServiceWithPermissions(t)
	_, err := noPerm.RuntimeSources(ctx, &RuntimeSourcesListRequest{})
	require.Error(t, err)

	// 缺 scope
	service, scopedCtx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	_, err = service.RuntimeSources(context.Background(), &RuntimeSourcesListRequest{})
	require.Error(t, err)

	// 非 provider: 前缀的 Provider 会话（普通本地注册）不进运行时导入清单
	store := service.svcCtx.RegistryStore
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "local-registry-agent",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: now,
		Providers: []registry.ProviderSession{
			{ProviderID: "custom:thing", FunctionIDs: []string{"fn.a"}, LastSeenUnix: now.Unix()},
			{ProviderID: "provider:shown", FunctionIDs: []string{"fn.b"}, LastSeenUnix: now.Unix()},
		},
	}))
	resp, err := service.RuntimeSources(scopedCtx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "provider:shown", resp.Items[0].ProviderID)

	// RegistryStore 未注入：空列表不报错
	service.svcCtx.RegistryStore = nil
	resp, err = service.RuntimeSources(scopedCtx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}

// RuntimeSources handler：成功 200；缺 scope 经统一错误对象 400。
func TestRuntimeSourcesHandler(t *testing.T) {
	service, scopedCtx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	handler := NewHandler(service)
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/runtime-sources", nil).WithContext(scopedCtx)
	handler.RuntimeSources(c)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"total":0`)

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	// 权限检查先于 scope 检查：带授权用户但剥离 game scope
	userNoScope := context.WithValue(context.Background(), "username", "openapi_tester")
	c2.Request = httptest.NewRequest(http.MethodGet, "/runtime-sources", nil).WithContext(userNoScope)
	handler.RuntimeSources(c2)
	assert.Equal(t, http.StatusBadRequest, rec2.Code, rec2.Body.String())
}
