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
	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
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
	resp := pageDraftResponseFromModel(p)
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
		Metadata:    req.Metadata,
	}
	if !isValidPageType(req.Type) {
		return nil, errorx.NewBadRequest("type must be resource, operation, task, or report")
	}
	categoryKey := strings.TrimSpace(req.Category.Key)
	if categoryKey == "" {
		categoryKey = inferCategoryFromKey(firstNonEmpty(req.ResourceKey, req.PageKey))
	}
	if categoryKey == "" {
		return nil, errorx.NewBadRequest("category.key is required or inferable from resourceKey/pageKey")
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

	current := pageSpecFromModel(p)
	generated, err := s.generateReplacementForDraft(ctx, current)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := applyPageSpecToModel(p, generated.PageSpec); err != nil {
		return nil, err
	}
	p.GameID = gameID
	p.Env = env
	p.Status = "draft"
	p.UpdatedBy = actor
	p.UpdatedAt = now
	p.DraftRevision++

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
		"action":             "regenerate_default",
		"source":             "generated_page",
		"draft_revision":     p.DraftRevision,
		"previous_revision":  *req.DraftRevision,
		"quality":            string(generated.Quality),
		"diagnostic_count":   len(generated.Diagnostics),
		"diagnostic_errors":  countErrors(generated.Diagnostics),
		"binding_count":      len(generated.Bindings),
		"resource_key":       generated.ResourceKey,
		"generated_page_key": generated.PageKey,
	})

	return &PageRegenerateResponse{
		PageKey:       p.PageKey,
		DraftRevision: p.DraftRevision,
		Page:          pageSpecFromModel(p),
		Diagnostics:   generated.Diagnostics,
		Quality:       generated.Quality,
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
	diags := s.validatePageSpec(ctx, pageSpecFromModel(p), true)
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
	pageSpec := pageSpecFromModel(p)
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

	pageSpec := pageSpecFromModel(p)
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

	now := time.Now()
	publishedVersion := p.DraftRevision
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
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
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		p.Status = "published"
		p.PublishedActive = true
		p.PublishedVersion = publishedVersion
		p.UpdatedAt = now
		if err := pageModel.Upsert(txCtx, p); err != nil {
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
	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	var rolledBack spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &rolledBack); err != nil {
		return nil, fmt.Errorf("decode page version: %w", err)
	}
	if err := applyPageSpecToModel(p, rolledBack); err != nil {
		return nil, err
	}
	p.GameID = gameID
	p.Env = env
	p.DraftRevision++
	p.Status = "draft"
	p.UpdatedAt = time.Now()
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
			PageKey:   req.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "rollback to version " + strconv.Itoa(target.Version),
			CreatedBy: actor,
			CreatedAt: p.UpdatedAt,
		})
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPageRollback, gameID, env, req.PageKey, map[string]interface{}{
		"from_version":       target.Version,
		"new_draft_revision": p.DraftRevision,
	})
	return &PageRollbackResponse{PageKey: req.PageKey, DraftRevision: p.DraftRevision}, nil
}

func (s *Service) generateReplacementForDraft(ctx context.Context, current spec.PageSpec) (spec.GeneratedPageSpec, error) {
	resourceKey := strings.TrimSpace(current.ResourceKey)
	if resourceKey == "" {
		return spec.GeneratedPageSpec{}, errorx.NewBadRequest("page resourceKey is required for default regeneration")
	}
	results, resources := normalizedDashboardSpecs(ctx, s.svcCtx)
	resource, ok := resources[resourceKey]
	if !ok || resource == nil {
		return spec.GeneratedPageSpec{}, errorx.NewBadRequestWithDetails("resource is not available for default regeneration", map[string]any{
			"resourceKey": resourceKey,
			"pageKey":     current.PageKey,
		})
	}
	pages := generator.GenerateForResource(*resource, generator.GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions:     functionsByID(results),
	})
	for _, candidate := range pages {
		if candidate.PageKey == current.PageKey {
			return candidate, nil
		}
	}
	return spec.GeneratedPageSpec{}, errorx.NewBadRequestWithDetails("no generated page candidate matches current pageKey", map[string]any{
		"pageKey":     current.PageKey,
		"resourceKey": resourceKey,
		"candidates":  generatedPageKeys(pages),
	})
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
	if len(page.Bindings) == 0 {
		diags = append(diags, diagnostic("bindings_missing", spec.SeverityError, "page must bind at least one function", "bindings"))
	}
	diags = append(diags, validatePageShape(page)...)

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
	if len(fn.InputSchema) == 0 || !schemaHasFields(fn.InputSchema) {
		return diags
	}
	if binding.Selectors == nil {
		diags = append(diags, diagnostic("binding_selector_missing", spec.SeverityError, "binding.selectors.input is required before publish", field+".selectors.input"))
		return diags
	}
	result := spec.ValidateSelector(binding.Selectors.Input, fn.InputSchema, selectorContextForPage(page))
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
	return diags
}

func selectorContextForPage(page spec.PageSpec) spec.SelectorContext {
	return spec.SelectorContext{
		PageType:      page.Type,
		HasListView:   page.Resource != nil && page.Resource.ListView != nil,
		HasDetailView: page.Resource != nil && page.Resource.DetailView != nil,
		FormSchema:    primaryFormSchema(page),
	}
}

func primaryFormSchema(page spec.PageSpec) spec.JSONSchema {
	switch {
	case page.Operation != nil && page.Operation.Form != nil:
		return page.Operation.Form.JSONSchema
	case page.Task != nil && page.Task.Form != nil:
		return page.Task.Form.JSONSchema
	case page.Report != nil && page.Report.QueryForm != nil:
		return page.Report.QueryForm.JSONSchema
	case page.Resource != nil && page.Resource.CreateForm != nil:
		return page.Resource.CreateForm.JSONSchema
	default:
		return nil
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
	case spec.BindingUsageQuery, spec.BindingUsageDetail, spec.BindingUsageAction, spec.BindingUsageTask, spec.BindingUsageReport:
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

func pageDraftResponseFromModel(p *model.PageSpec) *PageDraftResponse {
	pageSpec := pageSpecFromModel(p)
	return &PageDraftResponse{
		PageSpec:         pageSpec,
		GameID:           p.GameID,
		Env:              p.Env,
		Status:           p.Status,
		DraftRevision:    p.DraftRevision,
		PublishedVersion: p.PublishedVersion,
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:        p.UpdatedBy,
	}
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

func pageSpecFromModel(p *model.PageSpec) spec.PageSpec {
	if p == nil {
		return spec.PageSpec{}
	}
	var pageSpec spec.PageSpec
	if strings.TrimSpace(p.SpecJSON) != "" {
		if err := json.Unmarshal([]byte(p.SpecJSON), &pageSpec); err == nil && strings.TrimSpace(pageSpec.PageKey) != "" {
			return pageSpec
		}
	}
	return spec.PageSpec{
		PageKey:     p.PageKey,
		Type:        spec.PageType(p.Type),
		ResourceKey: p.ResourceKey,
		Title:       p.GetTitle(),
		Category: spec.PageCategorySpec{
			Key:    p.CategoryKey,
			Labels: p.GetCategoryLabels(),
			Order:  p.CategoryOrder,
		},
		Order: p.Order,
		Icon:  p.Icon,
	}
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
		p.CategoryKey = inferCategoryFromKey(firstNonEmpty(p.ResourceKey, p.PageKey))
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
	return marshalPageSpec(pageSpecFromModel(p))
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
	results, _ := normalizedDashboardSpecs(ctx, s.svcCtx)
	return functionsByID(results)
}

func normalizedDashboardSpecs(ctx context.Context, svcCtx *svc.ServiceContext) ([]normalizer.NormalizerResult, map[string]*spec.ResourceSpec) {
	inputs := descriptors.Collect(ctx, svcCtx)
	return normalizer.NormalizeBatch(inputs)
}

func functionsByID(results []normalizer.NormalizerResult) map[string]spec.FunctionSpec {
	out := make(map[string]spec.FunctionSpec, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Function.ID) != "" {
			out[result.Function.ID] = result.Function
		}
	}
	return out
}

func generatedPageKeys(pages []spec.GeneratedPageSpec) []string {
	keys := make([]string, 0, len(pages))
	for _, page := range pages {
		if key := strings.TrimSpace(page.PageKey); key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
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

func hasDefaultLocale(labels spec.LocalizedText) bool {
	return labels != nil && strings.TrimSpace(labels["zh-CN"]) != ""
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func inferCategoryFromKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
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
