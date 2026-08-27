package handler

import (
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
