// Package generator creates conservative default PageSpec suggestions from
// normalized function capability contracts. It does not read registration-side
// UI, menu, mapping, pagination, column, task view, or chart view extensions.
package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	DefaultLocale   string
	Functions       map[string]spec.FunctionSpec
	TaskSemantics   map[string]spec.TaskSemantic
	ReportSemantics map[string]spec.ReportSemantic
	// Terms maps "domain/alias" to localized display text sourced from the
	// platform term dictionary. The generator uses it to localize generated
	// category labels and title fallbacks; it never overrides explicit
	// summaries from registration.
	Terms TermDictionary
}

// TermDictionary maps term aliases to localized display labels, keyed by
// "domain/alias" (both lowercased). Domain is "resource" or "operation".
type TermDictionary map[string]spec.LocalizedText

// Lookup resolves a term by domain and alias.
func (t TermDictionary) Lookup(domain, alias string) (spec.LocalizedText, bool) {
	if t == nil {
		return nil, false
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	alias = strings.ToLower(strings.TrimSpace(alias))
	if domain == "" || alias == "" {
		return nil, false
	}
	text, ok := t[domain+"/"+alias]
	return text, ok
}

// DefaultGenerateOptions returns default options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{DefaultLocale: "zh-CN"}
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
			Category:    categoryForOperation(op.FunctionID, locale, opts.Terms),
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

// GenerateTaskPageForOperation creates an async task candidate. Without task
// status/event semantics it must be reviewed before publication.
func GenerateTaskPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	fn := opts.Functions[op.FunctionID]
	diags := assessBaseCandidate(op)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "inputSchema", fn.InputSchema)...)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "outputSchema", fn.OutputSchema)...)
	taskSemantic, hasTaskSemantic := opts.TaskSemantics[strings.TrimSpace(op.FunctionID)]
	startBinding := pageBinding(op, "task", spec.BindingUsageTask, spec.PageExecutionModeTask)
	applySelectors(&startBinding, fn)
	taskView := &spec.TaskViewSpec{
		TaskIDStateKey: "taskId",
	}
	bindings := []spec.PageFunctionBinding{startBinding}
	resultSchema := fn.OutputSchema
	if !hasTaskSemantic {
		diags = append(diags, diagnostic(
			"task_semantics_missing",
			spec.SeverityWarning,
			"task capability requires status/events/result semantics before it can be safely published",
			op.FunctionID,
			"capability",
		))
	} else {
		if !taskSemanticComplete(op, taskSemantic) {
			diags = append(diags, diagnostic(
				"task_semantics_incomplete",
				spec.SeverityWarning,
				"task capability requires start, task ID, status, events, result, and cancel semantics before it can be safely published",
				op.FunctionID,
				"capability",
			))
		}
		startBinding = buildTaskStartBinding(op, fn, taskSemantic)
		bindings[0] = startBinding
		taskView.TaskIDStateKey = "taskId"
		if statusBinding, ok, diag := buildTaskStatusBinding(taskSemantic, opts.Functions, taskView.TaskIDStateKey); ok {
			bindings = append(bindings, statusBinding)
			taskView.StatusBindingID = statusBinding.ID
			taskView.StatusStatePath = strings.TrimSpace(taskSemantic.Status.StatePath)
			taskView.ShowProgress = true
		} else if diag.Code != "" {
			diags = append(diags, diag)
		}
		if eventsBinding, ok, diag := buildTaskEventsBinding(taskSemantic, opts.Functions, taskView.TaskIDStateKey); ok {
			bindings = append(bindings, eventsBinding)
			taskView.EventsBindingID = eventsBinding.ID
			taskView.ShowTimeline = true
			taskView.ShowEvents = true
		} else if diag.Code != "" {
			diags = append(diags, diag)
		}
		if resultBinding, ok, diag := buildTaskResultBinding(taskSemantic, opts.Functions, taskView.TaskIDStateKey); ok {
			bindings = append(bindings, resultBinding)
			taskView.ResultBindingID = resultBinding.ID
			if resultFn, exists := opts.Functions[strings.TrimSpace(taskSemantic.Result.Function.FunctionID)]; exists && len(resultFn.OutputSchema) > 0 {
				resultSchema = resultFn.OutputSchema
			}
		} else if diag.Code != "" {
			diags = append(diags, diag)
		}
		if cancelBinding, ok, diag := buildTaskCancelBinding(taskSemantic, opts.Functions, taskView.TaskIDStateKey); ok {
			bindings = append(bindings, cancelBinding)
			taskView.CancelBindingID = cancelBinding.ID
			taskView.Cancelable = true
		} else if diag.Code != "" {
			diags = append(diags, diag)
		}
	}

	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeTask,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForOperation(op.FunctionID, locale, opts.Terms),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Task: &spec.TaskPageSpec{
				Form:     buildFormPresentation(op, opts),
				TaskView: taskView,
				ResultView: &spec.ResultViewSpec{
					Fields:         buildResultFields(resultSchema, locale),
					SuccessMessage: spec.LocalizedText{locale: "任务完成"},
					ErrorMessage:   spec.LocalizedText{locale: "任务失败"},
				},
			},
			Bindings: bindings,
		},
		Quality:     taskQuality(op, diags),
		Diagnostics: diags,
	}
}

func taskSemanticComplete(op spec.OperationSpec, semantic spec.TaskSemantic) bool {
	if strings.TrimSpace(semantic.Start.FunctionID) != strings.TrimSpace(op.FunctionID) ||
		strings.TrimSpace(semantic.TaskID.ResultPath) == "" ||
		strings.TrimSpace(semantic.Status.Function.FunctionID) == "" ||
		strings.TrimSpace(semantic.Status.TaskIDInput) == "" ||
		strings.TrimSpace(semantic.Status.StatePath) == "" ||
		semantic.Events == nil ||
		strings.TrimSpace(semantic.Events.Function.FunctionID) == "" ||
		strings.TrimSpace(semantic.Events.TaskIDInput) == "" ||
		strings.TrimSpace(semantic.Events.EventsPath) == "" ||
		semantic.Result == nil ||
		strings.TrimSpace(semantic.Result.Function.FunctionID) == "" ||
		strings.TrimSpace(semantic.Result.TaskIDInput) == "" ||
		strings.TrimSpace(semantic.Result.ResultPath) == "" ||
		semantic.Cancel == nil ||
		strings.TrimSpace(semantic.Cancel.Function.FunctionID) == "" ||
		strings.TrimSpace(semantic.Cancel.TaskIDInput) == "" {
		return false
	}
	return true
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
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "inputSchema", fn.InputSchema)...)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "outputSchema", fn.OutputSchema)...)
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)
	reportSemantic, hasReportSemantic := opts.ReportSemantics[strings.TrimSpace(op.FunctionID)]
	validSemantic := hasReportSemantic && reportSemanticComplete(op, reportSemantic)
	var dataset *spec.DatasetSpec
	if validSemantic {
		dataset = buildDatasetSpecFromSemantic(fn.OutputSchema, reportSemantic, locale)
		applyReportSemantic(&binding, reportSemantic)
	}
	charts := buildChartSpecs(dataset, title, locale)
	if !validSemantic || dataset == nil || len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
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
			Category:    categoryForOperation(op.FunctionID, locale, opts.Terms),
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

func reportSemanticComplete(op spec.OperationSpec, semantic spec.ReportSemantic) bool {
	return strings.TrimSpace(semantic.Query.FunctionID) == strings.TrimSpace(op.FunctionID) &&
		strings.TrimSpace(semantic.DatasetPath) != "" &&
		len(compactPointers(semantic.Dimensions)) > 0 &&
		len(compactPointers(semantic.Metrics)) > 0
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

func buildTaskStartBinding(op spec.OperationSpec, fn spec.FunctionSpec, taskSemantic spec.TaskSemantic) spec.PageFunctionBinding {
	binding := pageBinding(op, "task", spec.BindingUsageTask, spec.PageExecutionModeTask)
	applySelectors(&binding, fn)
	taskIDPath := strings.TrimSpace(taskSemantic.TaskID.ResultPath)
	if taskIDPath == "" {
		return binding
	}
	input := spec.SelectorAST{}
	if binding.Selectors != nil {
		input = binding.Selectors.Input
	}
	binding.Selectors = &spec.BindingSelectors{
		Input: input,
		Output: []spec.OutputAssignment{{
			StateKey: "taskId",
			Source:   taskIDPath,
			Shape:    spec.OutputShapeScalar,
		}},
	}
	return binding
}

func buildTaskStatusBinding(taskSemantic spec.TaskSemantic, functions map[string]spec.FunctionSpec, taskStateKey string) (spec.PageFunctionBinding, bool, spec.Diagnostic) {
	fn, ok := functions[strings.TrimSpace(taskSemantic.Status.Function.FunctionID)]
	if !ok {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_status_function_missing",
			spec.SeverityError,
			"task status semantic references a missing function contract",
			taskSemantic.Status.Function.FunctionID,
			"taskView.statusBindingId",
		)
	}
	if strings.TrimSpace(taskSemantic.Status.TaskIDInput) == "" || strings.TrimSpace(taskSemantic.Status.StatePath) == "" {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_status_semantic_invalid",
			spec.SeverityError,
			"task status semantic requires taskIdInput and statePath",
			taskSemantic.Status.Function.FunctionID,
			"taskView.statusBindingId",
		)
	}
	binding := spec.PageFunctionBinding{
		ID:         "status",
		FunctionID: strings.TrimSpace(fn.ID),
		Usage:      spec.BindingUsageTaskStatus,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: strings.TrimSpace(taskSemantic.Status.TaskIDInput),
					Source: spec.ValueSource{Kind: spec.SourcePageState, Key: taskStateKey},
				}},
			},
			Output: []spec.OutputAssignment{{
				StateKey: "taskStatus",
				Source:   "",
				Shape:    spec.OutputShapeObject,
			}},
		},
	}
	return binding, true, spec.Diagnostic{}
}

func buildTaskEventsBinding(taskSemantic spec.TaskSemantic, functions map[string]spec.FunctionSpec, taskStateKey string) (spec.PageFunctionBinding, bool, spec.Diagnostic) {
	if taskSemantic.Events == nil {
		return spec.PageFunctionBinding{}, false, spec.Diagnostic{}
	}
	fn, ok := functions[strings.TrimSpace(taskSemantic.Events.Function.FunctionID)]
	if !ok {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_events_function_missing",
			spec.SeverityWarning,
			"task events semantic references a missing function contract",
			taskSemantic.Events.Function.FunctionID,
			"taskView.eventsBindingId",
		)
	}
	if strings.TrimSpace(taskSemantic.Events.TaskIDInput) == "" || strings.TrimSpace(taskSemantic.Events.EventsPath) == "" {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_events_semantic_invalid",
			spec.SeverityWarning,
			"task events semantic requires taskIdInput and eventsPath",
			taskSemantic.Events.Function.FunctionID,
			"taskView.eventsBindingId",
		)
	}
	binding := spec.PageFunctionBinding{
		ID:         "events",
		FunctionID: strings.TrimSpace(fn.ID),
		Usage:      spec.BindingUsageTaskEvents,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: strings.TrimSpace(taskSemantic.Events.TaskIDInput),
					Source: spec.ValueSource{Kind: spec.SourcePageState, Key: taskStateKey},
				}},
			},
			Output: []spec.OutputAssignment{{
				StateKey: "taskEvents",
				Source:   strings.TrimSpace(taskSemantic.Events.EventsPath),
				Shape:    spec.OutputShapeCollection,
			}},
		},
	}
	return binding, true, spec.Diagnostic{}
}

func buildTaskResultBinding(taskSemantic spec.TaskSemantic, functions map[string]spec.FunctionSpec, taskStateKey string) (spec.PageFunctionBinding, bool, spec.Diagnostic) {
	if taskSemantic.Result == nil {
		return spec.PageFunctionBinding{}, false, spec.Diagnostic{}
	}
	fn, ok := functions[strings.TrimSpace(taskSemantic.Result.Function.FunctionID)]
	if !ok {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_result_function_missing",
			spec.SeverityWarning,
			"task result semantic references a missing function contract",
			taskSemantic.Result.Function.FunctionID,
			"taskView.resultBindingId",
		)
	}
	if strings.TrimSpace(taskSemantic.Result.TaskIDInput) == "" || strings.TrimSpace(taskSemantic.Result.ResultPath) == "" {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_result_semantic_invalid",
			spec.SeverityWarning,
			"task result semantic requires taskIdInput and resultPath",
			taskSemantic.Result.Function.FunctionID,
			"taskView.resultBindingId",
		)
	}
	binding := spec.PageFunctionBinding{
		ID:         "result",
		FunctionID: strings.TrimSpace(fn.ID),
		Usage:      spec.BindingUsageTaskResult,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: strings.TrimSpace(taskSemantic.Result.TaskIDInput),
					Source: spec.ValueSource{Kind: spec.SourcePageState, Key: taskStateKey},
				}},
			},
			Output: []spec.OutputAssignment{{
				StateKey: "taskResult",
				Source:   strings.TrimSpace(taskSemantic.Result.ResultPath),
				Shape:    taskOutputShapeForPointer(fn.OutputSchema, taskSemantic.Result.ResultPath),
			}},
		},
	}
	return binding, true, spec.Diagnostic{}
}

func buildTaskCancelBinding(taskSemantic spec.TaskSemantic, functions map[string]spec.FunctionSpec, taskStateKey string) (spec.PageFunctionBinding, bool, spec.Diagnostic) {
	if taskSemantic.Cancel == nil {
		return spec.PageFunctionBinding{}, false, spec.Diagnostic{}
	}
	fn, ok := functions[strings.TrimSpace(taskSemantic.Cancel.Function.FunctionID)]
	if !ok {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_cancel_function_missing",
			spec.SeverityWarning,
			"task cancel semantic references a missing function contract",
			taskSemantic.Cancel.Function.FunctionID,
			"taskView.cancelBindingId",
		)
	}
	if strings.TrimSpace(taskSemantic.Cancel.TaskIDInput) == "" {
		return spec.PageFunctionBinding{}, false, diagnostic(
			"task_cancel_semantic_invalid",
			spec.SeverityWarning,
			"task cancel semantic requires taskIdInput",
			taskSemantic.Cancel.Function.FunctionID,
			"taskView.cancelBindingId",
		)
	}
	binding := spec.PageFunctionBinding{
		ID:         "cancel",
		FunctionID: strings.TrimSpace(fn.ID),
		Usage:      spec.BindingUsageTaskCancel,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: strings.TrimSpace(taskSemantic.Cancel.TaskIDInput),
					Source: spec.ValueSource{Kind: spec.SourcePageState, Key: taskStateKey},
				}},
			},
		},
	}
	return binding, true, spec.Diagnostic{}
}

func taskOutputShapeForPointer(outputSchema spec.JSONSchema, pointer string) spec.OutputResultShape {
	root := parseJSONObject(jsonRaw(outputSchema))
	if strings.TrimSpace(pointer) == "" {
		return outputShapeForSchema(outputSchema)
	}
	node, ok := schemaAtPointer(root, strings.TrimSpace(pointer))
	if !ok {
		return spec.OutputShapeScalar
	}
	switch schemaTypeFromObject(node) {
	case "array":
		return spec.OutputShapeCollection
	case "object":
		return spec.OutputShapeObject
	default:
		return spec.OutputShapeScalar
	}
}

func buildFormPresentation(op spec.OperationSpec, opts GenerateOptions) *spec.FormPresentationSpec {
	fn, ok := opts.Functions[op.FunctionID]
	if !ok || len(fn.InputSchema) == 0 {
		return spec.DefaultFormPresentation(spec.JSONSchema(`{"type":"object","properties":{}}`))
	}
	fp := spec.DefaultFormPresentation(fn.InputSchema)
	// 自动生成字段展示信息（如果 schema 中没有 title）
	fp.Fields = buildFormFields(fn.InputSchema, opts.DefaultLocale)
	return fp
}

// buildFormFields 从 JSON Schema 自动生成 FormFieldSpec 列表。
// 每个顶层字段都产出条目（保证 ui:order 完整，与前端 derivePresentationSpec
// 一致）：x-ui-* hints 优先，其次按类型推导缺省控件；label 兜底链
// x-label > schema.title > humanize。
func buildFormFields(schema spec.JSONSchema, locale string) []spec.FormFieldSpec {
	if len(schema) == 0 {
		return nil
	}
	root := parseJSONObject(jsonRaw(schema))
	if schemaTypeFromObject(root) != "object" {
		return nil
	}
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return nil
	}
	keys := sortedRawMapKeys(properties)
	fields := make([]spec.FormFieldSpec, 0, len(keys))
	for _, key := range keys {
		prop := parseJSONObject(properties[key])
		field := spec.FormFieldSpec{Key: key}

		if widget := hintWidget(rawString(prop["x-widget"])); widget != "" {
			field.Widget = widget
		}
		if label := hintLocalized(prop["x-label"]); label != nil {
			field.Label = label
		}
		if placeholder := hintLocalized(prop["x-placeholder"]); placeholder != nil {
			field.Placeholder = placeholder
		}
		if description := hintLocalized(prop["x-description"]); description != nil {
			field.Description = description
		}
		if width := rawInt(prop["x-width"]); width >= 1 && width <= 12 {
			field.Width = width
		}
		if _, ok := prop["x-order"]; ok {
			field.Order = rawInt(prop["x-order"])
		}
		if disabled := rawBool(prop["x-disabled"]); disabled != nil {
			field.Disabled = disabled
		}
		if condition := hintCondition(prop["x-visible-when"]); condition != nil {
			field.VisibleWhen = condition
		}
		if enumOptions := hintEnumOptions(prop["x-enum-options"]); len(enumOptions) > 0 {
			field.EnumOptions = enumOptions
		}
		if widgetProps := hintWidgetProps(prop["x-widget-props"]); len(widgetProps) > 0 {
			field.WidgetProps = widgetProps
		}

		// x-widget 缺省时按类型推导缺省控件
		if field.Widget == "" {
			field.Widget = defaultWidgetForSchema(prop)
		}

		// label 兜底：x-label 缺省且 schema 无 title 时用 humanize，
		// 有 title 时不重复下发（渲染端直接读 schema.title）
		if field.Label == nil && rawString(prop["title"]) == "" {
			field.Label = spec.LocalizedText{
				locale: fallbackLabel(key),
			}
		}
		// string 输入的 placeholder 兜底（与历史行为一致）
		if field.Placeholder == nil && schemaTypeFromObject(prop) == "string" &&
			(field.Widget == "" || field.Widget == spec.FormWidgetInput) {
			field.Placeholder = spec.LocalizedText{
				locale: "请输入" + fallbackLabel(key),
			}
		}
		fields = append(fields, field)
	}
	// x-order 排序：未声明者按 key 序排后（与前端 MAX_SAFE_INTEGER 兜底一致）
	sort.SliceStable(fields, func(i, j int) bool {
		return orderSortKey(fields[i]) < orderSortKey(fields[j])
	})
	return fields
}

func orderSortKey(field spec.FormFieldSpec) int {
	if field.Order != 0 {
		return field.Order
	}
	return int(^uint(0) >> 1)
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

func categoryForResource(resourceKey string, locale string, terms TermDictionary) spec.PageCategorySpec {
	categoryKey := InferCategoryFromKey(resourceKey)
	return spec.PageCategorySpec{
		Key:    categoryKey,
		Labels: localizedKeyLabels(categoryKey, locale, "resource", terms),
	}
}

func categoryForOperation(functionID string, locale string, terms TermDictionary) spec.PageCategorySpec {
	categoryKey := InferCategoryFromKey(functionID)
	return spec.PageCategorySpec{
		Key:    categoryKey,
		Labels: localizedKeyLabels(categoryKey, locale, "resource", terms),
	}
}

// localizedKeyLabels resolves a key through the term dictionary first and
// falls back to humanizing the raw key in the system default locale.
func localizedKeyLabels(key string, locale string, domain string, terms TermDictionary) spec.LocalizedText {
	if text, ok := terms.Lookup(domain, key); ok && len(text) > 0 {
		return text
	}
	return spec.LocalizedText{locale: fallbackLabel(key)}
}

func localizedTitle(op spec.OperationSpec, pageKey string, locale string, opts GenerateOptions) spec.LocalizedText {
	if fn, ok := opts.Functions[op.FunctionID]; ok {
		if summary := strings.TrimSpace(fn.Summary[locale]); summary != "" {
			return spec.LocalizedText{locale: summary}
		}
	}
	fallbackKey := firstNonEmpty(op.Operation, op.FunctionID, pageKey)
	if text, ok := opts.Terms.Lookup("operation", op.Operation); ok && len(text) > 0 {
		return text
	}
	if text, ok := opts.Terms.Lookup("resource", fallbackKey); ok && len(text) > 0 {
		return text
	}
	return spec.LocalizedText{
		locale: fallbackLabel(fallbackKey),
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
	if hasErrorDiagnostic(diags) {
		return spec.GeneratedPageQualityNeedsReview
	}
	if hasDiagnosticCode(diags, "json_schema_generation_subset_unsupported") {
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
			Title:    spec.LocalizedText{locale: fallbackLabel(key)},
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
				Title:    spec.LocalizedText{locale: fallbackLabel(key)},
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
				Title:    spec.LocalizedText{locale: fallbackLabel(key)},
				DataType: dimensionType,
			})
		case "boolean":
			dataset.Dimensions = append(dataset.Dimensions, spec.DimensionSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: fallbackLabel(key)},
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
			Title:    spec.LocalizedText{locale: fallbackLabel(key)},
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
			Title:    spec.LocalizedText{locale: fallbackLabel(key)},
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

func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return value
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

func fallbackLabel(value string) string {
	return HumanizeKey(value)
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
