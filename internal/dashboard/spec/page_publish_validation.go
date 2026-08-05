package spec

import (
	"strconv"
	"strings"
)

// ValidatePublishablePageShape checks product-level requirements that only
// apply when a PageSpec is published into the runtime console.
func ValidatePublishablePageShape(page PageSpec) []Diagnostic {
	diags := validateRequiredPrimaryBinding(page)
	switch page.Type {
	case PageTypeResource:
		diags = append(diags, validatePublishableResourcePage(page.Resource)...)
	case PageTypeReport:
		diags = append(diags, validatePublishableReportPage(page.Report)...)
	default:
		return diags
	}
	return diags
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
