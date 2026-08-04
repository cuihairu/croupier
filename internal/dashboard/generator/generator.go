// Package generator creates conservative default PageSpec suggestions from
// normalized function capability contracts. It does not read registration-side
// UI, menu, mapping, pagination, column, task view, or chart view extensions.
package generator

import (
	"sort"
	"strings"
	"unicode"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	DefaultLocale string
	Functions     map[string]spec.FunctionSpec
}

// DefaultGenerateOptions returns default options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{DefaultLocale: "zh-CN"}
}

// GenerateForResource creates default PageSpec candidates for every operation
// under a resource. Resource CRUD generation requires persistent
// CapabilitySemantics and is handled by GenerateResourcePageProposal.
func GenerateForResource(resource spec.ResourceSpec, opts GenerateOptions) []spec.GeneratedPageSpec {
	if len(resource.Operations) == 0 {
		return nil
	}
	opts = normalizeOptions(opts)
	ops := sortedOperations(resource.Operations)
	pages := make([]spec.GeneratedPageSpec, 0, len(ops))
	for _, op := range ops {
		pages = append(pages, GenerateForOperation(op, opts))
	}
	return pages
}

// GenerateEntityPageForResource is intentionally disabled until persistent
// CapabilitySemantics exists. Function registration cannot provide enough
// semantics to safely generate ResourcePage CRUD pages.
func GenerateEntityPageForResource(spec.ResourceSpec, []spec.OperationSpec, GenerateOptions) (spec.GeneratedPageSpec, bool, []string) {
	return spec.GeneratedPageSpec{}, false, nil
}

// GenerateForOperation creates an operation, task, or report candidate from a
// single executable capability.
func GenerateForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	switch {
	case isReportOperation(op):
		return GenerateReportPageForOperation(op, opts)
	case isTaskOperation(op):
		return GenerateTaskPageForOperation(op, opts)
	default:
		return GenerateOperationPageForOperation(op, opts)
	}
}

// GenerateOperationPageForOperation creates a standalone OperationPage.
func GenerateOperationPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "main", spec.BindingUsageAction, executionModeForOperation(op))
	applySelectors(&binding, opts.Functions[op.FunctionID])
	diags := assessBaseCandidate(op)
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForOperation(op.FunctionID, locale),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Operation: &spec.OperationPageSpec{
				Form: buildFormPresentation(op, opts),
				ResultView: &spec.ResultViewSpec{
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

// GenerateTaskPageForOperation creates an async task candidate. Without task
// status/event semantics it must be reviewed before publication.
func GenerateTaskPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "task", spec.BindingUsageTask, spec.PageExecutionModeTask)
	applySelectors(&binding, opts.Functions[op.FunctionID])
	diags := append(assessBaseCandidate(op), diagnostic(
		"task_semantics_missing",
		spec.SeverityWarning,
		"task capability requires status/events/result semantics before it can be safely published",
		op.FunctionID,
		"capability",
	))
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeTask,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForOperation(op.FunctionID, locale),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Task: &spec.TaskPageSpec{
				Form: buildFormPresentation(op, opts),
				TaskView: &spec.TaskViewSpec{
					ShowTimeline: true,
					ShowProgress: true,
					ShowEvents:   true,
					Cancelable:   true,
					Retryable:    true,
				},
				ResultView: &spec.ResultViewSpec{
					SuccessMessage: spec.LocalizedText{locale: "任务完成"},
					ErrorMessage:   spec.LocalizedText{locale: "任务失败"},
				},
			},
			Bindings: []spec.PageFunctionBinding{binding},
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
	}
}

// GenerateReportPageForOperation creates a report candidate. Without dataset,
// dimension, metric, and chart semantics it must be reviewed before publication.
func GenerateReportPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "report", spec.BindingUsageReport, spec.PageExecutionModeSync)
	applySelectors(&binding, opts.Functions[op.FunctionID])
	diags := append(assessBaseCandidate(op), diagnostic(
		"report_semantics_missing",
		spec.SeverityWarning,
		"report capability requires dataset, dimension, metric, and chart semantics before it can be safely published",
		op.FunctionID,
		"capability",
	))
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeReport,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForOperation(op.FunctionID, locale),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Report: &spec.ReportPageSpec{
				QueryForm: buildFormPresentation(op, opts),
				Dataset: &spec.DatasetSpec{
					Dimensions: inferDimensions(op),
					Metrics:    inferMetrics(op),
				},
				Exportable: true,
			},
			Bindings: []spec.PageFunctionBinding{binding},
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
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
	return opts
}

func assessBaseCandidate(op spec.OperationSpec) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if strings.TrimSpace(op.FunctionID) == "" {
		diags = append(diags, diagnostic("function_id_missing", spec.SeverityError, "functionId is required", "", "functionId"))
	}
	if !op.Enabled {
		diags = append(diags, diagnostic("function_disabled", spec.SeverityError, "function is disabled and cannot be executed", op.FunctionID, "enabled"))
	}
	if strings.TrimSpace(op.ResourceKey) == "" {
		diags = append(diags, diagnostic("resource_missing", spec.SeverityWarning, "resource is missing; Page Studio must choose page grouping before publishing", op.FunctionID, "resourceKey"))
	}
	if strings.TrimSpace(op.Operation) == "" {
		diags = append(diags, diagnostic("operation_missing", spec.SeverityWarning, "operation is missing; Page Studio must name how this capability is used", op.FunctionID, "operation"))
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
	if binding == nil || len(fn.InputSchema) == 0 {
		return
	}
	binding.Selectors = &spec.BindingSelectors{
		Input: spec.DefaultSelector(fn.InputSchema),
	}
}

func buildFormPresentation(op spec.OperationSpec, opts GenerateOptions) *spec.FormPresentationSpec {
	fn, ok := opts.Functions[op.FunctionID]
	if !ok || len(fn.InputSchema) == 0 {
		return spec.DefaultFormPresentation(spec.JSONSchema(`{"type":"object","properties":{}}`))
	}
	return spec.DefaultFormPresentation(fn.InputSchema)
}

func executionModeForOperation(op spec.OperationSpec) spec.PageExecutionMode {
	if op.Execution == spec.FunctionExecutionTask {
		return spec.PageExecutionModeTask
	}
	return spec.PageExecutionModeSync
}

func sortedOperations(ops []spec.OperationSpec) []spec.OperationSpec {
	out := append([]spec.OperationSpec(nil), ops...)
	sort.SliceStable(out, func(i, j int) bool {
		left := firstNonEmpty(out[i].ResourceKey, "") + "." + firstNonEmpty(out[i].Operation, "") + "." + out[i].FunctionID
		right := firstNonEmpty(out[j].ResourceKey, "") + "." + firstNonEmpty(out[j].Operation, "") + "." + out[j].FunctionID
		return left < right
	})
	return out
}

func isTaskOperation(op spec.OperationSpec) bool {
	return op.Execution == spec.FunctionExecutionTask || op.Capability == spec.CapabilityTask
}

func isReportOperation(op spec.OperationSpec) bool {
	return op.Capability == spec.CapabilityReport
}

func categoryForResource(resourceKey string, locale string) spec.PageCategorySpec {
	categoryKey := InferCategoryFromKey(resourceKey)
	return spec.PageCategorySpec{
		Key: categoryKey,
		Labels: spec.LocalizedText{
			locale: humanizeKey(categoryKey),
		},
	}
}

func categoryForOperation(functionID string, locale string) spec.PageCategorySpec {
	categoryKey := InferCategoryFromKey(functionID)
	return spec.PageCategorySpec{
		Key: categoryKey,
		Labels: spec.LocalizedText{
			locale: humanizeKey(categoryKey),
		},
	}
}

func localizedTitle(op spec.OperationSpec, pageKey string, locale string, opts GenerateOptions) spec.LocalizedText {
	if fn, ok := opts.Functions[op.FunctionID]; ok {
		if summary := strings.TrimSpace(fn.Summary[locale]); summary != "" {
			return spec.LocalizedText{locale: summary}
		}
	}
	return spec.LocalizedText{
		locale: humanizeKey(firstNonEmpty(op.Operation, op.FunctionID, pageKey)),
	}
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
	quality := qualityFromDiagnostics(diags)
	if hasErrorDiagnostic(diags) {
		return quality
	}
	if quality == spec.GeneratedPageQualityReady {
		return spec.GeneratedPageQualityBasic
	}
	if quality == spec.GeneratedPageQualityNeedsReview && strings.TrimSpace(op.FunctionID) != "" {
		return spec.GeneratedPageQualityBasic
	}
	return quality
}

func hasErrorDiagnostic(diags []spec.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == spec.SeverityError {
			return true
		}
	}
	return false
}

func inferDimensions(spec.OperationSpec) []spec.DimensionSpec {
	return []spec.DimensionSpec{
		{Key: "date", Title: spec.LocalizedText{"zh-CN": "日期"}, DataType: "date"},
	}
}

func inferMetrics(spec.OperationSpec) []spec.MetricSpec {
	return []spec.MetricSpec{
		{Key: "count", Title: spec.LocalizedText{"zh-CN": "数量"}, DataType: "number", AggType: "sum"},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func humanizeKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var words []string
	var current strings.Builder
	for i, r := range value {
		if r == '.' || r == '_' || r == '-' || unicode.IsSpace(r) {
			flushWord(&words, &current)
			continue
		}
		if i > 0 && unicode.IsUpper(r) && current.Len() > 0 {
			flushWord(&words, &current)
		}
		current.WriteRune(r)
	}
	flushWord(&words, &current)
	if len(words) == 0 {
		return value
	}
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func flushWord(words *[]string, current *strings.Builder) {
	if current.Len() == 0 {
		return
	}
	*words = append(*words, current.String())
	current.Reset()
}

// InferPageType returns the strongest page shape supported by capability
// semantics. It never reads function names.
func InferPageType(ops []spec.OperationSpec) spec.PageType {
	for _, op := range ops {
		if isReportOperation(op) {
			return spec.PageTypeReport
		}
	}
	for _, op := range ops {
		if isTaskOperation(op) {
			return spec.PageTypeTask
		}
	}
	return spec.PageTypeOperation
}

// InferCategoryFromKey extracts category from a key.
func InferCategoryFromKey(key string) string {
	key = strings.TrimSpace(key)
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}

func diagnostic(code string, severity spec.DiagnosticSeverity, message string, functionID string, field string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:       code,
		Severity:   severity,
		Message:    message,
		FunctionID: functionID,
		Field:      field,
	}
}
