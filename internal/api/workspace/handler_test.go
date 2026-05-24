package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWorkspaceHandlerTest(t *testing.T) (*Handler, *Service) {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:                   db,
		Cache:                nullCache,
		CacheHelper:          cache.NewCacheHelper(nullCache),
		WorkspaceConfigModel: model.NewWorkspaceConfigModel(db),
		ConfigVersionModel:   model.NewConfigVersionModel(db),
	}

	service := NewService(svcCtx)
	return NewHandler(service), service
}

func newWorkspaceTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func decodeWorkspaceResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	return payload
}

func TestNewHandler(t *testing.T) {
	t.Parallel()

	service := &Service{}
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Same(t, service, handler.service)
}

func TestHandler_SaveConfig_InvalidLayoutReturnsWorkspaceInvalidConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, service := setupWorkspaceHandlerTest(t)
	router := gin.New()
	router.PUT("/api/v1/workspaces/:objectKey/config", handler.SaveConfig)

	body := `{"title":"玩家工作台","layout":{"type":"tabs","tabs":[{"key":"players","title":"玩家列表","layout":{"type":"list"}}]}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/player/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	payload := decodeWorkspaceResponse(t, rec)
	assert.Equal(t, "workspace_invalid_config", payload["error"])
	assert.Equal(t, "workspace_invalid_config", payload["code"])
	assert.Contains(t, payload["message"], "listFunction")

	versions, err := service.svcCtx.ConfigVersionModel.List(context.Background(), workspaceVersionKey("player"))
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func TestHandler_Rollback_BindsVersionIDFromJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, service := setupWorkspaceHandlerTest(t)
	ctx := context.WithValue(context.Background(), "username", "tester")

	_, err := service.SaveConfig(ctx, &SaveConfigRequest{
		ObjectKey: "player",
		Title:     "玩家工作台",
		Layout:    validWorkspaceLayout(),
		Status:    "draft",
		MenuOrder: 1,
	})
	require.NoError(t, err)

	router := gin.New()
	router.POST("/api/v1/workspaces/:objectKey/rollback", handler.Rollback)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/player/rollback", strings.NewReader(`{"versionId":"1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	payload := decodeWorkspaceResponse(t, rec)
	assert.Equal(t, "player", payload["objectKey"])
	assert.Equal(t, float64(2), payload["version"])

	current, err := service.GetConfig(ctx, &GetConfigRequest{ObjectKey: "player"})
	require.NoError(t, err)
	assert.Equal(t, "玩家工作台", current.WorkspaceConfig.Title)
}

func TestHandler_VersionDetail_BindsURIParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, service := setupWorkspaceHandlerTest(t)
	ctx := context.WithValue(context.Background(), "username", "tester")

	_, err := service.SaveConfig(ctx, &SaveConfigRequest{
		ObjectKey: "player",
		Title:     "玩家工作台",
		Layout:    validWorkspaceLayout(),
		Status:    "draft",
		MenuOrder: 1,
	})
	require.NoError(t, err)

	router := gin.New()
	router.GET("/api/v1/workspaces/:objectKey/versions/:versionId", handler.VersionDetail)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/player/versions/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	payload := decodeWorkspaceResponse(t, rec)
	record, ok := payload["workspaceVersionRecord"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "player", record["objectKey"])
	assert.Equal(t, "1", record["id"])
	assert.Equal(t, true, record["isCurrentDraft"])
}
