package versioning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"gorm.io/gorm"
)

// CompositePageOutcome composite 保存结果（T10：含 auto 发布分级结果）。
type CompositePageOutcome struct {
	Proposal *model.PageProposal
	// Published auto 策略下保存后已直接发布。
	Published bool
	// PublishError 非空 = auto 发布被质量门槛拒绝或失败（保存本身已成功，
	// 可走人工提案链重试）。
	PublishError string
}

func pageSpecFromProposalSnapshot(raw json.RawMessage) (spec.PageSpec, error) {
	var proposal model.PageProposal
	if err := json.Unmarshal(raw, &proposal); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode proposal snapshot: %w", err)
	}
	return pageSpecFromProposalModel(&proposal)
}

func pageSpecFromProposalModel(proposal *model.PageProposal) (spec.PageSpec, error) {
	if proposal == nil || len(proposal.PageSpec) == 0 {
		return spec.PageSpec{}, errorx.NewNotFound("proposal PageSpec not found")
	}
	var page spec.PageSpec
	if err := json.Unmarshal(proposal.PageSpec, &page); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode proposal PageSpec: %w", err)
	}
	return normalizePageSpec(page), nil
}

func (s *Service) proposalForPage(ctx context.Context, gameID, env string, page *model.PageSpec) (*model.PageProposal, error) {
	if page == nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	if key := strings.TrimSpace(page.BaseProposalKey); key != "" {
		return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, key)
	}
	return s.proposalModel.FindByScopeAndPageKey(ctx, gameID, env, strings.TrimSpace(page.PageKey))
}

func (s *Service) contractsForPage(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) ([]*model.FunctionContract, error) {
	seen := map[string]struct{}{}
	contracts := make([]*model.FunctionContract, 0, len(pageSpec.Bindings))
	for _, binding := range pageSpec.Bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			continue
		}
		if _, ok := seen[functionID]; ok {
			continue
		}
		seen[functionID] = struct{}{}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			// 区分两种情况：契约缺失（ErrRecordNotFound）是正常业务态——页面
			// 引用的函数可能尚未注册，跳过即可；其他错误（表故障/连接失败等）
			// 不能静默吞成「无契约」，否则契约表故障被掩盖且调用方的错误检查
			// 沦为死代码。
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, fmt.Errorf("load contract %s: %w", functionID, err)
		}
		contracts = append(contracts, contract)
	}
	return contracts, nil
}

// loadTermDictionary loads platform term dictionary entries for generated
// labels. Nil result keeps the humanize fallback path.
func (s *Service) loadTermDictionary(ctx context.Context) generator.TermDictionary {
	if s == nil || s.db == nil {
		return nil
	}
	items, err := model.NewTermDictionaryModel(s.db).List(ctx, "resource")
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

func (s *Service) regenerateStandaloneProposal(ctx context.Context, gameID, env string, pageSpec spec.PageSpec, page *model.PageSpec) error {
	mainContract, err := s.mainContractForStandalonePage(ctx, gameID, env, pageSpec)
	if err != nil {
		return err
	}
	functions, err := s.functionSpecsByID(ctx, gameID, env, pageSpec)
	if err != nil {
		return err
	}
	generated := generator.GenerateForOperation(service.OperationSpecFromContract(mainContract), generator.GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions:     functions,
		Terms:         s.loadTermDictionary(ctx),
	})
	// 错误级诊断（函数禁用/schema 非法等）= 重新生成无法产出可用页面，
	// 直接失败并把原因带回去——假装成功、刷新后报错依旧，只会更困惑。
	for i := range generated.Diagnostics {
		if generated.Diagnostics[i].Severity != spec.SeverityError {
			continue
		}
		return errorx.NewValidationError(fmt.Sprintf(
			"无法重新生成：%s（%s）。请先修复函数契约状态（如启用函数/修正 schema）后再试",
			generated.Diagnostics[i].Message, generated.Diagnostics[i].Code))
	}
	// 页面身份以现存 pageKey 为准：生成器派生 key 可能因契约 execution/
	// resource 语义变化而漂移，此时内容仍刷新到当前页面（重新生成 =
	// 以当前页为身份重写），漂移仅记录诊断，不再 409 卡死页面。
	if strings.TrimSpace(generated.PageKey) != strings.TrimSpace(pageSpec.PageKey) {
		slog.WarnContext(ctx, "regenerated pageKey drifted; keeping current page identity",
			"current", pageSpec.PageKey, "derived", generated.PageKey)
		generated.PageKey = pageSpec.PageKey
	}
	proposalKey := strings.TrimSpace(page.BaseProposalKey)
	if proposalKey == "" {
		proposalKey = proposalKeyForPage(pageSpec.Type, strings.TrimSpace(mainContract.FunctionID))
	}
	// proposalKey 恒非空：mainContract 来自 FindByScopeAndFunctionID(gameID,
	// env, functionID)，functionID 为 trim 后非空值且即查询条件，返回记录的
	// FunctionID 恒等于该非空值，proposalKeyForPage 对非空 functionID 恒返回
	// 非空 key——原 proposalKey=="" 防御分支为死代码，已删。
	return s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, []*model.FunctionContract{mainContract}, generated)
}

func (s *Service) mainContractForStandalonePage(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) (*model.FunctionContract, error) {
	if pageSpec.Type == spec.PageTypeResource {
		return nil, errorx.NewValidationError("resource pages must be regenerated from resource semantics")
	}
	for _, binding := range pageSpec.Bindings {
		if binding.Usage != spec.BindingUsageAction &&
			binding.Usage != spec.BindingUsageTask &&
			binding.Usage != spec.BindingUsageReport {
			continue
		}
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			continue
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, errorx.NewNotFound("main page function contract not found")
		}
		return contract, nil
	}
	return nil, errorx.NewValidationError("page has no main executable binding")
}

func (s *Service) functionSpecsByID(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) (map[string]spec.FunctionSpec, error) {
	contracts, err := s.contractsForPage(ctx, gameID, env, pageSpec)
	if err != nil {
		return nil, err
	}
	return service.FunctionSpecsFromContracts(contracts), nil
}

func (s *Service) upsertGeneratedProposal(ctx context.Context, gameID, env, proposalKey string, contracts []*model.FunctionContract, generated spec.GeneratedPageSpec) error {
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
		GeneratorVersion: service.PageProposalGeneratorVersion,
		FunctionDigest:   computeDigest(contracts),
		Title:            localizedTextToJSONMap(generated.Title),
		Description:      localizedTextToJSONMap(generated.Description),
		CategoryKey:      generated.Category.Key,
		PageSpec:         model.JSON(pageJSON),
		Diagnostics:      jsonValue(generated.Diagnostics),
		Status:           dbenum.ProposalStatusPending,
		UpdatedBy:        actorFromContext(ctx),
	}
	if err := s.proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return fmt.Errorf("upsert page proposal %s: %w", proposalKey, err)
	}
	if _, err := createProposalVersionSnapshot(ctx, s.proposalVersionModel, proposal, "regenerate proposal from latest page contracts", actorFromContext(ctx)); err != nil {
		return fmt.Errorf("snapshot page proposal %s: %w", proposalKey, err)
	}
	return nil
}

func createProposalVersionSnapshot(
	ctx context.Context,
	versionModel *model.PageProposalVersionModel,
	proposal *model.PageProposal,
	reason string,
	actor string,
) (int, error) {
	if proposal == nil || proposal.ID == 0 {
		return 0, errorx.NewValidationError("proposal must be persisted before snapshot")
	}
	latest, err := versionModel.LatestByProposalID(ctx, proposal.ID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		latest = nil
	}
	proposalJSON, err := json.Marshal(proposal)
	if err != nil {
		return 0, err
	}
	if latest != nil {
		var latestProposal model.PageProposal
		if err := json.Unmarshal(latest.Proposal, &latestProposal); err != nil {
			return 0, err
		}
		if latestProposal.FunctionDigest == proposal.FunctionDigest &&
			latestProposal.SemanticsDigest == proposal.SemanticsDigest &&
			proposalPageSpecsEqual(latestProposal.PageSpec, proposal.PageSpec) {
			return latest.Version, nil
		}
	}
	nextVersion, err := versionModel.GetNextVersion(ctx, proposal.ID)
	if err != nil {
		return 0, err
	}
	if err := versionModel.CreateVersion(ctx, &model.PageProposalVersion{
		ProposalID:      proposal.ID,
		Version:         nextVersion,
		Proposal:        proposalJSON,
		FunctionDigest:  proposal.FunctionDigest,
		SemanticsDigest: proposal.SemanticsDigest,
		ChangeReason:    reason,
		CreatedBy:       actor,
	}); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func proposalPageSpecsEqual(snapshot model.JSON, current model.JSON) bool {
	var snapshotPage, currentPage spec.PageSpec
	if err := json.Unmarshal(snapshot, &snapshotPage); err != nil {
		return false
	}
	if err := json.Unmarshal(current, &currentPage); err != nil {
		return false
	}
	snapshotJSON, err := json.Marshal(snapshotPage)
	if err != nil {
		return false
	}
	currentJSON, err := json.Marshal(currentPage)
	if err != nil {
		return false
	}
	return bytes.Equal(snapshotJSON, currentJSON)
}

// RegenerateProposalRequest is the request for regenerating proposal.
type RegenerateProposalRequest struct {
	GameID  string `json:"-"`
	Env     string `json:"-"`
	PageKey string `uri:"pageKey" json:"-"`
	Force   bool   `json:"force,omitempty"`
}

// RegenerateProposalResponse is the response for regenerating proposal.
type RegenerateProposalResponse struct {
	Message string `json:"message"`
}

// RegenerateProposal regenerates a proposal from current contracts and semantics.
func (s *Service) RegenerateProposal(ctx context.Context, req *RegenerateProposalRequest) (*RegenerateProposalResponse, error) {
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	pageSpec, err := pageSpecFromModel(page)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(page.ResourceKey) != "" && pageSpec.Type == spec.PageTypeResource {
		contractService := service.NewContractService(s.db)
		if err := contractService.RebuildProposalsForResource(ctx, req.GameID, req.Env, page.ResourceKey); err != nil {
			return nil, fmt.Errorf("regenerate proposal: %w", err)
		}
		return &RegenerateProposalResponse{
			Message: fmt.Sprintf("proposal regenerated for page %s", pageKey),
		}, nil
	}

	if err := s.regenerateStandaloneProposal(ctx, req.GameID, req.Env, pageSpec, page); err != nil {
		return nil, err
	}
	return &RegenerateProposalResponse{
		Message: fmt.Sprintf("proposal regenerated for page %s", pageKey),
	}, nil
}

// RepublishRequest is the request for republishing.
type RepublishRequest struct {
	GameID  string `json:"-"`
	Env     string `json:"-"`
	PageKey string `uri:"pageKey" json:"-"`
	Reason  string `json:"reason,omitempty"`
}

// RepublishResponse is the response for republishing.
type RepublishResponse struct {
	Version int    `json:"version"`
	Message string `json:"message"`
}

// Republish creates a new published version from current draft.
func (s *Service) Republish(ctx context.Context, req *RepublishRequest) (*RepublishResponse, error) {
	if err := s.requirePublishPermission(ctx); err != nil {
		return nil, err
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	actor := actorFromContext(ctx)
	currentRevision := page.DraftRevision
	var nextVersion int
	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		current, err := pageModel.FindByScopeAndPageKey(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return errorx.NewNotFound("page draft not found")
		}
		if current.DraftRevision != currentRevision {
			return draftRevisionConflict(currentRevision, current.DraftRevision)
		}
		if s.publishDraft == nil {
			return fmt.Errorf("canonical versioning publisher is not configured")
		}
		nextVersion, err = s.publishDraft.PublishVersioningDraft(txCtx, req.GameID, req.Env, actor, pageKey, currentRevision, firstNonEmpty(req.Reason, "republish current draft"))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &RepublishResponse{
		Version: nextVersion,
		Message: fmt.Sprintf("republished %s at version %d", pageKey, nextVersion),
	}, nil
}

func (s *Service) republishLocalCore(ctx context.Context, req *RepublishRequest, pageKey string, currentRevision int, actor string) (int, error) {
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return 0, errorx.NewNotFound("page draft not found")
	}
	if page.DraftRevision != currentRevision {
		return 0, draftRevisionConflict(currentRevision, page.DraftRevision)
	}
	pageSpec, err := pageSpecFromModel(page)
	if err != nil {
		return 0, err
	}
	contracts, err := s.buildBindingContracts(ctx, req.GameID, req.Env, pageSpec.Bindings)
	if err != nil {
		return 0, err
	}
	specJSON, _ := marshalPageSpec(pageSpec)
	contractsJSON, _ := json.Marshal(contracts)
	now := time.Now()
	var nextVersion int
	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		publishedModel := model.NewPublishedPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))
		var err error
		nextVersion, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		if err := publishedModel.DeactivatePage(txCtx, req.GameID, req.Env, pageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                req.GameID,
			Env:                   req.Env,
			PageKey:               pageKey,
			Version:               nextVersion,
			SpecJSON:              specJSON,
			BindingContractsJSON:  string(contractsJSON),
			RendererSchemaVersion: rendererSchemaVersion,
			BaseProposalKey:       page.BaseProposalKey,
			BaseProposalVersion:   page.BaseProposalVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		page.Status = "published"
		page.PublishedActive = true
		page.PublishedVersion = nextVersion
		page.DraftRevision = nextVersion
		page.UpdatedAt = now
		page.UpdatedBy = actor
		if err := pageModel.Upsert(txCtx, page); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextVersion,
			SpecJSON:  specJSON,
			Status:    "published",
			Message:   firstNonEmpty(req.Reason, "republish current draft"),
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func proposalKeyForPage(pageType spec.PageType, functionID string) string {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return ""
	}
	switch pageType {
	case spec.PageTypeTask:
		return "task:" + functionID
	case spec.PageTypeReport:
		return "report:" + functionID
	default:
		return "operation:" + functionID
	}
}

// CreateCompositePage 聚合多资源生成组合页提案（委托 ContractService，
// 复用提案/接受/发布既有工作流）。componentTemplates 为页面级模板快照
// （U11 更新提醒），原样透传进 PageSpec。
// CreateCompositePage 保存组合页提案；发布分级（T10）auto 策略下保存成功
// 后直接自动发布（发布失败不回滚保存——提案仍在，PublishError 带回前端
// 降级人工链）。
func (s *Service) CreateCompositePage(ctx context.Context, gameID, env, pageKey string, sections []service.CompositeSectionRequest, componentTemplates []spec.ComponentTemplateUsage) (*CompositePageOutcome, error) {
	proposal, err := service.NewContractService(s.db).CreateCompositeProposal(ctx, gameID, env, pageKey, sections, componentTemplates)
	if err != nil {
		return nil, err
	}
	outcome := &CompositePageOutcome{Proposal: proposal}
	if s.publishReviewPolicy == nil || s.publishReviewPolicy(env) != config.PublishReviewAuto || s.autoPublish == nil {
		return outcome, nil
	}
	if err := s.autoPublish(ctx, gameID, env, proposal.ProposalKey); err != nil {
		slog.Warn("auto publish composite failed; proposal kept for manual flow",
			"gameId", gameID, "env", env, "proposalKey", proposal.ProposalKey, "error", err)
		outcome.PublishError = err.Error()
		return outcome, nil
	}
	outcome.Published = true
	return outcome, nil
}

// PreviewCompositePage 生成组合页 PageSpec 而不落库：编辑器实时预览消费与
// 「保存→发布」同一份生成链（同一区块契约解析、绑定分配、static 交错、模板
// 快照与发布级 selector 校验），预览产物与发布产物逐字段一致。
// 纯只读：不产生提案/草稿/审计，故无需 pages:publish 门槛；契约可见性仍受
// 请求链上的 scope 与权限中间件约束。
func (s *Service) PreviewCompositePage(ctx context.Context, gameID, env, pageKey string, sections []service.CompositeSectionRequest, componentTemplates []spec.ComponentTemplateUsage) (spec.GeneratedPageSpec, error) {
	generated, _, err := service.NewContractService(s.db).BuildCompositePageSpec(ctx, gameID, env, pageKey, sections, componentTemplates)
	if err != nil {
		return spec.GeneratedPageSpec{}, err
	}
	return generated, nil
}
