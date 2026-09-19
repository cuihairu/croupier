package page

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopedPageRequest 构造带 scope 的 gin 上下文（handler 从 request context
// 读取游戏域）。
func scopedPageRequest(method, target, body string, scopeCtx context.Context) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, rec := newTestContext(method, target, body)
	ctx.Request = ctx.Request.WithContext(scopeCtx)
	return ctx, rec
}

// SetMenu handler：uri 绑定失败、JSON 绑定失败、挂载成功、解除挂载、
// 菜单不存在 → 404。
func TestPageHandlerSetMenuBranches(t *testing.T) {
	service, scopeCtx, _ := newPageTestService(t, "pages:edit")
	handler := NewHandler(service)
	saveTestPageDraft(t, service, scopeCtx)

	menu := &model.MenuItem{
		GameID:    "demo-game",
		Env:       "development",
		MenuKey:   "player",
		SortOrder: 1,
		IsVisible: true,
	}
	require.NoError(t, menu.SetLabels(map[string]string{"zh-CN": "玩家"}))
	require.NoError(t, service.svcCtx.MenuModel.Create(scopeCtx, menu))

	// uri 绑定失败（缺 pageKey param）→ 400
	ctx, rec := scopedPageRequest(http.MethodPut, "/api/v1/pages//menu", `{}`, scopeCtx)
	handler.SetMenu(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// JSON 绑定失败 → 400
	ctx, rec = scopedPageRequest(http.MethodPut, "/api/v1/pages/player.manage/menu", `{`, scopeCtx)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.SetMenu(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// 挂载成功
	ctx, rec = scopedPageRequest(http.MethodPut, "/api/v1/pages/player.manage/menu",
		`{"menuId":`+strconv.FormatInt(int64(menu.ID), 10)+`}`, scopeCtx)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.SetMenu(ctx)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"menuId":`+strconv.FormatInt(int64(menu.ID), 10))

	// menuId=0 解除挂载
	ctx, rec = scopedPageRequest(http.MethodPut, "/api/v1/pages/player.manage/menu", `{"menuId":0}`, scopeCtx)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.SetMenu(ctx)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"menuId":null`)

	// 菜单不存在 → 404
	ctx, rec = scopedPageRequest(http.MethodPut, "/api/v1/pages/player.manage/menu", `{"menuId":424242}`, scopeCtx)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.SetMenu(ctx)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	// 页面不存在：PageNotFoundError 未做 HTTP 分类，现行为是 500 + 原始消息
	ctx, rec = scopedPageRequest(http.MethodPut, "/api/v1/pages/nope/menu", `{"menuId":null}`, scopeCtx)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "nope"}}
	handler.SetMenu(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "page not found: nope")
}

// BulkRepublish/BulkSyncSelectors handler：空 body（io.EOF 容忍）走 service；
// 非法 JSON → 400。
func TestPageHandlerBulkRoutesBindAndEmptyBody(t *testing.T) {
	service, scopeCtx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	handler := NewHandler(service)

	// 空目标 scope：空 body 正常执行返回 200
	ctx, rec := scopedPageRequest(http.MethodPost, "/api/v1/pages/bulk-republish", "", scopeCtx)
	handler.BulkRepublish(ctx)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"total":0`)

	ctx, rec = scopedPageRequest(http.MethodPost, "/api/v1/pages/bulk-sync-selectors", "", scopeCtx)
	handler.BulkSyncSelectors(ctx)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// 非法 JSON → 400
	ctx, rec = scopedPageRequest(http.MethodPost, "/api/v1/pages/bulk-republish", `{`, scopeCtx)
	handler.BulkRepublish(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	ctx, rec = scopedPageRequest(http.MethodPost, "/api/v1/pages/bulk-sync-selectors", `{`, scopeCtx)
	handler.BulkSyncSelectors(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// 缺 scope → 400（service requireScope 经 handler 透传；权限检查先于
	// scope 检查，须带授权用户 page_tester 但剥离 game scope）
	noScope, rec2 := newTestContext(http.MethodPost, "/api/v1/pages/bulk-republish", "")
	noScope.Request = noScope.Request.WithContext(
		context.WithValue(context.Background(), "username", "page_tester"))
	handler.BulkRepublish(noScope)
	assert.Equal(t, http.StatusBadRequest, rec2.Code, rec2.Body.String())
}
