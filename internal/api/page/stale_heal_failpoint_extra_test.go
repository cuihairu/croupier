package page

// stale_heal.go 与 service.go 补测（failpoint/守卫口径）：愈合循环的
// 计划失败与提案失败跳过臂、planStaleHealRevision 的无变化 nil 臂与
// 快照读取失败臂、publishStaleHealRevision 的 nil 校验臂、
// publishPublishedBaseRevision 的五个模型错误臂、upsertStaleHealProposal
// 的 nil/查询失败/质量降级/写入失败臂；以及 service.go 的
// validatePageSpec scope 缺失、functionsForScope 查询失败、
// syncSelectorsCore 的系统愈合幂等护栏、版本化适配器转发、
// PublishFromEntry 的权限失败与 scope/actor 解析成功臂、
// ValidateVersioningRollbackTarget 的坏快照解析失败臂。
//
// 诚实跳过：
//   - 愈合循环的 revision==nil 继续臂（147）：bindingFreshness 报漂移才进
//     planStaleHealRevision，而漂移必然产生 changed/manual 项，nil 仅防御态。
//   - planStaleHealRevision 的 planSelectorSync 错误臂（252）：其唯一错误源
//     是 pageSpecFromModel(baseModel) 遇非法 SpecJSON，而 applyPageSpecToModel
//     恒产出合法 JSON，互斥。
//   - planStaleHealRevision 的 applyPageSpecToModel 错误臂（242）、
//     marshalPageSpec 错误臂（259）、buildBindingContracts 错误臂（263）、
//     pageJSONMarshal 错误臂（267）与 upsert 的 marshal 臂（394/405/424/428）：
//     输入均为可序列化结构体或经 parsePublishedPageForFreshness 的成功快照，
//     marshal/apply 恒成功；buildBindingContracts 失败仅当绑定引用不存在
//     函数，而规划侧已按当前契约重建绑定，二者互斥。
//   - publishEntryCore 的 buildBindingContracts/pageJSONMarshal 错误臂
//     （2096-2097/2099-2100）：同 validateBinding 先行诊断的互斥性。

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// pageDBFailpoint 按表内第 n 次指定操作注入确定性错误。
type pageDBFailpoint struct {
	table string
	op    string // query / update / create
	n     int
	err   error
}

func applyPageFailpoint(t *testing.T, db *gorm.DB, fp pageDBFailpoint) {
	t.Helper()
	var hits int64
	reg := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != fp.table {
			return
		}
		hits++
		if hits == int64(fp.n) {
			_ = tx.AddError(fp.err)
		}
	}
	name := fmt.Sprintf("test:heal_%s_%s", fp.table, fp.op)
	switch fp.op {
	case "query":
		require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg))
	case "update":
		require.NoError(t, db.Callback().Update().After("gorm:update").Register(name, reg))
	case "create":
		require.NoError(t, db.Callback().Create().After("gorm:create").Register(name, reg))
	default:
		t.Fatalf("unknown failpoint op %q", fp.op)
	}
}

var errHealFailpoint = errors.New("failpoint: heal storage down")

// seedStaleHealPage 建立「gift.send 契约 → 已发布页面」基线；drift=true 时
// 再改契约（uid→player_id）使发布页 stale。
func seedStaleHealPage(t *testing.T, drift bool) (*Service, context.Context, string, *gorm.DB) {
	t.Helper()
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
	if drift {
		rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
			`{"type":"object","properties":{"player_id":{"type":"string"},"count":{"type":"integer"}},"required":["player_id","count"]}`,
		))
	}
	return service, ctx, published.PageKey, db
}

// 愈合循环：planStaleHealRevision 快照读取失败 → continue（144, 235）。
// 注意 published_page_specs 查询 n=2：bindingFreshnessForPublishedDraft
// （1647）先查一次，planStaleHealRevision 内（233）为第二次。
func TestHealLoop_PlanRevisionError(t *testing.T) {
	service, _, _, db := seedStaleHealPage(t, true)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "published_page_specs", op: "query", n: 2, err: errHealFailpoint})
	require.NotPanics(t, func() {
		service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{})
	})
}

// 愈合循环：upsertStaleHealProposal 写入失败 → continue（161, 435）。
func TestHealLoop_UpsertProposalError(t *testing.T) {
	service, _, _, db := seedStaleHealPage(t, true)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "page_proposals", op: "create", n: 1, err: errHealFailpoint})
	require.NotPanics(t, func() {
		service.healStalePublishedPagesOnce(context.Background(), healStaleOptions{})
	})
}

// planStaleHealRevision：无变化页面返回 nil（255）。
func TestPlanStaleHealRevision_NoChangeNil(t *testing.T) {
	service, ctx, pageKey, _ := seedStaleHealPage(t, false)
	draft, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	rev, err := service.planStaleHealRevision(ctx, "demo-game", "development", draft)
	require.NoError(t, err)
	assert.Nil(t, rev, "无漂移页面规划结果必须为 nil（不落草稿/提案）")
}

// planStaleHealRevision：快照读取失败透传（235）。
func TestPlanStaleHealRevision_ReadError(t *testing.T) {
	service, ctx, pageKey, db := seedStaleHealPage(t, false)
	draft, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "published_page_specs", op: "query", n: 1, err: errHealFailpoint})
	_, err = service.planStaleHealRevision(ctx, "demo-game", "development", draft)
	require.ErrorIs(t, err, errHealFailpoint)
}

// publishStaleHealRevision：nil 修订 → bad request（293）。
func TestPublishStaleHealRevision_NilRevision(t *testing.T) {
	service, ctx, _, _ := seedStaleHealPage(t, false)
	_, err := service.publishStaleHealRevision(ctx, "demo-game", "development", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale heal revision is required")
}

// publishPublishedBaseRevision：五个模型错误臂（315/325/328/360/363）。
func TestPublishPublishedBaseRevision_Arms(t *testing.T) {
	cases := []struct {
		name    string
		fp      *pageDBFailpoint
		dropTbl string
	}{
		{"findLatest", &pageDBFailpoint{table: "published_page_specs", op: "query", n: 1, err: errHealFailpoint}, ""},
		{"nextVersion", nil, "page_versions"}, // Scan 表位不稳，DropTable 确定性触发
		{"deactivate", &pageDBFailpoint{table: "published_page_specs", op: "update", n: 1, err: errHealFailpoint}, ""},
		{"create", &pageDBFailpoint{table: "published_page_specs", op: "create", n: 1, err: errHealFailpoint}, ""},
		{"createVersion", &pageDBFailpoint{table: "page_versions", op: "create", n: 1, err: errHealFailpoint}, ""},
		{"updatePointer", &pageDBFailpoint{table: "page_specs", op: "update", n: 1, err: errHealFailpoint}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, ctx, pageKey, db := seedStaleHealPage(t, false)
			published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
			require.NoError(t, err)
			rev := &staleHealRevision{
				BaseVersion:   published.Version,
				Page:          spec.PageSpec{PageKey: pageKey},
				SpecJSON:      `{}`,
				ContractsJSON: `[]`,
				Reports:       []spec.BindingSelectorSyncReport{},
				Remaining:     []spec.Diagnostic{},
			}
			if tc.dropTbl != "" {
				require.NoError(t, db.Migrator().DropTable(tc.dropTbl))
			} else {
				applyPageFailpoint(t, db, *tc.fp)
			}
			_, err = service.publishPublishedBaseRevision(ctx, "demo-game", "development", staleHealActor, rev)
			require.Error(t, err)
		})
	}
}

// publishPublishedBaseRevision：基线版本冲突（319-321 分支）。
func TestPublishPublishedBaseRevision_BaseConflict(t *testing.T) {
	service, ctx, pageKey, _ := seedStaleHealPage(t, false)
	rev := &staleHealRevision{
		BaseVersion:   999,
		Page:          spec.PageSpec{PageKey: pageKey},
		SpecJSON:      `{}`,
		ContractsJSON: `[]`,
		Reports:       []spec.BindingSelectorSyncReport{},
		Remaining:     []spec.Diagnostic{},
	}
	_, err := service.publishPublishedBaseRevision(ctx, "demo-game", "development", staleHealActor, rev)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published base changed")
}

// upsertStaleHealProposal：nil 修订（390）。
func TestUpsertStaleHealProposal_NilRevision(t *testing.T) {
	service, ctx, _, _ := seedStaleHealPage(t, false)
	_, err := service.upsertStaleHealProposal(ctx, "demo-game", "development", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale heal revision is required")
}

// upsertStaleHealProposal：FindByScopeAndKey 非 not-found 错误透传（401）。
func TestUpsertStaleHealProposal_FindError(t *testing.T) {
	service, ctx, pageKey, db := seedStaleHealPage(t, false)
	rev := healRevisionFixture(pageKey)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "page_proposals", op: "query", n: 1, err: errHealFailpoint})
	_, err := service.upsertStaleHealProposal(ctx, "demo-game", "development", rev)
	require.ErrorIs(t, err, errHealFailpoint)
}

// upsertStaleHealProposal：剩余诊断 >0 → 质量降级 needs_review（432）。
func TestUpsertStaleHealProposal_NeedsReview(t *testing.T) {
	service, ctx, pageKey, _ := seedStaleHealPage(t, false)
	rev := healRevisionFixture(pageKey)
	rev.Remaining = []spec.Diagnostic{{Code: "selector_required_stale", Severity: spec.SeverityError}}
	created, err := service.upsertStaleHealProposal(ctx, "demo-game", "development", rev)
	require.NoError(t, err)
	assert.True(t, created)
	proposal, err := model.NewPageProposalModel(service.svcCtx.DB).
		FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, "needs_review", proposal.Quality)
}

// upsertStaleHealProposal：UpsertProposal 写入失败（435）。
func TestUpsertStaleHealProposal_UpsertError(t *testing.T) {
	service, ctx, pageKey, db := seedStaleHealPage(t, false)
	rev := healRevisionFixture(pageKey)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "page_proposals", op: "create", n: 1, err: errHealFailpoint})
	_, err := service.upsertStaleHealProposal(ctx, "demo-game", "development", rev)
	require.ErrorIs(t, err, errHealFailpoint)
}

// upsertStaleHealProposal：版本快照写入失败（445-446）。
func TestUpsertStaleHealProposal_VersionError(t *testing.T) {
	service, ctx, pageKey, db := seedStaleHealPage(t, false)
	rev := healRevisionFixture(pageKey)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "page_proposal_versions", op: "create", n: 1, err: errHealFailpoint})
	_, err := service.upsertStaleHealProposal(ctx, "demo-game", "development", rev)
	require.ErrorIs(t, err, errHealFailpoint)
}

func healRevisionFixture(pageKey string) *staleHealRevision {
	return &staleHealRevision{
		BaseVersion: 1,
		Page: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: "player",
			Title:       map[string]string{"zh-CN": "玩家管理", "en-US": "Player Management"},
			Category:    spec.PageCategorySpec{Key: "player"},
		},
		SpecJSON:      `{}`,
		ContractsJSON: `[]`,
		Reports:       []spec.BindingSelectorSyncReport{},
		Remaining:     []spec.Diagnostic{},
	}
}

// validatePageSpec：scope 缺失 → scope_missing 诊断（1393）。
func TestValidatePageSpec_ScopeMissing(t *testing.T) {
	service, _, _, _ := seedStaleHealPage(t, false)
	diags := service.validatePageSpec(context.Background(), spec.PageSpec{}, false)
	require.Len(t, diags, 1)
	assert.Equal(t, "scope_missing", diags[0].Code)
}

// functionsForScope：契约查询失败 → 空函数集（1808-1809）；nil receiver
// 走守卫空集（1803-1804）。
func TestFunctionsForScope_QueryError(t *testing.T) {
	service, _, _, db := seedStaleHealPage(t, false)
	applyPageFailpoint(t, db, pageDBFailpoint{table: "function_contracts", op: "query", n: 1, err: errHealFailpoint})
	fns := service.functionsForScope(context.Background(), "demo-game", "development")
	assert.Empty(t, fns)
	var nilS *Service
	assert.Empty(t, nilS.functionsForScope(context.Background(), "demo-game", "development"))
}

// syncSelectorsCore：系统愈合 actor + 无变化 → 幂等护栏返回（659-666）。
func TestSyncSelectorsCore_StaleHealGuard(t *testing.T) {
	service, ctx, pageKey, _ := seedStaleHealPage(t, false)
	draft, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	resp, err := service.syncSelectorsCore(ctx, "demo-game", "development", staleHealActor, draft, nil, false, draft.DraftRevision)
	require.NoError(t, err)
	assert.False(t, resp.Applied, "系统愈合无变化页不得 bump revision")
}

// 版本化适配器转发（2107/2113/2121/2129）。
func TestVersioningAdapters_Forward(t *testing.T) {
	service, ctx, pageKey, _ := seedStaleHealPage(t, false)
	// PublishVersioningDraftForService：公开面恒 internal-only 拦截。
	_, err := service.PublishVersioningDraftForService().
		PublishVersioningDraft(ctx, "demo-game", "development", "page_tester", pageKey, 1, "reason")
	require.Error(t, err)
	// ValidateVersioningRollbackTargetForService：快照成功路径。
	require.NoError(t, service.ValidateVersioningRollbackTargetForService().
		ValidateVersioningRollbackTarget(ctx, "demo-game", "development", pageKey, 1))
}

// PublishFromEntry：RequiresPublish 权限失败（2141-2143）。
func TestPublishFromEntry_RequiresPublishError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit") // 无 pages:publish
	rev := saveTestPageDraft(t, service, ctx)
	_, err := service.PublishFromEntry(ctx, pagePublishEntry{
		Caller: pagePublishCallerAPI, RequiresPublish: true, GameID: "demo-game", Env: "development", Actor: "a",
	}, &PagePublishRequest{PageKey: "player.manage", DraftRevision: &rev})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权发布页面")
}

// PublishFromEntry：scope/actor 从上下文解析成功（2168/2175）。
func TestPublishFromEntry_ScopeActorResolve(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	rev := saveTestPageDraft(t, service, ctx)
	// 空 GameID/Env + 有 scope ctx → 从 scope 解析（2168）。
	resp, err := service.PublishFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerAPI},
		&PagePublishRequest{PageKey: "player.manage", DraftRevision: &rev})
	require.NoError(t, err)
	assert.True(t, resp.Published)
}

// ValidateVersioningRollbackTarget：坏快照解析失败（2088）。
func TestValidateVersioningRollbackTarget_MalformedPublished(t *testing.T) {
	service, ctx, pageKey, db := seedStaleHealPage(t, false)
	// 直插一条 SpecJSON 非法（非 JSON）的发布快照，绕过 publish 校验。
	// 版本用 2（seed 已占 version 1），避免唯一索引冲突。
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: pageKey,
		Version: 2, SpecJSON: "not-json", Active: true,
	}).Error)
	err := service.ValidateVersioningRollbackTarget(ctx, "demo-game", "development", pageKey, 2)
	require.Error(t, err)
}

// newInternalProposalServiceForContext 的权限守卫拒绝臂（2448-2449）：
// 直插 pending/ready 提案后，无 pages:publish 的用户 accept 在发布前
// 被守卫拦截（守卫位于提案查找/状态/质量校验之后）。
func TestInternalProposalService_PermissionGuardDenied(t *testing.T) {
	_, _, _, db := seedStaleHealPage(t, false)
	limited, ctxLimited, _ := newPageTestService(t, "pages:edit")
	require.NoError(t, db.Create(&model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "proposal-guard", PageKey: "player.manage",
		Title:       datatypes.JSONMap{"zh-CN": "玩家管理"},
		PageSpec:    model.JSON(`{}`),
		Diagnostics: model.JSON(`[]`),
		Status:      dbenum.ProposalStatusPending,
		Quality:     "ready",
		UpdatedBy:   "tester",
	}).Error)
	_, err := limited.newInternalProposalServiceForContext(db).
		AcceptAndPublishProposal(ctxLimited, "demo-game", "development", "proposal-guard")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权发布页面")
}
