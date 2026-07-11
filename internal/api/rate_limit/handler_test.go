package rate_limit

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
	r.GET("/rate-limits", handler.List)
	r.GET("/rate-limits/:id", handler.Get)
	r.POST("/rate-limits", handler.Upsert)
	r.DELETE("/rate-limits/:id", handler.Delete)
	r.POST("/rate-limits/preview", handler.Preview)
	return r
}

func doJSON(t *testing.T, router *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandler_List_Returns200WithItemsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodGet, "/rate-limits", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]interface{})
	require.True(t, ok, "expected 'items' to be a JSON array, body=%s", rec.Body.String())
	assert.Empty(t, items)
}

func TestHandler_UpsertGet_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodPost, "/rate-limits",
		`{"name":"cap","resource":"function","limit":5,"window":30,"action":"reject","rules":{"env":"prod"}}`)
	assertStatus(t, rec, http.StatusOK)

	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	rec2 := doJSON(t, router, http.MethodGet, "/rate-limits/"+id, "")
	assertStatus(t, rec2, http.StatusOK)

	var fetched map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &fetched))
	assert.Equal(t, id, fetched["id"])
	assert.Equal(t, "cap", fetched["name"])
	assert.EqualValues(t, 5, fetched["limit"])
}

func TestHandler_Upsert_InvalidBody_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{invalid`},
		{"missing name", `{"resource":"function","limit":1,"window":1,"action":"reject"}`},
		{"bad action", `{"name":"x","resource":"function","limit":1,"window":1,"action":"drop"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(t, router, http.MethodPost, "/rate-limits", tt.body)
			assert.NotEqual(t, http.StatusOK, rec.Code, "should reject: %s", tt.name)
			if rec.Code != http.StatusOK {
				assertErrorShape(t, rec)
			}
		})
	}
}

func TestHandler_Get_UnknownID_ReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodGet, "/rate-limits/missing", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)
	assertErrorShape(t, rec)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Delete_UnknownID_Returns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodDelete, "/rate-limits/missing", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandler_Delete_AfterUpsert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodPost, "/rate-limits",
		`{"name":"gone","resource":"api","limit":1,"window":1,"action":"throttle"}`)
	assertStatus(t, rec, http.StatusOK)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created["id"].(string)

	delRec := doJSON(t, router, http.MethodDelete, "/rate-limits/"+id, "")
	assertStatus(t, delRec, http.StatusOK)
}

func TestHandler_Preview_Returns200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodPost, "/rate-limits/preview",
		`{"rules":{"game_id":"demo","env":"prod"}}`)
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp, "matches")
	assert.Contains(t, resp, "impact")
}

func TestHandler_Preview_EmptyRules_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodPost, "/rate-limits/preview", `{}`)
	assert.NotEqual(t, http.StatusOK, rec.Code)
	assertErrorShape(t, rec)
}

// Direct handler-call tests for binding validation (no router).

func TestHandler_Get_BindValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/rate-limits/abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	handler.Get(ctx)

	// Unknown id should still produce an error response (not 200 and no panic).
	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-200 for unknown id, got 200 body=%s", rec.Body.String())
	}
}
