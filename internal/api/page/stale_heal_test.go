package page

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/router"
	gsqlite "github.com/glebarez/sqlite"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
)

func healCtx() context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
}

// TestStaleHeal_RenameProposesOnlyThenUserAutoPublishes：契约漂移
// （uid→player_id rename）属语义判断，系统愈合默认只落提案：不同步草稿、
// 不发布、stale 保持可见；用户在编辑器带发布权限一键同步后自动接续发布，
// stale 清零；二轮扫描幂等。
func TestStaleHeal_RenameProposesOnlyThenUserAutoPublishes(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	db := service.svcCtx.DB
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"

	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"uid":{"type":"string"},"count":{"type":"integer"}},"required":["uid","count"]}`,
	))
	require.NoError(t, dashboardservice.NewContractService(db).RebuildProposalForFunction(ctx, "demo-game", "development", functionID))
	published, err := service.newInternalProposalServiceForContext(db).
		AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
	require.NoError(t, err)
	pageKey := published.PageKey

	// v2 漂移：uid → player_id（发布页 stale）
	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"player_id":{"type":"string"},"count":{"type":"integer"}},"required":["player_id","count"]}`,
	))
	page, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	require.NotEmpty(t, service.bindingFreshnessForPublishedDraft(ctx, page), "seeded drift must be stale before heal")

	service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{})

	pageAfter, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, page.DraftRevision, pageAfter.DraftRevision, "系统愈合不得改写人工草稿（rename 属语义决策）")
	assert.Equal(t, page.SpecJSON, pageAfter.SpecJSON)
	assert.Equal(t, page.PublishedVersion, pageAfter.PublishedVersion, "rename 属语义决策：系统愈合不得自动发布")
	assert.NotEmpty(t, service.bindingFreshnessForPublishedDraft(healCtx(), pageAfter), "published 未前进前 stale 保持可见")
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err, "rename 漂移必须留下待人工审核的提案")
	assert.Contains(t, string(proposal.PageSpec), "/player_id", "提案必须携带 rename 推断结果")

	// 用户一键同步（ctx 具备 publish 权限）→ 自动接续发布，stale 清零
	rev := pageAfter.DraftRevision
	syncResp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{PageKey: pageKey, DraftRevision: &rev})
	require.NoError(t, err)
	assert.True(t, syncResp.AutoPublished, "user sync with publish permission must auto-publish: %s", syncResp.AutoPublishError)
	pagePublished, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Empty(t, service.bindingFreshnessForPublishedDraft(healCtx(), pagePublished), "sync+auto publish must clear stale")

	// 幂等：二轮扫描无变化不得再 bump revision（护栏）。
	revAfter := pagePublished.DraftRevision
	service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{})
	pageAgain, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, revAfter, pageAgain.DraftRevision, "second round must be a no-op")
}

// TestStaleHeal_LeavesManualRequiredAlone：新 schema 存在 selector 无法
// 自动映射的 required 字段时，愈合循环不得自动发布（stale 保留给编辑器）。
func TestStaleHeal_LeavesManualRequiredAlone(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	db := service.svcCtx.DB
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"

	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"uid":{"type":"string"}},"required":["uid"]}`,
	))
	require.NoError(t, dashboardservice.NewContractService(db).RebuildProposalForFunction(ctx, "demo-game", "development", functionID))
	published, err := service.newInternalProposalServiceForContext(db).
		AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
	require.NoError(t, err)
	pageKey := published.PageKey

	// v2：uid 消失，新 required couponCode 无同名表单字段可映射
	rebuildFunctionContract(t, db, ctx, functionID, reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		InputSchema:  `{"type":"object","properties":{"couponCode":{"type":"string"}},"required":["couponCode"]}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	})

	service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{})

	after, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	// 底线：绝不自动发布——published 快照仍指向旧版本，stale 必须持续可见
	stale := service.bindingFreshnessForPublishedDraft(healCtx(), after)
	require.NotEmpty(t, stale, "manual-required drift must never be healed silently")
	var hasRequired bool
	for _, s := range stale {
		if s.Diagnostic.Code == "selector_required_stale" {
			hasRequired = true
		}
	}
	assert.True(t, hasRequired, "required gap must stay visible for manual mapping")
}

func TestStaleHeal_LoopLifecycle(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(healCtx())
	go func() {
		service.StartStaleHealLoop(ctx, time.Millisecond, healStaleOptions{})
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop must exit on ctx cancel")
	}
	var nilSvc *Service
	nilSvc.StartStaleHealLoop(healCtx(), time.Second, healStaleOptions{}) // 不得 panic
}

// TestStaleHeal_MultiGameHealsAcrossGameDBs（U1）：分库模式 page_specs 在
// game 库、meta 库无行——旧实现在 meta 直查得零 scope 静默跳过，愈合永不
// 执行。修复后 scope 从 game_envs 绑定枚举，页级查询经 Router 落到 game 库：
// 默认只把提案写进 game 库（人工草稿不动），显式自动发布时独立修订同样
// 落在 game 库并清零 stale。
func TestStaleHeal_MultiGameHealsAcrossGameDBs(t *testing.T) {
	service, gctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	metaDB := service.svcCtx.DB

	dir := t.TempDir()
	service.svcCtx.Router = router.New(router.Config{
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
	}, metaDB)

	// meta 侧登记绑定（分库 scope 枚举源）
	require.NoError(t, model.NewGameModel(metaDB).AddEnvBinding(gctx, "demo-game", "development", "game_demo_development", "", ""))

	// game 库内播种：发布 gift.send 页 → 契约漂移（uid→player_id rename）。
	// seedCtx 与 heal 循环同构（scope + dbctx game 库覆盖），播种与断言都
	// 经它落到 game 库。
	seedCtx, err := service.scopeDBContext(gctx, "demo-game", "development")
	require.NoError(t, err)
	_, gdb, err := service.svcCtx.Router.Resolve(gctx, "demo-game", "development")
	require.NoError(t, err)
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"
	rebuildFunctionContract(t, gdb, seedCtx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"uid":{"type":"string"},"count":{"type":"integer"}},"required":["uid","count"]}`,
	))
	require.NoError(t, dashboardservice.NewContractService(gdb).RebuildProposalForFunction(seedCtx, "demo-game", "development", functionID))
	published, err := service.newInternalProposalServiceForContext(gdb).
		AcceptAndPublishProposal(seedCtx, "demo-game", "development", proposalKey)
	require.NoError(t, err)
	pageKey := published.PageKey

	rebuildFunctionContract(t, gdb, seedCtx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"player_id":{"type":"string"},"count":{"type":"integer"}},"required":["player_id","count"]}`,
	))

	pageBefore, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(seedCtx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	require.NotEmpty(t, service.bindingFreshnessForPublishedDraft(seedCtx, pageBefore), "seeded drift must be stale before heal")

	// 默认路径：提案必须写进 game 库，草稿与发布快照不动。
	service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{})
	pageAfter, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(seedCtx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, pageBefore.DraftRevision, pageAfter.DraftRevision, "默认路径不得改写人工草稿")
	assert.Equal(t, pageBefore.PublishedVersion, pageAfter.PublishedVersion, "rename 属语义决策：系统愈合不得自动发布")
	proposal, err := model.NewPageProposalModel(gdb).FindByScopeAndPageKey(seedCtx, "demo-game", "development", pageKey)
	require.NoError(t, err, "分库模式下提案必须落在 game 库")
	assert.Contains(t, string(proposal.PageSpec), "/player_id")

	// 显式自动发布（安全漂移）：独立修订落在 game 库并清零 stale。
	// 先把 rename 换成 heal-safe 的 removed+kept 漂移。
	rebuildFunctionContract(t, gdb, gctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"count":{"type":"integer"}},"required":["count"]}`,
	))
	service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{AllowAutoPublish: true})
	pagePublished, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(seedCtx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Greater(t, pagePublished.PublishedVersion, pageBefore.PublishedVersion, "分库模式下自动发布必须写回 game 库的发布指针")
	assert.Empty(t, service.bindingFreshnessForPublishedDraft(seedCtx, pagePublished), "自动发布后 stale 必须清零")
}

// 单库路径不回归：Router 为 nil 时仍从 page_specs 枚举 scope（含多 scope）。
func TestStaleHealScopes_SingleDBEnumeratesFromPageSpecs(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	db := service.svcCtx.DB
	require.NoError(t, db.Exec(`INSERT INTO page_specs (game_id, env, page_key, spec_json, published_version, draft_revision, created_at, updated_at) VALUES
		('g1', 'prod', 'p1', '{}', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('g2', 'dev',  'p2', '{}', 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	scopes, err := service.healScopes(context.Background(), db)
	require.NoError(t, err)
	require.Len(t, scopes, 1, "只枚举有发布快照的 scope")
	assert.Equal(t, "g1", scopes[0].GameID)
	assert.Equal(t, "prod", scopes[0].Env)
}
