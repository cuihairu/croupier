package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RebuildProposalsForResource triggers incremental proposal recalculation
// for all proposals affected by changes to a resource.
// This should be called after:
// - Function registration/update
// - OpenAPI Source update
// - Catalog semantic update
func (s *ContractService) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	// Get current semantics
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.removeResourceProposal(ctx, gameID, env, resourceKey)
		}
		return fmt.Errorf("find capability semantics: %w", err)
	}

	// Get contracts for this resource
	contracts, err := s.contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return fmt.Errorf("list contracts: %w", err)
	}
	if len(contracts) == 0 {
		return s.removeResourceProposal(ctx, gameID, env, resourceKey)
	}

	// Compute new source digest
	newDigest := computeDigest(contracts)

	slog.Info("rebuilding page proposals",
		"game_id", gameID,
		"env", env,
		"resource_key", resourceKey,
		"old_digest", semantics.SourceDigest,
		"new_digest", newDigest)

	if semantics.SourceDigest != newDigest {
		semantics.SourceDigest = newDigest
		// 源契约 digest 已变（真实内容变化路径），changed 结果无需另行处理。
		if _, err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
			return fmt.Errorf("update semantics digest: %w", err)
		}
	}

	consumedBindings, err := s.upsertResourceProposal(ctx, gameID, env, semantics, contracts)
	if err != nil {
		return err
	}
	if err := s.upsertStandaloneProposals(ctx, gameID, env, contracts, semantics, consumedBindings); err != nil {
		return err
	}
	return nil
}

func (s *ContractService) removeResourceProposal(ctx context.Context, gameID, env, resourceKey string) error {
	resourceKey = strings.TrimSpace(resourceKey)
	if resourceKey == "" {
		return nil
	}
	if err := s.proposalModel.DeleteByScopeAndKey(ctx, gameID, env, resourceProposalKey(resourceKey)); err != nil {
		return fmt.Errorf("delete resource proposal %s: %w", resourceKey, err)
	}
	return s.resolveBlockedIssue(ctx, gameID, env, resourceKey, "")
}

// RebuildProposalForFunction creates or refreshes the standalone page proposal
// for a function that cannot be safely grouped into a ResourcePage.
// loadTermDictionary loads the platform term dictionary for generated title
// fallbacks (category display names are owned by the menu system since T-M8).
// A nil/empty result keeps generation on the humanize fallback path.
func (s *ContractService) loadTermDictionary(ctx context.Context) generator.TermDictionary {
	if s == nil || s.db == nil {
		return nil
	}
	items, err := model.NewTermDictionaryModel(s.db).List(ctx, "")
	if err != nil {
		return nil
	}
	out := make(generator.TermDictionary, len(items))
	for _, item := range items {
		text := spec.LocalizedText(model.NormalizeTermDisplay(item.Display))
		if len(text) == 0 {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(item.Domain))+"/"+strings.ToLower(strings.TrimSpace(item.Alias))] = text
	}
	return out
}

func (s *ContractService) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return nil
	}
	contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("find function contract %s: %w", functionID, err)
	}
	// FindByScopeAndFunctionID 用 gorm First 实现：err==nil 时恒返回非 nil
	// 指针（未命中走上方 ErrRecordNotFound 分支），原 contract==nil 防御
	// 分支为死代码，已删。
	if isCRUDCapability(contract.Capability.String()) && strings.TrimSpace(contract.ResourceKey) != "" {
		return nil
	}
	functions := map[string]spec.FunctionSpec{
		functionID: FunctionSpecFromContract(contract),
	}
	var taskSemantics map[string]spec.TaskSemantic
	var reportSemantics map[string]spec.ReportSemantic
	if resourceKey := strings.TrimSpace(contract.ResourceKey); resourceKey != "" {
		if semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err == nil {
			taskSemantics = taskSemanticsByStartFunction(semantics)
			reportSemantics = reportSemanticsByQueryFunction(semantics)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find capability semantics for %s: %w", resourceKey, err)
		}
	}
	generated := generator.GenerateForOperation(OperationSpecFromContract(contract), generator.GenerateOptions{
		DefaultLocale:   "zh-CN",
		Functions:       functions,
		TaskSemantics:   taskSemantics,
		ReportSemantics: reportSemantics,
		Terms:           s.loadTermDictionary(ctx),
	})
	if generator.ShouldBlockProposal(generated.Diagnostics) {
		return s.upsertBlockedIssue(ctx, gameID, env, contract.ResourceKey, contract.FunctionID, generated.Diagnostics, contractsForIssue(contract))
	}
	proposalKey := standaloneProposalKey(generated.Type, contract.FunctionID)
	if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, []*model.FunctionContract{contract}, generated); err != nil {
		return err
	}
	return s.resolveBlockedIssue(ctx, gameID, env, contract.ResourceKey, contract.FunctionID)
}

// shouldBlockResourceProposal 是 generator.ShouldBlockProposal 的包级接缝：
// 生产恒为真实实现，测试注入 true 驱动 upsertResourceProposal 的阻断分支
// （该分支的不可达论证见调用处注释——resource 生成器诊断 code 与阻断
// 清单不相交；主生成器两处调用点可达、无需接缝）。
var shouldBlockResourceProposal = generator.ShouldBlockProposal

func (s *ContractService) upsertResourceProposal(
	ctx context.Context,
	gameID string,
	env string,
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
) (map[string]struct{}, error) {
	resourceOptions := generator.DefaultGenerateOptions()
	resourceOptions.Terms = s.loadTermDictionary(ctx)
	generated, ok := generator.GenerateResourcePageProposal(semantics, contracts, resourceOptions)
	if !ok {
		if err := s.removeResourceProposal(ctx, gameID, env, semantics.ResourceKey); err != nil {
			return nil, err
		}
		return map[string]struct{}{}, nil
	}
	// 不可达论证（C 类）：ShouldBlockProposal 仅对 code ∈ {function_id_missing,
	// function_disabled} 的 Error 诊断返回 true，而这两个 code 只由主生成器
	// 的 assessBaseCandidate（generator.go）产生；GenerateResourcePageProposal
	// 的诊断来源（assessResourceSemantics / schemaSubsetDiagnostics /
	// buildInlineResourceActions / validateGeneratedResourceViews）的 code 集合
	// 与阻断清单不相交，故此分支生产恒 false，经 shouldBlockResourceProposal
	// 接缝测试驱动。保留原因：阻断机制是发布安全防线，resource 生成器未来
	// 新增 Error 诊断时该路径即被激活，删除会静默放行本应阻断的提案；
	// generator 的 code 常量为跨包隐式契约，无编译期保护，不宜按死分支删除。
	if shouldBlockResourceProposal(generated.Diagnostics) {
		if err := s.removeResourceProposal(ctx, gameID, env, semantics.ResourceKey); err != nil {
			return nil, err
		}
		if err := s.upsertBlockedIssue(ctx, gameID, env, semantics.ResourceKey, "", generated.Diagnostics, contracts); err != nil {
			return nil, err
		}
		return map[string]struct{}{}, nil
	}
	consumed := consumedPageBindings(generated.Bindings, contracts)
	if err := s.upsertGeneratedProposal(ctx, gameID, env, resourceProposalKey(semantics.ResourceKey), semantics, contracts, generated); err != nil {
		return nil, err
	}
	if err := s.resolveBlockedIssue(ctx, gameID, env, semantics.ResourceKey, ""); err != nil {
		return nil, err
	}
	return consumed, nil
}

func (s *ContractService) upsertStandaloneProposals(
	ctx context.Context,
	gameID string,
	env string,
	contracts []*model.FunctionContract,
	semantics *model.CapabilitySemantics,
	consumed map[string]struct{},
) error {
	taskSemantics := taskSemanticsByStartFunction(semantics)
	reportSemantics := reportSemanticsByQueryFunction(semantics)
	functions := make(map[string]spec.FunctionSpec, len(contracts))
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		functions[strings.TrimSpace(contract.FunctionID)] = FunctionSpecFromContract(contract)
	}
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		functionID := strings.TrimSpace(contract.FunctionID)
		if _, ok := consumed[functionID]; ok {
			if err := s.removeStandaloneProposalsForFunction(ctx, gameID, env, functionID); err != nil {
				return err
			}
			continue
		}
		generated := generator.GenerateForOperation(OperationSpecFromContract(contract), generator.GenerateOptions{
			DefaultLocale:   "zh-CN",
			Functions:       functions,
			TaskSemantics:   taskSemantics,
			ReportSemantics: reportSemantics,
			Terms:           s.loadTermDictionary(ctx),
		})
		if generator.ShouldBlockProposal(generated.Diagnostics) {
			if err := s.upsertBlockedIssue(ctx, gameID, env, contract.ResourceKey, functionID, generated.Diagnostics, contractsForIssue(contract)); err != nil {
				return err
			}
			continue
		}
		proposalKey := standaloneProposalKey(generated.Type, contract.FunctionID)
		if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, []*model.FunctionContract{contract}, generated); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContractService) removeStandaloneProposalsForFunction(ctx context.Context, gameID, env, functionID string) error {
	for _, pageType := range []spec.PageType{spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport} {
		if err := s.proposalModel.DeleteByScopeAndKey(ctx, gameID, env, standaloneProposalKey(pageType, functionID)); err != nil {
			return fmt.Errorf("delete standalone proposal for %s: %w", functionID, err)
		}
	}
	return nil
}

func contractsForIssue(contract *model.FunctionContract) []*model.FunctionContract {
	if contract == nil {
		return nil
	}
	return []*model.FunctionContract{contract}
}

func (s *ContractService) upsertBlockedIssue(ctx context.Context, gameID, env, resourceKey, functionID string, diagnostics []spec.Diagnostic, contracts []*model.FunctionContract) error {
	issue := generator.CreateBlockedProposalIssue(gameID, env, resourceKey, functionID, diagnostics)
	if err := s.blockedIssues.Upsert(ctx, &model.BlockedProposalIssue{
		GameID:        issue.GameID,
		Env:           issue.Env,
		ResourceKey:   issue.ResourceKey,
		FunctionID:    issue.FunctionID,
		SourceDigests: toJSON([]string{computeDigest(contracts)}),
		Diagnostics:   toJSON(issue.Diagnostics),
		RepairHint:    toJSONMap(issue.RepairHint),
		Status:        issue.Status,
		UpdatedBy:     "system",
	}); err != nil {
		return fmt.Errorf("upsert blocked proposal issue: %w", err)
	}
	return nil
}

func (s *ContractService) resolveBlockedIssue(ctx context.Context, gameID, env, resourceKey, functionID string) error {
	if err := s.blockedIssues.Resolve(ctx, gameID, env, resourceKey, functionID, "system"); err != nil {
		return fmt.Errorf("resolve blocked proposal issue: %w", err)
	}
	return nil
}

func consumedPageBindings(bindings []spec.PageFunctionBinding, contracts []*model.FunctionContract) map[string]struct{} {
	consumed := map[string]struct{}{}
	for _, binding := range bindings {
		contract := findContractByFunctionID(contracts, binding.FunctionID)
		if contract == nil {
			continue
		}
		functionID := strings.TrimSpace(contract.FunctionID)
		if functionID != "" {
			consumed[functionID] = struct{}{}
		}
	}
	return consumed
}

func findContractByFunctionID(contracts []*model.FunctionContract, functionID string) *model.FunctionContract {
	functionID = strings.TrimSpace(functionID)
	for _, contract := range contracts {
		if contract != nil && strings.TrimSpace(contract.FunctionID) == functionID {
			return contract
		}
	}
	return nil
}

func taskSemanticsByStartFunction(semantics *model.CapabilitySemantics) map[string]spec.TaskSemantic {
	out := map[string]spec.TaskSemantic{}
	if semantics == nil || len(semantics.Tasks) == 0 {
		return out
	}
	var items []spec.TaskSemantic
	if err := json.Unmarshal(semantics.Tasks, &items); err != nil {
		return out
	}
	for _, item := range items {
		functionID := strings.TrimSpace(item.Start.FunctionID)
		if functionID == "" {
			continue
		}
		out[functionID] = item
	}
	return out
}

func reportSemanticsByQueryFunction(semantics *model.CapabilitySemantics) map[string]spec.ReportSemantic {
	out := map[string]spec.ReportSemantic{}
	if semantics == nil || len(semantics.Reports) == 0 {
		return out
	}
	var items []spec.ReportSemantic
	if err := json.Unmarshal(semantics.Reports, &items); err != nil {
		return out
	}
	for _, item := range items {
		functionID := strings.TrimSpace(item.Query.FunctionID)
		if functionID == "" {
			continue
		}
		out[functionID] = item
	}
	return out
}

func (s *ContractService) upsertGeneratedProposal(
	ctx context.Context,
	gameID string,
	env string,
	proposalKey string,
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
	generated spec.GeneratedPageSpec,
) error {
	if strings.TrimSpace(proposalKey) == "" || strings.TrimSpace(generated.PageKey) == "" {
		return nil
	}
	pageJSON, err := json.Marshal(generated.PageSpec)
	if err != nil {
		return fmt.Errorf("marshal generated page spec: %w", err)
	}
	proposal := &model.PageProposal{
		GameID:           gameID,
		Env:              env,
		ProposalKey:      proposalKey,
		PageKey:          generated.PageKey,
		PageType:         string(generated.Type),
		ResourceKey:      generated.ResourceKey,
		Quality:          string(generated.Quality),
		GeneratorVersion: PageProposalGeneratorVersion,
		FunctionDigest:   computeDigest(contracts),
		SemanticsDigest:  semanticsComparableDigest(semantics),
		Title:            toJSONMap(generated.Title),
		Description:      toJSONMap(generated.Description),
		CategoryKey:      generated.Category.Key,
		PageSpec:         model.JSON(pageJSON),
		Diagnostics:      toJSON(generated.Diagnostics),
		Status:           dbenum.ProposalStatusPending,
		UpdatedBy:        "system",
	}
	changed := true
	if existing, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey); err == nil {
		changed = generatedProposalChanged(existing, proposal)
		if !changed {
			proposal.Status = preserveGeneratedProposalStatus(existing.Status)
			if existing.UpdatedBy != "" {
				proposal.UpdatedBy = existing.UpdatedBy
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find existing page proposal %s: %w", proposalKey, err)
	}
	if err := s.proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return fmt.Errorf("upsert page proposal %s: %w", proposalKey, err)
	}
	if changed {
		if _, err := createProposalVersionSnapshot(ctx, s.proposalVersions, proposal, "generate proposal from latest contracts", "system"); err != nil {
			return fmt.Errorf("snapshot page proposal %s: %w", proposalKey, err)
		}
	}
	return nil
}

func generatedProposalChanged(existing *model.PageProposal, next *model.PageProposal) bool {
	if existing == nil || next == nil {
		return true
	}
	return proposalComparableDigest(existing) != proposalComparableDigest(next)
}

// semanticsComparableDigest 只对语义的**稳定内容**做 digest，唯一实现下沉在
// model.CapabilitySemantics.StableContentDigest（model 层 UpsertSemantics 的
// 内容变更判定必须与 proposal 的 SemanticsDigest 同源，两份投影必然漂移）。
// 历史：BUG-031 两段——先修「整结构体吃 gorm 时间戳」，再修「行 Version 被
// 无条件自增又被投影吃进」（线上 resource 提案每次 agent 重连 +3 版，
// FunctionDigest/PageSpec 恒等、仅 SemanticsDigest 变），最终收敛为模型层
// 单一稳定内容投影，且 UpsertSemantics 内容未变时不再 bump Version。
func semanticsComparableDigest(semantics *model.CapabilitySemantics) string {
	// nil（无资源/无语义行）与全零语义投影等价：digest 保持 64 字符非空
	// （PageProposal.SemanticsDigest 的列契约），且与「空语义行」不可区分——
	// 二者本来就没有可比较的内容差异。
	if semantics == nil {
		semantics = &model.CapabilitySemantics{}
	}
	return semantics.StableContentDigest()
}

func proposalComparableDigest(proposal *model.PageProposal) string {
	if proposal == nil {
		return ""
	}
	return computeDigest(struct {
		ProposalKey      string
		PageKey          string
		PageType         string
		ResourceKey      string
		Quality          string
		GeneratorVersion string
		FunctionDigest   string
		SemanticsDigest  string
		Title            datatypes.JSONMap
		Description      datatypes.JSONMap
		CategoryKey      string
		PageSpec         model.JSON
		Diagnostics      model.JSON
	}{
		ProposalKey:      proposal.ProposalKey,
		PageKey:          proposal.PageKey,
		PageType:         proposal.PageType,
		ResourceKey:      proposal.ResourceKey,
		Quality:          proposal.Quality,
		GeneratorVersion: proposal.GeneratorVersion,
		FunctionDigest:   proposal.FunctionDigest,
		SemanticsDigest:  proposal.SemanticsDigest,
		Title:            proposal.Title,
		Description:      proposal.Description,
		CategoryKey:      proposal.CategoryKey,
		PageSpec:         canonicalJSONBytes(proposal.PageSpec),
		Diagnostics:      canonicalJSONBytes(proposal.Diagnostics),
	})
}

// canonicalJSONBytes 把 JSON 字节归一化为「解析后按键序重排再序列化」的
// 规范形态（与 model 层 contractSemanticallyEqual 的 canonicalJSON 同思路）。
// page_proposals.page_spec/diagnostics 是 jsonb：Postgres 回读与 fresh
// marshal 的字节形态永不一致（键重排/去空白），digest 直接吃原始字节会让
// generatedProposalChanged 恒判「有变化」——每轮提案重建灌一批版本快照，
// pages 界面出现成堆需人工处理的假性变更。
func canonicalJSONBytes(raw model.JSON) model.JSON {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return model.JSON(out)
}

func preserveGeneratedProposalStatus(status dbenum.ProposalStatus) dbenum.ProposalStatus {
	switch status {
	case dbenum.ProposalStatusAccepted, dbenum.ProposalStatusRejected:
		return status
	default:
		return dbenum.ProposalStatusPending
	}
}

func resourceProposalKey(resourceKey string) string {
	key := sanitizeSourceKey(resourceKey)
	if key == "" {
		return ""
	}
	return "resource:" + key
}

func standaloneProposalKey(pageType spec.PageType, functionID string) string {
	kind := string(pageType)
	switch pageType {
	case spec.PageTypeTask, spec.PageTypeReport:
	default:
		kind = string(spec.PageTypeOperation)
	}
	key := sanitizeSourceKey(functionID)
	if key == "" {
		return ""
	}
	return kind + ":" + key
}

// RebuildAllProposals triggers proposal recalculation for all resources in a scope.
func (s *ContractService) RebuildAllProposals(ctx context.Context, gameID, env string) error {
	// Get all resource capabilities
	capabilities, err := s.capabilityModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return fmt.Errorf("list capabilities: %w", err)
	}

	for _, cap := range capabilities {
		if err := s.RebuildProposalsForResource(ctx, gameID, env, cap.ResourceKey); err != nil {
			slog.Error("failed to rebuild proposals for resource",
				"game_id", gameID,
				"env", env,
				"resource_key", cap.ResourceKey,
				"error", err)
			// Continue with other resources
		}
	}

	return nil
}
