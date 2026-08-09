// Package page provides canonical PageSpec management API.
package page

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

const rendererSchemaVersion = "page-spec:1"

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) ListDrafts(ctx context.Context, req *PageDraftListRequest) (*PageDraftListResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	var pages []model.PageSpec
	if strings.TrimSpace(req.Status) != "" {
		pages, err = s.svcCtx.PageSpecModel.ListByScopeAndStatus(ctx, gameID, env, strings.TrimSpace(req.Status))
	} else {
		pages, err = s.svcCtx.PageSpecModel.ListByScope(ctx, gameID, env)
	}
	if err != nil {
		return nil, err
	}

	items := make([]spec.PageSpecDraftSummary, 0, len(pages))
	for _, p := range pages {
		if req.ResourceKey != "" && p.ResourceKey != req.ResourceKey {
			continue
		}
		items = append(items, spec.PageSpecDraftSummary{
			GameID:      p.GameID,
			Env:         p.Env,
			PageKey:     p.PageKey,
			Type:        spec.PageType(p.Type),
			ResourceKey: p.ResourceKey,
			Title:       p.GetTitle(),
			Category: spec.PageCategorySpec{
				Key:    p.CategoryKey,
				Labels: p.GetCategoryLabels(),
				Order:  p.CategoryOrder,
			},
			Status:           spec.PageDraftStatus(p.Status),
			DraftRevision:    p.DraftRevision,
			PublishedVersion: p.PublishedVersion,
			UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
			UpdatedBy:        p.UpdatedBy,
		})
	}
	return &PageDraftListResponse{Items: items}, nil
}

func (s *Service) GetDraft(ctx context.Context, req *PageDraftRequest) (*PageDraftResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	resp, err := pageDraftResponseFromModel(p)
	if err != nil {
		return nil, err
	}
	resp.BindingFreshness = s.bindingFreshnessForPublishedDraft(ctx, p)
	return resp, nil
}

func (s *Service) SaveDraft(ctx context.Context, req *PageSaveRequest) (*PageSaveResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}
	if strings.TrimSpace(req.PageKey) == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	pageSpec := spec.PageSpec{
		PageKey:     strings.TrimSpace(req.PageKey),
		Type:        req.Type,
		ResourceKey: strings.TrimSpace(req.ResourceKey),
		Navigation:  req.Navigation,
		Resource:    req.Resource,
		Operation:   req.Operation,
		Task:        req.Task,
		Report:      req.Report,
		Bindings:    req.Bindings,
	}
	if !isValidPageType(req.Type) {
		return nil, errorx.NewBadRequest("type must be resource, operation, task, or report")
	}
	categoryKey := strings.TrimSpace(req.Category.Key)
	if categoryKey == "" {
		return nil, errorx.NewBadRequest("category.key is required")
	}

	title := normalizeLocaleKeys(req.Title)
	if !hasDefaultLocale(title) {
		return nil, errorx.NewBadRequest("title must include zh-CN locale")
	}
	categoryLabels := normalizeLocaleKeys(req.Category.Labels)
	if !hasDefaultLocale(categoryLabels) {
		return nil, errorx.NewBadRequest("category.labels must include zh-CN locale")
	}
	pageSpec.Title = title
	pageSpec.Description = normalizeLocaleKeys(req.Description)
	pageSpec.Category = spec.PageCategorySpec{
		Key:    categoryKey,
		Labels: categoryLabels,
		Order:  req.Category.Order,
	}
	pageSpec.Order = req.Order
	pageSpec.Icon = strings.TrimSpace(req.Icon)

	specJSON, err := marshalPageSpec(pageSpec)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var nextRevision int
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		existing, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, req.PageKey)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if existing == nil && *req.DraftRevision != 0 {
			return errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
				"expected": 0,
				"current":  0,
			})
		}
		if existing != nil && existing.DraftRevision != *req.DraftRevision {
			return errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
				"expected": existing.DraftRevision,
				"current":  existing.DraftRevision,
				"provided": *req.DraftRevision,
			})
		}

		ps := &model.PageSpec{
			GameID:        gameID,
			Env:           env,
			PageKey:       strings.TrimSpace(req.PageKey),
			Type:          string(req.Type),
			ResourceKey:   strings.TrimSpace(req.ResourceKey),
			CategoryKey:   categoryKey,
			CategoryOrder: req.Category.Order,
			Order:         req.Order,
			Icon:          strings.TrimSpace(req.Icon),
			SpecJSON:      specJSON,
			Status:        "draft",
			UpdatedBy:     actor,
			UpdatedAt:     now,
		}
		if err := ps.SetTitle(title); err != nil {
			return err
		}
		if err := ps.SetCategoryLabels(categoryLabels); err != nil {
			return err
		}

		if existing != nil {
			ps.ID = existing.ID
			ps.CreatedAt = existing.CreatedAt
			ps.PublishedVersion = existing.PublishedVersion
			ps.PublishedActive = existing.PublishedActive
			ps.BaseProposalKey = existing.BaseProposalKey
			ps.BaseProposalVersion = existing.BaseProposalVersion
			ps.DraftRevision = existing.DraftRevision + 1
		} else {
			ps.CreatedAt = now
			ps.DraftRevision = 1
		}
		nextRevision = ps.DraftRevision

		if err := pageModel.Upsert(txCtx, ps); err != nil {
			return fmt.Errorf("save page draft: %w", err)
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   ps.PageKey,
			Version:   ps.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "save draft",
			CreatedBy: ps.UpdatedBy,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPageDraftSave, gameID, env, req.PageKey, map[string]interface{}{
		"draft_revision": nextRevision,
		"type":           string(req.Type),
		"resource_key":   strings.TrimSpace(req.ResourceKey),
		"category_key":   categoryKey,
		"binding_count":  len(req.Bindings),
	})
	return &PageSaveResponse{PageKey: req.PageKey, DraftRevision: nextRevision}, nil
}

func (s *Service) RegenerateDraft(ctx context.Context, req *PageRegenerateRequest) (*PageRegenerateResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}

	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	if p.DraftRevision != *req.DraftRevision {
		return nil, errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
			"expected": p.DraftRevision,
			"current":  p.DraftRevision,
			"provided": *req.DraftRevision,
		})
	}

	replacement, err := s.proposalReplacementForDraft(ctx, gameID, env, p)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := applyPageSpecToModel(p, replacement.PageSpec); err != nil {
		return nil, err
	}
	p.GameID = gameID
	p.Env = env
	p.Status = "draft"
	p.UpdatedBy = actor
	p.UpdatedAt = now
	p.DraftRevision++
	p.BaseProposalKey = replacement.ProposalKey
	p.BaseProposalVersion = replacement.ProposalVersion

	specJSON, err := buildPageSpecJSON(p)
	if err != nil {
		return nil, err
	}
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		if err := pageModel.Upsert(txCtx, p); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   p.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "regenerate default page from latest function contracts",
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	s.auditPageEvent(ctx, audit.EventPageDraftSave, gameID, env, req.PageKey, map[string]interface{}{
		"action":            "regenerate_default",
		"source":            "page_proposal",
		"draft_revision":    p.DraftRevision,
		"previous_revision": *req.DraftRevision,
		"quality":           string(replacement.Quality),
		"diagnostic_count":  len(replacement.Diagnostics),
		"diagnostic_errors": countErrors(replacement.Diagnostics),
		"binding_count":     len(replacement.Bindings),
		"resource_key":      replacement.ResourceKey,
		"proposal_key":      replacement.ProposalKey,
		"proposal_version":  replacement.ProposalVersion,
		"proposal_page_key": replacement.PageKey,
	})

	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return nil, err
	}
	return &PageRegenerateResponse{
		PageKey:       p.PageKey,
		DraftRevision: p.DraftRevision,
		Page:          pageSpec,
		Diagnostics:   replacement.Diagnostics,
		Quality:       replacement.Quality,
	}, nil
}

func (s *Service) Validate(ctx context.Context, req *PageValidateRequest) (*PageValidateResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return nil, err
	}
	diags := s.validatePageSpec(ctx, pageSpec, true)
	return &PageValidateResponse{Valid: countErrors(diags) == 0, Diagnostics: diags}, nil
}

func (s *Service) Preview(ctx context.Context, req *PagePreviewRequest) (*PagePreviewResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return nil, err
	}
	diags := s.validatePageSpec(ctx, pageSpec, false)
	if countErrors(diags) > 0 {
		return nil, errorx.NewValidationErrorWithDetails("page preview validation failed", diagnosticsToDetails(diags))
	}
	return &PagePreviewResponse{Page: pageSpec}, nil
}

func (s *Service) Publish(ctx context.Context, req *PagePublishRequest) (*PagePublishResponse, error) {
	if err := s.requirePagePublish(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}

	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	if p.DraftRevision != *req.DraftRevision {
		return nil, errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
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
	contracts, err := buildBindingContracts(pageSpec.Bindings, functions)
	if err != nil {
		return nil, err
	}
	specJSON, err := json.Marshal(pageSpec)
	if err != nil {
		return nil, err
	}
	contractsJSON, err := json.Marshal(contracts)
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
			return errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
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

func (s *Service) Versions(ctx context.Context, req *PageVersionsRequest) (*PageVersionsResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	p, _ := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	versions, err := s.svcCtx.PageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, err
	}
	items := make([]spec.PageVersionItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, spec.PageVersionItem{
			Version:            v.Version,
			Status:             v.Status,
			Message:            v.Message,
			IsCurrentDraft:     p != nil && v.Version == p.DraftRevision,
			IsCurrentPublished: p != nil && v.Version == p.PublishedVersion && p.PublishedVersion > 0,
			CreatedAt:          v.CreatedAt.Format(time.RFC3339),
			CreatedBy:          v.CreatedBy,
		})
	}
	resp := &PageVersionsResponse{Items: items}
	if p != nil {
		resp.CurrentDraftRevision = p.DraftRevision
		resp.CurrentPublishedVersion = p.PublishedVersion
	}
	return resp, nil
}

func (s *Service) VersionDetail(ctx context.Context, req *PageVersionDetailRequest) (*PageVersionDetailResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := s.svcCtx.PageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		if strconv.Itoa(v.Version) != req.VersionID {
			continue
		}
		var pageSpec spec.PageSpec
		if err := json.Unmarshal([]byte(v.SpecJSON), &pageSpec); err != nil {
			return nil, fmt.Errorf("decode page version: %w", err)
		}
		return &PageVersionDetailResponse{
			Version:   v.Version,
			Status:    v.Status,
			Message:   v.Message,
			CreatedAt: v.CreatedAt.Format(time.RFC3339),
			CreatedBy: v.CreatedBy,
			Page:      pageSpec,
		}, nil
	}
	return nil, errorx.NewNotFound("page version not found")
}

func (s *Service) Rollback(ctx context.Context, req *PageRollbackRequest) (*PageRollbackResponse, error) {
	if err := s.requirePageRollback(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := s.svcCtx.PageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, err
	}
	var target *model.PageVersion
	for _, v := range versions {
		if strconv.Itoa(v.Version) == req.VersionID {
			cp := v
			target = &cp
			break
		}
	}
	if target == nil {
		return nil, errorx.NewNotFound("page version not found")
	}
	if req.ExpectedDraftRevision == nil {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}
	var rolledBack spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &rolledBack); err != nil {
		return nil, fmt.Errorf("decode page version: %w", err)
	}
	nextRevision := 0
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		p, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, req.PageKey)
		if err != nil {
			return ErrPageNotFound(req.PageKey)
		}
		if p.DraftRevision != *req.ExpectedDraftRevision {
			return errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
				"expected": *req.ExpectedDraftRevision,
				"current":  p.DraftRevision,
			})
		}
		if err := applyPageSpecToModel(p, rolledBack); err != nil {
			return err
		}
		p.GameID = gameID
		p.Env = env
		p.DraftRevision++
		p.Status = "draft"
		p.UpdatedAt = time.Now()
		p.BaseProposalKey = ""
		p.BaseProposalVersion = 0
		specJSON, err := buildPageSpecJSON(p)
		if err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, p); err != nil {
			return err
		}
		if err := versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   req.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "rollback to version " + strconv.Itoa(target.Version),
			CreatedBy: actor,
			CreatedAt: p.UpdatedAt,
		}); err != nil {
			return err
		}
		nextRevision = p.DraftRevision
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPageRollback, gameID, env, req.PageKey, map[string]interface{}{
		"from_version":       target.Version,
		"new_draft_revision": nextRevision,
	})
	return &PageRollbackResponse{PageKey: req.PageKey, DraftRevision: nextRevision}, nil
}

func (s *Service) findDraft(ctx context.Context, pageKey string) (*model.PageSpec, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, pageKey)
	if err != nil {
		return nil, ErrPageNotFound(pageKey)
	}
	return p, nil
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
	if proposal == nil {
		return proposalReplacement{}, errorx.NewBadRequest("latest PageProposal is required for default regeneration")
	}
	if proposal.Status != "pending" && proposal.Status != "accepted" {
		return proposalReplacement{}, errorx.NewBadRequestWithDetails("PageProposal is not usable for regeneration", map[string]any{
			"proposalKey": proposal.ProposalKey,
			"status":      proposal.Status,
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

func requireScope(ctx context.Context) (string, string, error) {
	gameID, env := svc.GameScopeFromContext(ctx)
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}

func isValidPageType(t spec.PageType) bool {
	switch t {
	case spec.PageTypeResource, spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport:
		return true
	default:
		return false
	}
}

func (s *Service) validatePageSpec(ctx context.Context, page spec.PageSpec, publish bool) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if !isValidPageType(page.Type) {
		diags = append(diags, diagnostic("page_type_invalid", spec.SeverityError, "page type is invalid", "type"))
	}
	if !hasDefaultLocale(page.Title) {
		diags = append(diags, diagnostic("localized_text_missing", spec.SeverityError, "title must include zh-CN locale", "title"))
	}
	if strings.TrimSpace(page.Category.Key) == "" {
		diags = append(diags, diagnostic("category_key_missing", spec.SeverityError, "category.key is required", "category.key"))
	}
	if !hasDefaultLocale(page.Category.Labels) {
		diags = append(diags, diagnostic("category_label_missing", spec.SeverityError, "category.labels must include zh-CN locale", "category.labels"))
	}
	if publish {
		diags = append(diags, s.validatePublishedCategoryLabels(ctx, page)...)
	}
	if len(page.Bindings) == 0 {
		diags = append(diags, diagnostic("bindings_missing", spec.SeverityError, "page must bind at least one function", "bindings"))
	}
	diags = append(diags, validatePageShape(page)...)
	if publish {
		diags = append(diags, spec.ValidatePublishablePageShape(page)...)
	}

	bindingsByID := map[string]spec.PageFunctionBinding{}
	functions := s.normalizedFunctions(ctx)
	for i, binding := range page.Bindings {
		field := fmt.Sprintf("bindings[%d]", i)
		diags = append(diags, validateBinding(field, binding, functions, page, publish)...)
		if strings.TrimSpace(binding.ID) != "" {
			if _, exists := bindingsByID[binding.ID]; exists {
				diags = append(diags, diagnostic("binding_id_duplicate", spec.SeverityError, "binding id must be unique", field+".id"))
			}
			bindingsByID[binding.ID] = binding
		}
	}

	return diags
}

func (s *Service) validatePublishedCategoryLabels(ctx context.Context, page spec.PageSpec) []spec.Diagnostic {
	if s == nil || s.svcCtx == nil || s.svcCtx.PublishedPageSpecModel == nil {
		return nil
	}
	categoryKey := strings.TrimSpace(page.Category.Key)
	if categoryKey == "" {
		return nil
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil
	}
	published, err := s.svcCtx.PublishedPageSpecModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return []spec.Diagnostic{diagnostic("category_label_check_failed", spec.SeverityError, "failed to validate category labels", "category.labels")}
	}
	expected := normalizeLocaleKeys(page.Category.Labels)
	for _, item := range published {
		if item.PageKey == page.PageKey {
			continue
		}
		publishedPage, err := pageSpecFromPublishedModel(item)
		if err != nil {
			return []spec.Diagnostic{diagnostic("published_page_spec_invalid", spec.SeverityError, "published page contains invalid canonical PageSpec", "category.labels")}
		}
		if strings.TrimSpace(publishedPage.Category.Key) != categoryKey {
			continue
		}
		actual := normalizeLocaleKeys(publishedPage.Category.Labels)
		if !localizedTextEqual(actual, expected) {
			return []spec.Diagnostic{diagnostic(
				"category_label_conflict",
				spec.SeverityError,
				"category.labels must match existing published pages in the same category",
				"category.labels",
			)}
		}
	}
	return nil
}

func validatePageShape(page spec.PageSpec) []spec.Diagnostic {
	var diags []spec.Diagnostic
	requireOnly := func(field string, present bool) {
		if !present {
			diags = append(diags, diagnostic("page_shape_missing", spec.SeverityError, "page type requires "+field+" spec", field))
		}
	}
	switch page.Type {
	case spec.PageTypeResource:
		requireOnly("resource", page.Resource != nil)
	case spec.PageTypeOperation:
		requireOnly("operation", page.Operation != nil)
	case spec.PageTypeTask:
		requireOnly("task", page.Task != nil)
	case spec.PageTypeReport:
		requireOnly("report", page.Report != nil)
	}
	return diags
}

func bindingsByID(bindings []spec.PageFunctionBinding) map[string]spec.PageFunctionBinding {
	result := make(map[string]spec.PageFunctionBinding, len(bindings))
	for _, binding := range bindings {
		id := strings.TrimSpace(binding.ID)
		if id != "" {
			result[id] = binding
		}
	}
	return result
}

func validateBinding(field string, binding spec.PageFunctionBinding, functions map[string]spec.FunctionSpec, page spec.PageSpec, publish bool) []spec.Diagnostic {
	var diags []spec.Diagnostic
	bindingID := strings.TrimSpace(binding.ID)
	functionID := strings.TrimSpace(binding.FunctionID)
	if bindingID == "" {
		diags = append(diags, diagnostic("binding_id_missing", spec.SeverityError, "binding.id is required", field+".id"))
	}
	if functionID == "" {
		diags = append(diags, diagnostic("binding_function_missing", spec.SeverityError, "binding.functionId is required", field+".functionId"))
		return diags
	}
	fn, ok := functions[functionID]
	if !ok {
		d := diagnostic("binding_function_not_found", spec.SeverityError, "bound function does not exist: "+functionID, field+".functionId")
		d.FunctionID = functionID
		diags = append(diags, d)
		return diags
	}
	if !fn.Enabled {
		d := diagnostic("binding_function_disabled", spec.SeverityError, "bound function is disabled: "+functionID, field+".functionId")
		d.FunctionID = functionID
		diags = append(diags, d)
	}
	if !isValidUsage(binding.Usage) {
		diags = append(diags, diagnostic("binding_usage_invalid", spec.SeverityError, "binding.usage is invalid", field+".usage"))
	}
	if !isValidExecutionMode(binding.Execution.Mode) {
		diags = append(diags, diagnostic("binding_execution_mode_invalid", spec.SeverityError, "binding.execution.mode is invalid", field+".execution.mode"))
	}
	if binding.Usage == spec.BindingUsageTask && binding.Execution.Mode != spec.PageExecutionModeTask {
		diags = append(diags, diagnostic("binding_task_mode_mismatch", spec.SeverityError, "task binding must use execution.mode=task", field+".execution.mode"))
	}
	if publish {
		diags = append(diags, validatePublishBindingSelectors(field, binding, fn, page)...)
	}
	return diags
}

func validatePublishBindingSelectors(field string, binding spec.PageFunctionBinding, fn spec.FunctionSpec, page spec.PageSpec) []spec.Diagnostic {
	var diags []spec.Diagnostic
	requiresInputSelectors := len(fn.InputSchema) > 0 && schemaHasFields(fn.InputSchema)
	requiresOutputSelectors := bindingRequiresOutputSelectors(binding, page)
	if binding.Selectors == nil && requiresInputSelectors {
		diags = append(diags, diagnostic("binding_selector_missing", spec.SeverityError, "binding.selectors.input is required before publish", field+".selectors.input"))
		return diags
	}
	if binding.Selectors == nil && requiresOutputSelectors {
		diags = append(diags, diagnostic("binding_output_selector_missing", spec.SeverityError, "binding.selectors.output is required before publish", field+".selectors.output"))
		return diags
	}
	if binding.Selectors != nil && requiresInputSelectors {
		result := spec.ValidateSelector(binding.Selectors.Input, fn.InputSchema, spec.SelectorContextForBinding(page, binding))
		for _, item := range result.Errors {
			fieldPath := field + ".selectors.input"
			if strings.TrimSpace(item.Field) != "" {
				fieldPath += "." + item.Field
			}
			diags = append(diags, diagnostic("binding_selector_"+item.Code, spec.SeverityError, item.Message, fieldPath))
		}
		for _, item := range result.Warnings {
			fieldPath := field + ".selectors.input"
			if strings.TrimSpace(item.Field) != "" {
				fieldPath += "." + item.Field
			}
			diags = append(diags, diagnostic("binding_selector_"+item.Code, spec.SeverityWarning, item.Message, fieldPath))
		}
	}
	if binding.Selectors != nil && requiresOutputSelectors && len(binding.Selectors.Output) == 0 {
		diags = append(diags, diagnostic("binding_output_selector_missing", spec.SeverityError, "binding.selectors.output is required before publish", field+".selectors.output"))
	}
	if binding.Selectors != nil && len(binding.Selectors.Output) > 0 {
		outputResult := spec.ValidateOutputAssignments(binding.Selectors.Output, fn.OutputSchema)
		for _, item := range outputResult.Errors {
			fieldPath := field + ".selectors.output"
			if strings.TrimSpace(item.Field) != "" {
				fieldPath += "." + item.Field
			}
			diags = append(diags, diagnostic("binding_selector_"+item.Code, spec.SeverityError, item.Message, fieldPath))
		}
		for _, item := range outputResult.Warnings {
			fieldPath := field + ".selectors.output"
			if strings.TrimSpace(item.Field) != "" {
				fieldPath += "." + item.Field
			}
			diags = append(diags, diagnostic("binding_selector_"+item.Code, spec.SeverityWarning, item.Message, fieldPath))
		}
	}
	for _, item := range spec.ValidateRequiredOutputAssignments(binding, page) {
		item.Field = field + ".selectors.output"
		diags = append(diags, item)
	}
	return diags
}

func bindingRequiresOutputSelectors(binding spec.PageFunctionBinding, page spec.PageSpec) bool {
	switch binding.Usage {
	case spec.BindingUsageQuery:
		return page.Type == spec.PageTypeResource
	case spec.BindingUsageReport:
		return page.Type == spec.PageTypeReport
	case spec.BindingUsageTaskStatus, spec.BindingUsageTaskEvents, spec.BindingUsageTaskResult:
		return page.Type == spec.PageTypeTask
	default:
		return false
	}
}

func schemaHasFields(raw spec.JSONSchema) bool {
	if len(raw) == 0 {
		return false
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return true
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(parsed["properties"], &properties); err == nil && len(properties) > 0 {
		return true
	}
	var required []string
	if err := json.Unmarshal(parsed["required"], &required); err == nil && len(required) > 0 {
		return true
	}
	return false
}

func isValidUsage(usage spec.PageBindingUsage) bool {
	switch usage {
	case spec.BindingUsageQuery,
		spec.BindingUsageDetail,
		spec.BindingUsageAction,
		spec.BindingUsageTask,
		spec.BindingUsageTaskStatus,
		spec.BindingUsageTaskEvents,
		spec.BindingUsageTaskResult,
		spec.BindingUsageTaskCancel,
		spec.BindingUsageTaskRetry,
		spec.BindingUsageReport:
		return true
	default:
		return false
	}
}

func isValidExecutionMode(mode spec.PageExecutionMode) bool {
	switch mode {
	case spec.PageExecutionModeSync, spec.PageExecutionModeTask:
		return true
	default:
		return false
	}
}

func buildBindingContracts(bindings []spec.PageFunctionBinding, functions map[string]spec.FunctionSpec) ([]spec.BindingContractSnapshot, error) {
	out := make([]spec.BindingContractSnapshot, 0, len(bindings))
	for _, binding := range bindings {
		fn, ok := functions[strings.TrimSpace(binding.FunctionID)]
		if !ok {
			return nil, errorx.NewValidationError("bound function does not exist: " + binding.FunctionID)
		}
		out = append(out, spec.BindingContractSnapshot{
			BindingID:             strings.TrimSpace(binding.ID),
			FunctionID:            strings.TrimSpace(binding.FunctionID),
			FunctionVersion:       strings.TrimSpace(fn.Version),
			InputSchemaDigest:     digestRaw(fn.InputSchema),
			OutputSchemaDigest:    digestRaw(fn.OutputSchema),
			Risk:                  fn.Risk,
			Permission:            strings.TrimSpace(fn.Permission),
			Approval:              fn.Approval,
			ExecutionMode:         binding.Execution.Mode,
			RendererSchemaVersion: rendererSchemaVersion,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BindingID < out[j].BindingID })
	return out, nil
}

func digestRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func pageDraftResponseFromModel(p *model.PageSpec) (*PageDraftResponse, error) {
	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return nil, err
	}
	return &PageDraftResponse{
		PageSpec:         pageSpec,
		GameID:           p.GameID,
		Env:              p.Env,
		Status:           p.Status,
		DraftRevision:    p.DraftRevision,
		PublishedVersion: p.PublishedVersion,
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:        p.UpdatedBy,
	}, nil
}

func (s *Service) bindingFreshnessForPublishedDraft(ctx context.Context, p *model.PageSpec) []spec.BindingFreshnessDiagnostic {
	if s == nil || s.svcCtx == nil || s.svcCtx.PublishedPageSpecModel == nil || p == nil || p.PublishedVersion == 0 {
		return nil
	}
	published, err := s.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, p.GameID, p.Env, p.PageKey)
	if err != nil {
		return nil
	}
	pageSpec, contracts := parsePublishedPageForFreshness(*published)
	return freshness.EvaluatePublishedBindings(pageSpec.Bindings, contracts, s.normalizedFunctions(ctx))
}

func parsePublishedPageForFreshness(published model.PublishedPageSpec) (spec.PageSpec, []spec.BindingContractSnapshot) {
	var pageSpec spec.PageSpec
	_ = json.Unmarshal([]byte(published.SpecJSON), &pageSpec)
	var contracts []spec.BindingContractSnapshot
	if strings.TrimSpace(published.BindingContractsJSON) != "" {
		_ = json.Unmarshal([]byte(published.BindingContractsJSON), &contracts)
	}
	return pageSpec, contracts
}

func pageSpecFromModel(p *model.PageSpec) (spec.PageSpec, error) {
	if p == nil {
		return spec.PageSpec{}, errorx.NewBadRequest("page draft is required")
	}
	if strings.TrimSpace(p.SpecJSON) == "" {
		return spec.PageSpec{}, errorx.NewValidationError("page draft does not contain canonical PageSpec")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(p.SpecJSON), &pageSpec); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode canonical PageSpec: %w", err)
	}
	pageSpec.PageKey = strings.TrimSpace(pageSpec.PageKey)
	if pageSpec.PageKey == "" {
		return spec.PageSpec{}, errorx.NewValidationError("canonical PageSpec pageKey is required")
	}
	return pageSpec, nil
}

func pageSpecFromPublishedModel(p model.PublishedPageSpec) (spec.PageSpec, error) {
	if strings.TrimSpace(p.SpecJSON) == "" {
		return spec.PageSpec{}, errorx.NewValidationError("published page does not contain canonical PageSpec")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(p.SpecJSON), &pageSpec); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode published canonical PageSpec: %w", err)
	}
	pageSpec.PageKey = strings.TrimSpace(pageSpec.PageKey)
	if pageSpec.PageKey == "" {
		return spec.PageSpec{}, errorx.NewValidationError("published canonical PageSpec pageKey is required")
	}
	return pageSpec, nil
}

func pageSpecFromProposalModel(proposal *model.PageProposal) (spec.PageSpec, error) {
	if proposal == nil {
		return spec.PageSpec{}, errorx.NewBadRequest("proposal is required")
	}
	if len(proposal.PageSpec) == 0 {
		return spec.PageSpec{}, errorx.NewValidationError("proposal does not contain canonical PageSpec")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal(proposal.PageSpec, &pageSpec); err != nil {
		return spec.PageSpec{}, errorx.NewValidationError("proposal PageSpec is invalid JSON")
	}
	pageSpec.PageKey = strings.TrimSpace(pageSpec.PageKey)
	pageSpec.ResourceKey = strings.TrimSpace(pageSpec.ResourceKey)
	pageSpec.Icon = strings.TrimSpace(pageSpec.Icon)
	pageSpec.Title = normalizeLocaleKeys(pageSpec.Title)
	pageSpec.Description = normalizeLocaleKeys(pageSpec.Description)
	pageSpec.Category.Key = strings.TrimSpace(pageSpec.Category.Key)
	pageSpec.Category.Labels = normalizeLocaleKeys(pageSpec.Category.Labels)
	for i := range pageSpec.Bindings {
		pageSpec.Bindings[i].ID = strings.TrimSpace(pageSpec.Bindings[i].ID)
		pageSpec.Bindings[i].FunctionID = strings.TrimSpace(pageSpec.Bindings[i].FunctionID)
	}
	if strings.TrimSpace(pageSpec.PageKey) == "" {
		return spec.PageSpec{}, errorx.NewValidationError("proposal PageSpec pageKey is required")
	}
	return pageSpec, nil
}

func applyPageSpecToModel(p *model.PageSpec, ps spec.PageSpec) error {
	if p == nil {
		return fmt.Errorf("target page model is nil")
	}
	p.PageKey = strings.TrimSpace(ps.PageKey)
	p.Type = string(ps.Type)
	p.ResourceKey = strings.TrimSpace(ps.ResourceKey)
	p.CategoryKey = strings.TrimSpace(ps.Category.Key)
	if p.CategoryKey == "" {
		return errorx.NewValidationError("category.key is required in canonical PageSpec")
	}
	p.CategoryOrder = ps.Category.Order
	p.Order = ps.Order
	p.Icon = strings.TrimSpace(ps.Icon)
	if err := p.SetTitle(normalizeLocaleKeys(ps.Title)); err != nil {
		return err
	}
	if err := p.SetCategoryLabels(normalizeLocaleKeys(ps.Category.Labels)); err != nil {
		return err
	}
	raw, err := marshalPageSpec(ps)
	if err != nil {
		return err
	}
	p.SpecJSON = raw
	return nil
}

func buildPageSpecJSON(p *model.PageSpec) (string, error) {
	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return "", err
	}
	return marshalPageSpec(pageSpec)
}

func marshalPageSpec(page spec.PageSpec) (string, error) {
	page.PageKey = strings.TrimSpace(page.PageKey)
	page.ResourceKey = strings.TrimSpace(page.ResourceKey)
	page.Icon = strings.TrimSpace(page.Icon)
	page.Title = normalizeLocaleKeys(page.Title)
	page.Description = normalizeLocaleKeys(page.Description)
	page.Category.Key = strings.TrimSpace(page.Category.Key)
	page.Category.Labels = normalizeLocaleKeys(page.Category.Labels)
	for i := range page.Bindings {
		page.Bindings[i].ID = strings.TrimSpace(page.Bindings[i].ID)
		page.Bindings[i].FunctionID = strings.TrimSpace(page.Bindings[i].FunctionID)
	}
	b, err := json.Marshal(page)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) withPageTransaction(
	ctx context.Context,
	fn func(context.Context, *model.PageSpecModel, *model.PublishedPageSpecModel, *model.PageVersionModel) error,
) error {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return errorx.NewInternalError("page service database is not initialized")
	}
	db := dbctx.Resolve(ctx, s.svcCtx.DB)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := dbctx.WithDB(ctx, tx)
		return fn(txCtx, model.NewPageSpecModel(tx), model.NewPublishedPageSpecModel(tx), model.NewPageVersionModel(tx))
	})
}

func (s *Service) normalizedFunctions(ctx context.Context) map[string]spec.FunctionSpec {
	gameID, env, err := requireScope(ctx)
	if err != nil || s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return map[string]spec.FunctionSpec{}
	}
	functions, err := contractsvc.FunctionSpecsByScope(ctx, model.NewFunctionContractModel(s.svcCtx.DB), gameID, env)
	if err != nil {
		return map[string]spec.FunctionSpec{}
	}
	return functions
}

func (s *Service) requirePageRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看页面", "admin:all", "pages:read", "pages:edit", "pages:publish", "pages:rollback")
	return err
}

func (s *Service) requirePageEdit(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权编辑页面", "admin:all", "pages:edit")
	return err
}

func (s *Service) requirePagePublish(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权发布页面", "admin:all", "pages:publish")
	return err
}

func (s *Service) requirePageRollback(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权回滚页面", "admin:all", "pages:rollback")
	return err
}

func (s *Service) auditPageEvent(ctx context.Context, eventType audit.AuditEventType, gameID, env, pageKey string, details map[string]interface{}) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		actor = "unknown"
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	details["game_id"] = gameID
	details["env"] = env
	details["page_key"] = pageKey

	_, err = s.svcCtx.AuditService.Log(ctx, eventType,
		audit.WithActorID(actor, "user", actor),
		audit.WithResource(audit.ResourceInfo{
			Type:        "page",
			ID:          pageKey,
			Name:        pageKey,
			GameID:      gameID,
			Environment: env,
		}),
		audit.WithDetails(details),
		audit.WithOutcome("success", ""),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to write page audit event", "event", eventType, "pageKey", pageKey, "error", err)
	}
}

func normalizeLocaleKeys(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(k, "_", "-")) {
		case "zh", "zh-cn":
			out["zh-CN"] = v
		case "en", "en-us":
			out["en-US"] = v
		default:
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func diagnosticsToDetails(diags []spec.Diagnostic) map[string]string {
	details := make(map[string]string)
	for _, d := range diags {
		key := d.Field
		if key == "" {
			key = d.Code
		}
		details[key] = d.Message
	}
	return details
}

func diagnostic(code string, severity spec.DiagnosticSeverity, message string, field string) spec.Diagnostic {
	return spec.Diagnostic{Code: code, Severity: severity, Message: message, Field: field}
}

func diagnosticsFromJSON(raw []byte) []spec.Diagnostic {
	if len(raw) == 0 {
		return nil
	}
	var diagnostics []spec.Diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return []spec.Diagnostic{{
			Code:     "proposal_diagnostics_invalid",
			Severity: spec.SeverityWarning,
			Message:  "proposal diagnostics payload is not readable",
		}}
	}
	return diagnostics
}

func hasDefaultLocale(labels spec.LocalizedText) bool {
	return labels != nil && strings.TrimSpace(labels["zh-CN"]) != ""
}

func localizedTextEqual(left map[string]string, right map[string]string) bool {
	left = normalizeLocaleKeys(left)
	right = normalizeLocaleKeys(right)
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if strings.TrimSpace(right[key]) != strings.TrimSpace(leftValue) {
			return false
		}
	}
	return true
}

func countErrors(diags []spec.Diagnostic) int {
	count := 0
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			count++
		}
	}
	return count
}

func localizedTextFromJSONMap(values map[string]interface{}) spec.LocalizedText {
	if len(values) == 0 {
		return nil
	}
	out := make(spec.LocalizedText, len(values))
	for key, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out[key] = strings.TrimSpace(text)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func approvalPolicyFromJSONMap(values map[string]interface{}) spec.ApprovalPolicy {
	if len(values) == 0 {
		return spec.ApprovalPolicy{}
	}
	required, _ := values["required"].(bool)
	policyKey, _ := values["policyKey"].(string)
	if policyKey == "" {
		policyKey, _ = values["policy_key"].(string)
	}
	return spec.ApprovalPolicy{
		Required:  required,
		PolicyKey: strings.TrimSpace(policyKey),
	}
}

type pagePublishSource struct {
	BaseProposalKey     string
	BaseProposalVersion int
	FunctionDigest      string
	SemanticsDigest     string
	GeneratorVersion    string
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
