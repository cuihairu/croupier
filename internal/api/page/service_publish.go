package page

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"gorm.io/gorm"
)

type proposalDraftPublisher struct {
	service *Service
}

func (p proposalDraftPublisher) PublishProposalDraft(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, proposalKey string) error {
	return p.service.PublishProposalDraft(ctx, gameID, env, actor, pageKey, expectedDraftRevision, proposalKey)
}

func (s *Service) Publish(ctx context.Context, req *PagePublishRequest) (*PagePublishResponse, error) {
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := currentUsername(ctx)
	if err != nil {
		return nil, err
	}
	return s.publishCore(ctx, gameID, env, actor, req)
}

// publishCore 是 Publish 的核心（无权限/scope/actor 解析），供发布分级
// （T10 auto 保存即发布）以保存操作者身份复用；乐观锁与页面校验保留。
func (s *Service) publishCore(ctx context.Context, gameID, env, actor string, req *PagePublishRequest) (*PagePublishResponse, error) {
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}

	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	if p.DraftRevision != *req.DraftRevision {
		return nil, errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
			"expected": p.DraftRevision,
			"current":  p.DraftRevision,
			"provided": *req.DraftRevision,
		})
	}

	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return nil, err
	}
	diags := s.validatePageSpec(ctx, pageSpec, true)
	if countErrors(diags) > 0 {
		return nil, errorx.NewValidationErrorWithDetails("page validation failed", diagnosticsToDetails(diags))
	}

	functions := s.normalizedFunctions(ctx)
	functions = s.functionsForScope(ctx, gameID, env)
	contracts, err := buildBindingContracts(pageSpec.Bindings, functions)
	if err != nil {
		return nil, err
	}
	specJSON, err := pageJSONMarshal(pageSpec)
	if err != nil {
		return nil, err
	}
	contractsJSON, err := pageJSONMarshal(contracts)
	if err != nil {
		return nil, err
	}
	publishSource := s.pagePublishSource(ctx, gameID, env, p)

	now := time.Now()
	publishedVersion := p.DraftRevision
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		latestPage, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, req.PageKey)
		if err != nil {
			return ErrPageNotFound(req.PageKey)
		}
		if latestPage.DraftRevision != *req.DraftRevision {
			return errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
				"expected": *req.DraftRevision,
				"current":  latestPage.DraftRevision,
			})
		}
		if err := publishedModel.DeactivatePage(txCtx, gameID, env, req.PageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                gameID,
			Env:                   env,
			PageKey:               req.PageKey,
			Version:               publishedVersion,
			SpecJSON:              string(specJSON),
			BindingContractsJSON:  string(contractsJSON),
			RendererSchemaVersion: rendererSchemaVersion,
			BaseProposalKey:       publishSource.BaseProposalKey,
			BaseProposalVersion:   publishSource.BaseProposalVersion,
			FunctionDigest:        publishSource.FunctionDigest,
			SemanticsDigest:       publishSource.SemanticsDigest,
			GeneratorVersion:      publishSource.GeneratorVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		latestPage.Status = "published"
		latestPage.PublishedActive = true
		latestPage.PublishedVersion = publishedVersion
		latestPage.UpdatedAt = now
		if err := pageModel.Upsert(txCtx, latestPage); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   req.PageKey,
			Version:   publishedVersion,
			SpecJSON:  string(specJSON),
			Status:    "published",
			Message:   "publish",
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPagePublish, gameID, env, req.PageKey, map[string]interface{}{
		"draft_revision":    p.DraftRevision,
		"published_version": publishedVersion,
		"diagnostic_errors": countErrors(diags),
		"binding_count":     len(pageSpec.Bindings),
	})
	return &PagePublishResponse{PageKey: req.PageKey, Published: true, PublishedVersion: publishedVersion}, nil
}

func (s *Service) Unpublish(ctx context.Context, req *PageUnpublishRequest) (*PageUnpublishResponse, error) {
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	now := time.Now()
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, _ *model.PageVersionModel) error {
		p.Status = "draft"
		p.PublishedActive = false
		p.UpdatedAt = now
		if err := pageModel.Upsert(txCtx, p); err != nil {
			return err
		}
		return publishedModel.DeactivatePage(txCtx, gameID, env, req.PageKey, now)
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPageUnpublish, gameID, env, req.PageKey, map[string]interface{}{
		"draft_revision":    p.DraftRevision,
		"published_version": p.PublishedVersion,
	})
	return &PageUnpublishResponse{PageKey: req.PageKey, Published: false}, nil
}

type proposalReplacement struct {
	spec.PageSpec
	ProposalKey     string
	ProposalVersion int
	Quality         spec.GeneratedPageQuality
	Diagnostics     []spec.Diagnostic
}

func (s *Service) proposalReplacementForDraft(ctx context.Context, gameID string, env string, draft *model.PageSpec) (proposalReplacement, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return proposalReplacement{}, errorx.NewInternalError("page service database is not initialized")
	}
	if draft == nil {
		return proposalReplacement{}, errorx.NewBadRequest("page draft is required")
	}
	proposalModel := model.NewPageProposalModel(s.svcCtx.DB)
	proposalKey := strings.TrimSpace(draft.BaseProposalKey)
	var proposal *model.PageProposal
	var err error
	if proposalKey != "" {
		proposal, err = proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	} else {
		proposal, err = proposalModel.FindByScopeAndPageKey(ctx, gameID, env, draft.PageKey)
	}
	if err != nil {
		return proposalReplacement{}, errorx.NewBadRequestWithDetails("latest PageProposal is required for default regeneration", map[string]any{
			"pageKey":      draft.PageKey,
			"proposalKey":  proposalKey,
			"requiredFlow": "regenerate PageProposal from FunctionContract/CapabilitySemantics, then regenerate draft",
		})
	}
	// 注：FindByScopeAndKey/FindByScopeAndPageKey 的契约是「无记录时返回
	// (nil, ErrRecordNotFound)、有记录时返回 (&proposal, nil)」，err == nil 时
	// proposal 恒非 nil，原 proposal == nil 防御分支为死代码，已删（A 类）。
	if proposal.Status != dbenum.ProposalStatusPending && proposal.Status != dbenum.ProposalStatusAccepted {
		return proposalReplacement{}, errorx.NewBadRequestWithDetails("PageProposal is not usable for regeneration", map[string]any{
			"proposalKey": proposal.ProposalKey,
			"status":      proposal.Status.String(),
		})
	}
	pageSpec, err := pageSpecFromProposalModel(proposal)
	if err != nil {
		return proposalReplacement{}, err
	}
	if strings.TrimSpace(pageSpec.PageKey) != strings.TrimSpace(draft.PageKey) {
		return proposalReplacement{}, errorx.NewBadRequestWithDetails("PageProposal pageKey does not match draft", map[string]any{
			"draftPageKey":    draft.PageKey,
			"proposalPageKey": pageSpec.PageKey,
			"proposalKey":     proposal.ProposalKey,
		})
	}
	version, err := model.NewPageProposalVersionModel(s.svcCtx.DB).LatestByProposalID(ctx, proposal.ID)
	if err != nil || version == nil || version.Version <= 0 {
		return proposalReplacement{}, errorx.NewBadRequestWithDetails("PageProposal version is required for default regeneration", map[string]any{
			"proposalKey": proposal.ProposalKey,
			"pageKey":     draft.PageKey,
		})
	}
	return proposalReplacement{
		PageSpec:        pageSpec,
		ProposalKey:     strings.TrimSpace(proposal.ProposalKey),
		ProposalVersion: version.Version,
		Quality:         spec.GeneratedPageQuality(proposal.Quality),
		Diagnostics:     diagnosticsFromJSON(proposal.Diagnostics),
	}, nil
}

type pagePublishSource struct {
	BaseProposalKey     string
	BaseProposalVersion int
	FunctionDigest      string
	SemanticsDigest     string
	GeneratorVersion    string
}

// pagePublishEntry models the only permitted differences between publish
// callers. Everything else uses the same permission policy, draft optimistic
// lock, validation, contract snapshot, transaction, and audit contract.
type pagePublishEntry struct {
	Caller           pagePublishCaller
	Actor            string
	AllowsAutoPolicy bool
	RequiresPublish  bool
	InternalCaller   bool
	GameID           string
	Env              string
}

type pagePublishCaller string

const (
	pagePublishCallerAPI              pagePublishCaller = "api.publish"
	pagePublishCallerRepublish        pagePublishCaller = "page.republish"
	pagePublishCallerProposalAccept   pagePublishCaller = "proposal.accept_and_publish"
	pagePublishCallerAutoPublish      pagePublishCaller = "auto.publish_composite"
	pagePublishCallerVersionRepublish pagePublishCaller = "versioning.republish"
	pagePublishCallerVersionRollback  pagePublishCaller = "versioning.rollback_publish"
	pagePublishCallerStaleHeal        pagePublishCaller = "stale_heal"
)

// PublishProposalDraft adapts the package-safe proposal boundary to the same
// explicit proposal-accept entry used by HTTP and bulk acceptance.
func (s *Service) PublishProposalDraft(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, proposalKey string) error {
	entry := pagePublishEntry{
		Caller:           pagePublishCallerProposalAccept,
		Actor:            actor,
		AllowsAutoPolicy: false,
		RequiresPublish:  true,
		GameID:           gameID,
		Env:              env,
	}
	response, err := s.publishEntryCore(ctx, entry, &PagePublishRequest{PageKey: pageKey, DraftRevision: &expectedDraftRevision})
	if err != nil {
		return err
	}
	s.auditPageEvent(ctx, audit.EventPagePublish, gameID, env, pageKey, map[string]interface{}{
		"action":            "accept_and_publish",
		"caller":            pagePublishCallerProposalAccept,
		"proposal":          proposalKey,
		"draft_revision":    expectedDraftRevision,
		"published_version": response.PublishedVersion,
	})
	return nil
}

// PublishProposalDraftForService is the cross-package adapter used when the
// dashboard proposal service cannot import this package directly. It exposes
// only the canonical proposal-accept entry and never an escape hatch.
func (s *Service) PublishProposalDraftForService() contractsvc.ProposalDraftPublisher {
	return proposalDraftPublisher{service: s}
}

// PublishFromEntry is the shared boundary for cross-package entries whose
// authorization is owned by the caller (proposal accept/auto-save and
// versioning republish). It still enforces the canonical pages:publish rule.
func (s *Service) PublishFromEntry(ctx context.Context, entry pagePublishEntry, req *PagePublishRequest) (*PagePublishResponse, error) {
	return s.publishFromEntry(ctx, entry, req)
}

func (s *Service) publishFromEntry(ctx context.Context, entry pagePublishEntry, req *PagePublishRequest) (*PagePublishResponse, error) {
	if entry.RequiresPublish {
		if err := s.requirePagePublish(ctx); err != nil {
			return nil, err
		}
	}
	return s.publishEntryCore(ctx, entry, req)
}

func (s *Service) publishEntryCore(ctx context.Context, entry pagePublishEntry, req *PagePublishRequest) (*PagePublishResponse, error) {
	switch entry.Caller {
	case pagePublishCallerProposalAccept:
		if entry.InternalCaller {
			return nil, fmt.Errorf("proposal accept cannot bypass pages:publish")
		}
	case pagePublishCallerAPI, pagePublishCallerRepublish:
	case pagePublishCallerAutoPublish, pagePublishCallerVersionRepublish:
		if !entry.InternalCaller {
			return nil, fmt.Errorf("entry %q is internal-only", entry.Caller)
		}
	case pagePublishCallerVersionRollback:
	default:
		return nil, fmt.Errorf("unknown publish entry %q", entry.Caller)
	}
	if entry.GameID == "" || entry.Env == "" {
		gameID, env, err := requireScope(ctx)
		if err != nil {
			return nil, err
		}
		entry.GameID, entry.Env = gameID, env
	}
	if entry.Actor == "" {
		actor, err := currentUsername(ctx)
		if err != nil {
			return nil, err
		}
		entry.Actor = actor
	}
	if req == nil {
		return nil, errorx.NewBadRequest("publish request is required")
	}
	return s.publishCore(ctx, entry.GameID, entry.Env, entry.Actor, req)
}

// RepublishDraftFromEntry republishes an existing draft through the canonical
// core. Version-specific semantics (new revision from draft and stale-draft
// protection) remain in the caller; validation and snapshots do not.
type RepublishDraftRequest struct {
	PageKey               string
	ExpectedDraftRevision *int
}

func (s *Service) RepublishDraftFromEntry(ctx context.Context, entry pagePublishEntry, req *RepublishDraftRequest) (*PagePublishResponse, error) {
	if req == nil {
		return nil, errorx.NewBadRequest("republish request is required")
	}
	if entry.RequiresPublish {
		if err := s.requirePagePublish(ctx); err != nil {
			return nil, err
		}
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	if entry.GameID == "" || entry.Env == "" {
		gameID, env, err := requireScope(ctx)
		if err != nil {
			return nil, err
		}
		entry.GameID, entry.Env = gameID, env
	}
	if entry.Actor == "" {
		actor, err := currentUsername(ctx)
		if err != nil {
			return nil, err
		}
		entry.Actor = actor
	}
	switch entry.Caller {
	case pagePublishCallerRepublish, pagePublishCallerVersionRepublish:
	default:
		return nil, fmt.Errorf("entry %q cannot republish a draft", entry.Caller)
	}

	page, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, entry.GameID, entry.Env, pageKey)
	if err != nil {
		return nil, ErrPageNotFound(pageKey)
	}
	if req.ExpectedDraftRevision != nil && page.DraftRevision != *req.ExpectedDraftRevision {
		return nil, errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
			"expected": *req.ExpectedDraftRevision,
			"current":  page.DraftRevision,
		})
	}
	response, err := s.publishCore(ctx, entry.GameID, entry.Env, entry.Actor, &PagePublishRequest{
		PageKey:       pageKey,
		DraftRevision: &page.DraftRevision,
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPagePublish, entry.GameID, entry.Env, pageKey, map[string]interface{}{
		"action":            "republish_draft",
		"caller":            entry.Caller,
		"draft_revision":    page.DraftRevision,
		"published_version": response.PublishedVersion,
	})
	return response, nil
}

// PublishExistingDraftForProposal keeps auto-policy behavior explicit. It is
// internal-only: the caller must already hold pages:edit and have resolved the
// proposal-to-draft transition. The draft optimistic lock and validation are
// never bypassed.
func (s *Service) PublishExistingDraftForProposal(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, proposalKey string) (*PagePublishResponse, error) {
	entry := pagePublishEntry{
		Caller:           pagePublishCallerAutoPublish,
		Actor:            actor,
		AllowsAutoPolicy: true,
		InternalCaller:   true,
		GameID:           gameID,
		Env:              env,
	}
	response, err := s.publishEntryCore(ctx, entry, &PagePublishRequest{PageKey: pageKey, DraftRevision: &expectedDraftRevision})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPagePublish, gameID, env, pageKey, map[string]interface{}{
		"action":            "auto_publish_composite",
		"variant":           "regenerate_and_publish",
		"proposal":          proposalKey,
		"draft_revision":    expectedDraftRevision,
		"published_version": response.PublishedVersion,
	})
	return response, nil
}

func (s *Service) pagePublishSource(ctx context.Context, gameID string, env string, page *model.PageSpec) pagePublishSource {
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil || page == nil {
		return pagePublishSource{}
	}
	source := pagePublishSource{
		BaseProposalKey:     strings.TrimSpace(page.BaseProposalKey),
		BaseProposalVersion: page.BaseProposalVersion,
	}
	if source.BaseProposalKey == "" {
		return source
	}
	proposal, err := model.NewPageProposalModel(s.svcCtx.DB).FindByScopeAndKey(ctx, gameID, env, source.BaseProposalKey)
	if err != nil || proposal == nil {
		return source
	}
	source.FunctionDigest = strings.TrimSpace(proposal.FunctionDigest)
	source.SemanticsDigest = strings.TrimSpace(proposal.SemanticsDigest)
	source.GeneratorVersion = strings.TrimSpace(proposal.GeneratorVersion)
	return source
}

func ErrPageNotFound(key string) error {
	return &PageNotFoundError{Key: key}
}

type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return "page not found: " + e.Key
}

// RebuildAllProposals regenerates every page proposal in the current scope
// from the latest FunctionContract / CapabilitySemantics. Proposals whose
// content or generator version changed are updated and snapshotted; published
// pages keep their published snapshot until explicitly republished.
func (s *Service) RebuildAllProposals(ctx context.Context) (*PageProposalsRebuildResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	// 注：requireScope 成功即保证返回 TrimSpace 后非空的 gameID/env
	// （见其实现），原此处的重复空值防御分支为死代码，已删（A 类）。

	contractSvc := contractsvc.NewContractService(s.svcCtx.DB)
	if err := contractSvc.RebuildAllProposals(ctx, gameID, env); err != nil {
		return nil, err
	}
	slog.Default().Info("page proposals rebuilt",
		"game_id", gameID,
		"env", env,
	)
	return &PageProposalsRebuildResponse{GameID: gameID, Env: env}, nil
}

// BulkPublish 一键发布全部：先重算提案（拉取最新契约生成），随后将
// pending 且质量为 ready/basic 的提案逐一 accept-and-publish。
// 返回发布成功与失败清单（失败的包含原因）。
func (s *Service) BulkPublish(ctx context.Context, req *PageBulkRequest) (*PageBulkResult, error) {
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	contractSvc := contractsvc.NewContractService(s.svcCtx.DB)
	if err := contractSvc.RebuildAllProposals(ctx, gameID, env); err != nil {
		return nil, err
	}

	proposalSvc := s.newInternalProposalService()
	pending, err := proposalSvc.ListProposalsByStatus(ctx, gameID, env, dbenum.ProposalStatusPending)
	if err != nil {
		return nil, err
	}

	result := &PageBulkResult{Total: len(pending)}
	for _, proposal := range pending {
		if proposal.Quality != "ready" && proposal.Quality != "basic" {
			result.Skipped = append(result.Skipped, proposal.PageKey)
			continue
		}
		if _, err := proposalSvc.AcceptAndPublishProposal(ctx, gameID, env, proposal.ProposalKey); err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": proposal.PageKey,
				"error":   err.Error(),
			})
			continue
		}
		result.Published = append(result.Published, proposal.PageKey)
	}
	return result, nil
}

// AutoPublishComposite 发布分级（T10/D5，pages.publishReview=auto）：composite
// 保存链保存成功后调用，跳过人工提案接受直接发布。发布仍要求 pages:publish；
// env 免审核只豁免人工审核，不豁免发布权限。
//
// 质量门槛不因免审核降低：提案 error 级诊断（HasBlockingDiagnostics）与
// 页面校验（publishCore 的 validatePageSpec）照常拒绝发布，调用方将错误
// 降级为人工链提示（保存本身已成功）。
//
// 新页面走提案接受发布链（AcceptAndPublishProposal）；已存在页面走
// 「提案重建草稿 + 发布」组合（与 BulkRepublish 单页路径一致：乐观锁、
// published_page_specs 快照与 page_versions 历史照常记录）。
func (s *Service) AutoPublishComposite(ctx context.Context, gameID, env, proposalKey string) error {
	if err := s.requirePagePublish(ctx); err != nil {
		return err
	}
	proposalSvc := s.newInternalProposalService()
	proposal, err := proposalSvc.GetProposal(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("load proposal %s: %w", proposalKey, err)
	}
	if contractsvc.HasBlockingDiagnostics(proposal) {
		return fmt.Errorf("proposal %s has blocking diagnostics; resolve errors before publishing", proposalKey)
	}

	existing, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, proposal.PageKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load page draft %s: %w", proposal.PageKey, err)
	}
	if existing == nil {
		if _, err := proposalSvc.AcceptAndPublishProposal(ctx, gameID, env, proposalKey); err != nil {
			return err
		}
		s.auditPageEvent(ctx, audit.EventPagePublish, gameID, env, proposal.PageKey, map[string]interface{}{
			"action":   "auto_publish_composite",
			"variant":  "accept_and_publish",
			"proposal": proposalKey,
		})
		return nil
	}

	// 已存在页面：提案 → 草稿 → 发布（canonical core 已在上一步执行
	// pages:publish 检查，actor 取保存操作者）。
	actor, err := currentUsername(ctx)
	if err != nil {
		return err
	}
	revision := existing.DraftRevision
	regenerated, err := s.regenerateDraft(ctx, &PageRegenerateRequest{PageKey: proposal.PageKey, DraftRevision: &revision})
	if err != nil {
		return err
	}
	if _, err := s.PublishExistingDraftForProposal(ctx, gameID, env, actor, proposal.PageKey, regenerated.DraftRevision, proposalKey); err != nil {
		return err
	}
	return nil
}

// newInternalProposalService keeps every page-owned proposal publishing path
// on the same validation and canonical publish boundary used by HTTP routes.
func (s *Service) newInternalProposalService() *contractsvc.ProposalService {
	return s.newInternalProposalServiceForContext(s.svcCtx.DB)
}

func (s *Service) newInternalProposalServiceForContext(db *gorm.DB) *contractsvc.ProposalService {
	proposalSvc := contractsvc.NewProposalService(db)
	proposalSvc.SetProposalDraftPublisher(s.PublishProposalDraftForService())
	proposalSvc.SetPublishValidator(s)
	proposalSvc.SetPermissionGuard(func(ctx context.Context, actor string) (string, error) {
		if _, _, err := logicutils.RequireAnyPermission(
			ctx, s.svcCtx, "无权发布页面", "admin:all", "pages:publish",
		); err != nil {
			return "", err
		}
		return actor, nil
	})
	return proposalSvc
}

// BulkUnpublish 一键下架全部：将 scope 内所有已发布页面逐一下线
// （复用单页 Unpublish 的真实下线链路：快照停用 + 草稿回退）。
func (s *Service) BulkUnpublish(ctx context.Context, req *PageBulkRequest) (*PageBulkResult, error) {
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	pages, err := s.svcCtx.PageSpecModel.ListByScopeAndStatus(ctx, gameID, env, "published")
	if err != nil {
		return nil, err
	}

	result := &PageBulkResult{Total: len(pages)}
	for _, page := range pages {
		if _, err := s.Unpublish(ctx, &PageUnpublishRequest{PageKey: page.PageKey}); err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": page.PageKey,
				"error":   err.Error(),
			})
			continue
		}
		result.Unpublished = append(result.Unpublished, page.PageKey)
	}
	return result, nil
}

// BulkRepublish 一键重新发布（契约变更队列）：对契约已漂移的已发布页面
// 逐页「重生成草稿 → 发布」，把线上快照拉齐到最新契约。未显式指定
// pageKeys 时，复用审批收件箱同源的 stale 评估且只取已发布条目——草稿态
// 的契约变更不在此批量发布（从未上线的页面不应被一键上线）。单页失败
// 记录原因并继续，不中断其余页面。
func (s *Service) BulkRepublish(ctx context.Context, req *PageBulkRepublishRequest) (*PageBulkResult, error) {
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	pageKeys := make([]string, 0, len(req.PageKeys))
	for _, key := range req.PageKeys {
		if key = strings.TrimSpace(key); key != "" {
			pageKeys = append(pageKeys, key)
		}
	}
	if len(pageKeys) == 0 {
		proposalSvc := contractsvc.NewProposalService(s.svcCtx.DB)
		inbox, err := proposalSvc.Inbox(ctx, gameID, env, contractsvc.ProposalListFilter{})
		if err != nil {
			return nil, err
		}
		for _, change := range inbox.ContractChanges {
			if change.Kind == "published" {
				pageKeys = append(pageKeys, change.PageKey)
			}
		}
	}

	result := &PageBulkResult{Total: len(pageKeys)}
	for _, pageKey := range pageKeys {
		// regenerateDraft 带乐观锁：按当前草稿版本重生成；并发修改会在此
		// 冲突失败并记入 failed（与单页流程语义一致）。
		p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, pageKey)
		if err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   ErrPageNotFound(pageKey).Error(),
			})
			continue
		}
		revision := p.DraftRevision
		regenerated, err := s.regenerateDraft(ctx, &PageRegenerateRequest{PageKey: pageKey, DraftRevision: &revision})
		if err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   err.Error(),
			})
			continue
		}
		publishedRevision := regenerated.DraftRevision
		actor, err := currentUsername(ctx)
		if err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   err.Error(),
			})
			continue
		}
		// BulkRepublish 已验 pages:publish；逐页复用显式入口保证语义一致。
		if _, err := s.publishFromEntry(ctx, pagePublishEntry{
			Caller:           pagePublishCallerRepublish,
			Actor:            actor,
			AllowsAutoPolicy: false,
			RequiresPublish:  false,
			GameID:           gameID,
			Env:              env,
		}, &PagePublishRequest{PageKey: pageKey, DraftRevision: &publishedRevision}); err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   err.Error(),
			})
			continue
		}
		result.Published = append(result.Published, pageKey)
	}
	return result, nil
}

// SeedDemoData 填充演示数据（F：演示站点）：
//  1. Terms 词条字典写入 resource/operation 显示名；
//  2. RebuildAllProposals 按已注册契约重算提案（Terms 生效）；
//  3. 一键发布全部 ready/basic 提案（复用 BulkPublish 真实发布链路）；
//  4. 伪造演示注册警告（registry 内存存储）供 /functions/warnings 验证。
//
// 幂等性：Terms/警告按 key upsert；已发布提案跳过。只动 demo-game scope。
func (s *Service) SeedDemoData(ctx context.Context, req *PageSeedDemoRequest) (*PageSeedDemoResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}

	seeder := newDemoDataSeeder(s, gameID, env)
	summary, err := seeder.seed(ctx)
	if err != nil {
		return nil, err
	}
	return &PageSeedDemoResponse{Summary: summary}, nil
}

// PageSeedDemoRequest 演示数据填充请求。
type PageSeedDemoRequest struct{}

// PageSeedDemoResponse 演示数据填充结果摘要。
type PageSeedDemoResponse struct {
	Summary map[string]interface{} `json:"summary"`
}
