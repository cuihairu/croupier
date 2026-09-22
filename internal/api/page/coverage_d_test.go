package page

// 本文件补齐 internal/api/page 包覆盖率缺口（清单见 /tmp/gaps_D.txt，11 个函数）：
//   - stale_heal.go：StartStaleHealLoop（interval 默认/ticker 触发）、healScopes（绑定枚举失败）、
//     healStalePublishedPagesOnce（各错误/跳过/自动发布分支）、scopeDBContext（Resolve 失败）、
//     healSafeReports（纯函数全矩阵）；
//   - service.go：SetPageMenu（scope 缺失/MenuModel 未装配/FindByID 注错/UpdateMenuID 注错）、
//     autoPublishAfterSync（守卫短路/无发布权限/发布失败）、AutoPublishComposite（五个错误分支）、
//     BulkRepublish（Inbox 失败/重生成失败/currentUsername 失败/发布失败）、
//     BulkSyncSelectors（currentUsername 失败/Inbox 失败/dry-run 失败）；
//   - handler.go：BulkSyncSelectors（service 错误透传分支）。
//
// 错误注入手法（全部确定性，无时序依赖）：
//   - gorm Query After 回调按 表名 + SQL 片段 注错（区分 ListByScope 与 FindByScopeAndPageKey）；
//   - sqlite 触发器 RAISE 阻断 UPDATE/INSERT（同既有 bulk_sync_selectors_test 先例）；
//   - DROP TABLE 注非 NotFound 查询错误；
//   - currentUsername 包级接缝（coverage_gapfix_test 同款），顺序调用计数注入。

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// stale_heal.go — healSafeReports 纯函数表驱动矩阵
// ---------------------------------------------------------------------------

// TestHealSafeReports_GuardMatrix：系统愈合自动发布严门全矩阵——
// 输入侧仅 kept/removed 放行；输出侧 kept/removed/shape_updated 放行；
// Manual 非空与 renamed/added/type_changed/manual_required 一律拦截。
func TestHealSafeReports_GuardMatrix(t *testing.T) {
	inputEntry := func(action spec.SelectorSyncAction) spec.SelectorSyncInputEntry {
		return spec.SelectorSyncInputEntry{Target: "/f", Action: action}
	}
	outputEntry := func(action spec.SelectorSyncAction) spec.SelectorSyncOutputEntry {
		return spec.SelectorSyncOutputEntry{StateKey: "s", Source: "/f", Action: action}
	}
	cases := []struct {
		name    string
		reports []spec.BindingSelectorSyncReport
		want    bool
	}{
		{"空报告列表放行", nil, true},
		{"无 selector 的报告放行", []spec.BindingSelectorSyncReport{{BindingID: "b1"}}, true},
		{"输入 kept 放行", []spec.BindingSelectorSyncReport{{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncKept)}}}, true},
		{"输入 removed 放行", []spec.BindingSelectorSyncReport{{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncRemoved)}}}, true},
		{
			"输出 kept/removed/shape_updated 混合放行",
			[]spec.BindingSelectorSyncReport{{Output: []spec.SelectorSyncOutputEntry{
				outputEntry(spec.SelectorSyncKept),
				outputEntry(spec.SelectorSyncRemoved),
				outputEntry(spec.SelectorSyncShapeUpdated),
			}}},
			true,
		},
		{
			"输入输出全放行组合",
			[]spec.BindingSelectorSyncReport{{
				Input:  []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncKept), inputEntry(spec.SelectorSyncRemoved)},
				Output: []spec.SelectorSyncOutputEntry{outputEntry(spec.SelectorSyncKept)},
			}},
			true,
		},
		{
			"Manual 诊断非空拦截（即使 selector 全 kept）",
			[]spec.BindingSelectorSyncReport{{
				Input:  []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncKept)},
				Manual: []spec.Diagnostic{{Code: "binding_governance_stale"}},
			}},
			false,
		},
		{"输入 renamed 拦截", []spec.BindingSelectorSyncReport{{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncRenamed)}}}, false},
		{"输入 added 拦截", []spec.BindingSelectorSyncReport{{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncAdded)}}}, false},
		{"输入 type_changed 拦截", []spec.BindingSelectorSyncReport{{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncTypeChanged)}}}, false},
		{"输入 manual_required 拦截", []spec.BindingSelectorSyncReport{{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncManual)}}}, false},
		{"输出 renamed 拦截", []spec.BindingSelectorSyncReport{{Output: []spec.SelectorSyncOutputEntry{outputEntry(spec.SelectorSyncRenamed)}}}, false},
		{"输出 added 拦截", []spec.BindingSelectorSyncReport{{Output: []spec.SelectorSyncOutputEntry{outputEntry(spec.SelectorSyncAdded)}}}, false},
		{"输出 type_changed 拦截", []spec.BindingSelectorSyncReport{{Output: []spec.SelectorSyncOutputEntry{outputEntry(spec.SelectorSyncTypeChanged)}}}, false},
		{"输出 manual_required 拦截", []spec.BindingSelectorSyncReport{{Output: []spec.SelectorSyncOutputEntry{outputEntry(spec.SelectorSyncManual)}}}, false},
		{
			"多报告：首个干净、第二个 renamed 仍整体拦截",
			[]spec.BindingSelectorSyncReport{
				{Input: []spec.SelectorSyncInputEntry{inputEntry(spec.SelectorSyncKept)}},
				{Output: []spec.SelectorSyncOutputEntry{outputEntry(spec.SelectorSyncRenamed)}},
			},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, healSafeReports(tc.reports))
		})
	}
}

// ---------------------------------------------------------------------------
// stale_heal.go — StartStaleHealLoop
// ---------------------------------------------------------------------------

// TestStartStaleHealLoop_NilSvcCtxAndDefaultInterval：
//   - svcCtx 未装配（Service 零值）时循环不启动、立即返回；
//   - interval<=0 回落 5 分钟默认值；已取消的 ctx 使 select 立即走 Done 分支返回。
func TestStartStaleHealLoop_NilSvcCtxAndDefaultInterval(t *testing.T) {
	// svcCtx 为 nil：不 panic、不进 ticker
	(&Service{}).StartStaleHealLoop(healCtx(), time.Second)

	// svcCtx 非 nil（零值 ServiceContext）+ interval<=0：走默认 5 分钟分支；
	// ctx 预先取消 → select 立即命中 Done，循环同步返回。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		NewService(&svc.ServiceContext{}).StartStaleHealLoop(ctx, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("已取消 ctx 下 loop 必须立即退出")
	}
}

// TestStartStaleHealLoop_TickerFiresHealRound：ticker 触发分支——
// 先直接 INSERT 一行有发布快照的 page_specs（healScopes 的 distinct 枚举
// 命中该 scope），再以 gorm Query 回调捕获 ListByScope 查询作为「一轮扫描
// 已执行到页级」的确定性信号（不依赖睡眠时序），收到信号后取消 ctx 验证
// 循环退出。
func TestStartStaleHealLoop_TickerFiresHealRound(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	require.NoError(t, service.svcCtx.DB.Exec(
		`INSERT INTO page_specs (game_id, env, page_key, spec_json, published_version, draft_revision, created_at, updated_at)
		 VALUES ('demo-game', 'development', 'ticker-probe', '{}', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	fired := make(chan struct{}, 1)
	require.NoError(t, service.svcCtx.DB.Callback().Query().After("gorm:query").Register(
		"coverage_d:heal_ticker_fire", func(tx *gorm.DB) {
			if tx.Statement == nil || tx.Statement.Table != "page_specs" {
				return
			}
			// ListByScope 特有的排序子句（区分于 FindByScopeAndPageKey）。
			if strings.Contains(tx.Statement.SQL.String(), "category_order") {
				select {
				case fired <- struct{}{}:
				default:
				}
			}
		}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.StartStaleHealLoop(ctx, time.Millisecond)
		close(done)
	}()
	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatal("ticker 必须触发并执行一轮 heal 扫描")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop 必须随 ctx 取消退出")
	}
}

// ---------------------------------------------------------------------------
// stale_heal.go — healScopes / scopeDBContext / healStalePublishedPagesOnce 错误路径
// ---------------------------------------------------------------------------

// newMultiGameRouter 构造分库模式 Router（真实 sqlite 文件库，与既有
// TestStaleHeal_MultiGameHealsAcrossGameDBs 同款配置）。
func newMultiGameRouter(t *testing.T, service *Service) *router.Router {
	t.Helper()
	dir := t.TempDir()
	r := router.New(router.Config{
		Driver: "sqlite",
		NameForGame: func(gameID, env string) string {
			return filepath.Join(dir, "game_"+gameID+"_"+env+".db")
		},
		DSNForDatabase: func(_, dbName string) string { return dbName },
		EnsureDatabase: func(_, _, dbName string) (string, error) { return dbName, nil },
		Open: func(_, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
		MigrateGame: model.AutoMigrateGame,
	}, service.svcCtx.DB)
	service.svcCtx.Router = r
	return r
}

// TestHealScopes_ErrorPaths：scope 枚举失败分支——
//   - 单库模式：page_specs 表缺失（非 NotFound 查询错误）；
//   - 分库模式：meta 库 game_envs 绑定表枚举失败。
func TestHealScopes_ErrorPaths(t *testing.T) {
	t.Run("单库 page_specs 查询失败", func(t *testing.T) {
		service, _, _ := newPageTestService(t, "pages:edit")
		require.NoError(t, service.svcCtx.DB.Exec("DROP TABLE page_specs").Error)
		_, err := service.healScopes(context.Background(), service.svcCtx.DB)
		require.Error(t, err)
	})

	t.Run("分库绑定表枚举失败", func(t *testing.T) {
		service, _, _ := newPageTestService(t, "pages:edit")
		newMultiGameRouter(t, service)
		require.NoError(t, service.svcCtx.DB.Exec("DROP TABLE game_envs").Error)
		_, err := service.healScopes(context.Background(), service.svcCtx.DB)
		require.Error(t, err, "Router 已装配时 scope 事实源是 game_envs 绑定表")
	})
}

// TestScopeDBContext_ResolveError：分库模式 Router.Resolve 失败（建库
// EnsureDatabase 注错）必须原样上抛，不返回半成品 ctx。
func TestScopeDBContext_ResolveError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	dir := t.TempDir()
	injected := errors.New("injected ensure database failure")
	service.svcCtx.Router = router.New(router.Config{
		Driver:         "sqlite",
		NameForGame:    func(gameID, env string) string { return filepath.Join(dir, gameID+"_"+env+".db") },
		DSNForDatabase: func(_, dbName string) string { return dbName },
		EnsureDatabase: func(_, _, _ string) (string, error) { return "", injected },
		Open:           func(_, dsn string) (*gorm.DB, error) { return gorm.Open(gsqlite.Open(dsn), &gorm.Config{}) },
		MigrateGame:    model.AutoMigrateGame,
	}, service.svcCtx.DB)

	ctx, err := service.scopeDBContext(healCtx(), "demo-game", "development")
	require.Error(t, err)
	assert.Nil(t, ctx)
	assert.True(t, errors.Is(err, injected), "scopeDBContext() error = %v, want injected", err)
}

// TestHealOnce_ScopeListFails：healScopes 失败时本轮直接返回，不进入页级处理。
func TestHealOnce_ScopeListFails(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	require.NoError(t, service.svcCtx.DB.Exec("DROP TABLE page_specs").Error)
	// 只验证「提前返回不 panic」：表已缺失，任何页级访问都会崩，能跑完即证明在 scope 枚举处返回。
	service.healStalePublishedPagesOnce(context.Background())
}

// TestHealOnce_ResolveScopeFailsContinues：某个 scope 的 Router.Resolve 失败
// 只跳过该 scope（continue），不中断循环、不冒泡错误。
func TestHealOnce_ResolveScopeFailsContinues(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	metaDB := service.svcCtx.DB
	dir := t.TempDir()
	injected := errors.New("injected ensure database failure")
	service.svcCtx.Router = router.New(router.Config{
		Driver:         "sqlite",
		NameForGame:    func(gameID, env string) string { return filepath.Join(dir, gameID+"_"+env+".db") },
		DSNForDatabase: func(_, dbName string) string { return dbName },
		EnsureDatabase: func(_, _, _ string) (string, error) { return "", injected },
		Open:           func(_, dsn string) (*gorm.DB, error) { return gorm.Open(gsqlite.Open(dsn), &gorm.Config{}) },
		MigrateGame:    model.AutoMigrateGame,
	}, metaDB)
	// meta 侧登记绑定（scope 枚举成功，Resolve 才会被触达）
	require.NoError(t, model.NewGameModel(metaDB).AddEnvBinding(healCtx(), "demo-game", "development", "game_demo_development", "", ""))

	// 能跑完（不 panic、不返回错误）即证明 Resolve 失败走 continue 后循环正常收尾。
	service.healStalePublishedPagesOnce(context.Background())
}

// TestHealOnce_ListPagesFailsContinues：scope 的页级 ListByScope 查询失败
// 只跳过该 scope。分库模式下先预热建库，再 DROP game 库 page_specs——
// Router 连接已缓存不会重建迁移，ListByScope 得到非 NotFound 查询错误。
func TestHealOnce_ListPagesFailsContinues(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	metaDB := service.svcCtx.DB
	newMultiGameRouter(t, service)
	require.NoError(t, model.NewGameModel(metaDB).AddEnvBinding(healCtx(), "demo-game", "development", "game_demo_development", "", ""))

	// 预热：首次 Resolve 懒建 game 库（含 page_specs 迁移），拿到缓存连接。
	gctx := healCtx()
	_, gdb, err := service.svcCtx.Router.Resolve(gctx, "demo-game", "development")
	require.NoError(t, err)
	require.NoError(t, gdb.Exec("DROP TABLE page_specs").Error)

	// 以 game 库查询计数作为「heal 已尝试页级枚举」的确定性信号。
	var listAttempts int32
	require.NoError(t, gdb.Callback().Query().After("gorm:query").Register(
		"coverage_d:heal_list_attempt", func(tx *gorm.DB) {
			if tx.Statement != nil && tx.Statement.Table == "page_specs" {
				atomic.AddInt32(&listAttempts, 1)
			}
		}))

	service.healStalePublishedPagesOnce(context.Background())
	assert.GreaterOrEqual(t, atomic.LoadInt32(&listAttempts), int32(1),
		"heal 必须已尝试该 scope 的页级枚举（失败后 continue）")
}

// seedHealSafeDriftPage 播种一个「heal-safe 漂移」的已发布页：
// gift.send v1（uid+count required）走提案接受发布，随后契约 v2 漂移移除
// uid（count 保留 required）——selector /uid 变 removed、/count 保持 kept，
// 无 rename/added/manual，healSafeReports 放行自动发布。
func seedHealSafeDriftPage(t *testing.T, service *Service, ctx context.Context) string {
	t.Helper()
	db := service.svcCtx.DB
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"

	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"uid":{"type":"string"},"count":{"type":"integer"}},"required":["uid","count"]}`,
	))
	require.NoError(t, dashboardservice.NewContractService(db).RebuildProposalForFunction(ctx, "demo-game", "development", functionID))
	published, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
	require.NoError(t, err)

	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"count":{"type":"integer"}},"required":["count"]}`,
	))
	return published.PageKey
}

// TestHealOnce_HealSafeDriftAutoPublishes：heal-safe 漂移（removed+kept）由
// 系统身份同步草稿并自动接续发布（healed 计数路径）；同一轮里
// draft-only（published_version=0）与无快照页（freshness 空）被跳过不动；
// 二轮幂等（无变化不再 bump revision）。
func TestHealOnce_HealSafeDriftAutoPublishes(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	pageKey := seedHealSafeDriftPage(t, service, ctx)

	// 干扰页 1：draft-only（published_version=0）——heal 只处理已发布页；
	// 干扰页 2：published_version=1 但无发布快照——freshness 评估为空跳过。
	db := service.svcCtx.DB
	require.NoError(t, db.Exec(`INSERT INTO page_specs (game_id, env, page_key, spec_json, published_version, draft_revision, created_at, updated_at) VALUES
		('demo-game', 'development', 'zz-draft-only', '{}', 0, 7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('demo-game', 'development', 'zz-no-snapshot', '{}', 1, 9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	page, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	require.NotEmpty(t, service.bindingFreshnessForPublishedDraft(ctx, page), "播种漂移后必须先评估为 stale")

	service.healStalePublishedPagesOnce(context.Background())

	after, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Greater(t, after.DraftRevision, page.DraftRevision, "heal-safe 漂移必须同步草稿")
	// 自动发布：线上快照前进到同步后的草稿版本，stale 清零。
	publishedRow, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.EqualValues(t, after.DraftRevision, publishedRow.Version, "heal-safe 同步后必须自动接续发布")
	assert.Empty(t, service.bindingFreshnessForPublishedDraft(healCtx(), after), "自动发布后 stale 必须清零")

	// 干扰页草稿不动（两个跳过分支均不落库）。
	draftOnly, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", "zz-draft-only")
	require.NoError(t, err)
	assert.Equal(t, 7, draftOnly.DraftRevision, "draft-only 页不得被 heal 改写")
	noSnap, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", "zz-no-snapshot")
	require.NoError(t, err)
	assert.Equal(t, 9, noSnap.DraftRevision, "无快照页不得被 heal 改写")

	// 二轮幂等：已拉齐，无变化不 bump。
	revAfter := after.DraftRevision
	service.healStalePublishedPagesOnce(context.Background())
	again, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, revAfter, again.DraftRevision, "二轮扫描必须无变化")
}

// TestHealOnce_PublishFailureKeepsStale：同步成功但发布失败（published 快照
// INSERT 被触发器阻断）时不得标记 AutoPublished——草稿已同步、线上快照
// 保持旧版、stale 留给下一轮/人工。
func TestHealOnce_PublishFailureKeepsStale(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	pageKey := seedHealSafeDriftPage(t, service, ctx)
	db := service.svcCtx.DB

	page, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	before, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)

	// 只阻断发布快照落库：selector 同步（page_specs/page_versions）不受影响。
	require.NoError(t, db.Exec(
		"CREATE TRIGGER cov_d_block_published_insert BEFORE INSERT ON published_page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected publish failure heal'); END").Error)

	service.healStalePublishedPagesOnce(context.Background())

	after, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Greater(t, after.DraftRevision, page.DraftRevision, "同步不因发布失败回滚")
	publishedRow, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, before.Version, publishedRow.Version, "发布失败时线上快照不得前进")
	assert.NotEmpty(t, service.bindingFreshnessForPublishedDraft(healCtx(), after), "发布失败后 stale 必须保持可见")
}

// TestHealOnce_SyncConflictContinues：syncSelectorsCore 落库失败（乐观锁/事务
// 注错，此处以触发器阻断 page_specs UPDATE 模拟）只跳过该页，不 panic、
// 不冒泡——stale 保留待下轮。
func TestHealOnce_SyncConflictContinues(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	pageKey := seedHealSafeDriftPage(t, service, ctx)

	page, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	require.NotEmpty(t, service.bindingFreshnessForPublishedDraft(ctx, page))

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_pagespec_update BEFORE UPDATE ON page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected sync failure heal'); END").Error)

	service.healStalePublishedPagesOnce(context.Background())

	after, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, page.DraftRevision, after.DraftRevision, "同步失败时草稿不得前进")
	assert.NotEmpty(t, service.bindingFreshnessForPublishedDraft(healCtx(), after), "失败页保持 stale 待下轮")
}

// TestHealOnce_FindFreshDraftFailsContinues：精确重载（FindByScopeAndPageKey）
// 查询失败只跳过该页。以 gorm Query 回调按「page_specs + page_key 条件」
// 注错（一次性：命中后即失效，heal 内被拦、测试的验证查询不受影响）——
// 与 ListByScope（无 page_key 条件）、scope 枚举（不走 Query 回调）区分。
func TestHealOnce_FindFreshDraftFailsContinues(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	pageKey := seedHealSafeDriftPage(t, service, ctx)

	page, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	require.NotEmpty(t, service.bindingFreshnessForPublishedDraft(ctx, page))

	// 播种完成后再注册注错回调；一次性注入保证 heal 内的 Find 被拦、
	// 之后的验证查询（同样含 page_key 条件）正常执行。
	var blocked int32
	require.NoError(t, service.svcCtx.DB.Callback().Query().After("gorm:query").Register(
		"coverage_d:block_find_by_key", func(tx *gorm.DB) {
			if tx.Error != nil || tx.Statement == nil || tx.Statement.Table != "page_specs" {
				return
			}
			if strings.Contains(tx.Statement.SQL.String(), "page_key =") &&
				atomic.CompareAndSwapInt32(&blocked, 0, 1) {
				tx.AddError(errors.New("injected find fresh draft failure"))
			}
		}))

	service.healStalePublishedPagesOnce(context.Background())
	require.Equal(t, int32(1), atomic.LoadInt32(&blocked), "heal 内必须发生一次被拦的精确重载")

	after, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, page.DraftRevision, after.DraftRevision, "精确重载失败时该页本轮不动")
}

// ---------------------------------------------------------------------------
// service.go — SetPageMenu 错误分支
// ---------------------------------------------------------------------------

// TestSetPageMenu_MissingScope：权限通过但 ctx 无 GameScope → requireScope 拒绝。
func TestSetPageMenu_MissingScope(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	// 有身份（权限链需要）但无 scope。
	ctx := context.WithValue(context.Background(), "username", "page_tester")
	menuID := int64(1)
	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: &menuID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID")
}

// TestSetPageMenu_NilMenuModel：MenuModel 未装配（svcCtx 字段置空）时
// 挂载请求报内部错误而不是 panic。
func TestSetPageMenu_NilMenuModel(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	saveTestPageDraft(t, service, ctx)
	// 装配缺口模拟：MenuModel 缺失（svcCtx 其余模型保留，页面链路可用）。
	service.svcCtx.MenuModel = nil
	menuID := int64(1)
	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: &menuID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "菜单模型未初始化")
}

// TestSetPageMenu_FindByIDQueryError：菜单查询返回非 NotFound 错误
// （gorm 回调按 menu_items 表注错，一次性注入）时原样上抛。
func TestSetPageMenu_FindByIDQueryError(t *testing.T) {
	service, ctx, menuModel := newMenuAwarePageService(t, "pages:edit")
	menu := createTestMenu(t, menuModel, ctx, "resource")
	saveTestPageDraft(t, service, ctx)

	require.NoError(t, service.svcCtx.DB.Callback().Query().After("gorm:query").Register(
		"coverage_d:block_menu_find", func(tx *gorm.DB) {
			if tx.Error != nil || tx.Statement == nil || tx.Statement.Table != "menu_items" {
				return
			}
			tx.AddError(errors.New("injected menu find failure"))
		}))

	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: ptrInt64(int64(menu.ID))})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected menu find failure")
}

// TestSetPageMenu_UpdateMenuIDError：菜单关联落库失败（触发器阻断
// page_specs UPDATE）时错误透传。
func TestSetPageMenu_UpdateMenuIDError(t *testing.T) {
	service, ctx, menuModel := newMenuAwarePageService(t, "pages:edit", "pages:read")
	menu := createTestMenu(t, menuModel, ctx, "resource")
	saveTestPageDraft(t, service, ctx)

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_menu_update BEFORE UPDATE ON page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected menu update failure'); END").Error)

	resp, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: ptrInt64(int64(menu.ID))})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "injected menu update failure")
}

// ---------------------------------------------------------------------------
// service.go — autoPublishAfterSync 守卫与降级分支
// ---------------------------------------------------------------------------

// TestAutoPublishAfterSync_GuardShortCircuits：四类前置守卫（resp 为 nil、
// 未 Applied、页面从未发布、存在 manual 遗留）直接返回，无任何副作用。
func TestAutoPublishAfterSync_GuardShortCircuits(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")

	// nil resp 不 panic。
	service.autoPublishAfterSync(ctx, nil, "demo-game", "development", "page_tester", 1, 1)

	// 未 Applied。
	resp := &PageSyncSelectorsResponse{PageKey: "player.manage", Applied: false}
	service.autoPublishAfterSync(ctx, resp, "demo-game", "development", "page_tester", 1, 1)
	assert.False(t, resp.AutoPublished)
	assert.Empty(t, resp.AutoPublishError)

	// 页面从未发布（publishedVersion=0）。
	resp = &PageSyncSelectorsResponse{PageKey: "player.manage", Applied: true}
	service.autoPublishAfterSync(ctx, resp, "demo-game", "development", "page_tester", 0, 1)
	assert.False(t, resp.AutoPublished)
	assert.Empty(t, resp.AutoPublishError, "未发布页不触发权限/发布分支")

	// 存在 manual 遗留（governance 漂移）：绝不自动发布。
	resp = &PageSyncSelectorsResponse{
		PageKey:        "player.manage",
		Applied:        true,
		SyncedBindings: []spec.BindingSelectorSyncReport{{Manual: []spec.Diagnostic{{Code: "binding_governance_stale"}}}},
	}
	service.autoPublishAfterSync(ctx, resp, "demo-game", "development", "page_tester", 1, 1)
	assert.False(t, resp.AutoPublished)
	assert.Empty(t, resp.AutoPublishError)
}

// TestAutoPublishAfterSync_PublishPermissionMissing：同步适配完整但 ctx 无
// pages:publish 权限 → 降级提示，草稿保持已同步。
func TestAutoPublishAfterSync_PublishPermissionMissing(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	resp := &PageSyncSelectorsResponse{PageKey: "player.manage", Applied: true}
	service.autoPublishAfterSync(ctx, resp, "demo-game", "development", "page_tester", 1, 1)
	assert.False(t, resp.AutoPublished)
	assert.Equal(t, "publish_permission_required", resp.AutoPublishError)
}

// TestAutoPublishAfterSync_PublishFailure：发布失败（页面不存在）→ 错误文案
// 降级进 AutoPublishError，不 panic、不标记 AutoPublished。
func TestAutoPublishAfterSync_PublishFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	resp := &PageSyncSelectorsResponse{PageKey: "no.such.page", Applied: true}
	service.autoPublishAfterSync(ctx, resp, "demo-game", "development", "page_tester", 1, 1)
	assert.False(t, resp.AutoPublished)
	assert.Contains(t, resp.AutoPublishError, "no.such.page")
}

// ---------------------------------------------------------------------------
// service.go — AutoPublishComposite 错误分支
// ---------------------------------------------------------------------------

// TestAutoPublishComposite_LoadDraftQueryError：已存在页判定查询返回非
// NotFound 错误（gorm 回调按 page_specs + page_key 条件注错；页面必须真实
// 存在——回调在 After 阶段只对成功查询注入，NotFound 查询由既有分支处理）
// → 包装上抛。
func TestAutoPublishComposite_LoadDraftQueryError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)
	// 先发布制造「已存在页」，使被测查询命中存在的行（tx.Error==nil）才会被注入。
	_, err := dashboardservice.NewProposalService(service.svcCtx.DB).
		AcceptAndPublishProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Callback().Query().After("gorm:query").Register(
		"coverage_d:block_apc_load_draft", func(tx *gorm.DB) {
			if tx.Error != nil || tx.Statement == nil || tx.Statement.Table != "page_specs" {
				return
			}
			if strings.Contains(tx.Statement.SQL.String(), "page_key =") {
				tx.AddError(errors.New("injected load draft failure"))
			}
		}))

	err = service.AutoPublishComposite(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load page draft")
	assert.Contains(t, err.Error(), "injected load draft failure")
}

// TestAutoPublishComposite_AcceptAndPublishError：新页面（无草稿）走提案
// 接受发布链，发布快照落库被触发器阻断 → 错误上抛，不产生半发布状态。
func TestAutoPublishComposite_AcceptAndPublishError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_apc_publish BEFORE INSERT ON published_page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected accept publish failure'); END").Error)

	err := service.AutoPublishComposite(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected accept publish failure")
}

// TestAutoPublishComposite_CurrentUsernameError：已存在页路径的身份解析失败
// （currentUsername 接缝注入）→ 原样上抛。
func TestAutoPublishComposite_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)
	// 先以正常身份发布，制造「已存在页」路径。
	_, err := dashboardservice.NewProposalService(service.svcCtx.DB).
		AcceptAndPublishProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	injected := errors.New("injected username failure apc")
	withCurrentUsernameError(t, injected)
	err = service.AutoPublishComposite(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.True(t, errors.Is(err, injected), "AutoPublishComposite() error = %v, want injected", err)
}

// TestAutoPublishComposite_RegenerateError：已存在页路径的草稿重生成失败
// （触发器阻断 page_specs UPDATE）→ 上抛，页面保持原发布状态。
func TestAutoPublishComposite_RegenerateError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)
	_, err := dashboardservice.NewProposalService(service.svcCtx.DB).
		AcceptAndPublishProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_apc_regen BEFORE UPDATE ON page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected regenerate failure apc'); END").Error)

	err = service.AutoPublishComposite(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected regenerate failure apc")
}

// TestAutoPublishComposite_PublishError：已存在页路径的重生成成功但发布失败
// （published 快照 INSERT 被阻断）→ 上抛。
func TestAutoPublishComposite_PublishError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)
	_, err := dashboardservice.NewProposalService(service.svcCtx.DB).
		AcceptAndPublishProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	// 重生成需要新鲜提案与漂移契约。
	driftFunctionContract(t, service)

	// 只阻断发布快照：重生成（page_specs/page_versions）不受影响。
	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_apc_publish2 BEFORE INSERT ON published_page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected publish failure apc'); END").Error)

	err = service.AutoPublishComposite(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected publish failure apc")
}

// ---------------------------------------------------------------------------
// service.go — BulkRepublish 错误分支
// ---------------------------------------------------------------------------

// TestBulkRepublish_InboxError：未显式指定 pageKeys 时目标集合来自审批收件箱，
// Inbox 查询失败（page_proposals 表缺失）→ 整体失败。
func TestBulkRepublish_InboxError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	require.NoError(t, service.svcCtx.DB.Exec("DROP TABLE page_proposals").Error)

	resp, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// TestBulkRepublish_RegenerateFailure：单页重生成失败（触发器阻断
// page_specs UPDATE）计入 Failed 并继续，不中断批量入口。
func TestBulkRepublish_RegenerateFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)
	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)
	driftFunctionContract(t, service)

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_repub_regen BEFORE UPDATE ON page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected regenerate failure bulk-republish'); END").Error)

	resp, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{})
	require.NoError(t, err, "单页失败不冒泡")
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "resource--player", resp.Failed[0]["pageKey"])
	assert.Contains(t, resp.Failed[0]["error"], "injected regenerate failure bulk-republish")
	assert.Empty(t, resp.Published)
}

// TestBulkRepublish_CurrentUsernameFailure：单页身份解析失败（接缝顺序注入：
// regenerateDraft 内首次调用成功、BulkRepublish 自身调用失败）计入 Failed。
// 该分支在真实请求下不可达（权限链先行校验 username），接缝注入是唯一
// 确定性驱动手段。
func TestBulkRepublish_CurrentUsernameFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)
	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)
	driftFunctionContract(t, service)

	injected := errors.New("injected username failure bulk-republish")
	orig := currentUsername
	var calls int32
	currentUsername = func(c context.Context) (string, error) {
		// 第 1 次调用发生在 regenerateDraft 内（须成功），第 2 次是
		// BulkRepublish 自身（注入失败）——单线程顺序调用，计数确定。
		if atomic.AddInt32(&calls, 1) >= 2 {
			return "", injected
		}
		return orig(c)
	}
	t.Cleanup(func() { currentUsername = orig })

	resp, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{PageKeys: []string{"resource--player"}})
	require.NoError(t, err)
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "resource--player", resp.Failed[0]["pageKey"])
	assert.Contains(t, resp.Failed[0]["error"], "injected username failure bulk-republish")
}

// TestBulkRepublish_PublishFailure：单页发布失败（published 快照 INSERT 被阻断）
// 计入 Failed 并继续。
func TestBulkRepublish_PublishFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)
	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)
	driftFunctionContract(t, service)

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER cov_d_block_repub_publish BEFORE INSERT ON published_page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected publish failure bulk-republish'); END").Error)

	resp, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{PageKeys: []string{"resource--player"}})
	require.NoError(t, err)
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "resource--player", resp.Failed[0]["pageKey"])
	assert.Contains(t, resp.Failed[0]["error"], "injected publish failure bulk-republish")
	assert.Empty(t, resp.Published)
}

// ---------------------------------------------------------------------------
// service.go — BulkSyncSelectors 错误分支
// ---------------------------------------------------------------------------

// TestBulkSyncSelectors_MissingScope：权限通过但 ctx 无 GameScope →
// requireScope 拒绝（X-Game-ID 缺失）。
func TestBulkSyncSelectors_MissingScope(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	// 有身份（权限链需要）但无 scope。
	ctx := context.WithValue(context.Background(), "username", "page_tester")
	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "X-Game-ID")
}

// TestBulkSyncSelectors_CurrentUsernameError：身份解析失败（接缝注入；权限
// 链走 logicutils.CurrentUsername 不经此接缝，可恒定注入）→ 整体失败。
func TestBulkSyncSelectors_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	injected := errors.New("injected username failure bulk-sync")
	withCurrentUsernameError(t, injected)

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected))
}

// TestBulkSyncSelectors_InboxError：pageKeys 为空时目标集合来自审批收件箱，
// Inbox 查询失败 → 整体失败。
func TestBulkSyncSelectors_InboxError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	require.NoError(t, service.svcCtx.DB.Exec("DROP TABLE page_proposals").Error)

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// TestBulkSyncSelectors_DryRunFailure：dry-run 判定失败（草稿 spec_json 损坏，
// pageSpecFromModel 解析失败）计入 Failed，不进入 apply。
func TestBulkSyncSelectors_DryRunFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	seedResourceProposal(t, service)
	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)
	const pageKey = "resource--player"

	draftBefore, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)

	// 损坏草稿的 canonical PageSpec：dry-run 的 pageSpecFromModel 直接失败。
	require.NoError(t, service.svcCtx.DB.Exec(
		"UPDATE page_specs SET spec_json = 'not-json' WHERE game_id = ? AND env = ? AND page_key = ?",
		"demo-game", "development", pageKey).Error)

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{PageKeys: []string{pageKey}})
	require.NoError(t, err)
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, pageKey, resp.Failed[0]["pageKey"])

	draftAfter, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, draftBefore.DraftRevision, draftAfter.DraftRevision, "dry-run 失败不得落库")
}

// ---------------------------------------------------------------------------
// handler.go — BulkSyncSelectors 错误透传分支
// ---------------------------------------------------------------------------

// TestHandler_BulkSyncSelectors_ServiceError：请求体合法（空 JSON）但 service
// 层失败（请求 ctx 无身份，权限链拒绝）→ 统一错误响应（非 2xx）。
func TestHandler_BulkSyncSelectors_ServiceError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	handler := NewHandler(service)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages/bulk-sync-selectors", "{}")
	handler.BulkSyncSelectors(ctx)

	assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest,
		"service 失败必须走错误分支而非 Success，body=%s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "error")
}
