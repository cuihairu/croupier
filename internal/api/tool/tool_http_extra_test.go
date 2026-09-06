package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTTP 全端点分支：绑定失败 / 校验失败 / 更新不存在 / 删除非法 ID。
func TestToolHTTPHandlerBranches(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/tools" && c.Request.Method == http.MethodPost {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "ops-admin"))
		}
		c.Next()
	})
	r.GET("/tools", h.List)
	r.POST("/tools", h.Create)
	r.PUT("/tools/:id", h.Update)
	r.DELETE("/tools/:id", h.Delete)

	do := func(method, target, body string) int {
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, target, nil)
		} else {
			req = httptest.NewRequest(method, target, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// List：成功（请求字段全为可选 string，query 绑定不会失败）
	assert.Equal(t, http.StatusOK, do(http.MethodGet, "/tools?gameId=g1&env=prod", ""))

	// Create：JSON 非法 → 绑定失败
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/tools", "{broken"))
	// Create：缺名称 → 校验失败 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/tools",
		`{"url":"https://example.com"}`))
	// Create：成功（带用户名上下文 → CreatedBy=ops-admin）
	assert.Equal(t, http.StatusOK, do(http.MethodPost, "/tools",
		`{"name":"Grafana","url":"https://grafana.example.com"}`))

	// Update：无可更新字段 → 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPut, "/tools/1", `{}`))
	// Update：目标不存在（回读列表缺失）→ 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPut, "/tools/9999",
		`{"name":"Ghost"}`))

	// Delete：非法 ID → 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodDelete, "/tools/abc", ""))
	// Delete：成功
	assert.Equal(t, http.StatusOK, do(http.MethodDelete, "/tools/1", ""))
}

// 底层表缺失：Create/Update 模型错误透传。
func TestToolHTTPModelFailure(t *testing.T) {
	db := newToolTestDB(t)
	require.NoError(t, db.Migrator().DropTable("tool_links"))
	h := newToolHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/tools", h.Create)
	r.PUT("/tools/:id", h.Update)

	do := func(method, target, body string) int {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.NotEqual(t, http.StatusOK, do(http.MethodPost, "/tools",
		`{"name":"X","url":"https://x.example.com"}`))
	assert.NotEqual(t, http.StatusOK, do(http.MethodPut, "/tools/1", `{"name":"Y"}`))
}
