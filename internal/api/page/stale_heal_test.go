package page

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
)

func healCtx() context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
}

// TestStaleHeal_RenameSyncsDraftOnlyThenUserAutoPublishes：契约漂移
// （uid→player_id rename）→ 系统愈合只同步草稿（rename 属语义判断，防
// 「删旧+增新」误映射，绝不机器发布）；用户在编辑器一键同步（具备发布
// 权限）自动接续发布，stale 清零；二轮扫描幂等。
func TestStaleHeal_RenameSyncsDraftOnlyThenUserAutoPublishes(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	db := service.svcCtx.DB
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"

	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"uid":{"type":"string"},"count":{"type":"integer"}},"required":["uid","count"]}`,
	))
	require.NoError(t, dashboardservice.NewContractService(db).RebuildProposalForFunction(ctx, "demo-game", "development", functionID))
	published, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
	require.NoError(t, err)
	pageKey := published.PageKey

	// v2 漂移：uid → player_id（发布页 stale）
	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"player_id":{"type":"string"},"count":{"type":"integer"}},"required":["player_id","count"]}`,
	))
	page, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	require.NotEmpty(t, service.bindingFreshnessForPublishedDraft(ctx, page), "seeded drift must be stale before heal")

	service.healStalePublishedPagesOnce(context.Background())

	pageAfter, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(healCtx(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Greater(t, pageAfter.DraftRevision, page.DraftRevision, "heal must sync the drifted draft")
	assert.Equal(t, page.PublishedVersion, pageAfter.PublishedVersion, "rename 属语义决策：系统愈合不得自动发布")
	assert.NotEmpty(t, service.bindingFreshnessForPublishedDraft(healCtx(), pageAfter), "published 未前进前 stale 保持可见")

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
	service.healStalePublishedPagesOnce(context.Background())
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
	published, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
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

	service.healStalePublishedPagesOnce(context.Background())

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
		service.StartStaleHealLoop(ctx, time.Millisecond)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop must exit on ctx cancel")
	}
	var nilSvc *Service
	nilSvc.StartStaleHealLoop(healCtx(), time.Second) // 不得 panic
}
