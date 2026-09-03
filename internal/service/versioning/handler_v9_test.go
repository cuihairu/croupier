package versioning

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newVersioningHandlerRouterV9(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(db))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = svc.WithGameScope(ctx, svc.GameScope{
			GameID: c.GetHeader("X-Game-ID"),
			Env:    c.GetHeader("X-Env"),
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	group := router.Group("/api/versioning/pages/:pageKey")
	group.POST("/republish", handler.Republish)
	router.DELETE("/api/versioning/pages/:pageKey", handler.DeletePage)
	router.DELETE("/api/versioning/pages", handler.DeletePage)
	router.POST("/api/versioning/pages/composite", handler.CreateCompositePage)
	return router
}

func doVersioningRequestV9(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Game-ID", "demo-game")
	req.Header.Set("X-Env", "development")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedCompositeContractsV9(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&model.TermDictionary{}))
	svcContract := service.NewContractService(db)
	ctx := context.Background()
	require.NoError(t, svcContract.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "agent-v9", spec.FunctionContractInput{
		ID:           "player.get",
		Resource:     "player",
		Capability:   "item_query",
		Execution:    "sync",
		Enabled:      true,
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		OutputSchema: `{"type":"object","properties":{"player":{"type":"object"},"gold":{"type":"integer"}}}`,
	}))
	require.NoError(t, svcContract.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "agent-v9", spec.FunctionContractInput{
		ID:           "order.list",
		Resource:     "order",
		Capability:   "collection_query",
		Execution:    "sync",
		Enabled:      true,
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"integer"}}}`,
	}))
}

// ---------------------------------------------------------------------------
// Handler.Republish service error branch
// ---------------------------------------------------------------------------

func TestV9Handler_RepublishServiceError(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/missing/republish", `{}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------------------------------------------------------------------------
// Handler.CreateCompositePage
// ---------------------------------------------------------------------------

func TestV9Handler_CreateCompositePage_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite", `{bad`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestV9Handler_CreateCompositePage_TooFewSections(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite",
		`{"pageKey":"composite--single","sections":[{"functionId":"player.get","view":"fields"}]}`)
	require.NotEqual(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.String())
}

func TestV9Handler_CreateCompositePage_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite",
		`{"pageKey":"composite--v9","sections":[
			{"functionId":"player.get","view":"fields","title":"玩家信息"},
			{"functionId":"order.list","view":"table","title":"订单","refreshOn":["player.get"]}
		]}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		ProposalKey string `json:"proposalKey"`
		PageKey     string `json:"pageKey"`
		PageType    string `json:"pageType"`
		Quality     string `json:"quality"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, "composite--v9", payload.PageKey)
	assert.NotEmpty(t, payload.ProposalKey)
}

// ---------------------------------------------------------------------------
// Handler.DeletePage
// ---------------------------------------------------------------------------

func TestV9Handler_DeletePage_MissingPageKey(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodDelete, "/api/versioning/pages", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "pageKey is required")
}

func TestV9Handler_DeletePage_ServiceError(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	require.NoError(t, db.Exec("DROP TABLE page_proposals").Error)

	rec := doVersioningRequestV9(router, http.MethodDelete, "/api/versioning/pages/operation--gone", "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestV9Handler_DeletePage_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	page := extraOperationPage("operation--deleted")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	rec := doVersioningRequestV9(router, http.MethodDelete, "/api/versioning/pages/"+page.PageKey, "")
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Deleted string `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, page.PageKey, payload.Deleted)
}
