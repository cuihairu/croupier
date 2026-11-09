package page

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

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

func isValidPageType(t spec.PageType) bool {
	switch t {
	case spec.PageTypeResource, spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport,
		spec.PageTypeComposite:
		return true
	default:
		return false
	}
}

func (s *Service) ValidatePageSpecInScope(ctx context.Context, gameID, env string, page spec.PageSpec, publish bool) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if !isValidPageType(page.Type) {
		diags = append(diags, diagnostic("page_type_invalid", spec.SeverityError, "page type is invalid", "type"))
	}
	if !hasDefaultLocale(page.Title) {
		diags = append(diags, diagnostic("localized_text_missing", spec.SeverityError, "title must include a non-empty value in at least one locale", "title"))
	}
	if strings.TrimSpace(page.Category.Key) == "" {
		diags = append(diags, diagnostic("category_key_missing", spec.SeverityError, "category.key is required", "category.key"))
	}
	// category.labels 校验已随菜单系统接管一并移除（T-M8）：分类名称由
	// menu_items.labels 提供，页面规格不再承载。
	if len(page.Bindings) == 0 {
		diags = append(diags, diagnostic("bindings_missing", spec.SeverityError, "page must bind at least one function", "bindings"))
	}
	diags = append(diags, validatePageShape(page)...)
	if publish {
		diags = append(diags, spec.ValidatePublishablePageShape(page)...)
	}

	bindingsByID := map[string]spec.PageFunctionBinding{}
	functions := s.functionsForScope(ctx, gameID, env)
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

func (s *Service) validatePageSpec(ctx context.Context, page spec.PageSpec, publish bool) []spec.Diagnostic {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return []spec.Diagnostic{diagnostic("scope_missing", spec.SeverityError, "game and environment scope are required", "scope")}
	}
	return s.ValidatePageSpecInScope(ctx, gameID, env, page, publish)
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
	case spec.PageTypeComposite:
		requireOnly("composite", page.Composite != nil && len(page.Composite.Sections) > 0)
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
		result := validateSelectorFn(binding.Selectors.Input, fn.InputSchema, spec.SelectorContextForBinding(page, binding))
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
		outputResult := validateOutputAssignmentsFn(binding.Selectors.Output, fn.OutputSchema)
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

// digestRaw 冻结 binding 契约快照 digest。必须与 freshness 评估端共用
// CanonicalDigest：raw 字节哈希会因 jsonb 列键序/空格形态与 canonical 序不同，
// 同一 schema 也恒不等，导致 bindingFreshness 永远报 schema stale（2026-09-19
// 线上 operation 页面 5 changes 无法收敛的根因）。
func digestRaw(raw []byte) string {
	return freshness.CanonicalDigest(raw)
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
		MenuID:           menuIDToDTO(p.MenuID),
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:        p.UpdatedBy,
	}, nil
}

// menuIDToDTO 把模型层 *uint 菜单外键转成 API 契约的 *int64。
func menuIDToDTO(menuID *uint) *int64 {
	if menuID == nil {
		return nil
	}
	v := int64(*menuID)
	return &v
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
	// SetTitle 恒返回 nil（model 层实现为 `b, _ := json.Marshal(
	// map[string]string)`，无出错路径），err 检查已删。
	_ = p.SetTitle(normalizeLocaleKeys(ps.Title))
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

// hasDefaultLocale 校验默认名称必填、翻译可选（T12）：任一 locale 有
// 非空值即通过——zh-CN 是第一推荐展示语言（渲染回退链首位），但不再
// 强制 en-US 双写，仅有 en-US 的存量页面也不被误拒。全空/全空白拒绝。
func hasDefaultLocale(labels spec.LocalizedText) bool {
	if labels == nil {
		return false
	}
	for _, value := range labels {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
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
