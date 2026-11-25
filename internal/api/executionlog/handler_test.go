package executionlog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userMiddleware injects the username into the request context (simulating an
// auth middleware).
func userMiddleware(username string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", username))
		c.Next()
	}
}

func newExecLogRouter(t *testing.T, username string) (*gin.Engine, *svc.ServiceContext) {
	t.Helper()
	s, svcCtx, _ := newExecLogTestService(t, username)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(userMiddleware(username))
	h := NewHandler(s)
	r.GET("/api/v1/execution-logs", h.List)
	r.GET("/api/v1/execution-logs/:id", h.Get)
	return r, svcCtx
}

func TestHandlerListBindError(t *testing.T) {
	r, _ := newExecLogRouter(t, "alice")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs?page=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlerListMineSuccess(t *testing.T) {
	r, svcCtx := newExecLogRouter(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs?mine=true&page=1&pageSize=10", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 1)
}

func TestHandlerListPermissionError(t *testing.T) {
	r, _ := newExecLogRouter(t, "alice")
	// mine=false → audit permission required → 403 via the unified error
	// response
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandlerGetSuccessAndBadRequest(t *testing.T) {
	r, svcCtx := newExecLogRouter(t, "alice")
	seeded := seedExecLog(t, svcCtx, "alice", "mail.send")

	// Success
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs/"+itoa(seeded.ID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp ExecutionLogDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "mail.send", resp.FunctionID)

	// Invalid ID → service-layer BadRequest
	req = httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs/abc", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
