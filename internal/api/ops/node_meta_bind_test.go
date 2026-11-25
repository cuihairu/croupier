// Production-incident regression (observed live on 2026-10-02, deploy 444d7f0,
// constant 500): GET /ops/nodes/:id/meta had two defects — (1) OpsNodeMetaRequest
// carried only a uri tag and BindQueryCompat does not bind uri, so NodeID stayed
// empty (NodeDetail had the c.Param fallback while Meta missed it); (2) a miss
// raised a bare errors.New that response.Error mapped to 500 internal_error (the
// contract calls for 404). This file locks: the path-parameter fallback works +
// a miss returns the unified 404 error object.
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

	// hit: the path parameter feeds the fallback → 200 + labels (before the fix
	// NodeID stayed empty → always "node not found" → 500)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/a1/meta", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Labels map[string]string `json:"labels"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "a1-host", out.Labels["hostname"])

	// miss: the unified 404 error object (before the fix, 500 internal_error)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/nope/meta", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}

// when the query explicitly provides nodeId it still wins (BindQueryCompat
// binds first; c.Param only fills in empty values); when both are given the
// query takes precedence — consistent with OpsNodeDetail's fallback semantics.
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

// direct test of the service-layer miss error code (aligned with the opsNodeDetail 404 contract).
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
