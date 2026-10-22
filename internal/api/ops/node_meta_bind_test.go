// 线上事故回归（2026-10-02 实测 deploy 444d7f0 恒 500）：GET /ops/nodes/:id/meta
// 两病灶——① OpsNodeMetaRequest 仅 uri tag，BindQueryCompat 不绑 uri，
// NodeID 恒空（NodeDetail 有 c.Param 兜底、Meta 漏配）；② 未命中裸
// errors.New 被 response.Error 映射 500 internal_error（契约应为 404）。
// 本文件锁定：路径参数兜底生效 + 未命中 404 统一错误对象。
package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeMetaHandlerRouteBindsPathParam(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/nodes/:nodeId/meta", h.NodeMeta)

	// 命中：路径参数进兜底 → 200 + labels（修复前 NodeID 恒空 → 恒
	// "node not found" → 500）
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/a1/meta", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Labels map[string]string `json:"labels"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "a1-host", out.Labels["hostname"])

	// 未命中：404 统一错误对象（修复前 500 internal_error）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/nope/meta", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}

// query 显式给 nodeId 时仍生效（BindQueryCompat 先绑，c.Param 只兜空值）；
// 两者同给时以 query 为准——与 OpsNodeDetail 的兜底语义一致。
func TestNodeMetaQueryStillWinsOverPath(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	seedDetailAgent(store, "b2", "g1", "2.0.0")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/nodes/:nodeId/meta", h.NodeMeta)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/b2/meta?nodeId=a1", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Labels map[string]string `json:"labels"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "a1-host", out.Labels["hostname"], "query nodeId=a1 应优先于路径 b2")
}

// service 层未命中错误码直测（与 opsNodeDetail 的 404 契约对齐）。
func TestOpsNodeMetaServiceNotFoundIs404(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	s := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := s.OpsNodeMeta(t.Context(), &OpsNodeMetaRequest{NodeID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "a1-host", resp.Labels["hostname"])

	_, err = s.OpsNodeMeta(t.Context(), &OpsNodeMetaRequest{NodeID: "nope"})
	require.Error(t, err)
	var notFound *errorx.CodeError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, http.StatusNotFound, notFound.Code)
}
