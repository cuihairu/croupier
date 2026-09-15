package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctx := &svc.ServiceContext{}
	RegisterHandlers(r, ctx)
}

// TestRegisterHandlers_SiteSettingsPaths 回归：网站设置路由必须是
// /api/v1/site/... 而不是挂错组产生的 /api/v1/site/site/...（曾全线 404）。
func TestRegisterHandlers_SiteSettingsPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHandlers(r, &svc.ServiceContext{})

	paths := make(map[string]bool)
	for _, ri := range r.Routes() {
		paths[ri.Method+" "+ri.Path] = true
		assert.NotContains(t, ri.Path, "/site/site/",
			"site 路由出现双前缀（RegisterAdmin 挂错组）: %s %s", ri.Method, ri.Path)
	}
	for _, want := range []string{
		"GET /api/v1/site/features",
		"GET /api/v1/site/observability",
		"GET /api/v1/site/notification",
		"PUT /api/v1/site/:key",
		"DELETE /api/v1/site/:key",
	} {
		assert.True(t, paths[want], "缺少路由 %s", want)
	}
}

// TestRegisterHandlers_MetaRootNoTrailingSlash 回归：生产 server 关闭了
// RedirectTrailingSlash，GET /api/v1（无尾斜杠）必须直接 200 而非 404。
func TestRegisterHandlers_MetaRootNoTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.RedirectTrailingSlash = false // 与 cmd/server/root.go 一致
	RegisterHandlers(r, &svc.ServiceContext{})

	for _, target := range []string{"/api/v1", "/api/v1/"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "GET %s 应返回 200", target)
	}
}

// TestRegisterHandlers_RuntimeSourcesPath 回归：runtime-sources 端点必须
// 注册在 server 真正使用的路由表（internal/handler/routes.go）。
// 首版只挂在了无引用方的 internal/router/router.go——CI 与 handler 单测全绿，
// 线上却 404（Gin debug 路由表实证），故此处直接断言完整路径存在。
func TestRegisterHandlers_RuntimeSourcesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHandlers(r, &svc.ServiceContext{})

	paths := make(map[string]bool)
	for _, ri := range r.Routes() {
		paths[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/openapi/sources",
		"GET /api/v1/openapi/runtime-sources",
	} {
		assert.True(t, paths[want], "缺少路由 %s", want)
	}
}
