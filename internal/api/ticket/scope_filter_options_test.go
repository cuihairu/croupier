package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestList_ScopeHeaderOverridesQueryParams 复刻 #21 的核心诉求：
// 页面去掉游戏/环境过滤后，列表归属由顶栏 scope（GameDBMiddleware 注入）
// 决定，请求参数里的 gameId/env 不再作为查询依据。
func TestList_ScopeHeaderOverridesQueryParams(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "A1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "A2", Category: "billing", Assignee: "bob", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "B1", Category: "bug", Assignee: "carol", GameID: "game-b", Env: "dev"}))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?gameId=game-b&env=dev", "")
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "game-a", Env: "prod"}))
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, int64(2), resp.Total)
	for _, item := range resp.Items {
		assert.Equal(t, "game-a", item.GameId)
	}
}

// TestCreate_StampScopeFromContext：新建工单归属跟随请求 scope，
// body 里的 gameId/env 不再被信任（scoped 组下中间件已鉴权）。
func TestCreate_StampScopeFromContext(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"S1","content":"c","category":"bug","gameId":"game-b","env":"dev"}`)
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "game-a", Env: "prod"}))
	handler.Create(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var created CreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "game-a", created.GameId)
	assert.Equal(t, "prod", created.Env)
}

// TestFilterOptions_ServerAggregatesUnderScope：分类/处理人过滤选项由服务端
// 聚合提供——distinct 全集 + 计数、剔除空值、按 scope 过滤（#21 ②）。
func TestFilterOptions_ServerAggregatesUnderScope(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T2", Category: "bug", Assignee: "bob", GameID: "game-a", Env: "prod"}))
	// 空分类/空处理人不应成为下拉选项
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T3", Category: "billing", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T4", Category: "other-game", Assignee: "carol", GameID: "game-b", Env: "dev"}))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/filter-options", "")
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "game-a", Env: "prod"}))
	handler.FilterOptions(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FilterOptionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, []TicketFilterOption{
		{Name: "billing", Count: 1},
		{Name: "bug", Count: 2},
	}, resp.Categories)
	assert.Equal(t, []TicketFilterOption{
		{Name: "alice", Count: 1},
		{Name: "bob", Count: 1},
	}, resp.Assignees)
}

// TestFilterOptions_AssigneesErrorSurfaced：分类聚合成功、处理人聚合失败时
// service.go 的第二错误分支（ListAssignees err → nil, err）必须透传而非
// 返回半截 options。通过 DropColumn("assignee") 模拟存量老库缺列形态：
// category 查询不受影响，assignee 查询确定性报错（与迁移漏配事故同构）。
func TestFilterOptions_AssigneesErrorSurfaced(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, db.Migrator().DropColumn(&model.Ticket{}, "assignee"))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/filter-options", "")
	handler.FilterOptions(ctx)

	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
}

// TestFilterOptions_CategoriesErrorSurfaced：分类聚合失败时 service.go 的
// 第一错误分支（ListCategories err → nil, err）短路返回——处理人聚合不得
// 被执行，下拉也不得返回空壳 options（#21 ② 契约：选项要么真实，要么报错）。
func TestFilterOptions_CategoriesErrorSurfaced(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, db.Migrator().DropColumn(&model.Ticket{}, "category"))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/filter-options", "")
	handler.FilterOptions(ctx)

	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
	assert.NotContains(t, rec.Body.String(), `"categories"`)
}

// TestFilterOptions_EmptyScopeStillReturnsShape：无 scope（直调方兼容路径）
// 时返回全量聚合与空数组而非 null。
func TestFilterOptions_EmptyScopeStillReturnsShape(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))

	ctx, rec := newTicketRequest(http.MethodGet, fmt.Sprintf("/api/v1/tickets/filter-options"), "")
	handler.FilterOptions(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FilterOptionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []TicketFilterOption{{Name: "bug", Count: 1}}, resp.Categories)
	assert.Equal(t, []TicketFilterOption{{Name: "alice", Count: 1}}, resp.Assignees)
}
