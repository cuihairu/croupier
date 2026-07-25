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

type pageSchemaNode struct {
	Type           string                    `json:"type,omitempty"`
	Component      string                    `json:"x-component,omitempty"`
	ComponentProps json.RawMessage           `json:"x-component-props,omitempty"`
	Properties     map[string]pageSchemaNode `json:"properties,omitempty"`
}

type consolePageProps struct {
	SchemaVersion string `json:"schemaVersion"`
	ResourceKey   string `json:"resourceKey,omitempty"`
}

type bindingComponentProps struct {
	BindingID string `json:"bindingId"`
}

type dataTableProps struct {
	BindingID      string                         `json:"bindingId"`
	PageField      string                         `json:"pageField"`
	PageSizeField  string                         `json:"pageSizeField"`
	ItemsPath      string                         `json:"itemsPath"`
	TotalPath      string                         `json:"totalPath"`
	ColumnsPath    string                         `json:"columnsPath,omitempty"`
	Columns        []spec.PageTableColumnContract `json:"columns,omitempty"`
	SelectionState string                         `json:"selectionState,omitempty"`
	RowActions     []tableActionRef               `json:"rowActions,omitempty"`
}

type tableActionRef struct {
	BindingID string         `json:"bindingId"`
	Label     string         `json:"label"`
	Risk      spec.RiskLevel `json:"risk,omitempty"`
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

	bindings := buildEntityBindings(ops)

	// Assess quality
	quality, diags := assessEntityPageQuality(resource, ops)
	schema := buildEntityPageSchema(resource, ops, bindings, locale, quality == "blocked")

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

func buildEntityBindings(ops classifiedOps) []spec.PageFunctionBinding {
	var bindings []spec.PageFunctionBinding
	if len(ops.list) > 0 {
		bindings = append(bindings, pageBinding(ops.list[0], "list", spec.BindingUsageQuery, spec.PageExecutionModeSync))
	}
	if len(ops.get) > 0 {
		bindings = append(bindings, pageBinding(ops.get[0], "detail", spec.BindingUsageDetail, spec.PageExecutionModeSync))
	}
	for _, op := range ops.rowActions {
		bindings = append(bindings, pageBinding(op, op.Operation, spec.BindingUsageAction, spec.PageExecutionModeSync))
	}
	for _, op := range ops.toolbarActions {
		bindings = append(bindings, pageBinding(op, op.Operation, spec.BindingUsageAction, spec.PageExecutionModeSync))
	}
	return bindings
}

func buildEntityPageSchema(resource spec.ResourceSpec, ops classifiedOps, bindings []spec.PageFunctionBinding, locale string, blocked bool) spec.FormilySchema {
	listBindingID := findBindingID(bindings, spec.BindingUsageQuery)
	detailBindingID := findBindingID(bindings, spec.BindingUsageDetail)
	root := consolePageNode(resource.Key)
	if blocked {
		return marshalSchema(root)
	}

	// Add QueryForm if we have list operations
	if listBindingID != "" {
		root.Properties["query"] = componentNode("QueryForm", bindingComponentProps{BindingID: listBindingID})
	}

	if listBindingID != "" {
		tableProps, ok := buildDataTableProps(listBindingID, ops.list[0].PageContract)
		if ok {
			tableProps.SelectionState = "selection"
			tableProps.RowActions = buildTableActionRefs(ops.rowActions, locale)
			root.Properties["table"] = componentNode("DataTable", tableProps)
		}
	}

	// Add DetailPanel if we have get operations
	if detailBindingID != "" {
		root.Properties["detail"] = componentNode("DetailPanel", bindingComponentProps{BindingID: detailBindingID})
	}

	return marshalSchema(root)
}

func assessEntityPageQuality(resource spec.ResourceSpec, ops classifiedOps) (string, []spec.Diagnostic) {
	var diags []spec.Diagnostic

	if len(ops.list) == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:     "list_operation_missing",
			Severity: spec.SeverityError,
			Message:  "Entity page requires a list operation",
		})
	} else {
		diags = append(diags, validateTableDataContract("list operation", ops.list[0])...)
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

	for _, op := range append(append([]spec.OperationSpec{}, ops.rowActions...), ops.detailActions...) {
		if !hasJSONMapping(op.PageContract, "input") {
			diags = append(diags, spec.Diagnostic{
				Code:       "action_input_mapping_missing",
				Severity:   spec.SeverityWarning,
				Message:    "Action inputMapping is required before publishing action bindings",
				FunctionID: op.FunctionID,
				Field:      "pageContract.inputMapping",
			})
		}
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
				pageBinding(op, "main", spec.BindingUsageAction, spec.PageExecutionModeSync),
			},
		},
		Quality:     quality,
		Diagnostics: diags,
	}
}

func buildOperationPageSchema(op spec.OperationSpec, locale string) spec.FormilySchema {
	bindingID := bindingIDForOperation(op)
	root := consolePageNode(op.ResourceKey)
	root.Properties["form"] = componentNode("QueryForm", bindingComponentProps{BindingID: bindingID})
	root.Properties["result"] = emptyComponentNode("ResultPanel")
	return marshalSchema(root)
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
	if op.Kind != spec.OperationKindAction || op.Placement != spec.PlacementStandalone {
		diags = append(diags, spec.Diagnostic{
			Code:       "standalone_action_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    "Operation page should use operationKind=action and placement=standalone",
			FunctionID: op.FunctionID,
			Field:      "operationKind",
		})
	}
	if !hasJSONMapping(op.PageContract, "input") {
		diags = append(diags, spec.Diagnostic{
			Code:       "operation_input_mapping_missing",
			Severity:   spec.SeverityWarning,
			Message:    "Operation page has no explicit inputMapping; Page Studio must confirm request payload mapping",
			FunctionID: op.FunctionID,
			Field:      "pageContract.inputMapping",
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
	quality, diags := assessTaskPageQuality(op)

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
				pageBinding(op, "task", spec.BindingUsageTask, spec.PageExecutionModeTask),
			},
		},
		Quality:     quality,
		Diagnostics: diags,
	}
}

func buildTaskPageSchema(op spec.OperationSpec, locale string) spec.FormilySchema {
	bindingID := bindingIDForOperation(op)
	root := consolePageNode(op.ResourceKey)
	root.Properties["form"] = componentNode("QueryForm", bindingComponentProps{BindingID: bindingID})
	root.Properties["timeline"] = emptyComponentNode("TaskTimeline")
	root.Properties["result"] = emptyComponentNode("ResultPanel")
	return marshalSchema(root)
}

func assessTaskPageQuality(op spec.OperationSpec) (string, []spec.Diagnostic) {
	var diags []spec.Diagnostic
	if op.Kind != spec.OperationKindTask {
		diags = append(diags, spec.Diagnostic{
			Code:       "operation_kind_task_required",
			Severity:   spec.SeverityError,
			Message:    "Task page requires operationKind=task",
			FunctionID: op.FunctionID,
			Field:      "operationKind",
		})
	}
	if op.PageContract == nil || op.PageContract.Task == nil || strings.TrimSpace(op.PageContract.Task.TaskIDPath) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "task_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    "Task page requires taskIdPath/status/events/result contract before it can be ready",
			FunctionID: op.FunctionID,
			Field:      "pageContract.task",
		})
	}
	if !hasJSONMapping(op.PageContract, "input") {
		diags = append(diags, spec.Diagnostic{
			Code:       "task_input_mapping_missing",
			Severity:   spec.SeverityWarning,
			Message:    "Task page has no explicit inputMapping; Page Studio must confirm request payload mapping",
			FunctionID: op.FunctionID,
			Field:      "pageContract.inputMapping",
		})
	}
	return qualityFromDiagnostics(diags), diags
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
	quality, diags := assessReportPageQuality(op)

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
				pageBinding(op, "report", spec.BindingUsageReport, spec.PageExecutionModeSync),
			},
		},
		Quality:     quality,
		Diagnostics: diags,
	}
}

func buildReportPageSchema(op spec.OperationSpec, locale string) spec.FormilySchema {
	bindingID := bindingIDForOperation(op)
	root := consolePageNode(op.ResourceKey)
	root.Properties["filter"] = componentNode("QueryForm", bindingComponentProps{BindingID: bindingID})
	if op.PageContract != nil && op.PageContract.Report != nil && reportContractReady(op.PageContract.Report) {
		root.Properties["chart"] = componentNode("ChartPanel", op.PageContract.Report)
	}
	if tableProps, ok := buildDataTableProps(bindingID, op.PageContract); ok {
		root.Properties["table"] = componentNode("DataTable", tableProps)
	}
	root.Properties["result"] = emptyComponentNode("ResultPanel")
	return marshalSchema(root)
}

func assessReportPageQuality(op spec.OperationSpec) (string, []spec.Diagnostic) {
	var diags []spec.Diagnostic
	if op.Kind != spec.OperationKindReport {
		diags = append(diags, spec.Diagnostic{
			Code:       "operation_kind_report_required",
			Severity:   spec.SeverityError,
			Message:    "Report page requires operationKind=report",
			FunctionID: op.FunctionID,
			Field:      "operationKind",
		})
	}
	if op.PageContract == nil || op.PageContract.Report == nil || !reportContractReady(op.PageContract.Report) {
		diags = append(diags, spec.Diagnostic{
			Code:       "report_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    "Report page requires explicit chart contract before it can be ready",
			FunctionID: op.FunctionID,
			Field:      "pageContract.report",
		})
	}
	diags = append(diags, validateTableDataContract("report operation", op)...)
	return qualityFromDiagnostics(diags), diags
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func consolePageNode(resourceKey string) pageSchemaNode {
	return componentNode("ConsolePage", consolePageProps{
		SchemaVersion: "formily-page:1",
		ResourceKey:   strings.TrimSpace(resourceKey),
	})
}

func componentNode[T any](component string, props T) pageSchemaNode {
	return pageSchemaNode{
		Type:           "void",
		Component:      component,
		ComponentProps: mustJSONRaw(props),
		Properties:     map[string]pageSchemaNode{},
	}
}

func emptyComponentNode(component string) pageSchemaNode {
	return pageSchemaNode{
		Type:       "void",
		Component:  component,
		Properties: map[string]pageSchemaNode{},
	}
}

func marshalSchema(root pageSchemaNode) spec.FormilySchema {
	b, _ := json.Marshal(root)
	return spec.FormilySchema(b)
}

func mustJSONRaw[T any](value T) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func buildDataTableProps(bindingID string, contract *spec.PageContract) (dataTableProps, bool) {
	if contract == nil || contract.Pagination == nil || contract.Table == nil {
		return dataTableProps{}, false
	}
	pagination := contract.Pagination
	if strings.TrimSpace(pagination.PageField) == "" ||
		strings.TrimSpace(pagination.PageSizeField) == "" ||
		strings.TrimSpace(pagination.ItemsPath) == "" ||
		strings.TrimSpace(pagination.TotalPath) == "" {
		return dataTableProps{}, false
	}
	table := contract.Table
	if strings.TrimSpace(table.ColumnsPath) == "" && len(table.Columns) == 0 {
		return dataTableProps{}, false
	}
	return dataTableProps{
		BindingID:     bindingID,
		PageField:     strings.TrimSpace(pagination.PageField),
		PageSizeField: strings.TrimSpace(pagination.PageSizeField),
		ItemsPath:     strings.TrimSpace(pagination.ItemsPath),
		TotalPath:     strings.TrimSpace(pagination.TotalPath),
		ColumnsPath:   strings.TrimSpace(table.ColumnsPath),
		Columns:       table.Columns,
	}, true
}

func buildTableActionRefs(ops []spec.OperationSpec, locale string) []tableActionRef {
	if len(ops) == 0 {
		return nil
	}
	actions := make([]tableActionRef, 0, len(ops))
	for _, op := range ops {
		actions = append(actions, tableActionRef{
			BindingID: bindingIDForOperation(op),
			Label:     getLocalizedText(op.Labels, locale, op.Operation),
			Risk:      op.Risk,
		})
	}
	return actions
}

func validateTableDataContract(label string, op spec.OperationSpec) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if op.PageContract == nil {
		return []spec.Diagnostic{{
			Code:       "page_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    label + " has no pageContract; generator cannot verify table data mapping",
			FunctionID: op.FunctionID,
			Field:      "pageContract",
		}}
	}
	if op.PageContract.Pagination == nil {
		diags = append(diags, spec.Diagnostic{
			Code:       "pagination_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    label + " requires pagination contract for a ready DataTable",
			FunctionID: op.FunctionID,
			Field:      "pageContract.pagination",
		})
	} else {
		pagination := op.PageContract.Pagination
		for _, required := range []struct {
			field string
			value string
		}{
			{field: "pageField", value: pagination.PageField},
			{field: "pageSizeField", value: pagination.PageSizeField},
			{field: "itemsPath", value: pagination.ItemsPath},
			{field: "totalPath", value: pagination.TotalPath},
		} {
			if strings.TrimSpace(required.value) == "" {
				diags = append(diags, spec.Diagnostic{
					Code:       "pagination_contract_field_missing",
					Severity:   spec.SeverityWarning,
					Message:    label + " pagination contract is missing " + required.field,
					FunctionID: op.FunctionID,
					Field:      "pageContract.pagination." + required.field,
				})
			}
		}
	}
	if op.PageContract.Table == nil {
		diags = append(diags, spec.Diagnostic{
			Code:       "table_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    label + " requires table columns or columnsPath for a ready DataTable",
			FunctionID: op.FunctionID,
			Field:      "pageContract.table",
		})
	} else if strings.TrimSpace(op.PageContract.Table.ColumnsPath) == "" && len(op.PageContract.Table.Columns) == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:       "table_columns_missing",
			Severity:   spec.SeverityWarning,
			Message:    label + " requires stable columns or columnsPath; runtime row sampling is forbidden",
			FunctionID: op.FunctionID,
			Field:      "pageContract.table.columns",
		})
	}
	if !hasJSONMapping(op.PageContract, "input") {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_input_mapping_missing",
			Severity:   spec.SeverityWarning,
			Message:    label + " has no explicit inputMapping",
			FunctionID: op.FunctionID,
			Field:      "pageContract.inputMapping",
		})
	}
	if !hasJSONMapping(op.PageContract, "output") {
		diags = append(diags, spec.Diagnostic{
			Code:       "binding_output_mapping_missing",
			Severity:   spec.SeverityWarning,
			Message:    label + " has no explicit outputMapping",
			FunctionID: op.FunctionID,
			Field:      "pageContract.outputMapping",
		})
	}
	return diags
}

func hasJSONMapping(contract *spec.PageContract, kind string) bool {
	if contract == nil {
		return false
	}
	switch kind {
	case "input":
		return len(contract.InputMapping) > 0 && string(contract.InputMapping) != "null"
	case "output":
		return len(contract.OutputMapping) > 0 && string(contract.OutputMapping) != "null"
	default:
		return false
	}
}

func reportContractReady(contract *spec.PageReportContract) bool {
	return contract != nil &&
		strings.TrimSpace(contract.ChartType) != "" &&
		strings.TrimSpace(contract.CategoryPath) != "" &&
		strings.TrimSpace(contract.ValuePath) != ""
}

func qualityFromDiagnostics(diags []spec.Diagnostic) string {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return "blocked"
		}
	}
	if len(diags) > 0 {
		return "needs_review"
	}
	return "ready"
}

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

func pageBinding(op spec.OperationSpec, suffix string, usage spec.PageBindingUsage, mode spec.PageExecutionMode) spec.PageFunctionBinding {
	binding := spec.PageFunctionBinding{
		ID:         bindingIDForOperationWithSuffix(op, suffix),
		FunctionID: op.FunctionID,
		Usage:      usage,
		Execution: spec.PageBindingExecution{
			Mode: mode,
		},
	}
	if op.PageContract != nil {
		binding.InputMapping = op.PageContract.InputMapping
		binding.OutputMapping = op.PageContract.OutputMapping
		if op.PageContract.ExecutionMode != "" {
			binding.Execution.Mode = op.PageContract.ExecutionMode
		}
	}
	return binding
}

func findBindingID(bindings []spec.PageFunctionBinding, usage spec.PageBindingUsage) string {
	for _, binding := range bindings {
		if binding.Usage == usage {
			return binding.ID
		}
	}
	return ""
}

func bindingIDForOperation(op spec.OperationSpec) string {
	return bindingIDForOperationWithSuffix(op, op.Operation)
}

func bindingIDForOperationWithSuffix(op spec.OperationSpec, suffix string) string {
	parts := []string{op.ResourceKey, suffix}
	if strings.TrimSpace(parts[0]) == "" {
		parts[0] = op.FunctionID
	}
	if strings.TrimSpace(parts[1]) == "" {
		parts[1] = string(op.Kind)
	}
	return sanitizeBindingID(strings.Join(parts, "."))
}

func sanitizeBindingID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "binding"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('.')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), ".")
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
