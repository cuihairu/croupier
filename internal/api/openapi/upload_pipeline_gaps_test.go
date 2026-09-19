package openapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startUploadPipelineTracker 快照采集：成功路径字段齐备；模板快照失败与
// 提案快照失败分别包装错误。
func TestStartUploadPipelineTrackerBranches(t *testing.T) {
	t.Run("success snapshots operations diagnostics and maps", func(t *testing.T) {
		service := setupOpenAPITestService(t)
		parsed := &parsedOpenAPISource{
			Operations:  make([]OpenAPISourceOperation, 3),
			Diagnostics: []spec.Diagnostic{{Code: "rest_capability_inferred"}},
		}
		tracker, err := service.startUploadPipelineTracker(openAPITestContext(), "demo-game", "development", parsed)
		require.NoError(t, err)
		assert.Equal(t, 3, tracker.operations)
		require.Len(t, tracker.diagnostics, 1)
		assert.NotNil(t, tracker.templatesBefore)
		assert.NotNil(t, tracker.proposalsBefore)
	})

	t.Run("builtin template snapshot failure", func(t *testing.T) {
		failing := setupOpenAPITestService(t)
		sqlDB, err := failing.svcCtx.DB.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = failing.startUploadPipelineTracker(openAPITestContext(), "demo-game", "development", &parsedOpenAPISource{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "snapshot builtin component templates")
	})

	t.Run("page proposal snapshot failure", func(t *testing.T) {
		partial := setupOpenAPITestService(t)
		require.NoError(t, partial.svcCtx.DB.Migrator().DropTable("page_proposals"))

		_, err := partial.startUploadPipelineTracker(openAPITestContext(), "demo-game", "development", &parsedOpenAPISource{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "snapshot page proposals")
	})
}

// builtinTemplateDigests：分页循环取全（>100 条跨页）、查询失败报错。
func TestBuiltinTemplateDigestsPaginationAndFailure(t *testing.T) {
	service := setupOpenAPITestService(t)
	db := service.svcCtx.DB
	ctx := openAPITestContext()

	// 105 条内置模板：跨过单页 100 上限，验证翻页终止条件
	for i := 0; i < 105; i++ {
		tpl := &model.ComponentTemplate{
			Key:      fmt.Sprintf("fn--bulk-%03d", i),
			Name:     model.JSON(`{"zh-CN":"批量"}`),
			Category: "运营",
			Tree:     model.JSON(`[{"type":"fnTable","props":{"functionId":"player.list"}}]`),
			Builtin:  true,
		}
		require.NoError(t, db.Create(tpl).Error)
	}
	digests, err := builtinTemplateDigests(ctx, db)
	require.NoError(t, err)
	assert.Len(t, digests, 105, "分页循环必须取回全部内置模板")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = builtinTemplateDigests(ctx, db)
	assert.Error(t, err)
}

// proposalKeysInScope：成功聚合 key、查询失败报错。
func TestProposalKeysInScopeBranches(t *testing.T) {
	service := setupOpenAPITestService(t)
	db := service.svcCtx.DB
	ctx := openAPITestContext()

	require.NoError(t, db.Create(&model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
	}).Error)
	require.NoError(t, db.Create(&model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "  fn:standalone  ",
	}).Error)

	keys, err := proposalKeysInScope(ctx, db, "demo-game", "development")
	require.NoError(t, err)
	assert.Contains(t, keys, "resource:player")
	assert.Contains(t, keys, "fn:standalone", "key 需 TrimSpace 归一")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = proposalKeysInScope(ctx, db, "demo-game", "development")
	assert.Error(t, err)
}

// rebuildProposalsForUnboundContracts：空清单零成本；DB 故障时资源/独立
// 函数两条路径分别包装错误。
func TestRebuildProposalsForUnboundContractsBranches(t *testing.T) {
	service := setupOpenAPITestService(t)
	ctx := openAPITestContext()

	// 空清单：no-op
	require.NoError(t, service.rebuildProposalsForUnboundContracts(ctx, "demo-game", "development", nil))

	closed := setupOpenAPITestService(t)
	sqlDB, err := closed.svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = closed.rebuildProposalsForUnboundContracts(ctx, "demo-game", "development",
		[]dashboardservice.FunctionMetaInput{{ID: "player.list", Resource: "player"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild unbound resource capability")

	err = closed.rebuildProposalsForUnboundContracts(ctx, "demo-game", "development",
		[]dashboardservice.FunctionMetaInput{{ID: "fn.standalone"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild unbound standalone proposal")
}

// finishUploadPipeline 剩余分支：模板重建失败仅告警继续；提案重建失败
// 中断返回错误；后置快照失败（模板/提案表缺失）；摘要 diff 计数
// （digest 变化计 TemplatesUpdated、新提案计 ProposalsCreated）。
// 注：模板重建器是进程级单例，本用例不并行（与管线主用例同一约束）。
func TestFinishUploadPipelineFailureAndDiffBranches(t *testing.T) {
	ctx := openAPITestContext()

	t.Run("template regen failure warns and continues", func(t *testing.T) {
		service := setupOpenAPITestService(t)
		dashboardservice.SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
			return assert.AnError
		})
		t.Cleanup(func() { dashboardservice.SetContractTemplateRegenerator(nil) })
		tracker := &uploadPipelineTracker{templatesBefore: map[string]string{}, proposalsBefore: map[string]struct{}{}}
		summary, err := service.finishUploadPipeline(ctx, "demo-game", "development", "src-fail", tracker, nil)
		require.NoError(t, err, "模板重建失败不阻断上传管线")
		assert.Equal(t, 0, summary.TemplatesUpdated)
	})

	t.Run("proposal rebuild failure aborts", func(t *testing.T) {
		service := setupOpenAPITestService(t)
		sqlDB, err := service.svcCtx.DB.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		tracker := &uploadPipelineTracker{templatesBefore: map[string]string{}, proposalsBefore: map[string]struct{}{}}
		_, err = service.finishUploadPipeline(ctx, "demo-game", "development", "src-fail", tracker,
			[]dashboardservice.FunctionMetaInput{{ID: "player.list", Resource: "player"}})
		require.Error(t, err)
	})

	t.Run("after-snapshots failure surfaces wrapped errors", func(t *testing.T) {
		noProposalsTable := setupOpenAPITestService(t)
		require.NoError(t, noProposalsTable.svcCtx.DB.Migrator().DropTable("page_proposals"))
		tracker := &uploadPipelineTracker{templatesBefore: map[string]string{}, proposalsBefore: map[string]struct{}{}}
		_, err := noProposalsTable.finishUploadPipeline(ctx, "demo-game", "development", "src-fail", tracker, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "snapshot page proposals")

		noTemplatesTable := setupOpenAPITestService(t)
		require.NoError(t, noTemplatesTable.svcCtx.DB.Migrator().DropTable("component_templates"))
		_, err = noTemplatesTable.finishUploadPipeline(ctx, "demo-game", "development", "src-fail", tracker, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "snapshot builtin component templates")
	})

	t.Run("diff counts digest changes and new proposals", func(t *testing.T) {
		service := setupOpenAPITestService(t)
		db := service.svcCtx.DB
		tpl := &model.ComponentTemplate{
			Key:      "fn--diff",
			Name:     model.JSON(`{"zh-CN":"差异"}`),
			Category: "运营",
			Tree:     model.JSON(`[{"type":"fnTable","props":{"functionId":"player.list"}}]`),
			Builtin:  true,
		}
		require.NoError(t, db.Create(tpl).Error)

		before, err := builtinTemplateDigests(ctx, db)
		require.NoError(t, err)
		proposalsBefore, err := proposalKeysInScope(ctx, db, "demo-game", "development")
		require.NoError(t, err)
		tracker := &uploadPipelineTracker{
			operations:      2,
			templatesBefore: before,
			proposalsBefore: proposalsBefore,
		}

		// 快照之后：模板 digest 变化 + 新增一条提案
		tpl.Tree = model.JSON(`[{"type":"fnForm","props":{"functionId":"player.get"}}]`)
		tpl.Digest = model.ComputeTemplateDigest(tpl.Tree)
		require.NoError(t, db.Save(tpl).Error)
		require.NoError(t, db.Create(&model.PageProposal{
			GameID:      "demo-game",
			Env:         "development",
			ProposalKey: "resource:new",
		}).Error)

		summary, err := service.finishUploadPipeline(ctx, "demo-game", "development", "src-diff", tracker, nil)
		require.NoError(t, err)
		assert.Equal(t, 2, summary.Operations)
		assert.Equal(t, 1, summary.TemplatesUpdated, "digest 变化的内置模板计入 TemplatesUpdated")
		assert.Equal(t, 1, summary.ProposalsCreated, "快照后新增的提案计入 ProposalsCreated")
	})
}
