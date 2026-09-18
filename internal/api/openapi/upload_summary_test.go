package openapi

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/api/component"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	dashboardservice "github.com/cuihairu/croupier/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T5/D4 验收：上传单请求完成全链生成（契约→组件模板→页面提案）后，
// 响应摘要各计数与库内实际产出一致，解析诊断透传。
// 不并行：注入的模板重建器是进程级单例（与 routes.go 装配同构），
// 并行用例的契约落库会经全局钩子写入本用例的库（并行用例只与其他
// 并行用例并发，顺序用例恢复 nil 后才轮到它们）。
// 文件型 sqlite：见 setupOpenAPITestServiceWithDSN 注释。
func TestService_CreateSourcePipelineSummaryMatchesActualOutput(t *testing.T) {
	service, _, _ := setupOpenAPITestServiceWithDSN(t, filepath.Join(t.TempDir(), "pipeline.db"),
		"openapi_sources:read", "openapi_sources:write")
	db := service.svcCtx.DB

	componentHandler := component.NewHandler(
		model.NewComponentTemplateModel(db),
		model.NewFunctionContractModel(db))
	dashboardservice.SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
		contracts, err := model.NewFunctionContractModel(db).ListByScope(ctx, gameID, env)
		if err != nil {
			return err
		}
		return componentHandler.RegenerateFromContracts(ctx, contracts)
	})
	t.Cleanup(func() { dashboardservice.SetContractTemplateRegenerator(nil) })

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Unbound API",
		Spec: rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Summary)

	assert.Equal(t, 3, resp.Summary.Operations)
	assert.Equal(t, 3, resp.Summary.ContractsCreated)

	// 模板计数与库内内置模板总数一致（首传全部为新建）。
	_, templateTotal, err := model.NewComponentTemplateModel(db).List(context.Background(),
		model.ComponentTemplateListOptions{
			PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 100},
			BuiltinOnly:       true,
		})
	require.NoError(t, err)
	require.Greater(t, templateTotal, int64(0), "首传应产出内置组件模板")
	assert.Equal(t, int(templateTotal), resp.Summary.TemplatesUpdated)

	// 提案计数与 scope 内提案总数一致（首传全部为新建）。
	proposals, err := model.NewPageProposalModel(db).ListByScope(openAPITestContext(), "demo-game", "development")
	require.NoError(t, err)
	require.NotEmpty(t, proposals, "首传应产出页面提案")
	assert.Equal(t, len(proposals), resp.Summary.ProposalsCreated)

	// 诊断透传：REST 能力推断产生 info 级诊断。
	require.NotEmpty(t, resp.Summary.Diagnostics)
	assert.Contains(t, diagnosticCodes(resp.Summary.Diagnostics), "rest_capability_inferred")

	// 重传幂等：无新建契约/提案，模板内容不变（digest 一致）。
	updated, err := service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: resp.Source.SourceID,
		Spec:     rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Summary)
	assert.Equal(t, 3, updated.Summary.Operations)
	assert.Equal(t, 0, updated.Summary.ContractsCreated, "重传无新建契约")
	assert.Equal(t, 0, updated.Summary.TemplatesUpdated, "重传模板内容不变（digest 一致）")
	assert.Equal(t, 0, updated.Summary.ProposalsCreated, "重传无新提案")
}

// T5 验收（无重建器注入的环境）：摘要结构稳定携带、重传计数归零；
// GetSource 响应不携带摘要（wire 兼容，summary 仅 create/update 响应）。
func TestService_UpdateSourcePipelineSummaryIdempotent(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Unbound API",
		Spec: rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)
	require.NotNil(t, created.Summary)
	assert.Equal(t, 3, created.Summary.Operations)
	assert.Equal(t, 3, created.Summary.ContractsCreated)
	assert.Greater(t, created.Summary.ProposalsCreated, 0)
	require.NotEmpty(t, created.Summary.Diagnostics)

	updated, err := service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Summary)
	assert.Equal(t, 3, updated.Summary.Operations)
	assert.Equal(t, 0, updated.Summary.ContractsCreated, "重传无新建契约")
	assert.Equal(t, 0, updated.Summary.ProposalsCreated, "重传无新提案")
	assert.Equal(t, 0, updated.Summary.TemplatesUpdated, "重传模板内容不变（digest 一致）")
	assert.NotEmpty(t, updated.Summary.Diagnostics, "诊断每次透传")

	got, err := service.GetSource(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: created.Source.SourceID})
	require.NoError(t, err)
	assert.Nil(t, got.Summary, "GetSource 不携带管线摘要")
}

func diagnosticCodes(diags []spec.Diagnostic) []string {
	codes := make([]string, 0, len(diags))
	for _, diag := range diags {
		codes = append(codes, diag.Code)
	}
	return codes
}

// M6 大文档护栏：finishUploadPipeline 对超阈值（openapi.
// pipelineOperationGuard，0=默认 500，负数=禁用）的文档追加 warn 级
// large_document_pipeline diagnostic（提示分批/拆分，不阻断）。直接以
// tracker 注入 operations 数——不需要真实大文档 fixture。不注入模板
// 重建器，可并行。
func TestFinishUploadPipeline_LargeDocumentGuard(t *testing.T) {
	newTracker := func(operations int) *uploadPipelineTracker {
		return &uploadPipelineTracker{
			operations:      operations,
			templatesBefore: map[string]string{},
			proposalsBefore: map[string]struct{}{},
		}
	}
	findGuard := func(diags []spec.Diagnostic) *spec.Diagnostic {
		for i := range diags {
			if diags[i].Code == "large_document_pipeline" {
				return &diags[i]
			}
		}
		return nil
	}

	t.Run("over default threshold appends warning", func(t *testing.T) {
		t.Parallel()
		service := setupOpenAPITestService(t) // Config 零值 → 默认阈值 500
		summary, err := service.finishUploadPipeline(openAPITestContext(), "demo-game", "development", "src-guard", newTracker(501), nil)
		require.NoError(t, err)
		guard := findGuard(summary.Diagnostics)
		require.NotNil(t, guard, "超默认阈值应有大文档护栏诊断")
		assert.Equal(t, spec.SeverityWarning, guard.Severity)
		assert.Contains(t, guard.Message, "501")
	})

	t.Run("at default threshold omits warning", func(t *testing.T) {
		t.Parallel()
		service := setupOpenAPITestService(t)
		summary, err := service.finishUploadPipeline(openAPITestContext(), "demo-game", "development", "src-guard", newTracker(500), nil)
		require.NoError(t, err)
		assert.Nil(t, findGuard(summary.Diagnostics), "恰好到达阈值不触发（严格大于）")
	})

	t.Run("negative config disables guard even over default", func(t *testing.T) {
		t.Parallel()
		service := setupOpenAPITestService(t)
		service.svcCtx.Config.OpenAPI.PipelineOperationGuard = -1
		summary, err := service.finishUploadPipeline(openAPITestContext(), "demo-game", "development", "src-guard", newTracker(501), nil)
		require.NoError(t, err)
		assert.Nil(t, findGuard(summary.Diagnostics), "负数禁用：超默认阈值也不提示")
	})

	t.Run("injected small threshold warns", func(t *testing.T) {
		t.Parallel()
		service := setupOpenAPITestService(t)
		service.svcCtx.Config.OpenAPI.PipelineOperationGuard = 2
		summary, err := service.finishUploadPipeline(openAPITestContext(), "demo-game", "development", "src-guard", newTracker(3), nil)
		require.NoError(t, err)
		require.NotNil(t, findGuard(summary.Diagnostics), "注入小阈值：3 operations 即触发")
	})
}
