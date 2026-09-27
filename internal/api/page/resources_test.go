package page

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedPageDraft 落一条指定 resourceKey 的草稿（不发布）。
func seedPageDraft(t *testing.T, service *Service, ctx context.Context, pageKey, resourceKey string, categoryKeys ...string) {
	t.Helper()
	categoryKey := resourceKey
	if len(categoryKeys) > 0 && categoryKeys[0] != "" {
		categoryKey = categoryKeys[0]
	}
	revision := 0
	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       pageKey,
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   resourceKey,
		Title:         map[string]string{"zh-CN": pageKey, "en-US": pageKey},
		Category:      spec.PageCategorySpec{Key: categoryKey},
		Operation:     testOperationPageSpec(),
		Bindings:      nil,
	})
	require.NoError(t, err)
}

// listDraftSummaries 以指定 query 走 handler 层拉取草稿列表并解码 items。
func listDraftSummaries(t *testing.T, service *Service, ctx context.Context, query string) []spec.PageSpecDraftSummary {
	t.Helper()
	ginCtx, rec := newTestContext(http.MethodGet, "/api/v1/pages"+query, "")
	ginCtx.Request = ginCtx.Request.WithContext(ctx)
	NewHandler(service).ListDrafts(ginCtx)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Items []spec.PageSpecDraftSummary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Items
}

// TestHandler_ListDrafts_ResourceKeyQueryFilters 修复前红：handler 从未把
// query 绑进 PageDraftListRequest，resourceKey 参数被静默忽略
// （OPEN-ISSUES #13/#30 病灶）。修复后：?resourceKey= 只返回该资源页面。
func TestHandler_ListDrafts_ResourceKeyQueryFilters(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedPageDraft(t, service, ctx, "player.manage", "player")
	seedPageDraft(t, service, ctx, "guild.manage", "guild")

	items := listDraftSummaries(t, service, ctx, "?resourceKey=player")
	require.Len(t, items, 1, "resourceKey 过滤必须生效：仅 player 资源页面命中")
	assert.Equal(t, "player.manage", items[0].PageKey)
	assert.Equal(t, "player", items[0].ResourceKey)
}

// TestHandler_ListDrafts_StatusQueryFilters 同型回归：status 参数绑定。
func TestHandler_ListDrafts_StatusQueryFilters(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit", "pages:publish")
	seedPageDraft(t, service, ctx, "guild.manage", "guild")

	// player.manage 保存后发布；draft 状态过滤应只剩 guild.manage
	revision := 0
	saveResp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理", "en-US": "Player Management"},
		Category:      spec.PageCategorySpec{Key: "player"},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.NoError(t, err)
	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: &saveResp.DraftRevision})
	require.NoError(t, err)

	items := listDraftSummaries(t, service, ctx, "?status=draft")
	require.Len(t, items, 1)
	assert.Equal(t, "guild.manage", items[0].PageKey)
}

// TestService_Resources_AggregatesPageCounts 服务端聚合下拉选项：scope 内
// 页面涉及资源 + 页面数，空 resource 不成项（OPEN-ISSUES #13）。
func TestService_Resources_AggregatesPageCounts(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedPageDraft(t, service, ctx, "player.manage", "player")
	seedPageDraft(t, service, ctx, "player.stats", "player")
	seedPageDraft(t, service, ctx, "guild.manage", "guild")
	// 无资源页面（组合页等）不进聚合
	seedPageDraft(t, service, ctx, "mixed.board", "", "mixed")

	resp, err := service.Resources(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	// 排序稳定：resourceKey 升序
	assert.Equal(t, "guild", resp.Items[0].ResourceKey)
	assert.Equal(t, 1, resp.Items[0].PageCount)
	assert.Equal(t, "player", resp.Items[1].ResourceKey)
	assert.Equal(t, 2, resp.Items[1].PageCount)
}

// TestService_Resources_ScopeIsolated 不同 scope 的页面不进聚合。
// （SaveDraft 的权限按 scope 校验，跨 scope 行直插模型层。）
func TestService_Resources_ScopeIsolated(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedPageDraft(t, service, ctx, "player.manage", "player")

	otherScope := svc.WithGameScope(ctx, svc.GameScope{GameID: "other-game", Env: "development"})
	require.NoError(t, service.svcCtx.PageSpecModel.Upsert(otherScope, &model.PageSpec{
		GameID:      "other-game",
		Env:         "development",
		PageKey:     "guild.manage",
		Type:        string(spec.PageTypeOperation),
		ResourceKey: "guild",
		Status:      "draft",
	}))

	resp, err := service.Resources(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].ResourceKey)
}

// TestResourcesRouteRegistered 静态段路由存在且优先于 /:pageKey。
func TestResourcesRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterDraftRoutes(engine.Group("/api/v1/pages"), &svc.ServiceContext{})

	for _, route := range engine.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/pages/resources" {
			return
		}
	}
	t.Fatal("GET /api/v1/pages/resources is not registered")
}
