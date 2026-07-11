package workspace

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

// newRouter registers non-conflicting routes for workspace handler testing.
func newRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/configs", handler.ListConfigs)
	r.GET("/published", handler.ListPublished)
	r.GET("/configs/:objectKey", handler.GetConfig)
	r.PUT("/configs/:objectKey", handler.SaveConfig)
	r.DELETE("/configs/:objectKey", handler.DeleteConfig)
	r.POST("/configs/:objectKey/publish", handler.Publish)
	r.POST("/configs/:objectKey/unpublish", handler.Unpublish)
	r.GET("/configs/:objectKey/versions", handler.Versions)
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

func TestHandler_ListConfigs_Returns200WithItemsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/configs", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]interface{})
	require.True(t, ok, "expected 'items' array, body=%s", rec.Body.String())
	assert.Empty(t, items)
}

func TestHandler_ListPublished_Returns200WithItemsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/published", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, ok := resp["items"].([]interface{})
	require.True(t, ok, "expected 'items' array, body=%s", rec.Body.String())
}

func TestHandler_SaveAndGetConfig_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPut, "/configs/demo",
		`{"title":"Demo","layout":{"type":"tabs"},"menuOrder":1}`)
	assertStatus(t, rec, http.StatusOK)

	rec2 := doReq(t, router, http.MethodGet, "/configs/demo", "")
	assertStatus(t, rec2, http.StatusOK)

	var fetched map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &fetched))
	cfg, ok := fetched["workspaceConfig"].(map[string]interface{})
	require.True(t, ok, "expected 'workspaceConfig', body=%s", rec2.Body.String())
	assert.Equal(t, "demo", cfg["objectKey"])
	assert.Equal(t, "Demo", cfg["title"])
}

func TestHandler_SaveConfig_InvalidJSON_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPut, "/configs/bad", `{invalid`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_GetConfig_NotFound_Returns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/configs/ghost", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandler_DeleteConfig_NotFound_Returns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodDelete, "/configs/ghost", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandler_DeleteConfig_AfterSave(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPut, "/configs/tmp",
		`{"title":"Tmp","layout":{"type":"form"}}`)
	assertStatus(t, rec, http.StatusOK)

	delRec := doReq(t, router, http.MethodDelete, "/configs/tmp", "")
	assertStatus(t, delRec, http.StatusOK)
}

func TestHandler_Publish_AfterSave(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPut, "/configs/pub",
		`{"title":"Pub","layout":{"type":"tabs"}}`)
	assertStatus(t, rec, http.StatusOK)

	pubRec := doReq(t, router, http.MethodPost, "/configs/pub/publish", `{"publishedBy":"tester"}`)
	assertStatus(t, pubRec, http.StatusOK)
	var pub map[string]interface{}
	require.NoError(t, json.Unmarshal(pubRec.Body.Bytes(), &pub))
	assert.True(t, pub["published"].(bool))
}

func TestHandler_Versions_AfterSave(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPut, "/configs/ver",
		`{"title":"Ver","layout":{"type":"tabs"}}`)
	assertStatus(t, rec, http.StatusOK)

	verRec := doReq(t, router, http.MethodGet, "/configs/ver/versions", "")
	assertStatus(t, verRec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(verRec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]interface{})
	require.True(t, ok, "expected 'items' array, body=%s", verRec.Body.String())
	assert.NotEmpty(t, items)
}

// Direct handler-call binding validation: empty objectKey path param must be
// rejected (binding:"required") rather than reaching the service.
func TestHandler_GetConfig_EmptyObjectKey_BindValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/configs/", "")
	// No params set -> ShouldBindUri required fails -> 400.
	handler.GetConfig(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, "empty objectKey must not succeed")
	if rec.Code != http.StatusOK {
		assertErrorShape(t, rec)
	}
}
