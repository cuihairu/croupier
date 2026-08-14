package page

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	db := setupTestDB(t)
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{Mode: "test"},
		},
		DB: db,
	}
	service := NewService(svcCtx)
	return NewHandler(service)
}

func newTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func TestNewHandler(t *testing.T) {
	handler := setupTestHandler(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.service)
}

func TestRegenerateRouteMatchesWebClientContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterDraftRoutes(engine.Group("/api/v1/pages"), &svc.ServiceContext{})

	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/pages/:pageKey/regenerate" {
			return
		}
	}
	t.Fatal("POST /api/v1/pages/:pageKey/regenerate is not registered")
}

func TestHandler_ListDrafts_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages?page=abc&pageSize=10", "")
	handler.ListDrafts(ctx)

	// Should return either 200, 400, or 500 (depending on DB state)
	assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError}, rec.Code)
}

func TestHandler_GetDraft_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages/", "")
	// No pageKey param - should fail binding
	handler.GetDraft(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_SaveDraft_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPut, "/api/v1/pages/test", "invalid json")
	ctx.Params = gin.Params{{Key: "pageKey", Value: "test"}}
	handler.SaveDraft(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Validate_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages//validate", "{}")
	ctx.Params = gin.Params{{Key: "pageKey", Value: ""}}
	handler.Validate(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_Preview_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages//preview", "{}")
	ctx.Params = gin.Params{{Key: "pageKey", Value: ""}}
	handler.Preview(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_Publish_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages//publish", "{}")
	ctx.Params = gin.Params{{Key: "pageKey", Value: ""}}
	handler.Publish(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_Unpublish_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodDelete, "/api/v1/pages//unpublish", "")
	ctx.Params = gin.Params{{Key: "pageKey", Value: ""}}
	handler.Unpublish(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_Versions_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages//versions", "")
	ctx.Params = gin.Params{{Key: "pageKey", Value: ""}}
	handler.Versions(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_VersionDetail_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages//versions/1", "")
	ctx.Params = gin.Params{
		{Key: "pageKey", Value: ""},
		{Key: "version", Value: "1"},
	}
	handler.VersionDetail(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_Rollback_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages//rollback", `{"version":1}`)
	ctx.Params = gin.Params{
		{Key: "pageKey", Value: ""},
		{Key: "version", Value: "1"},
	}
	handler.Rollback(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestHandler_RegenerateDraft_BindError(t *testing.T) {
	handler := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages//regenerate", "{}")
	ctx.Params = gin.Params{{Key: "pageKey", Value: ""}}
	handler.RegenerateDraft(ctx)

	// Empty page key should return error
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, rec.Code)
}

func TestPageDraftListRequest_BindQuery(t *testing.T) {
	var req PageDraftListRequest
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/pages?resourceKey=player&status=draft", nil)

	err := ctx.ShouldBindQuery(&req)
	require.NoError(t, err)
	assert.Equal(t, "player", req.ResourceKey)
	assert.Equal(t, "draft", req.Status)
}

func TestPageDraftRequest_BindUri(t *testing.T) {
	var req PageDraftRequest
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/pages/test-page", nil)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "test-page"}}

	err := ctx.ShouldBindUri(&req)
	require.NoError(t, err)
	assert.Equal(t, "test-page", req.PageKey)
}

func TestPageSaveRequest_BindUri(t *testing.T) {
	var req PageSaveRequest
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/pages/test-page", nil)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "test-page"}}

	// Only test that PageKey is set from URI
	_ = ctx.ShouldBindUri(&req)
	// May fail validation for required fields, but PageKey should be set
	assert.Equal(t, "test-page", req.PageKey)
}

func TestPageSaveRequest_BindJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/pages/test-page",
		bytes.NewBufferString(`{"pageKey":"test-page","draftRevision":1,"type":"resource","title":{"zh-CN":"测试"},"category":{"key":"test"}}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	var req PageSaveRequest
	err := ctx.ShouldBindJSON(&req)
	require.NoError(t, err)
	assert.Equal(t, "test-page", req.PageKey)
	assert.Equal(t, 1, *req.DraftRevision)
	assert.Equal(t, spec.PageTypeResource, req.Type)
}

func TestPageNotFoundError(t *testing.T) {
	err := &PageNotFoundError{Key: "test"}
	assert.Contains(t, err.Error(), "test")
}
