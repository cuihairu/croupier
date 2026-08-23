package schema

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
	r.GET("/schemas", handler.List)
	r.POST("/schemas", handler.Create)
	r.GET("/schemas/:id", handler.Get)
	r.PUT("/schemas/:id", handler.Update)
	r.DELETE("/schemas/:id", handler.Delete)
	r.POST("/schemas/:id/validate", handler.Validate)
	r.POST("/schemas/raw-validate", handler.RawValidate)
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

func TestHandler_List_Returns200WithItemsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/schemas", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]interface{})
	require.True(t, ok, "expected 'items' array, body=%s", rec.Body.String())
	assert.Empty(t, items)
	total, ok := resp["total"].(float64)
	require.True(t, ok, "expected 'total' number, body=%s", rec.Body.String())
	assert.Equal(t, float64(0), total)
}

func TestHandler_CreateGet_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/schemas",
		`{"name":"Player","schema":`+validSchemaJSON+`}`)
	assertStatus(t, rec, http.StatusOK)

	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	getRec := doReq(t, router, http.MethodGet, "/schemas/"+id, "")
	assertStatus(t, getRec, http.StatusOK)

	var fetched map[string]interface{}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &fetched))
	assert.Equal(t, id, fetched["id"])
	assert.Equal(t, "Player", fetched["name"])
}

func TestHandler_Create_InvalidJSON_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/schemas", `{not-json`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_Create_InvalidSchemaDefinition_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/schemas",
		`{"name":"Bad","schema":{"required":"name"}}`)
	assert.NotEqual(t, http.StatusOK, rec.Code, "invalid schema definition must be rejected")
	assertErrorShape(t, rec)
}

func TestHandler_Get_NotFound_Returns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/schemas/missing", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandler_Delete_NotFound_Returns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodDelete, "/schemas/missing", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandler_Validate_BadJSONBody_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/schemas/some-id/validate", `{broken`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_Validate_AgainstSchema(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	// Create a schema first.
	rec := doReq(t, router, http.MethodPost, "/schemas",
		`{"name":"V","schema":`+validSchemaJSON+`}`)
	assertStatus(t, rec, http.StatusOK)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created["id"].(string)

	// Validate a valid payload.
	valRec := doReq(t, router, http.MethodPost, "/schemas/"+id+"/validate",
		`{"data":{"name":"alice"}}`)
	assertStatus(t, valRec, http.StatusOK)
	var valResp map[string]interface{}
	require.NoError(t, json.Unmarshal(valRec.Body.Bytes(), &valResp))
	assert.True(t, valResp["valid"].(bool))
}
