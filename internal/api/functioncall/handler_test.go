package functioncall

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(NewService(setupSvcCtx(t)))
}

func newRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/calls", handler.List)
	r.GET("/calls/stats", handler.Stats)
	r.GET("/calls/:id", handler.Detail)
	r.POST("/calls/:id/cancel", handler.Cancel)
	r.POST("/calls/:id/rerun", handler.Rerun)
	return r
}

func doReq(t *testing.T, router *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandler_List_Returns200WithCallsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/calls", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	calls, ok := resp["calls"].([]interface{})
	require.True(t, ok, "expected 'calls' array, body=%s", rec.Body.String())
	assert.Empty(t, calls)
	total, ok := resp["total"].(float64)
	require.True(t, ok, "expected 'total' number, body=%s", rec.Body.String())
	assert.Equal(t, float64(0), total)
}

func TestHandler_Stats_Returns200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/calls/stats", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, ok := resp["total"].(float64)
	require.True(t, ok, "expected 'total' number, body=%s", rec.Body.String())
}

func TestHandler_Detail_UnknownID_ReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/calls/unknown", "")
	// Unknown id propagates a record-not-found error from the task store.
	assert.NotEqual(t, http.StatusOK, rec.Code, "unknown call id must not succeed")
	assertErrorShape(t, rec)
}

func TestHandler_Detail_Found(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "h-found", "player.ban", "running")
	handler := NewHandler(NewService(svcCtx))
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/calls/h-found", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "h-found", resp["id"])
	assert.Equal(t, "running", resp["status"])
}

func TestHandler_Cancel_OnExistingTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "h-cancel", "player.ban", "running")
	handler := NewHandler(NewService(svcCtx))
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/calls/h-cancel/cancel", "")
	assertStatus(t, rec, http.StatusOK)
}

func TestHandler_Rerun_AlwaysReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/calls/any/rerun", `{}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

// Direct handler-call binding validation: DetailRequest id is binding:"required".
func TestHandler_Detail_EmptyID_BindValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/calls/", "")
	// No params set -> ShouldBindUri required fails -> 400, no panic.
	handler.Detail(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, "empty id must not succeed")
	if rec.Code != http.StatusOK {
		assertErrorShape(t, rec)
	}
}
