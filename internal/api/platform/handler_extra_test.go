package platform

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newPlatformRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := NewService(&svc.ServiceContext{})
	r := gin.New()
	h := NewHandler(service)
	r.POST("/api/v1/platform/call", h.Call)
	r.GET("/api/v1/platform/platforms", h.ListPlatforms)
	r.GET("/api/v1/platform/platforms/:platform/methods", h.ListMethods)
	return r, service
}

// Handler.Call 参数绑定失败 → 400
func TestHandlerCallBindError(t *testing.T) {
	r, _ := newPlatformRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/call", bytes.NewReader([]byte("{invalid")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Call 命中 Dispatcher 但 Invoke 失败 → CodeError(500) 经统一错误响应
func TestHandlerCallDispatcherError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := reg.NewStore()
	// 注册了一个目标函数但没有任何可用 agent 会话 → Invoke 失败
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:1",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.list_apps": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store, Dispatcher: dispatch.NewDispatcher(store)})
	r := gin.New()
	h := NewHandler(service)
	r.POST("/api/v1/platform/call", h.Call)

	body := []byte(`{"platform":"onepanel","method":"list_apps","request":"{}"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/call", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ListPlatforms 成功 → writeListPlatformsResponse 成功分支
func TestHandlerListPlatformsSuccess(t *testing.T) {
	r, _ := newPlatformRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/platforms", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "{}") // 空平台列表（omitempty）
}

// ListMethods 未知平台 → 404 CodeError 分支
func TestHandlerListMethodsNotFound(t *testing.T) {
	r, _ := newPlatformRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/platforms/nope/methods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ListMethods 成功 → writeListMethodsResponse 成功分支
func TestHandlerListMethodsSuccess(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.list_apps": {Enabled: true},
		},
	})
	gin.SetMode(gin.TestMode)
	service := NewService(&svc.ServiceContext{RegistryStore: store})
	r := gin.New()
	h := NewHandler(service)
	r.GET("/api/v1/platform/platforms/:platform/methods", h.ListMethods)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/platforms/onepanel/methods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "list_apps")
}

// Service.Call：Dispatcher 存在但无可用会话 → 500 且 source=extension
func TestServiceCallDispatcherInvokeError(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:1",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.list_apps": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store, Dispatcher: dispatch.NewDispatcher(store)})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "onepanel",
		Method:   "list_apps",
	})
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.Code)
	assert.Equal(t, "extension", resp.Source)
}

// discoverExternalPlatforms：svcCtx 为 nil → 空结果
func TestDiscoverExternalPlatformsNilSvcCtx(t *testing.T) {
	service := NewService(nil)
	got := service.discoverExternalPlatforms(context.Background())
	assert.Empty(t, got)
}
