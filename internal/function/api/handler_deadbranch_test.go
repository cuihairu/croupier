package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 路由 :id 恒非空，直接以无 Params 的 gin.Context 调用 handler，
// 触发 ShouldBindQuery/Uri 的 required 校验失败分支。

func invokeWithoutIDParamV11(t *testing.T, method, body string, fn func(*gin.Context)) int {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, "/api/v1/metadata/functions", nil)
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Body = io.NopCloser(strings.NewReader(body))
	}
	fn(c)
	return rec.Code
}

func TestHandler_GetFunction_MissingURIParamV11(t *testing.T) {
	_, service := setupTestRouter()
	handler := NewHandler(service)
	code := invokeWithoutIDParamV11(t, http.MethodGet, "", handler.GetFunction)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestHandler_UpdateFunction_MissingURIParamV11(t *testing.T) {
	_, service := setupTestRouter()
	handler := NewHandler(service)
	code := invokeWithoutIDParamV11(t, http.MethodPut, `{}`, handler.UpdateFunction)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestHandler_DeleteFunction_MissingURIParamV11(t *testing.T) {
	_, service := setupTestRouter()
	handler := NewHandler(service)
	code := invokeWithoutIDParamV11(t, http.MethodDelete, "", handler.DeleteFunction)
	assert.Equal(t, http.StatusBadRequest, code)
}
