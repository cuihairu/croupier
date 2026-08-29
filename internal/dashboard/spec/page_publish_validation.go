package spec

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidatePublishablePageShape checks product-level requirements that only
// apply when a PageSpec is published into the runtime console.
func ValidatePublishablePageShape(page PageSpec) []Diagnostic {
	diags := validatePageVariant(page)
	if len(diags) > 0 {
		return diags
	}
	diags = append(diags, validateRequiredPrimaryBinding(page)...)
	switch page.Type {
	case PageTypeResource:
		diags = append(diags, validatePublishableResourcePage(page.Resource)...)
	case PageTypeOperation:
		diags = append(diags, validatePublishableOperationPage(page.Operation, page.Bindings)...)
	case PageTypeTask:
		diags = append(diags, validatePublishableTaskPage(page.Task, page.Bindings)...)
	case PageTypeReport:
		diags = append(diags, validatePublishableReportPage(page.Report)...)
	case PageTypeComposite:
		diags = append(diags, validatePublishableCompositePage(page.Composite)...)
	default:
		return diags
	}
	return diags
}

// validatePublishableCompositePage 校验组合页：至少一个区块，binding
// 引用有效、view 形态合法、span 在界内。
func validatePublishableCompositePage(comp *CompositePageSpec) []Diagnostic {
	var diags []Diagnostic
	if comp == nil || len(comp.Sections) == 0 {
		return []Diagnostic{publishShapeDiagnostic("composite_empty", "composite page must contain at least one section", "composite.sections")}
	}
	validViews := map[string]bool{"table": true, "fields": true, "form": true, "actions": true}
	for i, sec := range comp.Sections {
		field := fmt.Sprintf("composite.sections[%d]", i)
		if strings.TrimSpace(sec.Key) == "" {
			diags = append(diags, publishShapeDiagnostic("composite_section_key_missing", "section key is required", field+".key"))
		}
		if strings.TrimSpace(sec.BindingID) == "" {
			diags = append(diags, publishShapeDiagnostic("composite_section_binding_missing", "section bindingId is required", field+".bindingId"))
		}
		if !validViews[sec.View] {
			diags = append(diags, publishShapeDiagnostic("composite_section_view_invalid", "section view must be table|fields|form|actions", field+".view"))
		}
		if sec.Span < 0 || sec.Span > 24 {
			diags = append(diags, publishShapeDiagnostic("composite_section_span_invalid", "section span must be within 0-24", field+".span"))
		}
		if sec.View == "table" && sec.Table == nil {
			diags = append(diags, publishShapeDiagnostic("composite_section_table_missing", "table section requires table config", field+".table"))
		}
	}
	return diags
}

// validatePageVariant keeps the JSON DTO aligned with the frontend
// discriminated union: exactly one page body must match page.type.
func validatePageVariant(page PageSpec) []Diagnostic {
	variantCount := 0
	if page.Resource != nil {
		variantCount++
	}
	if page.Operation != nil {
		variantCount++
	}
	if page.Task != nil {
		variantCount++
	}
	if page.Report != nil {
		variantCount++
	}
	if page.Composite != nil {
		variantCount++
	}
	if variantCount != 1 {
		return []Diagnostic{publishShapeDiagnostic("page_variant_invalid", "page must contain exactly one page body matching type", "type")}
	}

	switch page.Type {
	case PageTypeResource:
		if page.Resource != nil {
			return nil
		}
	case PageTypeOperation:
		if page.Operation != nil {
			return nil
		}
	case PageTypeTask:
		if page.Task != nil {
			return nil
		}
	case PageTypeReport:
		if page.Report != nil {
			return nil
		}
	case PageTypeComposite:
		if page.Composite != nil {
			return nil
		}
	default:
		return []Diagnostic{publishShapeDiagnostic("page_type_invalid", "page type must be resource, operation, task, report or composite", "type")}
	}
	return []Diagnostic{publishShapeDiagnostic("page_variant_type_mismatch", "page body must match page type", "type")}
}

func validateRequiredPrimaryBinding(page PageSpec) []Diagnostic {
	switch page.Type {
	case PageTypeResource:
		return requireBindingUsage(page.Bindings, BindingUsageQuery, "resource page requires a query binding before publish")
	case PageTypeOperation:
		return requireBindingUsage(page.Bindings, BindingUsageAction, "operation page requires an action binding before publish")
	case PageTypeTask:
		return requireBindingUsage(page.Bindings, BindingUsageTask, "task page requires a task binding before publish")
	case PageTypeReport:
		return requireBindingUsage(page.Bindings, BindingUsageReport, "report page requires a report binding before publish")
	default:
		return nil
	}
}

func requireBindingUsage(bindings []PageFunctionBinding, usage PageBindingUsage, message string) []Diagnostic {
	for _, binding := range bindings {
		if binding.Usage == usage && strings.TrimSpace(binding.ID) != "" {
			return nil
		}
	}
	return []Diagnostic{publishShapeDiagnostic("page_primary_binding_missing", message, "bindings")}
}

func validatePublishableResourcePage(resource *ResourcePageSpec) []Diagnostic {
	if resource == nil || resource.ListView == nil {
		return nil
	}
	identityKey := strings.TrimSpace(resource.ListView.IdentityKey)
	if identityKey == "" {
		return []Diagnostic{publishShapeDiagnostic("resource_identity_key_missing", "resource listView.identityKey is required before publish", "resource.listView.identityKey")}
	}
	for _, column := range resource.ListView.Columns {
		if strings.TrimSpace(column.Key) == identityKey {
			return nil
		}
	}
	return []Diagnostic{publishShapeDiagnostic("resource_identity_key_invalid", "resource listView.identityKey must reference a list column", "resource.listView.identityKey")}
}

func validatePublishableOperationPage(operation *OperationPageSpec, bindings []PageFunctionBinding) []Diagnostic {
	if operation == nil {
		return nil
	}
	diags := validatePublishableResultView(operation.ResultView, "operation.resultView")
	if operation.Confirm != nil && !hasBindingUsage(bindings, operation.Confirm.BindingID, BindingUsageAction) {
		diags = append(diags, publishShapeDiagnostic(
			"operation_confirm_binding_invalid",
			"operation.confirm.bindingId must reference an action binding",
			"operation.confirm.bindingId",
		))
	}
	return diags
}

func validatePublishableTaskPage(task *TaskPageSpec, bindings []PageFunctionBinding) []Diagnostic {
	if task == nil {
		return nil
	}
	var diags []Diagnostic
	if task.Form == nil {
		diags = append(diags, publishShapeDiagnostic("task_form_missing", "task.form is required before publish", "task.form"))
	}
	if task.TaskView == nil {
		diags = append(diags, publishShapeDiagnostic("task_view_missing", "task.taskView is required before publish", "task.taskView"))
	} else {
		if strings.TrimSpace(task.TaskView.TaskIDStateKey) == "" {
			diags = append(diags, publishShapeDiagnostic("task_id_state_key_missing", "task.taskView.taskIdStateKey is required before publish", "task.taskView.taskIdStateKey"))
		}
		if strings.TrimSpace(task.TaskView.StatusBindingID) == "" {
			diags = append(diags, publishShapeDiagnostic("task_status_binding_missing", "task.taskView.statusBindingId is required before publish", "task.taskView.statusBindingId"))
		} else if !hasBindingUsage(bindings, task.TaskView.StatusBindingID, BindingUsageTaskStatus) {
			diags = append(diags, publishShapeDiagnostic("task_status_binding_invalid", "task.taskView.statusBindingId must reference a task_status binding", "task.taskView.statusBindingId"))
		}
		if strings.TrimSpace(task.TaskView.StatusStatePath) == "" {
			diags = append(diags, publishShapeDiagnostic("task_status_state_path_missing", "task.taskView.statusStatePath is required before publish", "task.taskView.statusStatePath"))
		} else if !isJSONPointer(task.TaskView.StatusStatePath) {
			diags = append(diags, publishShapeDiagnostic("task_status_state_path_invalid", "task.taskView.statusStatePath must be a JSON Pointer", "task.taskView.statusStatePath"))
		}
		if task.TaskView.ShowEvents && strings.TrimSpace(task.TaskView.EventsBindingID) == "" {
			diags = append(diags, publishShapeDiagnostic("task_events_binding_missing", "task events require task.taskView.eventsBindingId before publish", "task.taskView.eventsBindingId"))
		}
		if strings.TrimSpace(task.TaskView.EventsBindingID) != "" && !hasBindingUsage(bindings, task.TaskView.EventsBindingID, BindingUsageTaskEvents) {
			diags = append(diags, publishShapeDiagnostic("task_events_binding_invalid", "task.taskView.eventsBindingId must reference a task_events binding", "task.taskView.eventsBindingId"))
		}
		if task.TaskView.Cancelable && strings.TrimSpace(task.TaskView.CancelBindingID) == "" {
			diags = append(diags, publishShapeDiagnostic("task_cancel_binding_missing", "cancelable task page requires task.taskView.cancelBindingId before publish", "task.taskView.cancelBindingId"))
		}
		if strings.TrimSpace(task.TaskView.CancelBindingID) != "" && !hasBindingUsage(bindings, task.TaskView.CancelBindingID, BindingUsageTaskCancel) {
			diags = append(diags, publishShapeDiagnostic("task_cancel_binding_invalid", "task.taskView.cancelBindingId must reference a task_cancel binding", "task.taskView.cancelBindingId"))
		}
		if strings.TrimSpace(task.TaskView.ResultBindingID) != "" && !hasBindingUsage(bindings, task.TaskView.ResultBindingID, BindingUsageTaskResult) {
			diags = append(diags, publishShapeDiagnostic("task_result_binding_invalid", "task.taskView.resultBindingId must reference a task_result binding", "task.taskView.resultBindingId"))
		}
		if task.TaskView.Retryable || strings.TrimSpace(task.TaskView.RetryBindingID) != "" {
			diags = append(diags, publishShapeDiagnostic("task_retry_unavailable", "task retry runtime is not available", "task.taskView.retryable"))
		}
	}
	diags = append(diags, validatePublishableResultView(task.ResultView, "task.resultView")...)
	return diags
}

func hasBindingUsage(bindings []PageFunctionBinding, bindingID string, usage PageBindingUsage) bool {
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return false
	}
	for _, binding := range bindings {
		if strings.TrimSpace(binding.ID) == bindingID && binding.Usage == usage {
			return true
		}
	}
	return false
}

func validatePublishableResultView(resultView *ResultViewSpec, field string) []Diagnostic {
	if resultView == nil || len(resultView.Fields) == 0 {
		return nil
	}
	var diags []Diagnostic
	seen := map[string]struct{}{}
	for i, resultField := range resultView.Fields {
		itemField := field + ".fields[" + strconv.Itoa(i) + "]"
		key := strings.TrimSpace(resultField.Key)
		if key == "" {
			diags = append(diags, publishShapeDiagnostic("result_field_key_missing", "result field key is required", itemField+".key"))
			continue
		}
		if _, ok := seen[key]; ok {
			diags = append(diags, publishShapeDiagnostic("result_field_key_duplicate", "result field key must be unique", itemField+".key"))
		}
		seen[key] = struct{}{}
		if !hasDefaultLocale(resultField.Title) {
			diags = append(diags, publishShapeDiagnostic("result_field_title_missing", "result field title must include zh-CN locale", itemField+".title"))
		}
		if strings.TrimSpace(resultField.DataType) == "" {
			diags = append(diags, publishShapeDiagnostic("result_field_type_missing", "result field dataType is required", itemField+".dataType"))
		}
	}
	return diags
}

func validatePublishableReportPage(report *ReportPageSpec) []Diagnostic {
	if report == nil {
		return nil
	}
	var diags []Diagnostic
	if report.QueryForm == nil {
		diags = append(diags, publishShapeDiagnostic("report_query_form_missing", "report.queryForm is required before publish", "report.queryForm"))
	}
	if report.Dataset == nil {
		diags = append(diags, publishShapeDiagnostic("report_dataset_missing", "report.dataset is required before publish", "report.dataset"))
		return diags
	}
	if len(report.Dataset.Dimensions) == 0 {
		diags = append(diags, publishShapeDiagnostic("report_dimensions_missing", "report.dataset.dimensions is required before publish", "report.dataset.dimensions"))
	}
	if len(report.Dataset.Metrics) == 0 {
		diags = append(diags, publishShapeDiagnostic("report_metrics_missing", "report.dataset.metrics is required before publish", "report.dataset.metrics"))
	}
	for i, dim := range report.Dataset.Dimensions {
		field := "report.dataset.dimensions[" + strconv.Itoa(i) + "]"
		if strings.TrimSpace(dim.Key) == "" {
			diags = append(diags, publishShapeDiagnostic("report_dimension_key_missing", "report dataset dimension key is required", field+".key"))
		}
		if !hasDefaultLocale(dim.Title) {
			diags = append(diags, publishShapeDiagnostic("report_dimension_title_missing", "report dataset dimension title must include zh-CN locale", field+".title"))
		}
		if !isReportDimensionType(dim.DataType) {
			diags = append(diags, publishShapeDiagnostic("report_dimension_type_invalid", "report dataset dimension dataType must be string, number, or date", field+".dataType"))
		}
	}
	for i, metric := range report.Dataset.Metrics {
		field := "report.dataset.metrics[" + strconv.Itoa(i) + "]"
		if strings.TrimSpace(metric.Key) == "" {
			diags = append(diags, publishShapeDiagnostic("report_metric_key_missing", "report dataset metric key is required", field+".key"))
		}
		if !hasDefaultLocale(metric.Title) {
			diags = append(diags, publishShapeDiagnostic("report_metric_title_missing", "report dataset metric title must include zh-CN locale", field+".title"))
		}
		if strings.TrimSpace(metric.DataType) != "number" {
			diags = append(diags, publishShapeDiagnostic("report_metric_type_invalid", "report dataset metric dataType must be number", field+".dataType"))
		}
	}
	return diags
}

// ValidateRequiredOutputAssignments checks renderer-required page_state outputs.
// The renderer is intentionally deterministic and does not infer arbitrary raw
// result shapes at runtime.
func ValidateRequiredOutputAssignments(binding PageFunctionBinding, page PageSpec) []Diagnostic {
	switch {
	case page.Type == PageTypeResource && binding.Usage == BindingUsageQuery:
		return validateOutputStateKey(binding, "items", OutputShapeCollection, "resource list query must map collection output to pageState.items")
	case page.Type == PageTypeResource && binding.Usage == BindingUsageDetail:
		return validateOutputStateKey(binding, "detail", OutputShapeObject, "resource detail query must map object output to pageState.detail")
	case page.Type == PageTypeReport && binding.Usage == BindingUsageReport:
		return validateOutputStateKey(binding, "dataset", OutputShapeDataset, "report query must map dataset output to pageState.dataset")
	case page.Type == PageTypeTask && binding.Usage == BindingUsageTaskStatus:
		return validateOutputStateKey(binding, "taskStatus", OutputShapeObject, "task status must map status output to pageState.taskStatus")
	case page.Type == PageTypeTask && binding.Usage == BindingUsageTaskEvents:
		return validateOutputStateKey(binding, "taskEvents", OutputShapeCollection, "task events must map events output to pageState.taskEvents")
	case page.Type == PageTypeTask && binding.Usage == BindingUsageTaskResult:
		return validateOutputStateKeyAnyShape(binding, "taskResult", "task result must map result output to pageState.taskResult")
	default:
		return nil
	}
}

func validateOutputStateKey(binding PageFunctionBinding, stateKey string, shape OutputResultShape, message string) []Diagnostic {
	if binding.Selectors == nil || len(binding.Selectors.Output) == 0 {
		return []Diagnostic{publishShapeDiagnostic("binding_output_selector_missing", message, "bindings."+binding.ID+".selectors.output")}
	}
	for _, assignment := range binding.Selectors.Output {
		if strings.TrimSpace(assignment.StateKey) == stateKey && assignment.Shape == shape {
			return nil
		}
	}
	return []Diagnostic{publishShapeDiagnostic("binding_output_selector_invalid", message, "bindings."+binding.ID+".selectors.output")}
}

func validateOutputStateKeyAnyShape(binding PageFunctionBinding, stateKey string, message string) []Diagnostic {
	if binding.Selectors == nil || len(binding.Selectors.Output) == 0 {
		return []Diagnostic{publishShapeDiagnostic("binding_output_selector_missing", message, "bindings."+binding.ID+".selectors.output")}
	}
	for _, assignment := range binding.Selectors.Output {
		if strings.TrimSpace(assignment.StateKey) == stateKey {
			return nil
		}
	}
	return []Diagnostic{publishShapeDiagnostic("binding_output_selector_invalid", message, "bindings."+binding.ID+".selectors.output")}
}

func publishShapeDiagnostic(code string, message string, field string) Diagnostic {
	return Diagnostic{Code: code, Severity: SeverityError, Message: message, Field: field}
}

func hasDefaultLocale(labels LocalizedText) bool {
	return labels != nil && strings.TrimSpace(labels["zh-CN"]) != ""
}

func isReportDimensionType(dataType string) bool {
	switch strings.TrimSpace(dataType) {
	case "string", "number", "date":
		return true
	default:
		return false
	}
}
