package page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// Service provides Page workspace API operations.
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a new Page Service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// ListDrafts returns all page drafts.
func (s *Service) ListDrafts(ctx context.Context, req *PageDraftListRequest) (*PageDraftListResponse, error) {
	var pages []model.PageSpec
	var err error

	if req.Status != "" {
		pages, err = s.svcCtx.PageSpecModel.ListByStatus(ctx, req.Status)
	} else {
		pages, err = s.svcCtx.PageSpecModel.ListAll(ctx)
	}
	if err != nil {
		return nil, err
	}

	items := make([]spec.PageSpecDraftSummary, 0, len(pages))
	for _, p := range pages {
		// Apply resourceKey filter
		if req.ResourceKey != "" && p.ResourceKey != req.ResourceKey {
			continue
		}

		items = append(items, spec.PageSpecDraftSummary{
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
			DraftVersion:     p.DraftVersion,
			PublishedVersion: p.PublishedVersion,
			UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
			UpdatedBy:        p.UpdatedBy,
		})
	}

	return &PageDraftListResponse{Items: items}, nil
}

// GetDraft returns a single page draft.
func (s *Service) GetDraft(ctx context.Context, req *PageDraftRequest) (*PageDraftResponse, error) {
	p, err := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	pageSpec := pageSpecFromModel(p)
	return &PageDraftResponse{
		PageSpec:         pageSpec,
		Status:           p.Status,
		DraftVersion:     p.DraftVersion,
		PublishedVersion: p.PublishedVersion,
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:        p.UpdatedBy,
	}, nil
}

// SaveDraft saves a page draft.
func (s *Service) SaveDraft(ctx context.Context, req *PageSaveRequest) (*PageSaveResponse, error) {
	// Validate page key matches path
	if req.PageKey == "" {
		return nil, fmt.Errorf("pageKey is required")
	}

	// Validate Formily schema
	if !isValidFormilySchema(req.Schema) {
		return nil, fmt.Errorf("schema must be a valid Formily JSON Schema")
	}

	// Validate required fields
	if req.Title == nil || req.Title["zh-CN"] == "" {
		return nil, fmt.Errorf("title must include zh-CN locale")
	}
	categoryKey := strings.TrimSpace(req.Category.Key)
	if categoryKey == "" {
		categoryKey = inferCategoryFromKey(firstNonEmpty(req.ResourceKey, req.PageKey))
	}
	if categoryKey == "" {
		return nil, fmt.Errorf("category.key is required or inferable from resourceKey/pageKey")
	}
	categoryLabels := normalizeLocaleKeys(req.Category.Labels)
	if categoryLabels == nil || categoryLabels["zh-CN"] == "" {
		return nil, fmt.Errorf("category.labels must include zh-CN locale")
	}

	// Normalize locale keys
	title := normalizeLocaleKeys(req.Title)
	description := normalizeLocaleKeys(req.Description)

	now := time.Now()
	draftVersion := 0

	if err := s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		existing, err := pageModel.FindByPageKey(ctx, req.PageKey)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		ps := &model.PageSpec{
			PageKey:       req.PageKey,
			Type:          req.Type,
			ResourceKey:   req.ResourceKey,
			CategoryKey:   categoryKey,
			CategoryOrder: req.Category.Order,
			Order:         req.Order,
			Icon:          req.Icon,
			SchemaJSON:    string(req.Schema),
			Status:        "draft",
			UpdatedBy:     "",
		}

		if err := ps.SetTitle(title); err != nil {
			return fmt.Errorf("failed to set title: %w", err)
		}
		if description != nil {
			b, err := json.Marshal(description)
			if err != nil {
				return fmt.Errorf("failed to set description: %w", err)
			}
			ps.DescriptionJSON = string(b)
		}
		if err := ps.SetCategoryLabels(categoryLabels); err != nil {
			return fmt.Errorf("failed to set category labels: %w", err)
		}
		if err := ps.SetBindings(convertBindingsToModel(req.Bindings)); err != nil {
			return fmt.Errorf("failed to set bindings: %w", err)
		}
		if req.Metadata != nil {
			b, err := json.Marshal(req.Metadata)
			if err != nil {
				return fmt.Errorf("failed to set metadata: %w", err)
			}
			ps.MetadataJSON = string(b)
		}

		if existing != nil {
			ps.ID = existing.ID
			ps.CreatedAt = existing.CreatedAt
			ps.DraftVersion = existing.DraftVersion + 1
			ps.PublishedVersion = existing.PublishedVersion
			ps.PublishedActive = existing.PublishedActive
		} else {
			ps.CreatedAt = now
			ps.DraftVersion = 1
		}
		ps.UpdatedAt = now
		draftVersion = ps.DraftVersion

		if err := pageModel.Upsert(ctx, ps); err != nil {
			return fmt.Errorf("failed to save page: %w", err)
		}

		specJSON, err := buildPageSpecJSON(ps)
		if err != nil {
			return fmt.Errorf("failed to encode page version: %w", err)
		}
		if err := versionModel.Create(ctx, &model.PageVersion{
			PageKey:   req.PageKey,
			Version:   ps.DraftVersion,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "save draft",
			CreatedBy: ps.UpdatedBy,
			CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("failed to save page version: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &PageSaveResponse{
		PageKey:      req.PageKey,
		DraftVersion: draftVersion,
	}, nil
}

// Validate validates a page draft.
func (s *Service) Validate(ctx context.Context, req *PageValidateRequest) (*PageValidateResponse, error) {
	p, err := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	var diags []spec.Diagnostic
	pageSpec := pageSpecFromModel(p)

	// Validate Formily schema
	if !isValidFormilySchema(p.GetSchema()) {
		diags = append(diags, spec.Diagnostic{
			Code:     "formily_schema_invalid",
			Severity: spec.SeverityError,
			Message:  "schema is not a valid Formily JSON Schema",
		})
	}

	// Validate title
	title := p.GetTitle()
	if title == nil || title["zh-CN"] == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "localized_text_missing",
			Severity: spec.SeverityError,
			Message:  "title must include zh-CN locale",
			Field:    "title",
		})
	}

	// Validate category
	if p.CategoryKey == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "category_key_missing",
			Severity: spec.SeverityError,
			Message:  "category.key is required",
			Field:    "category.key",
		})
	}

	catLabels := p.GetCategoryLabels()
	if catLabels == nil || catLabels["zh-CN"] == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "category_label_missing",
			Severity: spec.SeverityError,
			Message:  "category.labels must include zh-CN locale",
			Field:    "category.labels",
		})
	}

	if len(pageSpec.Bindings) == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:     "bindings_missing",
			Severity: spec.SeverityError,
			Message:  "page must bind at least one function before publishing",
			Field:    "bindings",
		})
	}

	functions := s.normalizedFunctions(ctx)
	for i, binding := range pageSpec.Bindings {
		field := fmt.Sprintf("bindings[%d]", i)
		if strings.TrimSpace(binding.FunctionID) == "" {
			diags = append(diags, spec.Diagnostic{
				Code:     "binding_function_missing",
				Severity: spec.SeverityError,
				Message:  "binding.functionId is required",
				Field:    field + ".functionId",
			})
			continue
		}
		fn, ok := functions[strings.TrimSpace(binding.FunctionID)]
		if !ok {
			diags = append(diags, spec.Diagnostic{
				Code:       "binding_function_not_found",
				Severity:   spec.SeverityError,
				Message:    "bound function does not exist: " + binding.FunctionID,
				FunctionID: binding.FunctionID,
				Field:      field + ".functionId",
			})
			continue
		}
		diags = append(diags, validateBoundFunction(field, binding, fn)...)
		if strings.TrimSpace(string(binding.Role)) == "" {
			diags = append(diags, spec.Diagnostic{
				Code:     "binding_role_missing",
				Severity: spec.SeverityError,
				Message:  "binding.role is required",
				Field:    field + ".role",
			})
		}
	}

	// Check for error-level diagnostics
	valid := true
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			valid = false
			break
		}
	}

	return &PageValidateResponse{
		Valid:       valid,
		Diagnostics: diags,
	}, nil
}

// Preview returns a preview of the page draft.
func (s *Service) Preview(ctx context.Context, req *PagePreviewRequest) (*PagePreviewResponse, error) {
	p, err := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	return &PagePreviewResponse{
		Page: pageSpecFromModel(p),
	}, nil
}

// Publish publishes a page draft.
func (s *Service) Publish(ctx context.Context, req *PagePublishRequest) (*PagePublishResponse, error) {
	p, err := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	// Validate before publishing
	validateReq := &PageValidateRequest{PageKey: req.PageKey}
	validateResp, err := s.Validate(ctx, validateReq)
	if err != nil {
		return nil, err
	}
	if !validateResp.Valid {
		return nil, fmt.Errorf("page validation failed: %d errors", countErrors(validateResp.Diagnostics))
	}

	// Published snapshot version follows the full PageSpec draft version.
	publishedVersion := p.DraftVersion
	specJSON, err := buildPageSpecJSON(p)
	if err != nil {
		return nil, fmt.Errorf("failed to encode published page: %w", err)
	}
	now := time.Now()

	if err := s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		if err := publishedModel.DeactivatePage(ctx, req.PageKey, now); err != nil {
			return fmt.Errorf("failed to deactivate previous published page: %w", err)
		}
		if err := publishedModel.Create(ctx, &model.PublishedPageSpec{
			PageKey:     req.PageKey,
			Version:     publishedVersion,
			SpecJSON:    specJSON,
			Active:      true,
			PublishedAt: now,
			PublishedBy: req.PublishedBy,
		}); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}

		p.Status = "published"
		p.PublishedActive = true
		p.PublishedVersion = publishedVersion
		p.UpdatedAt = now
		if err := pageModel.Upsert(ctx, p); err != nil {
			return fmt.Errorf("failed to update draft status: %w", err)
		}
		if err := versionModel.Create(ctx, &model.PageVersion{
			PageKey:   req.PageKey,
			Version:   publishedVersion,
			SpecJSON:  specJSON,
			Status:    "published",
			Message:   "publish",
			CreatedBy: req.PublishedBy,
			CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("failed to save published version: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &PagePublishResponse{
		PageKey:          req.PageKey,
		Published:        true,
		PublishedVersion: publishedVersion,
	}, nil
}

// Unpublish unpublishes a page.
func (s *Service) Unpublish(ctx context.Context, req *PageUnpublishRequest) (*PageUnpublishResponse, error) {
	p, err := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	now := time.Now()
	if err := s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, _ *model.PageVersionModel) error {
		p.Status = "draft"
		p.PublishedActive = false
		p.UpdatedAt = now
		if err := pageModel.Upsert(ctx, p); err != nil {
			return fmt.Errorf("failed to unpublish: %w", err)
		}
		if err := publishedModel.DeactivatePage(ctx, req.PageKey, now); err != nil {
			return fmt.Errorf("failed to deactivate published page: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &PageUnpublishResponse{
		PageKey:   req.PageKey,
		Published: false,
	}, nil
}

// Versions returns version history for a page.
func (s *Service) Versions(ctx context.Context, req *PageVersionsRequest) (*PageVersionsResponse, error) {
	p, _ := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	versions, err := s.svcCtx.PageVersionModel.ListByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}

	items := make([]spec.PageVersionItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, spec.PageVersionItem{
			Version:            v.Version,
			Status:             v.Status,
			Message:            v.Message,
			IsCurrentDraft:     p != nil && v.Version == p.DraftVersion,
			IsCurrentPublished: p != nil && v.Version == p.PublishedVersion && p.PublishedVersion > 0,
			CreatedAt:          v.CreatedAt.Format(time.RFC3339),
			CreatedBy:          v.CreatedBy,
		})
	}

	draftVersion := 0
	publishedVersion := 0
	if p != nil {
		draftVersion = p.DraftVersion
		publishedVersion = p.PublishedVersion
	}

	return &PageVersionsResponse{
		CurrentDraftVersion:     draftVersion,
		CurrentPublishedVersion: publishedVersion,
		Items:                   items,
	}, nil
}

// VersionDetail returns a full PageSpec snapshot for a version.
func (s *Service) VersionDetail(ctx context.Context, req *PageVersionDetailRequest) (*PageVersionDetailResponse, error) {
	versions, err := s.svcCtx.PageVersionModel.ListByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if fmt.Sprint(v.Version) != req.VersionID {
			continue
		}
		var pageSpec spec.PageSpec
		if err := json.Unmarshal([]byte(v.SpecJSON), &pageSpec); err != nil {
			return nil, fmt.Errorf("failed to decode page version: %w", err)
		}
		if pageSpec.PageKey == "" {
			return nil, fmt.Errorf("page version is not a complete PageSpec")
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

	return nil, fmt.Errorf("version not found: %s", req.VersionID)
}

// Rollback rolls back a page to a previous version.
func (s *Service) Rollback(ctx context.Context, req *PageRollbackRequest) (*PageRollbackResponse, error) {
	// Find the target version
	versions, err := s.svcCtx.PageVersionModel.ListByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}

	var targetVersion *model.PageVersion
	for _, v := range versions {
		if fmt.Sprintf("%d", v.Version) == req.VersionID || fmt.Sprint(v.Version) == req.VersionID {
			targetVersion = &v
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("version not found: %s", req.VersionID)
	}

	// Get current draft
	p, err := s.svcCtx.PageSpecModel.FindByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	var rolledBack spec.PageSpec
	if err := json.Unmarshal([]byte(targetVersion.SpecJSON), &rolledBack); err != nil {
		return nil, fmt.Errorf("failed to decode page version: %w", err)
	}
	if rolledBack.PageKey == "" {
		return nil, fmt.Errorf("page version is not a complete PageSpec")
	}

	// Update draft with rolled back full PageSpec
	if err := applyPageSpecToModel(p, rolledBack); err != nil {
		return nil, fmt.Errorf("failed to apply page version: %w", err)
	}
	p.DraftVersion = p.DraftVersion + 1
	p.Status = "draft"
	p.UpdatedAt = time.Now()
	specJSON, err := buildPageSpecJSON(p)
	if err != nil {
		return nil, fmt.Errorf("failed to encode rollback version: %w", err)
	}

	if err := s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		if err := pageModel.Upsert(ctx, p); err != nil {
			return fmt.Errorf("failed to rollback: %w", err)
		}
		if err := versionModel.Create(ctx, &model.PageVersion{
			PageKey:   req.PageKey,
			Version:   p.DraftVersion,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   fmt.Sprintf("rollback to version %d", targetVersion.Version),
			CreatedAt: p.UpdatedAt,
		}); err != nil {
			return fmt.Errorf("failed to save rollback version: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &PageRollbackResponse{
		PageKey: req.PageKey,
		Version: p.DraftVersion,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// isValidFormilySchema performs basic validation on Formily schema.
func isValidFormilySchema(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return false
	}
	// Must have at least "type" or "properties" or "x-component"
	_, hasType := parsed["type"]
	_, hasProps := parsed["properties"]
	_, hasXComp := parsed["x-component"]
	return hasType || hasProps || hasXComp
}

// normalizeLocaleKeys converts short locale keys to full keys.
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
		switch strings.ToLower(k) {
		case "zh", "zh-cn", "zh_cn":
			out["zh-CN"] = v
		case "en", "en-us", "en_us":
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

func getDescriptionFromJSON(jsonStr string) map[string]string {
	if jsonStr == "" {
		return nil
	}
	var desc map[string]string
	json.Unmarshal([]byte(jsonStr), &desc)
	return desc
}

func getMetadataFromJSON(jsonStr string) map[string]json.RawMessage {
	if jsonStr == "" {
		return nil
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &metadata); err != nil {
		return nil
	}
	return metadata
}

func convertBindingsToSpec(bindings []model.PageFunctionBindingBinding) []spec.PageFunctionBinding {
	if bindings == nil {
		return nil
	}
	result := make([]spec.PageFunctionBinding, len(bindings))
	for i, b := range bindings {
		result[i] = spec.PageFunctionBinding{
			FunctionID: b.FunctionID,
			Role:       spec.OperationPlacement(b.Role),
		}
	}
	return result
}

func convertBindingsToModel(bindings []spec.PageFunctionBinding) []model.PageFunctionBindingBinding {
	if bindings == nil {
		return nil
	}
	result := make([]model.PageFunctionBindingBinding, len(bindings))
	for i, b := range bindings {
		result[i] = model.PageFunctionBindingBinding{
			FunctionID: b.FunctionID,
			Role:       string(b.Role),
		}
	}
	return result
}

func pageSpecFromModel(p *model.PageSpec) spec.PageSpec {
	if p == nil {
		return spec.PageSpec{}
	}
	return spec.PageSpec{
		PageKey:     p.PageKey,
		Type:        spec.PageType(p.Type),
		ResourceKey: p.ResourceKey,
		Title:       p.GetTitle(),
		Description: getDescriptionFromJSON(p.DescriptionJSON),
		Category: spec.PageCategorySpec{
			Key:    p.CategoryKey,
			Labels: p.GetCategoryLabels(),
			Order:  p.CategoryOrder,
		},
		Order:    p.Order,
		Icon:     p.Icon,
		Schema:   spec.FormilySchema(p.GetSchema()),
		Bindings: convertBindingsToSpec(p.GetBindings()),
		Metadata: getMetadataFromJSON(p.MetadataJSON),
	}
}

func applyPageSpecToModel(p *model.PageSpec, ps spec.PageSpec) error {
	if p == nil {
		return fmt.Errorf("target page model is nil")
	}
	if strings.TrimSpace(ps.PageKey) != "" {
		p.PageKey = strings.TrimSpace(ps.PageKey)
	}
	p.Type = string(ps.Type)
	p.ResourceKey = strings.TrimSpace(ps.ResourceKey)
	p.CategoryKey = strings.TrimSpace(ps.Category.Key)
	if p.CategoryKey == "" {
		p.CategoryKey = inferCategoryFromKey(firstNonEmpty(p.ResourceKey, p.PageKey))
	}
	p.CategoryOrder = ps.Category.Order
	p.Order = ps.Order
	p.Icon = ps.Icon
	p.SchemaJSON = string(ps.Schema)
	if err := p.SetTitle(normalizeLocaleKeys(ps.Title)); err != nil {
		return err
	}
	if ps.Description != nil {
		b, err := json.Marshal(normalizeLocaleKeys(ps.Description))
		if err != nil {
			return err
		}
		p.DescriptionJSON = string(b)
	} else {
		p.DescriptionJSON = ""
	}
	if err := p.SetCategoryLabels(normalizeLocaleKeys(ps.Category.Labels)); err != nil {
		return err
	}
	if err := p.SetBindings(convertBindingsToModel(ps.Bindings)); err != nil {
		return err
	}
	if ps.Metadata != nil {
		b, err := json.Marshal(ps.Metadata)
		if err != nil {
			return err
		}
		p.MetadataJSON = string(b)
	} else {
		p.MetadataJSON = ""
	}
	return nil
}

func buildPageSpecJSON(p *model.PageSpec) (string, error) {
	ps := pageSpecFromModel(p)
	b, err := json.Marshal(ps)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) withPageTransaction(
	ctx context.Context,
	fn func(*model.PageSpecModel, *model.PublishedPageSpecModel, *model.PageVersionModel) error,
) error {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return fmt.Errorf("page service database is not initialized")
	}
	return s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(
			model.NewPageSpecModel(tx),
			model.NewPublishedPageSpecModel(tx),
			model.NewPageVersionModel(tx),
		)
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) normalizedFunctions(ctx context.Context) map[string]spec.FunctionSpec {
	inputs := descriptors.Collect(ctx, s.svcCtx)
	out := make(map[string]spec.FunctionSpec, len(inputs))
	for _, input := range inputs {
		result := normalizer.Normalize(input)
		if result.Function.ID != "" {
			out[result.Function.ID] = result.Function
		}
	}
	return out
}

func validateBoundFunction(field string, binding spec.PageFunctionBinding, fn spec.FunctionSpec) []spec.Diagnostic {
	var diags []spec.Diagnostic
	functionID := strings.TrimSpace(binding.FunctionID)
	role := spec.OperationPlacement(strings.TrimSpace(string(binding.Role)))

	if !fn.Enabled {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_function_disabled",
			Severity:   spec.SeverityError,
			Message:    "bound function is disabled: " + functionID,
			FunctionID: functionID,
			Field:      field + ".functionId",
		})
	}
	if strings.TrimSpace(fn.Operation) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_operation_missing",
			Severity:   spec.SeverityError,
			Message:    "bound function must declare operation before publishing",
			FunctionID: functionID,
			Field:      field + ".functionId",
		})
	}
	if fn.OperationKind == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_operation_kind_missing",
			Severity:   spec.SeverityError,
			Message:    "bound function must declare operationKind before publishing",
			FunctionID: functionID,
			Field:      field + ".functionId",
		})
	}
	if fn.Placement == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_placement_missing",
			Severity:   spec.SeverityError,
			Message:    "bound function must declare placement before publishing",
			FunctionID: functionID,
			Field:      field + ".functionId",
		})
	}
	if role != "" && fn.Placement != "" && role != fn.Placement {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_role_mismatch",
			Severity:   spec.SeverityError,
			Message:    fmt.Sprintf("binding role %q does not match function placement %q", role, fn.Placement),
			FunctionID: functionID,
			Field:      field + ".role",
		})
	}
	if !hasDefaultLocale(fn.OperationDisplay) {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_operation_label_missing",
			Severity:   spec.SeverityError,
			Message:    "bound function operationDisplay must include zh-CN before publishing",
			FunctionID: functionID,
			Field:      field + ".functionId",
		})
	}
	return diags
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

func getLocalizedText(labels spec.LocalizedText, lang, fallback string) string {
	if labels == nil {
		return fallback
	}
	if v, ok := labels[lang]; ok && v != "" {
		return v
	}
	if v, ok := labels["zh-CN"]; ok && v != "" {
		return v
	}
	for _, v := range labels {
		if v != "" {
			return v
		}
	}
	return fallback
}

func inferCategoryFromKey(key string) string {
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}

type categoryGroup struct {
	key    string
	labels spec.LocalizedText
	order  int
	pages  []pageEntry
}

type pageEntry struct {
	key   string
	title spec.LocalizedText
	icon  string
	order int
}

// ErrPageNotFound returns a not-found error.
func ErrPageNotFound(key string) error {
	return &PageNotFoundError{Key: key}
}

// PageNotFoundError indicates a page was not found.
type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return "page not found: " + e.Key
}
