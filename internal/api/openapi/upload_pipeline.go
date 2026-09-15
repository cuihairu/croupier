package openapi

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"gorm.io/gorm"
)

// uploadPipelineTracker 「上传即成页」管线摘要追踪器（D4 后半、T5）：
// 在契约落库前对内置模板（全局表，key→digest）与 scope 提案（key 集合）
// 采集基线快照，管线结束后再次快照 diff 出本次上传的实际产出。必须在
// 事务开启前启动：T2 的逐契约模板联动在事务内即会写模板，事后才开始
// 快照会漏计。
type uploadPipelineTracker struct {
	operations      int
	diagnostics     []spec.Diagnostic
	templatesBefore map[string]string
	proposalsBefore map[string]struct{}
}

// startUploadPipelineTracker 采集摘要基线快照（契约事务开启前调用）。
func (s *Service) startUploadPipelineTracker(ctx context.Context, gameID, env string, parsed *parsedOpenAPISource) (*uploadPipelineTracker, error) {
	tracker := &uploadPipelineTracker{
		operations:  len(parsed.Operations),
		diagnostics: parsed.Diagnostics,
	}
	var err error
	if tracker.templatesBefore, err = builtinTemplateDigests(ctx, s.svcCtx.DB); err != nil {
		return nil, fmt.Errorf("snapshot builtin component templates: %w", err)
	}
	if tracker.proposalsBefore, err = proposalKeysInScope(ctx, s.svcCtx.DB, gameID, env); err != nil {
		return nil, fmt.Errorf("snapshot page proposals: %w", err)
	}
	return tracker, nil
}

// finishUploadPipeline 收尾上传管线（T5/D4 后半），在契约事务提交后调用：
//  1. 显式重建一次组件模板——unbound 契约在事务内不触发 T2 逐契约联动
//     （见 rebuildContract 尾注：事务内联动大文档下重复 N 次且跨连接写
//     全局模板表在 sqlite 下必锁），此处单次全量重建收口，失败仅 warn
//     不阻断（手动 regenerate 兜底语义不变）；
//  2. 对本次新建的 unbound 契约触发既有 proposal rebuild；
//  3. 事后快照 diff 出摘要（operations/contractsCreated/templatesUpdated/
//     proposalsCreated/diagnostics）——快照同时覆盖 bound 重建路径
//     （rebuildContractsForSourceBindings）经 T2 联动产出的模板变更。
//
// 已知边界：超大文档（>500 operations）在同步管线下有超时风险（全 scope
// 模板重建 + 逐资源提案生成），异步化另议。
func (s *Service) finishUploadPipeline(
	ctx context.Context,
	gameID, env, sourceID string,
	tracker *uploadPipelineTracker,
	created []dashboardservice.FunctionMetaInput,
) (*OpenAPISourcePipelineSummary, error) {
	if err := dashboardservice.RegenerateContractTemplates(ctx, gameID, env); err != nil {
		slog.WarnContext(ctx, "upload pipeline template regen failed (manual regenerate remains as fallback)",
			"game_id", gameID,
			"env", env,
			"source_id", sourceID,
			"err", err)
	}
	if err := s.rebuildProposalsForUnboundContracts(ctx, gameID, env, created); err != nil {
		return nil, err
	}
	templatesAfter, err := builtinTemplateDigests(ctx, s.svcCtx.DB)
	if err != nil {
		return nil, fmt.Errorf("snapshot builtin component templates: %w", err)
	}
	proposalsAfter, err := proposalKeysInScope(ctx, s.svcCtx.DB, gameID, env)
	if err != nil {
		return nil, fmt.Errorf("snapshot page proposals: %w", err)
	}
	summary := &OpenAPISourcePipelineSummary{
		Operations:       tracker.operations,
		ContractsCreated: len(created),
		Diagnostics:      tracker.diagnostics,
	}
	for key, digest := range templatesAfter {
		if tracker.templatesBefore[key] != digest {
			summary.TemplatesUpdated++
		}
	}
	for key := range proposalsAfter {
		if _, ok := tracker.proposalsBefore[key]; !ok {
			summary.ProposalsCreated++
		}
	}
	return summary, nil
}

// rebuildProposalsForUnboundContracts 对本次上传新建的 unbound 契约触发既有
// proposal rebuild（T5/D4）：镜像 rebuildContractForSourceBinding 尾部——
// resource 非空按资源聚合（能力 + 资源提案，同资源多操作只重建一次），
// resource 为空的走单函数提案。仅对新建契约触发：重传幂等（无新建零成本，
// 与 T2 digest 联动同一取舍）。排序保证多资源/多函数时的执行顺序确定。
func (s *Service) rebuildProposalsForUnboundContracts(ctx context.Context, gameID, env string, created []dashboardservice.FunctionMetaInput) error {
	if len(created) == 0 {
		return nil
	}
	contractService := dashboardservice.NewContractService(s.svcCtx.DB)
	resources := map[string]struct{}{}
	standalone := make([]string, 0, len(created))
	for _, meta := range created {
		if resourceKey := strings.TrimSpace(meta.Resource); resourceKey != "" {
			resources[resourceKey] = struct{}{}
			continue
		}
		if functionID := strings.TrimSpace(meta.ID); functionID != "" {
			standalone = append(standalone, functionID)
		}
	}
	resourceKeys := make([]string, 0, len(resources))
	for resourceKey := range resources {
		resourceKeys = append(resourceKeys, resourceKey)
	}
	sort.Strings(resourceKeys)
	sort.Strings(standalone)
	for _, resourceKey := range resourceKeys {
		if err := contractService.RebuildResourceCapability(ctx, gameID, env, resourceKey); err != nil {
			return fmt.Errorf("rebuild unbound resource capability %s: %w", resourceKey, err)
		}
		if err := contractService.RebuildProposalsForResource(ctx, gameID, env, resourceKey); err != nil {
			return fmt.Errorf("rebuild unbound page proposals %s: %w", resourceKey, err)
		}
	}
	for _, functionID := range standalone {
		if err := contractService.RebuildProposalForFunction(ctx, gameID, env, functionID); err != nil {
			return fmt.Errorf("rebuild unbound standalone proposal %s: %w", functionID, err)
		}
	}
	return nil
}

// builtinTemplateDigests 全量分页拉取内置组件模板的 key→digest 快照。
// 模板表为全局表（不分 game/env）；List 分页上限 100，需循环取全——
// 大文档上传可产出数百个 fn--*/query--* 模板。
func builtinTemplateDigests(ctx context.Context, db *gorm.DB) (map[string]string, error) {
	templateModel := model.NewComponentTemplateModel(db)
	out := map[string]string{}
	for page := 1; ; page++ {
		items, total, err := templateModel.List(ctx, model.ComponentTemplateListOptions{
			PaginationOptions: model.PaginationOptions{Page: page, PageSize: 100},
			BuiltinOnly:       true,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			out[item.Key] = strings.TrimSpace(item.Digest)
		}
		if len(items) == 0 || int64(len(out)) >= total {
			return out, nil
		}
	}
}

// proposalKeysInScope 采集 scope 内全部页面提案的 key 集合快照。
func proposalKeysInScope(ctx context.Context, db *gorm.DB, gameID, env string) (map[string]struct{}, error) {
	proposals, err := model.NewPageProposalModel(db).ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(proposals))
	for _, proposal := range proposals {
		out[strings.TrimSpace(proposal.ProposalKey)] = struct{}{}
	}
	return out, nil
}
