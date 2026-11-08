package ops

// capture_gates.go handler 面补测：五个 gin handler 的往返（GET 空形态、PUT
// 发布递增、validate 预检、回滚、版本历史）与坏载荷 400 面。service 语义
// 分支由 capture_gates_test.go 覆盖。

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCaptureGatesHandlersRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, _ := newCaptureGatesService(t)
	h := NewHandler(s)

	// GET 未发布 → 200 空形态
	c, w := withScopeRequest(t, http.MethodGet, "/ops/capture/gates", "")
	h.CaptureGatesGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got CaptureGatesGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "capture.gates", got.Key)
	require.Equal(t, 0, got.Version)

	// PUT 坏 JSON → 400
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates", "{bad")
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// PUT 发布 → 200 + 版本号
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates",
		`{"content":"{\"reconcile\":{\"rules\":[{\"name\":\"r\",\"itemTable\":\"game.items\",\"ledgerTable\":\"game.ledger\",\"itemKey\":\"1\",\"ledgerKey\":\"0\"}]}}","message":"first"}`)
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusOK, w.Code)
	var put CaptureGatesPutResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &put))
	require.Equal(t, 1, put.Version)

	// Validate 坏 JSON → 400；合法 → 200
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/validate", "{bad")
	h.CaptureGatesValidate(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/validate",
		`{"content":"{\"reconcile\":{\"rules\":[{\"name\":\"r\",\"itemTable\":\"game.items\",\"ledgerTable\":\"game.ledger\",\"itemKey\":\"1\",\"ledgerKey\":\"0\"}]}}"}`)
	h.CaptureGatesValidate(c)
	require.Equal(t, http.StatusOK, w.Code)
	var val CaptureGatesValidateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &val))
	require.True(t, val.Valid)
	require.Equal(t, 1, val.Reconcile)

	// Rollback 坏 JSON → 400；合法（回滚到 v1）→ 200 新版本
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/rollback", "{bad")
	h.CaptureGatesRollback(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/rollback", `{"version":1}`)
	h.CaptureGatesRollback(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Versions → 200 两条历史
	c, w = withScopeRequest(t, http.MethodGet, "/ops/capture/gates/versions", "")
	h.CaptureGatesVersionsGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var versions []CaptureGatesVersionItem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &versions))
	require.Len(t, versions, 2)
}
