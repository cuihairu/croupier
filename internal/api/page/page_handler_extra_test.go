// 覆盖目标：page handler 的 RebuildProposals（0%）与 Validate/Rollback 的
// service 错误路径。
package page

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_RebuildProposals_EmptyScope(t *testing.T) {
	service, svcCtx, _ := newPageTestService(t, "pages:edit")
	handler := NewHandler(service)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages/rebuild-proposals", "")
	ctx.Request = ctx.Request.WithContext(svcCtx)
	handler.RebuildProposals(ctx)

	// 空库重建：成功返回（无 proposal 可比对/发布）
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Validate_UnknownPage_BadRequest(t *testing.T) {
	service, svcCtx, _ := newPageTestService(t, "pages:edit")
	handler := NewHandler(service)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages/ghost/validate", "")
	ctx.Params = gin.Params{{Key: "pageKey", Value: "ghost"}}
	ctx.Request = ctx.Request.WithContext(svcCtx)
	handler.Validate(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Rollback_UnknownVersion(t *testing.T) {
	service, svcCtx, _ := newPageTestService(t, "pages:edit")
	handler := NewHandler(service)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages/some/versions/99/rollback", "")
	ctx.Params = gin.Params{{Key: "pageKey", Value: "some"}}
	ctx.Request = ctx.Request.WithContext(svcCtx)
	handler.Rollback(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Versions_UnknownPage(t *testing.T) {
	service, svcCtx, _ := newPageTestService(t, "pages:edit")
	handler := NewHandler(service)

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages/ghost/versions", "")
	ctx.Params = gin.Params{{Key: "pageKey", Value: "ghost"}}
	ctx.Request = ctx.Request.WithContext(svcCtx)
	handler.Versions(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "items")
}
