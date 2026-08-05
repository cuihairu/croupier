// Package generator creates conservative default PageSpec suggestions from
// normalized function capability contracts. It does not read registration-side
// UI, menu, mapping, pagination, column, task view, or chart view extensions.
package generator

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	DefaultLocale string
	Functions     map[string]spec.FunctionSpec
	TaskSemantics map[string]spec.TaskSemantic
	ReportSemantics map[string]spec.ReportSemantic
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
	fn := opts.Functions[op.FunctionID]
	applySelectors(&binding, fn)
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

// GenerateTaskPageForOperation creates an async task candidate. Without task
// status/event semantics it must be reviewed before publication.
func GenerateTaskPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "task", spec.BindingUsageTask, spec.PageExecutionModeTask)
	fn := opts.Functions[op.FunctionID]
	applySelectors(&binding, fn)
	diags := assessBaseCandidate(op)
	taskSemantic, hasTaskSemantic := opts.TaskSemantics[strings.TrimSpace(op.FunctionID)]
	if !hasTaskSemantic {
		diags = append(diags, diagnostic(
			"task_semantics_missing",
			spec.SeverityWarning,
			"task capability requires status/events/result semantics before it can be safely published",
			op.FunctionID,
			"capability",
		))
	}

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
					Cancelable:   hasTaskSemantic && taskSemantic.Cancel != nil,
					Retryable:    hasTaskSemantic && taskSemantic.Retry != nil,
				},
				ResultView: &spec.ResultViewSpec{
					Fields:         buildResultFields(fn.OutputSchema, locale),
					SuccessMessage: spec.LocalizedText{locale: "任务完成"},
					ErrorMessage:   spec.LocalizedText{locale: "任务失败"},
				},
			},
			Bindings: []spec.PageFunctionBinding{binding},
		},
		Quality:     taskQuality(op, diags),
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
	fn := opts.Functions[op.FunctionID]
	applySelectors(&binding, fn)
	diags := assessBaseCandidate(op)
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)
	reportSemantic, hasReportSemantic := opts.ReportSemantics[strings.TrimSpace(op.FunctionID)]
	dataset := buildDatasetSpecFromSemantic(fn.OutputSchema, reportSemantic, locale)
	if dataset == nil {
		dataset = buildDatasetSpec(fn.OutputSchema, locale)
	}
	charts := buildChartSpecs(dataset, title, locale)
	if hasReportSemantic {
		applyReportSemantic(&binding, reportSemantic)
	}
	if !hasReportSemantic || dataset == nil || len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
		diags = append(diags, diagnostic(
			"report_dataset_missing",
			spec.SeverityWarning,
			"report capability requires dataset, dimension, and metric semantics before it can be safely published",
			op.FunctionID,
			"capability",
		))
		dataset = &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{},
			Metrics:    []spec.MetricSpec{},
		}
		charts = nil
	}

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
				QueryForm:  buildFormPresentation(op, opts),
				Dataset:    dataset,
				Charts:     charts,
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

func applyReportSemantic(binding *spec.PageFunctionBinding, reportSemantic spec.ReportSemantic) {
	if binding == nil || strings.TrimSpace(reportSemantic.Query.FunctionID) == "" {
		return
	}
	if binding.Selectors == nil {
		binding.Selectors = &spec.BindingSelectors{}
	}
	binding.Selectors.Output = []spec.OutputAssignment{{
		StateKey: "dataset",
		Source:   strings.TrimSpace(reportSemantic.DatasetPath),
		Shape:    spec.OutputShapeDataset,
	}}
}

func buildResultFields(outputSchema spec.JSONSchema, locale string) []spec.ResultFieldSpec {
	if len(outputSchema) == 0 {
		return nil
	}
	root := parseJSONObject(jsonRaw(outputSchema))
	if schemaTypeFromObject(root) != "object" {
		return []spec.ResultFieldSpec{{
			Key:      "result",
			Title:    spec.LocalizedText{locale: "结果"},
			DataType: dataTypeFromSchema(root),
		}}
	}
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return nil
	}
	keys := sortedRawMapKeys(properties)
	fields := make([]spec.ResultFieldSpec, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, spec.ResultFieldSpec{
			Key:      key,
			Title:    spec.LocalizedText{locale: humanizeKey(key)},
			DataType: dataTypeFromSchema(parseJSONObject(properties[key])),
		})
	}
	return fields
}

func buildDatasetSpec(outputSchema spec.JSONSchema, locale string) *spec.DatasetSpec {
	itemSchema := datasetItemSchema(outputSchema)
	if len(itemSchema) == 0 {
		return nil
	}
	properties := objectProperty(itemSchema, "properties")
	if len(properties) == 0 {
		return nil
	}
	keys := sortedRawMapKeys(properties)
	dataset := &spec.DatasetSpec{
		Dimensions: []spec.DimensionSpec{},
		Metrics:    []spec.MetricSpec{},
	}
	for _, key := range keys {
		prop := parseJSONObject(properties[key])
		switch schemaTypeFromObject(prop) {
		case "integer", "number":
			dataset.Metrics = append(dataset.Metrics, spec.MetricSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: humanizeKey(key)},
				DataType: "number",
				AggType:  "sum",
			})
		case "string":
			dimensionType := "string"
			if rawString(prop["format"]) == "date" || rawString(prop["format"]) == "date-time" {
				dimensionType = "date"
			}
			dataset.Dimensions = append(dataset.Dimensions, spec.DimensionSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: humanizeKey(key)},
				DataType: dimensionType,
			})
		case "boolean":
			dataset.Dimensions = append(dataset.Dimensions, spec.DimensionSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: humanizeKey(key)},
				DataType: "string",
			})
		}
	}
	if len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
		return nil
	}
	return dataset
}

func buildDatasetSpecFromSemantic(outputSchema spec.JSONSchema, report spec.ReportSemantic, locale string) *spec.DatasetSpec {
	if strings.TrimSpace(report.Query.FunctionID) == "" || len(outputSchema) == 0 {
		return nil
	}
	itemSchema := datasetItemSchemaAtPointer(outputSchema, strings.TrimSpace(report.DatasetPath))
	if len(itemSchema) == 0 {
		return nil
	}
	dimensions := buildDimensionsFromPointers(itemSchema, report.Dimensions, locale)
	metrics := buildMetricsFromPointers(itemSchema, report.Metrics, locale)
	if len(dimensions) == 0 || len(metrics) == 0 {
		return nil
	}
	return &spec.DatasetSpec{
		Dimensions: dimensions,
		Metrics:    metrics,
	}
}

func buildDimensionsFromPointers(itemSchema map[string]json.RawMessage, pointers []string, locale string) []spec.DimensionSpec {
	out := make([]spec.DimensionSpec, 0, len(pointers))
	for _, pointer := range compactPointers(pointers) {
		prop, ok := schemaAtPointer(itemSchema, pointer)
		if !ok {
			continue
		}
		key := keyFromPointer(pointer)
		dataType := dataTypeFromSchema(prop)
		if dataType == "datetime" {
			dataType = "date"
		}
		if dataType != "string" && dataType != "number" && dataType != "date" {
			dataType = "string"
		}
		out = append(out, spec.DimensionSpec{
			Key:      key,
			Title:    spec.LocalizedText{locale: humanizeKey(key)},
			DataType: dataType,
		})
	}
	return out
}

func buildMetricsFromPointers(itemSchema map[string]json.RawMessage, pointers []string, locale string) []spec.MetricSpec {
	out := make([]spec.MetricSpec, 0, len(pointers))
	for _, pointer := range compactPointers(pointers) {
		prop, ok := schemaAtPointer(itemSchema, pointer)
		if !ok {
			continue
		}
		if schemaTypeFromObject(prop) != "number" && schemaTypeFromObject(prop) != "integer" {
			continue
		}
		key := keyFromPointer(pointer)
		out = append(out, spec.MetricSpec{
			Key:      key,
			Title:    spec.LocalizedText{locale: humanizeKey(key)},
			DataType: "number",
			AggType:  "sum",
		})
	}
	return out
}

func datasetItemSchemaAtPointer(outputSchema spec.JSONSchema, pointer string) map[string]json.RawMessage {
	root := parseJSONObject(jsonRaw(outputSchema))
	node := root
	if pointer != "" {
		for _, token := range pointerTokens(pointer) {
			properties := objectProperty(node, "properties")
			if len(properties) == 0 {
				return nil
			}
			node = parseJSONObject(properties[token])
			if len(node) == 0 {
				return nil
			}
		}
	}
	if schemaTypeFromObject(node) != "array" {
		return nil
	}
	return objectProperty(node, "items")
}

func schemaAtPointer(root map[string]json.RawMessage, pointer string) (map[string]json.RawMessage, bool) {
	if pointer == "" {
		return root, len(root) > 0
	}
	node := root
	for _, token := range pointerTokens(pointer) {
		properties := objectProperty(node, "properties")
		if len(properties) == 0 {
			return nil, false
		}
		node = parseJSONObject(properties[token])
		if len(node) == 0 {
			return nil, false
		}
	}
	return node, true
}

func compactPointers(pointers []string) []string {
	out := make([]string, 0, len(pointers))
	seen := map[string]struct{}{}
	for _, pointer := range pointers {
		pointer = strings.TrimSpace(pointer)
		if pointer == "" || !strings.HasPrefix(pointer, "/") {
			continue
		}
		if _, ok := seen[pointer]; ok {
			continue
		}
		seen[pointer] = struct{}{}
		out = append(out, pointer)
	}
	return out
}

func pointerTokens(pointer string) []string {
	if pointer == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

func keyFromPointer(pointer string) string {
	tokens := pointerTokens(pointer)
	if len(tokens) == 0 {
		return "value"
	}
	return tokens[len(tokens)-1]
}

func buildChartSpecs(dataset *spec.DatasetSpec, title spec.LocalizedText, locale string) []spec.ChartSpec {
	if dataset == nil || len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
		return nil
	}
	return []spec.ChartSpec{{
		Type:   "line",
		Title:  title,
		XField: dataset.Dimensions[0].Key,
		YField: dataset.Metrics[0].Key,
	}}
}

func datasetItemSchema(outputSchema spec.JSONSchema) map[string]json.RawMessage {
	if len(outputSchema) == 0 {
		return nil
	}
	root := parseJSONObject(jsonRaw(outputSchema))
	if schemaTypeFromObject(root) == "array" {
		return objectProperty(root, "items")
	}
	properties := objectProperty(root, "properties")
	for _, key := range []string{"dataset", "items", "rows", "data"} {
		prop := objectProperty(properties, key)
		if schemaTypeFromObject(prop) != "array" {
			continue
		}
		if items := objectProperty(prop, "items"); len(items) > 0 {
			return items
		}
	}
	return nil
}

func collectionOutputAssignments(outputSchema spec.JSONSchema, arrayKeys []string, stateKey string) []spec.OutputAssignment {
	source := collectionOutputSource(outputSchema, arrayKeys)
	if source == "" && schemaRootType(outputSchema) != "array" {
		return nil
	}
	assignments := []spec.OutputAssignment{{
		StateKey: stateKey,
		Source:   source,
		Shape:    spec.OutputShapeCollection,
	}}
	if totalSource := scalarPropertySource(outputSchema, []string{"total", "count", "total_count"}); totalSource != "" {
		assignments = append(assignments, spec.OutputAssignment{
			StateKey: "total",
			Source:   totalSource,
			Shape:    spec.OutputShapeScalar,
		})
	}
	return assignments
}

func collectionOutputSource(outputSchema spec.JSONSchema, keys []string) string {
	if schemaRootType(outputSchema) == "array" {
		return ""
	}
	root := parseJSONObject(jsonRaw(outputSchema))
	properties := objectProperty(root, "properties")
	for _, key := range keys {
		prop := objectProperty(properties, key)
		if schemaTypeFromObject(prop) == "array" {
			return "/" + escapeJSONPointerToken(key)
		}
	}
	return ""
}

func scalarPropertySource(outputSchema spec.JSONSchema, keys []string) string {
	root := parseJSONObject(jsonRaw(outputSchema))
	properties := objectProperty(root, "properties")
	for _, key := range keys {
		prop := objectProperty(properties, key)
		switch schemaTypeFromObject(prop) {
		case "integer", "number":
			return "/" + escapeJSONPointerToken(key)
		}
	}
	return ""
}

func outputShapeForSchema(outputSchema spec.JSONSchema) spec.OutputResultShape {
	switch schemaRootType(outputSchema) {
	case "array":
		return spec.OutputShapeCollection
	case "object":
		return spec.OutputShapeObject
	default:
		return spec.OutputShapeScalar
	}
}

func schemaRootType(outputSchema spec.JSONSchema) string {
	return schemaTypeFromObject(parseJSONObject(jsonRaw(outputSchema)))
}

func schemaTypeFromObject(obj map[string]json.RawMessage) string {
	return rawString(obj["type"])
}

func jsonRaw(schema spec.JSONSchema) json.RawMessage {
	return json.RawMessage(schema)
}

func sortedRawMapKeys(input map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func dataTypeFromSchema(obj map[string]json.RawMessage) string {
	switch schemaTypeFromObject(obj) {
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	case "string":
		if rawString(obj["format"]) == "date-time" {
			return "datetime"
		}
		if rawString(obj["format"]) == "date" {
			return "date"
		}
		return "string"
	default:
		return "string"
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

// ShouldBlockProposal checks if a proposal should be blocked (cannot be materialized).
// Returns true if the proposal has blocking issues that prevent materialization.
func ShouldBlockProposal(diags []spec.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			// Check for specific blocking conditions
			switch d.Code {
			case "function_id_missing", "function_disabled":
				return true
			}
		}
	}
	return false
}

// CreateBlockedProposalIssue creates a BlockedProposalIssue from diagnostics.
// It only contains diagnostics and repair hints, not a spec.
func CreateBlockedProposalIssue(
	gameID string,
	env string,
	resourceKey string,
	functionID string,
	diags []spec.Diagnostic,
	locale string,
) spec.BlockedProposalIssue {
	// Filter to only error diagnostics
	var errorDiags []spec.Diagnostic
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			errorDiags = append(errorDiags, d)
		}
	}

	// Generate repair hint based on diagnostics
	repairHint := generateRepairHint(errorDiags, locale)

	return spec.BlockedProposalIssue{
		GameID:      gameID,
		Env:         env,
		ResourceKey: resourceKey,
		FunctionID:  functionID,
		Diagnostics: errorDiags,
		RepairHint:  repairHint,
		Status:      "open",
	}
}

// generateRepairHint creates a repair hint from diagnostics.
func generateRepairHint(diags []spec.Diagnostic, locale string) spec.LocalizedText {
	if len(diags) == 0 {
		return spec.LocalizedText{locale: "No issues found"}
	}

	// Use the first error as the primary hint
	primaryDiag := diags[0]
	switch primaryDiag.Code {
	case "function_id_missing":
		return spec.LocalizedText{locale: "Function ID is required. Please register the function first."}
	case "function_disabled":
		return spec.LocalizedText{locale: "Function is disabled. Please enable it before creating a page."}
	default:
		return spec.LocalizedText{locale: primaryDiag.Message}
	}
}
