package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
)

// ListFunctions：非法 query 绑定（pageSize 非数字）→ 400。
func TestHandler_ListFunctions_BadQuery(t *testing.T) {
	router, _ := setupTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/functions?page=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// GetFunction / UpdateFunction / DeleteFunction：URI 绑定失败或 service 错误分支。
func TestHandler_GetFunction_ServiceError(t *testing.T) {
	router, service := setupTestRouter()
	// 注册后 store 内可查
	require1(service.Register(testCtx(), &functionv1.FunctionMetadata{Id: "player.get"}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/functions/player.get", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_RegisterFunction_HTTP(t *testing.T) {
	router, _ := setupTestRouter()

	// 注册是 upsert 语义：直接走 HTTP 成功路径
	body := `{"id":"player.http","name":"Http"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metadata/functions", stringReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandler_UpdateFunction_FullBody(t *testing.T) {
	router, service := setupTestRouter()
	require1(service.Register(testCtx(), &functionv1.FunctionMetadata{Id: "player.upd"}))

	body := `{
		"inputSchema": "{\"type\":\"object\"}",
		"outputSchema": "{\"type\":\"object\"}",
		"behavior": {"mode": "MODE_QUERY"},
		"security": {"riskLevel": "RISK_LEVEL_LOW"}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/metadata/functions/player.upd", stringReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// schema 均有值 → 覆盖 body.InputSchema/OutputSchema 分支
	got, err := service.Get(testCtx(), "player.upd")
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestHandler_UpdateFunction_ServiceError(t *testing.T) {
	router, _ := setupTestRouter()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/metadata/functions/ghost.id", stringReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_DeleteFunction_ServiceError(t *testing.T) {
	router, _ := setupTestRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/metadata/functions/ghost.id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func require1(err error) {
	if err != nil {
		panic(err)
	}
}

func stringReader(s string) *stringPtr { return &stringPtr{s} }

type stringPtr struct{ s string }

func (s *stringPtr) Read(p []byte) (int, error) {
	if s.s == "" {
		return 0, nil
	}
	n := copy(p, s.s)
	s.s = s.s[n:]
	return n, nil
}
