package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOperationPageForOperation creates a standalone OperationPage.
func GenerateOperationPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "main", spec.BindingUsageAction, executionModeForOperation(op))
	fn := opts.Functions[op.FunctionID]
	applySelectors(&binding, fn)
	diags := assessBaseCandidate(op)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "inputSchema", fn.InputSchema)...)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "outputSchema", fn.OutputSchema)...)
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForOperation(op.FunctionID),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Operation: &spec.OperationPageSpec{
				Form:    buildFormPresentation(op, opts),
				Confirm: buildOperationConfirm(op, locale),
				ResultView: &spec.ResultViewSpec{
					Fields:         buildResultFields(fn.OutputSchema, locale),
					SuccessMessage: spec.LocalizedText{locale: "操作成功"},
					ErrorMessage:   spec.LocalizedText{locale: "操作失败"},
				},
			},
			Bindings: []spec.PageFunctionBinding{binding},
		},
		Quality:     operationQuality(op, diags),
		Diagnostics: diags,
	}
}

func buildOperationConfirm(op spec.OperationSpec, locale string) *spec.ConfirmActionSpec {
	if !operationRequiresConfirmation(op) {
		return nil
	}
	return &spec.ConfirmActionSpec{
		Title:       spec.LocalizedText{locale: "确认操作"},
		Description: spec.LocalizedText{locale: "此操作可能影响线上数据，请确认后继续。"},
		ConfirmText: spec.LocalizedText{locale: "确认执行"},
		CancelText:  spec.LocalizedText{locale: "取消"},
		BindingID:   bindingIDForOperationWithSuffix(op, "main"),
		Permission:  strings.TrimSpace(op.Permission),
		Risk:        string(op.Risk),
	}
}

func operationRequiresConfirmation(op spec.OperationSpec) bool {
	if op.Approval.Required {
		return true
	}
	switch op.Risk {
	case spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
}

func operationPageKey(op spec.OperationSpec, opts GenerateOptions) string {
	_ = opts
	functionID := sanitizeSourceKey(op.FunctionID)
	if functionID == "" {
		functionID = "unbound"
	}
	return string(pageKindForOperation(op)) + "--" + functionID
}

func normalizeOptions(opts GenerateOptions) GenerateOptions {
	if strings.TrimSpace(opts.DefaultLocale) == "" {
		opts.DefaultLocale = "zh-CN"
	}
	if opts.Functions == nil {
		opts.Functions = map[string]spec.FunctionSpec{}
	}
	if opts.TaskSemantics == nil {
		opts.TaskSemantics = map[string]spec.TaskSemantic{}
	}
	if opts.ReportSemantics == nil {
		opts.ReportSemantics = map[string]spec.ReportSemantic{}
	}
	return opts
}

func assessBaseCandidate(op spec.OperationSpec) []spec.Diagnostic {
	// Contract-level findings (SUI-008 unresolved references, SUI-011 request
	// compatibility) travel on the operation from the projection. Without them
	// a request the executor cannot send would still generate a directly
	// executable page: the projected input schema no longer carries the media
	// types and parameter locations those findings name.
	diags := append([]spec.Diagnostic(nil), op.Diagnostics...)
	if strings.TrimSpace(op.FunctionID) == "" {
		diags = append(diags, diagnostic("function_id_missing", spec.SeverityError, "functionId is required", "", "functionId"))
	}
	if !op.Enabled {
		diags = append(diags, diagnostic("function_disabled", spec.SeverityError, "function is disabled and cannot be executed", op.FunctionID, "enabled"))
	}
	if strings.TrimSpace(op.Operation) == "" {
		diags = append(diags, diagnostic("operation_missing", spec.SeverityWarning, "operation is missing; generated title falls back to functionId", op.FunctionID, "operation"))
	}
	return diags
}

func pageBinding(op spec.OperationSpec, suffix string, usage spec.PageBindingUsage, mode spec.PageExecutionMode) spec.PageFunctionBinding {
	return spec.PageFunctionBinding{
		ID:         bindingIDForOperationWithSuffix(op, suffix),
		FunctionID: op.FunctionID,
		Usage:      usage,
		Execution:  spec.PageBindingExecution{Mode: mode},
	}
}

func applySelectors(binding *spec.PageFunctionBinding, fn spec.FunctionSpec) {
	if binding == nil {
		return
	}
	selectors := &spec.BindingSelectors{}
	if len(fn.InputSchema) > 0 {
		selectors.Input = spec.DefaultSelector(fn.InputSchema)
	}
	selectors.Output = defaultOutputAssignments(binding.Usage, fn.OutputSchema)
	if len(selectors.Input.Assignments) == 0 && len(selectors.Output) == 0 {
		return
	}
	binding.Selectors = selectors
}

func buildFormPresentation(op spec.OperationSpec, opts GenerateOptions) *spec.FormPresentationSpec {
	fn, ok := opts.Functions[op.FunctionID]
	if !ok || len(fn.InputSchema) == 0 {
		// 兑现 normalizer「single payload field」承诺：无 schema 函数也能得到
		// 一个可操作的表单（单 JSON 文本字段），而不是渲染为空的废页。
		return payloadOnlyFormPresentation()
	}
	// SUI-025：与调试页同源默认派生（声明序字段、控件、分组、layout hints；
	// 无 hints 时只给纵向 layout，渲染端兜底合成标准表单）。
	derived := deriveFormPresentation(fn.InputSchema)
	fp := spec.DefaultFormPresentation(fn.InputSchema)
	fp.Layout = derived.layout
	if derived.hasHints {
		fp.Fields = derived.fields
		fp.Groups = derived.groups
	}
	return fp
}

func executionModeForOperation(op spec.OperationSpec) spec.PageExecutionMode {
	if op.Execution == spec.FunctionExecutionTask {
		return spec.PageExecutionModeTask
	}
	return spec.PageExecutionModeSync
}

func isTaskOperation(op spec.OperationSpec) bool {
	return op.Execution == spec.FunctionExecutionTask || op.Capability == spec.CapabilityTask
}

func isReportOperation(op spec.OperationSpec) bool {
	return op.Capability == spec.CapabilityReport
}

// categoryForResource / categoryForOperation 只产出分组 key：
// 分类名称（labels）由菜单系统（menu_items.labels）接管（T-M8）。
func categoryForResource(resourceKey string) spec.PageCategorySpec {
	return spec.PageCategorySpec{Key: InferCategoryFromKey(resourceKey)}
}

func categoryForOperation(functionID string) spec.PageCategorySpec {
	return spec.PageCategorySpec{Key: InferCategoryFromKey(functionID)}
}

// ensureDefaultLocale 把任意来源的 LocalizedText（SDK Summary、词条、
// humanize 兜底）规整为「默认名称必填、翻译可选」形态（T12）：空白值
// 剔除；zh-CN（第一推荐展示语言）缺失时取任意既有值补位，其余 locale
// 不再强制补写——仅有其他语言的文本直接透传，发布校验按任一非空放行。
// 已有双 key 的不删除，词条字典补录翻译后自然覆盖。
func ensureDefaultLocale(text spec.LocalizedText) spec.LocalizedText {
	out := make(spec.LocalizedText, len(text))
	for k, v := range text {
		if strings.TrimSpace(v) != "" {
			out[k] = strings.TrimSpace(v)
		}
	}
	if len(out) == 0 {
		return out
	}
	if _, ok := out["zh-CN"]; !ok {
		// zh-CN 缺失时取任意既有值补位，保证第一推荐语言不空
		for _, v := range out {
			out["zh-CN"] = v
			break
		}
	}
	return out
}

// localizedTitleFallback humanizes a raw key into a multi-locale LocalizedText:
// the humanized label is written into zh-CN and en-US (natural bilingual
// fallback data, not a publish requirement) plus the requested locale when
// it differs.
func localizedTitleFallback(key string, locale string) spec.LocalizedText {
	label := fallbackLabel(key)
	if label == "" {
		label = key
	}
	text := spec.LocalizedText{"zh-CN": label, "en-US": label}
	if locale != "" && locale != "zh-CN" && locale != "en-US" {
		text[locale] = label
	}
	return text
}

func localizedTitle(op spec.OperationSpec, pageKey string, locale string, opts GenerateOptions) spec.LocalizedText {
	if fn, ok := opts.Functions[op.FunctionID]; ok {
		if summary := strings.TrimSpace(fn.Summary[locale]); summary != "" {
			return ensureDefaultLocale(spec.LocalizedText{locale: summary})
		}
	}
	fallbackKey := firstNonEmpty(op.Operation, op.FunctionID, pageKey)
	if text, ok := opts.Terms.Lookup("operation", op.Operation); ok && len(text) > 0 {
		return ensureDefaultLocale(text)
	}
	if text, ok := opts.Terms.Lookup("resource", fallbackKey); ok && len(text) > 0 {
		return ensureDefaultLocale(text)
	}
	return ensureDefaultLocale(spec.LocalizedText{
		locale: fallbackLabel(fallbackKey),
	})
}

func bindingIDForOperationWithSuffix(op spec.OperationSpec, suffix string) string {
	parts := []string{op.ResourceKey, suffix}
	if strings.TrimSpace(parts[0]) == "" {
		parts[0] = op.FunctionID
	}
	if strings.TrimSpace(parts[1]) == "" {
		parts[1] = "main"
	}
	return sanitizeBindingID(strings.Join(parts, "."))
}

func sanitizeBindingID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "binding"
	}
	var b strings.Builder
	lastDot := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDot = false
			continue
		}
		if !lastDot {
			b.WriteByte('.')
			lastDot = true
		}
	}
	return strings.Trim(b.String(), ".")
}

func sanitizePageKey(value string) string {
	return sanitizeBindingID(value)
}

func sanitizeSourceKey(value string) string {
	return strings.Trim(sanitizePageKey(value), ".")
}

func pageKindForOperation(op spec.OperationSpec) spec.PageType {
	switch {
	case isReportOperation(op):
		return spec.PageTypeReport
	case isTaskOperation(op):
		return spec.PageTypeTask
	default:
		return spec.PageTypeOperation
	}
}

func qualityFromDiagnostics(diags []spec.Diagnostic) spec.GeneratedPageQuality {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return spec.GeneratedPageQualityNeedsReview
		}
	}
	if len(diags) > 0 {
		return spec.GeneratedPageQualityNeedsReview
	}
	return spec.GeneratedPageQualityReady
}

func operationQuality(op spec.OperationSpec, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	if hasErrorDiagnostic(diags) {
		return spec.GeneratedPageQualityNeedsReview
	}
	// SUI-011: a request the executor cannot send gets no directly executable
	// default proposal. The degraded tier is deliberately not gated: its JSON
	// editing path stays usable.
	if hasDiagnosticCode(diags, "json_schema_generation_subset_unsupported") ||
		hasDiagnosticCode(diags, spec.DiagSchemaCompatibilityManual) {
		return spec.GeneratedPageQualityNeedsReview
	}
	if strings.TrimSpace(op.FunctionID) == "" {
		return spec.GeneratedPageQualityNeedsReview
	}
	return spec.GeneratedPageQualityBasic
}

func taskQuality(op spec.OperationSpec, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	quality := qualityFromDiagnostics(diags)
	if hasErrorDiagnostic(diags) {
		return quality
	}
	// Task pages without explicit task semantics should be needs_review
	if hasWarningDiagnostic(diags) {
		return spec.GeneratedPageQualityNeedsReview
	}
	if strings.TrimSpace(op.FunctionID) != "" {
		return spec.GeneratedPageQualityBasic
	}
	return quality
}

func hasWarningDiagnostic(diags []spec.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == spec.SeverityWarning {
			return true
		}
	}
	return false
}

func hasErrorDiagnostic(diags []spec.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == spec.SeverityError {
			return true
		}
	}
	return false
}

func hasDiagnosticCode(diags []spec.Diagnostic, code string) bool {
	for _, diag := range diags {
		if diag.Code == code {
			return true
		}
	}
	return false
}

func defaultOutputAssignments(usage spec.PageBindingUsage, outputSchema spec.JSONSchema) []spec.OutputAssignment {
	if len(outputSchema) == 0 {
		return nil
	}
	switch usage {
	case spec.BindingUsageQuery:
		return collectionOutputAssignments(outputSchema, []string{"items", "list", "rows", "data"}, "items")
	case spec.BindingUsageReport:
		if source := collectionOutputSource(outputSchema, []string{"dataset", "items", "rows", "data"}); source != "" || schemaRootType(outputSchema) == "array" {
			return []spec.OutputAssignment{{
				StateKey: "dataset",
				Source:   source,
				Shape:    spec.OutputShapeDataset,
			}}
		}
		return nil
	case spec.BindingUsageAction, spec.BindingUsageDetail:
		return []spec.OutputAssignment{{
			StateKey: "result",
			Source:   "",
			Shape:    outputShapeForSchema(outputSchema),
		}}
	default:
		return nil
	}
}

// RecomputeDefaultOutputs 是 defaultOutputAssignments 的导出包装，供
// sync-selectors planner 注入（spec.OutputRechooser）：必需输出
// （items/dataset 等）的重推导与生成期共用同一默认推导，避免两处
// 规则漂移。task 类辅助绑定（status/events/result）生成期由
// TaskSemantic 指定路径，不经此函数，推导返回 nil 由 planner 走
// rename/根对象 fallback。
func RecomputeDefaultOutputs(usage spec.PageBindingUsage, outputSchema spec.JSONSchema) []spec.OutputAssignment {
	return defaultOutputAssignments(usage, outputSchema)
}
