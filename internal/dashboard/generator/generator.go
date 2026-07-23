// Package generator creates default PageSpec suggestions from normalized
// ResourceSpec and OperationSpec. The generator does NOT publish pages;
// it only produces suggestions that enter the Page workspace for user
// confirmation and editing.
//
// The generator produces four page types:
//   - Entity Page: object lifecycle management (list/detail/actions)
//   - Operation Page: standalone synchronous action
//   - Task Page: async/batch task with progress tracking
//   - Report Page: analytics query with charts/tables
//
// Each generated page carries a quality indicator:
//   - "ready": all required fields present, can be published
//   - "needs_review": some fields missing or inferred, user should review
//   - "blocked": critical fields missing, cannot be published
package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	// DefaultLocale is the default locale for labels.
	DefaultLocale string

	// PageKeyPrefix is prepended to generated page keys.
	// e.g., "auto." -> "auto.player.manage"
	PageKeyPrefix string
}

// DefaultGenerateOptions returns default options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		DefaultLocale: "zh-CN",
		PageKeyPrefix: "",
	}
}

// GenerateForResource generates default PageSpec suggestions for a resource.
// It examines the operations available and produces appropriate page types.
func GenerateForResource(resource spec.ResourceSpec, opts GenerateOptions) []spec.GeneratedPageSpec {
	if len(resource.Operations) == 0 {
		return nil
	}

	// Classify operations by kind
	classified := classifyOperations(resource.Operations)

	var pages []spec.GeneratedPageSpec

	// 1. Entity Page: if we have list/get operations
	if len(classified.list) > 0 || len(classified.get) > 0 {
		page := generateEntityPage(resource, classified, opts)
		pages = append(pages, page)
	}

	// 2. Operation Page: standalone actions not tied to entity lifecycle
	for _, op := range classified.standaloneActions {
		page := generateOperationPage(resource, op, opts)
		pages = append(pages, page)
	}

	// 3. Task Page: async/batch tasks
	for _, op := range classified.tasks {
		page := generateTaskPage(resource, op, opts)
		pages = append(pages, page)
	}

	// 4. Report Page: analytics/report queries
	for _, op := range classified.reports {
		page := generateReportPage(resource, op, opts)
		pages = append(pages, page)
	}

	return pages
}

// GenerateForOperation generates a default page for a single operation.
func GenerateForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	switch op.Kind {
	case spec.OperationKindTask:
		return generateTaskPage(spec.ResourceSpec{Key: op.ResourceKey}, op, opts)
	case spec.OperationKindReport:
		return generateReportPage(spec.ResourceSpec{Key: op.ResourceKey}, op, opts)
	case spec.OperationKindAction:
		if op.Placement == spec.PlacementStandalone {
			return generateOperationPage(spec.ResourceSpec{Key: op.ResourceKey}, op, opts)
		}
	}

	// Default: generate a simple operation page
	return generateOperationPage(spec.ResourceSpec{Key: op.ResourceKey}, op, opts)
}

// ---------------------------------------------------------------------------
// Internal: Operation classification
// ---------------------------------------------------------------------------

type classifiedOps struct {
	list              []spec.OperationSpec
	get               []spec.OperationSpec
	create            []spec.OperationSpec
	update            []spec.OperationSpec
	delete            []spec.OperationSpec
	rowActions        []spec.OperationSpec
	detailActions     []spec.OperationSpec
	toolbarActions    []spec.OperationSpec
	batchActions      []spec.OperationSpec
	standaloneActions []spec.OperationSpec
	tasks             []spec.OperationSpec
	reports           []spec.OperationSpec
	query             []spec.OperationSpec
}

func classifyOperations(ops []spec.OperationSpec) classifiedOps {
	var c classifiedOps
	for _, op := range ops {
		switch op.Kind {
		case spec.OperationKindList:
			c.list = append(c.list, op)
		case spec.OperationKindGet:
			c.get = append(c.get, op)
		case spec.OperationKindCreate:
			c.create = append(c.create, op)
		case spec.OperationKindUpdate:
			c.update = append(c.update, op)
		case spec.OperationKindDelete:
			c.delete = append(c.delete, op)
		case spec.OperationKindAction:
			switch op.Placement {
			case spec.PlacementRowAction:
				c.rowActions = append(c.rowActions, op)
			case spec.PlacementDetailAction:
				c.detailActions = append(c.detailActions, op)
			case spec.PlacementToolbarAction:
				c.toolbarActions = append(c.toolbarActions, op)
			case spec.PlacementBatchAction:
				c.batchActions = append(c.batchActions, op)
			case spec.PlacementStandalone:
				c.standaloneActions = append(c.standaloneActions, op)
			default:
				c.standaloneActions = append(c.standaloneActions, op)
			}
		case spec.OperationKindTask:
			c.tasks = append(c.tasks, op)
		case spec.OperationKindReport:
			c.reports = append(c.reports, op)
		}

		// Also classify by placement for query
		if op.Placement == spec.PlacementQuery {
			c.query = append(c.query, op)
		}
	}
	return c
}

// ---------------------------------------------------------------------------
// Internal: Entity Page generation
// ---------------------------------------------------------------------------

func generateEntityPage(resource spec.ResourceSpec, ops classifiedOps, opts GenerateOptions) spec.GeneratedPageSpec {
	pageKey := opts.PageKeyPrefix + resource.Key + ".manage"
	locale := opts.DefaultLocale

	// Build title
	title := spec.LocalizedText{
		locale: getLocalizedText(resource.Labels, locale, resource.Key) + "管理",
	}

	// Build Formily schema
	schema := buildEntityPageSchema(resource, ops, locale)

	// Build bindings
	var bindings []spec.PageFunctionBinding
	for _, op := range ops.list {
		bindings = append(bindings, spec.PageFunctionBinding{
			FunctionID: op.FunctionID,
			Role:       spec.PlacementTableData,
		})
	}
	for _, op := range ops.get {
		bindings = append(bindings, spec.PageFunctionBinding{
			FunctionID: op.FunctionID,
			Role:       spec.PlacementDetailData,
		})
	}
	for _, op := range ops.rowActions {
		bindings = append(bindings, spec.PageFunctionBinding{
			FunctionID: op.FunctionID,
			Role:       spec.PlacementRowAction,
		})
	}
	for _, op := range ops.toolbarActions {
		bindings = append(bindings, spec.PageFunctionBinding{
			FunctionID: op.FunctionID,
			Role:       spec.PlacementToolbarAction,
		})
	}

	// Assess quality
	quality, diags := assessEntityPageQuality(resource, ops)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeEntity,
			ResourceKey: resource.Key,
			Title:       title,
			Category: spec.PageCategorySpec{
				Key:    resource.Category.Key,
				Labels: resource.Category.Labels,
				Order:  resource.Category.Order,
			},
			Schema:   schema,
			Bindings: bindings,
		},
		Quality:     quality,
		Diagnostics: diags,
	}
}

func buildEntityPageSchema(resource spec.ResourceSpec, ops classifiedOps, locale string) spec.FormilySchema {
	schema := map[string]interface{}{
		"type":        "void",
		"x-component": "ConsolePage",
		"x-component-props": map[string]interface{}{
			"resourceKey": resource.Key,
		},
		"properties": map[string]interface{}{},
	}

	props := schema["properties"].(map[string]interface{})

	// Add QueryForm if we have list operations
	if len(ops.list) > 0 {
		queryOp := ops.list[0]
		props["query"] = map[string]interface{}{
			"type":        "void",
			"x-component": "QueryForm",
			"x-component-props": map[string]interface{}{
				"functionId": queryOp.FunctionID,
			},
		}
	}

	// Add DataTable if we have list operations
	if len(ops.list) > 0 {
		listOp := ops.list[0]
		props["table"] = map[string]interface{}{
			"type":        "void",
			"x-component": "DataTable",
			"x-component-props": map[string]interface{}{
				"queryFunctionId": listOp.FunctionID,
				"pagination": map[string]interface{}{
					"pageField":     "page",
					"pageSizeField": "pageSize",
					"totalField":    "$.response.total",
					"itemsField":    "$.response.items",
				},
			},
		}

		// Add row actions
		if len(ops.rowActions) > 0 {
			actions := make([]interface{}, 0, len(ops.rowActions))
			for _, op := range ops.rowActions {
				actions = append(actions, map[string]interface{}{
					"functionId": op.FunctionID,
					"label":      getLocalizedText(op.Labels, locale, op.Operation),
					"risk":       op.Risk,
				})
			}
			props["table"].(map[string]interface{})["x-component-props"].(map[string]interface{})["rowActions"] = actions
		}
	}

	// Add DetailPanel if we have get operations
	if len(ops.get) > 0 {
		props["detail"] = map[string]interface{}{
			"type":        "void",
			"x-component": "DetailPanel",
			"x-component-props": map[string]interface{}{
				"functionId": ops.get[0].FunctionID,
			},
		}
	}

	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

func assessEntityPageQuality(resource spec.ResourceSpec, ops classifiedOps) (string, []spec.Diagnostic) {
	var diags []spec.Diagnostic

	if len(ops.list) == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:     "list_operation_missing",
			Severity: spec.SeverityWarning,
			Message:  "No list operation found; table data source will be empty",
		})
	}

	if len(ops.get) == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:     "get_operation_missing",
			Severity: spec.SeverityInfo,
			Message:  "No get operation found; detail panel will be unavailable",
		})
	}

	// Check for missing labels
	if resource.Labels == nil || resource.Labels["zh-CN"] == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "resource_label_missing",
			Severity: spec.SeverityWarning,
			Message:  "Resource missing zh-CN label",
		})
	}

	// Determine quality
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return "blocked", diags
		}
	}
	if len(diags) > 0 {
		return "needs_review", diags
	}
	return "ready", diags
}

// ---------------------------------------------------------------------------
// Internal: Operation Page generation
// ---------------------------------------------------------------------------

func generateOperationPage(resource spec.ResourceSpec, op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	pageKey := opts.PageKeyPrefix + op.FunctionID
	locale := opts.DefaultLocale

	title := spec.LocalizedText{
		locale: getLocalizedText(op.Labels, locale, op.Operation),
	}

	schema := buildOperationPageSchema(op, locale)

	quality, diags := assessOperationPageQuality(op)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: resource.Key,
			Title:       title,
			Category: spec.PageCategorySpec{
				Key:    resource.Category.Key,
				Labels: resource.Category.Labels,
			},
			Schema: schema,
			Bindings: []spec.PageFunctionBinding{
				{FunctionID: op.FunctionID, Role: spec.PlacementStandalone},
			},
		},
		Quality:     quality,
		Diagnostics: diags,
	}
}

func buildOperationPageSchema(op spec.OperationSpec, locale string) spec.FormilySchema {
	schema := map[string]interface{}{
		"type":        "void",
		"x-component": "ConsolePage",
		"properties": map[string]interface{}{
			"form": map[string]interface{}{
				"type":        "void",
				"x-component": "QueryForm",
				"x-component-props": map[string]interface{}{
					"functionId": op.FunctionID,
				},
			},
			"result": map[string]interface{}{
				"type":        "void",
				"x-component": "ResultPanel",
			},
		},
	}

	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

func assessOperationPageQuality(op spec.OperationSpec) (string, []spec.Diagnostic) {
	var diags []spec.Diagnostic

	if op.Kind == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "operation_kind_missing",
			Severity: spec.SeverityError,
			Message:  "operation_kind is required",
		})
	}

	if op.Labels == nil || op.Labels["zh-CN"] == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "operation_label_missing",
			Severity: spec.SeverityWarning,
			Message:  "Operation missing zh-CN label",
		})
	}

	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return "blocked", diags
		}
	}
	if len(diags) > 0 {
		return "needs_review", diags
	}
	return "ready", diags
}

// ---------------------------------------------------------------------------
// Internal: Task Page generation
// ---------------------------------------------------------------------------

func generateTaskPage(resource spec.ResourceSpec, op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	pageKey := opts.PageKeyPrefix + op.FunctionID
	locale := opts.DefaultLocale

	title := spec.LocalizedText{
		locale: getLocalizedText(op.Labels, locale, op.Operation),
	}

	schema := buildTaskPageSchema(op, locale)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeTask,
			ResourceKey: resource.Key,
			Title:       title,
			Category: spec.PageCategorySpec{
				Key:    resource.Category.Key,
				Labels: resource.Category.Labels,
			},
			Schema: schema,
			Bindings: []spec.PageFunctionBinding{
				{FunctionID: op.FunctionID, Role: spec.PlacementStandalone},
			},
		},
		Quality: "needs_review", // Task pages always need review for task integration
		Diagnostics: []spec.Diagnostic{
			{
				Code:     "task_integration_manual",
				Severity: spec.SeverityInfo,
				Message:  "Task page requires manual configuration of task start/events/result integration",
			},
		},
	}
}

func buildTaskPageSchema(op spec.OperationSpec, locale string) spec.FormilySchema {
	schema := map[string]interface{}{
		"type":        "void",
		"x-component": "ConsolePage",
		"properties": map[string]interface{}{
			"form": map[string]interface{}{
				"type":        "void",
				"x-component": "QueryForm",
				"x-component-props": map[string]interface{}{
					"functionId": op.FunctionID,
				},
			},
			"timeline": map[string]interface{}{
				"type":        "void",
				"x-component": "TaskTimeline",
			},
			"result": map[string]interface{}{
				"type":        "void",
				"x-component": "ResultPanel",
			},
		},
	}

	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

// ---------------------------------------------------------------------------
// Internal: Report Page generation
// ---------------------------------------------------------------------------

func generateReportPage(resource spec.ResourceSpec, op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	pageKey := opts.PageKeyPrefix + op.FunctionID
	locale := opts.DefaultLocale

	title := spec.LocalizedText{
		locale: getLocalizedText(op.Labels, locale, op.Operation),
	}

	schema := buildReportPageSchema(op, locale)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeReport,
			ResourceKey: resource.Key,
			Title:       title,
			Category: spec.PageCategorySpec{
				Key:    resource.Category.Key,
				Labels: resource.Category.Labels,
			},
			Schema: schema,
			Bindings: []spec.PageFunctionBinding{
				{FunctionID: op.FunctionID, Role: spec.PlacementStandalone},
			},
		},
		Quality: "needs_review", // Report pages need manual chart/table config
		Diagnostics: []spec.Diagnostic{
			{
				Code:     "report_config_manual",
				Severity: spec.SeverityInfo,
				Message:  "Report page requires manual configuration of chart/table data mapping",
			},
		},
	}
}

func buildReportPageSchema(op spec.OperationSpec, locale string) spec.FormilySchema {
	schema := map[string]interface{}{
		"type":        "void",
		"x-component": "ConsolePage",
		"properties": map[string]interface{}{
			"filter": map[string]interface{}{
				"type":        "void",
				"x-component": "QueryForm",
				"x-component-props": map[string]interface{}{
					"functionId": op.FunctionID,
				},
			},
			"chart": map[string]interface{}{
				"type":        "void",
				"x-component": "ChartPanel",
			},
			"table": map[string]interface{}{
				"type":        "void",
				"x-component": "DataTable",
			},
		},
	}

	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func getLocalizedText(labels spec.LocalizedText, locale, fallback string) string {
	if labels == nil {
		return fallback
	}
	if v, ok := labels[locale]; ok && v != "" {
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

// FormatPageKey creates a page key from resource and page name.
func FormatPageKey(resourceKey, pageName string) string {
	return fmt.Sprintf("%s.%s", resourceKey, pageName)
}

// InferPageType infers the page type from operations.
func InferPageType(ops []spec.OperationSpec) spec.PageType {
	hasTask := false
	hasReport := false
	hasList := false

	for _, op := range ops {
		switch op.Kind {
		case spec.OperationKindTask:
			hasTask = true
		case spec.OperationKindReport:
			hasReport = true
		case spec.OperationKindList:
			hasList = true
		}
	}

	if hasTask {
		return spec.PageTypeTask
	}
	if hasReport {
		return spec.PageTypeReport
	}
	if hasList {
		return spec.PageTypeEntity
	}
	return spec.PageTypeOperation
}

// InferCategoryFromKey extracts category from a key.
func InferCategoryFromKey(key string) string {
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}
