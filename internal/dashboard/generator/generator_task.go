package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

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
			Category:    categoryForOperation(op.FunctionID),
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
