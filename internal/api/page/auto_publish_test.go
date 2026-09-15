package page

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
)

// T10 发布分级：AutoPublishComposite 的三形态。
// 权限语义：不做 pages:publish 检查——env 已由 pages.publishReview=auto
// 策略声明免审核，权限由保存入口（pages:edit）覆盖，此处 fixture 只带
// pages:edit 锁定该契约。
func latestProposalForPage(t *testing.T, service *Service, ctx context.Context, pageKey string) *model.PageProposal {
	t.Helper()
	proposal, err := model.NewPageProposalModel(service.svcCtx.DB).
		FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	return proposal
}

// 形态一：提案带 error 级诊断 → 拒绝发布（质量门槛不因免审核降低），
// 页面保持未发布。
func TestAutoPublishComposite_BlockingDiagnosticsRejected(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	proposal := latestProposalForPage(t, service, ctx, "resource--player")
	proposal.Diagnostics = model.JSON([]byte(
		`[{"code":"schema_invalid","severity":"error","message":"output schema is not a valid object","field":"outputSchema"}]`))
	require.NoError(t, model.NewPageProposalModel(service.svcCtx.DB).UpsertProposal(ctx, proposal))

	err := service.AutoPublishComposite(ctx, "demo-game", "development", proposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocking diagnostics")

	published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.Error(t, err, "诊断拒绝时不得产生发布快照")
	assert.Nil(t, published)
}

// 形态二：新页面（无草稿）→ 走提案接受发布链，published_page_specs 落库。
func TestAutoPublishComposite_NewPageAcceptsAndPublishes(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	proposal := latestProposalForPage(t, service, ctx, "resource--player")
	require.NoError(t, service.AutoPublishComposite(ctx, "demo-game", "development", proposal.ProposalKey))

	published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	assert.True(t, published.Active)
	assert.Equal(t, "resource--player", published.PageKey)
}

// 形态三：已存在页面 → 提案重生成草稿 + 发布（与 BulkRepublish 单页组合
// 同路径：乐观锁、快照与版本历史照常）。发布后线上快照对齐漂移契约。
// 数据准备走 service 层 accept-and-publish（无权限检查），fixture 保持
// 只带 pages:edit——被测对象 AutoPublishComposite 自身不查 pages:publish。
func TestAutoPublishComposite_ExistingPageRegeneratesAndPublishes(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	proposal := latestProposalForPage(t, service, ctx, "resource--player")
	_, err := contractsvc.NewProposalService(service.svcCtx.DB).
		AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.NoError(t, err)
	before, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)

	driftFunctionContract(t, service)
	drifted := latestProposalForPage(t, service, ctx, "resource--player")

	require.NoError(t, service.AutoPublishComposite(ctx, "demo-game", "development", drifted.ProposalKey))

	after, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	assert.True(t, after.Active)
	assert.Greater(t, after.Version, before.Version, "重发布必须产生新版本")
}

// 提案不存在：加载失败明确报错（配置漂移/误删提案的降级路径）。
func TestAutoPublishComposite_MissingProposalFails(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")

	err := service.AutoPublishComposite(ctx, "demo-game", "development", "resource:nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load proposal")
}
