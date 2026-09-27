package page

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	return decodeListResponse(t, service, ctx, query).Items
}

// decodeListResponse 解码完整分页响应（#30：items + total/page/pageSize）。
func decodeListResponse(t *testing.T, service *Service, ctx context.Context, query string) PageDraftListResponse {
	t.Helper()
	ginCtx, rec := newTestContext(http.MethodGet, "/api/v1/pages"+query, "")
	ginCtx.Request = ginCtx.Request.WithContext(ctx)
	NewHandler(service).ListDrafts(ginCtx)
	require.Equal(t, http.StatusOK, rec.Code)

	var body PageDraftListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

// seedBoundPage 落一条带 player 函数 binding 的页面：列上 resourceKey 可为空，
// 关联资源经 binding 函数契约推导（#30 多资源页形态）。
func seedBoundPage(t *testing.T, service *Service, ctx context.Context, pageKey, resourceKey string, categoryKeys ...string) {
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
		Bindings:      testPageBindings(),
	})
	require.NoError(t, err)
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

// TestService_ListDrafts_PaginationServerSide 分页由服务端执行（#30）：
// 缺省窗口 20、total 为过滤后命中总数、非法参数钳制、越界页返回空集。
func TestService_ListDrafts_PaginationServerSide(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	for i := 1; i <= 25; i++ {
		seedPageDraft(t, service, ctx, fmt.Sprintf("bulk.%02d", i), "bulk")
	}

	// 缺省：page=1 / pageSize=20，total 是全量命中数
	resp := decodeListResponse(t, service, ctx, "")
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
	require.Len(t, resp.Items, 20)
	assert.Equal(t, "bulk.01", resp.Items[0].PageKey)

	// 显式窗口与翻页
	resp = decodeListResponse(t, service, ctx, "?page=2&pageSize=10")
	assert.Equal(t, 25, resp.Total)
	require.Len(t, resp.Items, 10)
	assert.Equal(t, "bulk.11", resp.Items[0].PageKey)

	resp = decodeListResponse(t, service, ctx, "?page=3&pageSize=10")
	assert.Equal(t, 25, resp.Total)
	require.Len(t, resp.Items, 5)

	// 越界页：空集但 total 不丢
	resp = decodeListResponse(t, service, ctx, "?page=99")
	assert.Equal(t, 25, resp.Total)
	assert.Empty(t, resp.Items)

	// 非法/超限参数钳制而非报错
	resp = decodeListResponse(t, service, ctx, "?page=0&pageSize=-5")
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
	resp = decodeListResponse(t, service, ctx, "?pageSize=99999")
	assert.Equal(t, 200, resp.PageSize)
	require.Len(t, resp.Items, 25)
}

// TestService_ListDrafts_KeywordPushdown 关键词在服务端过滤（#30）：
// 大小写不敏感，命中 pageKey / 各 locale 标题 / 涉及资源。
func TestService_ListDrafts_KeywordPushdown(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedPageDraft(t, service, ctx, "player.manage", "player")
	revision := 0
	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "guild.board",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "guild",
		Title:         map[string]string{"zh-CN": "公会面板", "en-US": "Guild Board"},
		Category:      spec.PageCategorySpec{Key: "guild"},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.NoError(t, err)

	cases := []struct {
		keyword string
		wantAll []string
	}{
		// player 命中 player.manage（pageKey/resource）与 guild.board
		// （binding 关联资源含 player——#30 关联语义参与关键词匹配）
		{"player", []string{"guild.board", "player.manage"}},
		{"PLAYER", []string{"guild.board", "player.manage"}}, // 大小写不敏感
		{"公会", []string{"guild.board"}},                      // zh-CN 标题
		{"GUILD", []string{"guild.board"}},                   // en-US 标题大小写
	}
	for _, tc := range cases {
		resp := decodeListResponse(t, service, ctx, "?keyword="+url.QueryEscape(tc.keyword))
		require.Len(t, resp.Items, len(tc.wantAll), "keyword=%s", tc.keyword)
		got := make([]string, 0, len(resp.Items))
		for _, item := range resp.Items {
			got = append(got, item.PageKey)
		}
		assert.ElementsMatch(t, tc.wantAll, got, "keyword=%s", tc.keyword)
		assert.Equal(t, len(tc.wantAll), resp.Total)
	}

	resp := decodeListResponse(t, service, ctx, "?keyword=zzz-no-hit")
	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Items)
}

// TestService_ListDrafts_ResourceFilterFollowsBindings 资源过滤按关联集合
// 命中（#30）：列上 resourceKey 为空、binding 指向 player 资源函数的页面，
// 在 ?resourceKey=player 下命中；guild 不命中。
func TestService_ListDrafts_ResourceFilterFollowsBindings(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedBoundPage(t, service, ctx, "mixed.board", "", "mixed")

	resp := decodeListResponse(t, service, ctx, "?resourceKey=player")
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "mixed.board", resp.Items[0].PageKey)
	assert.Equal(t, []string{"player"}, resp.Items[0].Resources)

	resp = decodeListResponse(t, service, ctx, "?resourceKey=guild")
	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Items)
}

// TestService_Resources_CountsMultiResourcePages 聚合口径（#30）：多资源页
// 在每个涉及资源下各计一次；空关联页面不成项。
func TestService_Resources_CountsMultiResourcePages(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedPageDraft(t, service, ctx, "player.manage", "player")
	// mixed.board 列 resourceKey 为空、binding 带来 player → 只涉及 player
	seedBoundPage(t, service, ctx, "mixed.board", "", "mixed")
	// order.board 列是 order、binding 带来 player → 涉及 order+player 双资源
	seedBoundPage(t, service, ctx, "order.board", "order")

	resp, err := service.Resources(ctx)
	require.NoError(t, err)
	byKey := map[string]int{}
	for _, item := range resp.Items {
		byKey[item.ResourceKey] = item.PageCount
	}
	assert.Equal(t, 1, byKey["order"], "仅 order.board 自身")
	assert.Equal(t, 3, byKey["player"], "player.manage + mixed.board + order.board(binding)")
	assert.NotContains(t, byKey, "", "空关联页面不成项")
}

// TestHandler_Resources_AggregatesScopedOptions 补 handler 层缺口（覆盖率
// 排查中 Resources handler 此前 0%）：GET /pages/resources 的 HTTP 面——
// scope 内聚合、直接 JSON 无 envelope、按 resourceKey 升序
// （OPEN-ISSUES #13/#30）。
func TestHandler_Resources_AggregatesScopedOptions(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read", "pages:edit")
	seedPageDraft(t, service, ctx, "guild.manage", "guild")
	seedPageDraft(t, service, ctx, "player.manage", "player")
	// order.board 列是 order、binding 带来 player → 双资源页参与两处计数
	seedBoundPage(t, service, ctx, "order.board", "order")

	ginCtx, rec := newTestContext(http.MethodGet, "/api/v1/pages/resources", "")
	ginCtx.Request = ginCtx.Request.WithContext(ctx)
	NewHandler(service).Resources(ginCtx)
	require.Equal(t, http.StatusOK, rec.Code)

	var body PageResourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 3)
	assert.Equal(t, "guild", body.Items[0].ResourceKey)
	assert.Equal(t, 1, body.Items[0].PageCount)
	assert.Equal(t, "order", body.Items[1].ResourceKey)
	assert.Equal(t, 1, body.Items[1].PageCount)
	assert.Equal(t, "player", body.Items[2].ResourceKey)
	assert.Equal(t, 2, body.Items[2].PageCount, "player.manage 列 + order.board binding 关联")
}
