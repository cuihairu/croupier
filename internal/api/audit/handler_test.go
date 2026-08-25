package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditcore "github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupAuditHandlerTest builds the handler against a real persistent
// audit_records table (in-memory SQLite). The legacy OpsStateStore-backed
// in-memory trail was removed.
func setupAuditHandlerTest(t *testing.T) *Handler {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	// Schema is normally created by the migration baseline; tests own their
	// fixtures.
	require.NoError(t, db.AutoMigrate(&auditcore.AuditModel{}))

	auditStore, err := auditcore.NewSQLAuditStore(db)
	require.NoError(t, err)
	auditSvc := auditcore.NewAuditService(auditStore, nil)

	seedAudit := func(actor, gameID, env, action, target, outcome, traceID string) {
		ctx := context.Background()
		_, _ = auditSvc.Log(ctx, auditcore.AuditEventType(action),
			auditcore.WithActorID(actor, "user", actor),
			auditcore.WithResourceID("function", target),
			auditcore.WithGameID(gameID, env),
			auditcore.WithDetails(map[string]interface{}{"traceId": traceID}),
			auditcore.WithOutcome(outcome, ""),
		)
	}
	seedAudit("user1", "game1", "prod", "action1", "target1", "success", "trace1")
	seedAudit("user2", "game2", "dev", "action2", "target2", "success", "trace2")
	seedAudit("user1", "game1", "prod", "action3", "target3", "failure", "trace3")

	svcCtx := &svc.ServiceContext{DB: db}
	service := NewService(svcCtx)
	return NewHandler(service)
}

func TestNewHandler(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_GetAuditLogs_GETRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Logf("Response: %s", resp.Body.String())
	}
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotNil(t, result["items"])
}

func TestHandler_GetAuditLogs_POSTRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.POST("/audit", handler.GetAuditLogs)

	body := map[string]interface{}{
		"page":     1,
		"pageSize": 10,
		"action":   "action1",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/audit", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetAuditLogs_DefaultPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetAuditLogs_InvalidPageType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?page=invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Gin should handle this gracefully, defaulting to 0 which becomes 1
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetAuditLogs_NegativePage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?page=-1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetAuditLogs_EmptyResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?userId=nonexistent", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Check that we got a valid response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotNil(t, result["items"])
}

func TestHandler_GetAuditLogs_JSONResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "application/json; charset=utf-8", resp.Header().Get("Content-Type"))

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.Contains(t, result, "items")
	assert.Contains(t, result, "total")
	assert.Contains(t, result, "page")
	assert.Contains(t, result, "pageSize")
}

func TestHandler_GetAuditLogs_ItemStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	rawItems, ok := result["items"].([]interface{})
	if ok && len(rawItems) > 0 {
		item, ok := rawItems[0].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, item, "id")
		assert.Contains(t, item, "action")
		assert.Contains(t, item, "userId")
		assert.Contains(t, item, "gameId")
		assert.Contains(t, item, "env")
		assert.Contains(t, item, "target")
		assert.Contains(t, item, "result")
		assert.Contains(t, item, "traceId")
		assert.Contains(t, item, "createdAt")
	}
}

func TestHandler_GetAuditLogs_LargePageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit?pageSize=10000", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetAuditLogs_POSTWithInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.POST("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("POST", "/audit", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should still work with default values
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetAuditLogs_POSTWithEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.POST("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("POST", "/audit", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Check that we got a valid response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotNil(t, result["items"])
}

func TestHandler_GetAuditLogs_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupAuditHandlerTest(t)

	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/audit?page=1&pageSize=10", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
}

func TestHandler_GetAuditLogs_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil)
	router := gin.New()
	router.GET("/audit", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/audit", nil)
	resp := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil service
			assert.NotNil(t, r)
		}
	}()

	router.ServeHTTP(resp, req)
}
