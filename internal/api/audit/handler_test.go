package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Singleton test store for handler tests
var (
	testAuditHandlerStore     *svc.OpsStateStore
	testAuditHandlerStoreOnce sync.Once
	testAuditHandlerStoreMu   sync.Mutex
)

// setupAuditHandlerStore creates or resets the shared test store
func setupAuditHandlerStore(t *testing.T) *svc.OpsStateStore {
	testAuditHandlerStoreMu.Lock()
	defer testAuditHandlerStoreMu.Unlock()

	testAuditHandlerStoreOnce.Do(func() {
		testAuditHandlerStore = svc.NewOpsStateStore("")
	})

	// Clear audit entries before each test
	testAuditHandlerStore.Update(func(state *svc.OpsState) {
		state.Audit.Entries = nil
		state.Audit.UpdatedAt = time.Now()
	})

	return testAuditHandlerStore
}

// addTestAuditEntry adds a test entry to the store
func addTestAuditEntry(store *svc.OpsStateStore, userID, gameID, env, action, target, result, traceID string, metadata map[string]interface{}) {
	store.Update(func(state *svc.OpsState) {
		entry := svc.OpsAuditEntry{
			ID:        traceID,
			Action:    action,
			UserID:    userID,
			GameID:    gameID,
			Env:       env,
			Target:    target,
			Result:    result,
			TraceID:   traceID,
			Metadata:  metadata,
			CreatedAt: time.Now(),
		}
		state.Audit.Entries = append(state.Audit.Entries, entry)
		state.Audit.UpdatedAt = time.Now()
	})
}

func setupAuditHandlerTest(t *testing.T) *Handler {
	store := setupAuditHandlerStore(t)

	// Add some test entries
	addTestAuditEntry(store, "user1", "game1", "prod", "action1", "target1", "success", "trace1", nil)
	addTestAuditEntry(store, "user2", "game2", "dev", "action2", "target2", "success", "trace2", nil)
	addTestAuditEntry(store, "user1", "game1", "prod", "action3", "target3", "failure", "trace3", nil)

	svcCtx := &svc.ServiceContext{
		OpsStateStore: store,
	}

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

	assert.Equal(t, float64(0), result["code"])
	assert.NotNil(t, result["data"])
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

	assert.Equal(t, float64(0), result["code"])
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
	store := setupAuditHandlerStore(t)

	svcCtx := &svc.ServiceContext{
		OpsStateStore: store,
	}
	service := NewService(svcCtx)
	handler := NewHandler(service)

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
	assert.Equal(t, float64(0), result["code"])
	assert.NotNil(t, result["data"])
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

	assert.Contains(t, result, "code")
	assert.Contains(t, result, "message")
	assert.Contains(t, result, "data")

	// response.Success wraps AuditResponse, so we need to access the nested data
	// result = {code: 0, message: "success", data: AuditResponse}
	// AuditResponse = {code: 0, message: "OK", data: {items, total, page, size}}
	auditResp := result["data"].(map[string]interface{})
	innerData := auditResp["data"].(map[string]interface{})

	// Direct key existence check
	_, hasItems := innerData["items"]
	_, hasTotal := innerData["total"]
	_, hasPage := innerData["page"]
	_, hasSize := innerData["size"]
	assert.True(t, hasItems, "data should contain 'items' key")
	assert.True(t, hasTotal, "data should contain 'total' key")
	assert.True(t, hasPage, "data should contain 'page' key")
	assert.True(t, hasSize, "data should contain 'size' key")
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

	data := result["data"].(map[string]interface{})
	items, ok := data["items"].([]map[string]interface{})

	if ok && len(items) > 0 {
		item := items[0]
		assert.Contains(t, item, "id")
		assert.Contains(t, item, "action")
		assert.Contains(t, item, "userId")
		assert.Contains(t, item, "gameId")
		assert.Contains(t, item, "env")
		assert.Contains(t, item, "target")
		assert.Contains(t, item, "result")
		assert.Contains(t, item, "traceId")
		assert.Contains(t, item, "createdAt")
		assert.Contains(t, item, "metadata")
	}
}

func TestHandler_GetAuditLogs_LargePageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := setupAuditHandlerStore(t)

	// Add many entries
	for i := 0; i < 100; i++ {
		addTestAuditEntry(store, "user", "game", "prod", "action", "target", "success", "trace", nil)
	}

	svcCtx := &svc.ServiceContext{
		OpsStateStore: store,
	}
	service := NewService(svcCtx)
	handler := NewHandler(service)

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
	assert.Equal(t, float64(0), result["code"])
	assert.NotNil(t, result["data"])
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
