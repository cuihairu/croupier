package page

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

const rendererSchemaVersion = "formily-page:1"

var supportedPageComponents = map[string]struct{}{
	"ConsolePage":  {},
	"QueryForm":    {},
	"DataTable":    {},
	"DetailPanel":  {},
	"ActionButton": {},
	"ActionGroup":  {},
	"ResultPanel":  {},
	"TaskTimeline": {},
	"ChartPanel":   {},
}

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) ListDrafts(ctx context.Context, req *PageDraftListRequest) (*PageDraftListResponse, error) {
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
	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	return pageDraftResponseFromModel(p), nil
}

func (s *Service) SaveDraft(ctx context.Context, req *PageSaveRequest) (*PageSaveResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}
	if strings.TrimSpace(req.PageKey) == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	if !isValidPageType(req.Type) {
		return nil, errorx.NewBadRequest("type must be entity, operation, task, or report")
	}
	if !isValidFormilySchema(req.Schema) {
		return nil, errorx.NewBadRequest("schema must be a valid Formily JSON Schema")
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

	now := time.Now()
	var nextRevision int
	err = s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		existing, err := pageModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
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
			SchemaJSON:    string(req.Schema),
			Status:        "draft",
			UpdatedBy:     strings.TrimSpace(req.UpdatedBy),
			UpdatedAt:     now,
		}
		if err := ps.SetTitle(title); err != nil {
			return err
		}
		if description := normalizeLocaleKeys(req.Description); description != nil {
			b, err := json.Marshal(description)
			if err != nil {
				return err
			}
			ps.DescriptionJSON = string(b)
		}
		if err := ps.SetCategoryLabels(categoryLabels); err != nil {
			return err
		}
		if err := ps.SetBindings(convertBindingsToModel(req.Bindings)); err != nil {
			return err
		}
		if req.Metadata != nil {
			b, err := json.Marshal(req.Metadata)
			if err != nil {
				return err
			}
			ps.MetadataJSON = string(b)
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

		if err := pageModel.Upsert(ctx, ps); err != nil {
			return fmt.Errorf("save page draft: %w", err)
		}
		specJSON, err := buildPageSpecJSON(ps)
		if err != nil {
			return err
		}
		return versionModel.Create(ctx, &model.PageVersion{
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
	return &PageSaveResponse{PageKey: req.PageKey, DraftRevision: nextRevision}, nil
}

func (s *Service) Validate(ctx context.Context, req *PageValidateRequest) (*PageValidateResponse, error) {
	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	diags := s.validatePageSpec(ctx, pageSpecFromModel(p), true)
	return &PageValidateResponse{Valid: countErrors(diags) == 0, Diagnostics: diags}, nil
}

func (s *Service) Preview(ctx context.Context, req *PagePreviewRequest) (*PagePreviewResponse, error) {
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
	gameID, env, err := requireScope(ctx)
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
	err = s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		if err := publishedModel.DeactivatePage(ctx, gameID, env, req.PageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(ctx, &model.PublishedPageSpec{
			GameID:                gameID,
			Env:                   env,
			PageKey:               req.PageKey,
			Version:               publishedVersion,
			SpecJSON:              string(specJSON),
			BindingContractsJSON:  string(contractsJSON),
			RendererSchemaVersion: rendererSchemaVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           strings.TrimSpace(req.PublishedBy),
		}); err != nil {
			return err
		}
		p.Status = "published"
		p.PublishedActive = true
		p.PublishedVersion = publishedVersion
		p.UpdatedAt = now
		if err := pageModel.Upsert(ctx, p); err != nil {
			return err
		}
		return versionModel.Create(ctx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   req.PageKey,
			Version:   publishedVersion,
			SpecJSON:  string(specJSON),
			Status:    "published",
			Message:   "publish",
			CreatedBy: req.PublishedBy,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return &PagePublishResponse{PageKey: req.PageKey, Published: true, PublishedVersion: publishedVersion}, nil
}

func (s *Service) Unpublish(ctx context.Context, req *PageUnpublishRequest) (*PageUnpublishResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	now := time.Now()
	err = s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, _ *model.PageVersionModel) error {
		p.Status = "draft"
		p.PublishedActive = false
		p.UpdatedAt = now
		if err := pageModel.Upsert(ctx, p); err != nil {
			return err
		}
		return publishedModel.DeactivatePage(ctx, gameID, env, req.PageKey, now)
	})
	if err != nil {
		return nil, err
	}
	return &PageUnpublishResponse{PageKey: req.PageKey, Published: false}, nil
}

func (s *Service) Versions(ctx context.Context, req *PageVersionsRequest) (*PageVersionsResponse, error) {
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
	gameID, env, err := requireScope(ctx)
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
	err = s.withPageTransaction(ctx, func(pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		if err := pageModel.Upsert(ctx, p); err != nil {
			return err
		}
		return versionModel.Create(ctx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   req.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "rollback to version " + strconv.Itoa(target.Version),
			CreatedAt: p.UpdatedAt,
		})
	})
	if err != nil {
		return nil, err
	}
	return &PageRollbackResponse{PageKey: req.PageKey, DraftRevision: p.DraftRevision}, nil
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
	case spec.PageTypeEntity, spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport:
		return true
	default:
		return false
	}
}

func isValidFormilySchema(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return false
	}
	return parsed["type"] != nil || parsed["properties"] != nil || parsed["x-component"] != nil
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

	bindingsByID := map[string]spec.PageFunctionBinding{}
	functions := s.normalizedFunctions(ctx)
	for i, binding := range page.Bindings {
		field := fmt.Sprintf("bindings[%d]", i)
		diags = append(diags, validateBinding(field, binding, functions)...)
		if strings.TrimSpace(binding.ID) != "" {
			if _, exists := bindingsByID[binding.ID]; exists {
				diags = append(diags, diagnostic("binding_id_duplicate", spec.SeverityError, "binding id must be unique", field+".id"))
			}
			bindingsByID[binding.ID] = binding
		}
	}

	diags = append(diags, validatePageSchema(page.Schema, bindingsByID, publish)...)
	return diags
}

func validateBinding(field string, binding spec.PageFunctionBinding, functions map[string]spec.FunctionSpec) []spec.Diagnostic {
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
	return diags
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

func validatePageSchema(schema spec.FormilySchema, bindings map[string]spec.PageFunctionBinding, publish bool) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if len(schema) == 0 {
		return []spec.Diagnostic{diagnostic("page_schema_missing", spec.SeverityError, "schema is required", "schema")}
	}
	var root map[string]any
	if err := json.Unmarshal(schema, &root); err != nil {
		return []spec.Diagnostic{diagnostic("page_schema_invalid_json", spec.SeverityError, "schema must be JSON object", "schema")}
	}
	component, _ := root["x-component"].(string)
	if component != "ConsolePage" {
		diags = append(diags, diagnostic("page_root_component_invalid", spec.SeverityError, "schema root x-component must be ConsolePage", "schema.x-component"))
	}
	if publish {
		props, _ := root["x-component-props"].(map[string]any)
		version, _ := props["schemaVersion"].(string)
		if version != rendererSchemaVersion {
			diags = append(diags, diagnostic("page_schema_version_invalid", spec.SeverityError, "ConsolePage schemaVersion must be "+rendererSchemaVersion, "schema.x-component-props.schemaVersion"))
		}
	}
	walkSchema(root, "schema", bindings, &diags)
	return diags
}

func walkSchema(node map[string]any, path string, bindings map[string]spec.PageFunctionBinding, diags *[]spec.Diagnostic) {
	component, _ := node["x-component"].(string)
	if component != "" {
		if _, ok := supportedPageComponents[component]; !ok {
			*diags = append(*diags, diagnostic("page_component_unknown", spec.SeverityError, "unknown page component: "+component, path+".x-component"))
		}
	}
	props, _ := node["x-component-props"].(map[string]any)
	if _, hasFunctionID := props["functionId"]; hasFunctionID {
		*diags = append(*diags, diagnostic("page_schema_function_id_forbidden", spec.SeverityError, "page schema must reference bindingId, not functionId", path+".x-component-props.functionId"))
	}
	if bindingValue, hasBinding := props["bindingId"]; hasBinding {
		bindingID, ok := bindingValue.(string)
		if !ok || strings.TrimSpace(bindingID) == "" {
			*diags = append(*diags, diagnostic("page_schema_binding_id_invalid", spec.SeverityError, "bindingId must be a non-empty string", path+".x-component-props.bindingId"))
		} else if _, exists := bindings[bindingID]; !exists {
			*diags = append(*diags, diagnostic("page_schema_binding_unknown", spec.SeverityError, "bindingId is not defined: "+bindingID, path+".x-component-props.bindingId"))
		}
	}
	if component == "DataTable" {
		requireStringProp(props, "bindingId", path, diags)
		requireStringProp(props, "itemsPath", path, diags)
		requireStringProp(props, "totalPath", path, diags)
		requireStringProp(props, "pageField", path, diags)
		requireStringProp(props, "pageSizeField", path, diags)
		if _, hasColumns := props["columns"]; !hasColumns {
			requireStringProp(props, "columnsPath", path, diags)
		}
	}
	for key, child := range objectChildren(node["properties"]) {
		walkSchema(child, path+".properties."+key, bindings, diags)
	}
}

func requireStringProp(props map[string]any, key, path string, diags *[]spec.Diagnostic) {
	value, ok := props[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		*diags = append(*diags, diagnostic("page_component_prop_missing", spec.SeverityError, key+" is required", path+".x-component-props."+key))
	}
}

func objectChildren(value any) map[string]map[string]any {
	props, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]map[string]any)
	for key, raw := range props {
		child, ok := raw.(map[string]any)
		if ok {
			out[key] = child
		}
	}
	return out
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

func convertBindingsToSpec(bindings []model.PageFunctionBindingBinding) []spec.PageFunctionBinding {
	if bindings == nil {
		return nil
	}
	result := make([]spec.PageFunctionBinding, len(bindings))
	for i, b := range bindings {
		result[i] = spec.PageFunctionBinding{
			ID:            b.ID,
			FunctionID:    b.FunctionID,
			Usage:         spec.PageBindingUsage(b.Usage),
			InputMapping:  b.InputMapping,
			OutputMapping: b.OutputMapping,
			Execution: spec.PageBindingExecution{
				Mode:           spec.PageExecutionMode(b.Execution.Mode),
				RequireConfirm: b.Execution.RequireConfirm,
			},
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
			ID:            strings.TrimSpace(b.ID),
			FunctionID:    strings.TrimSpace(b.FunctionID),
			Usage:         string(b.Usage),
			InputMapping:  b.InputMapping,
			OutputMapping: b.OutputMapping,
			Execution: model.BindingExecutionBinding{
				Mode:           string(b.Execution.Mode),
				RequireConfirm: b.Execution.RequireConfirm,
			},
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
	b, err := json.Marshal(pageSpecFromModel(p))
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
		return errorx.NewInternalError("page service database is not initialized")
	}
	return s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(model.NewPageSpecModel(tx), model.NewPublishedPageSpecModel(tx), model.NewPageVersionModel(tx))
	})
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

func getDescriptionFromJSON(jsonStr string) map[string]string {
	if jsonStr == "" {
		return nil
	}
	var desc map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &desc); err != nil {
		return nil
	}
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
