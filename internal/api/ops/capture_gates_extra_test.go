package ops

// Extra handler coverage for capture_gates.go: round trips across the five gin
// handlers (empty GET shape, PUT publish increments, validate precheck,
// rollback, version history) plus the bad-payload 400 surface. Service
// semantics branches are covered by capture_gates_test.go.

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

	// GET before any publish -> 200 with the empty shape
	c, w := withScopeRequest(t, http.MethodGet, "/ops/capture/gates", "")
	h.CaptureGatesGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got CaptureGatesGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "capture.gates", got.Key)
	require.Equal(t, 0, got.Version)

	// PUT with bad JSON -> 400
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates", "{bad")
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// PUT publish -> 200 + version number
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates",
		`{"content":"{\"reconcile\":{\"rules\":[{\"name\":\"r\",\"itemTable\":\"game.items\",\"ledgerTable\":\"game.ledger\",\"itemKey\":\"1\",\"ledgerKey\":\"0\"}]}}","message":"first"}`)
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusOK, w.Code)
	var put CaptureGatesPutResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &put))
	require.Equal(t, 1, put.Version)

	// Validate bad JSON -> 400; valid -> 200
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

	// Rollback bad JSON -> 400; valid (roll back to v1) -> 200 with a new version
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/rollback", "{bad")
	h.CaptureGatesRollback(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/rollback", `{"version":1}`)
	h.CaptureGatesRollback(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Versions -> 200 with two history entries
	c, w = withScopeRequest(t, http.MethodGet, "/ops/capture/gates/versions", "")
	h.CaptureGatesVersionsGet(c)
	require.Equal(t, http.StatusOK, w.Code)
	var versions []CaptureGatesVersionItem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &versions))
	require.Len(t, versions, 2)
}
