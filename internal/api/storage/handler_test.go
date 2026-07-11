package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(NewService(setupSvcCtxWithStore(t)))
}

func setupHandlerNoStore(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(NewService(setupSvcCtxNoStore(t)))
}

func newRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/storage/signed-url", handler.SignedUrl)
	r.GET("/storage/objects", handler.ListObjects)
	r.DELETE("/storage/objects", handler.DeleteObject)
	r.POST("/storage/objects/batch-delete", handler.BatchDeleteObjects)
	r.POST("/storage/directories", handler.CreateDirectory)
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

func TestHandler_ListObjects_Returns200WithObjectsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/storage/objects", "")
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	objects, ok := resp["objects"].([]interface{})
	require.True(t, ok, "expected 'objects' array, body=%s", rec.Body.String())
	assert.Empty(t, objects)
}

func TestHandler_NotConfigured_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandlerNoStore(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/storage/objects", "")
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_DeleteObject_MissingPath_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodDelete, "/storage/objects", "")
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_BatchDeleteObjects_InvalidJSON_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/storage/objects/batch-delete", `{broken`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_BatchDeleteObjects_EmptyPaths_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/storage/objects/batch-delete", `{"paths":[]}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

// TestHandler_BatchDeleteObjects_OK seeds two objects through the same service
// instance the handler uses, then drives the batch-delete through the router.
func TestHandler_BatchDeleteObjects_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx := setupSvcCtxWithStore(t)
	svc := NewService(svcCtx)
	handler := NewHandler(svc)
	router := newRouter(handler)

	for _, p := range []string{"del-1.txt", "del-2.txt"} {
		_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
			Path: p, File: strings.NewReader("x"), Size: 1,
		})
		require.NoError(t, err)
	}

	rec := doReq(t, router, http.MethodPost, "/storage/objects/batch-delete",
		`{"paths":["del-1.txt","del-2.txt"]}`)
	assertStatus(t, rec, http.StatusOK)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	deleted, ok := resp["deleted"].([]interface{})
	require.True(t, ok, "expected 'deleted' array, body=%s", rec.Body.String())
	assert.Len(t, deleted, 2)
}

func TestHandler_CreateDirectory_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/storage/directories", `{"prefix":"assets"}`)
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "assets/", resp["prefix"])
}

func TestHandler_CreateDirectory_MissingPrefix_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/storage/directories", `{}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandler_SignedUrl_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/storage/signed-url?path=doc.txt&expire=60", "")
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["url"])
}

func TestHandler_SignedUrl_MissingPath_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/storage/signed-url", "")
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}
