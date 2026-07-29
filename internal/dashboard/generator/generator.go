// Package generator creates conservative PageSpec suggestions from normalized
// resource capabilities. It only uses explicit PageContract data to select
// entity, task, report, and standalone operation page shapes.
package generator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	DefaultLocale string
	PageKeyPrefix string
	Functions     map[string]spec.FunctionSpec
}

// DefaultGenerateOptions returns default options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		DefaultLocale: "zh-CN",
		PageKeyPrefix: "",
	}
}

type pageSchemaNode struct {
	Type           string                     `json:"type,omitempty"`
	Component      string                     `json:"x-component,omitempty"`
	ComponentProps json.RawMessage            `json:"x-component-props,omitempty"`
	Properties     map[string]json.RawMessage `json:"properties,omitempty"`
}

type consolePageProps struct {
	SchemaVersion string `json:"schemaVersion"`
	PageKey       string `json:"pageKey,omitempty"`
	ResourceKey   string `json:"resourceKey,omitempty"`
}

type queryFormProps struct {
	BindingID      string          `json:"bindingId"`
	InputMapping   json.RawMessage `json:"inputMapping,omitempty"`
	ResultStateKey string          `json:"resultStateKey,omitempty"`
}

type dataTableProps struct {
	BindingID     string            `json:"bindingId"`
	ItemsPath     string            `json:"itemsPath"`
	TotalPath     string            `json:"totalPath"`
	PageField     string            `json:"pageField"`
	PageSizeField string            `json:"pageSizeField"`
	Columns       []dataTableColumn `json:"columns,omitempty"`
	ColumnsPath   string            `json:"columnsPath,omitempty"`
	RowActions    []rowAction       `json:"rowActions,omitempty"`
}

type dataTableColumn struct {
	Title     string `json:"title"`
	DataIndex string `json:"dataIndex"`
	Key       string `json:"key,omitempty"`
}

type rowAction struct {
	BindingID    string          `json:"bindingId"`
	Label        string          `json:"label,omitempty"`
	Risk         string          `json:"risk,omitempty"`
	InputMapping json.RawMessage `json:"inputMapping"`
}

type resultPanelProps struct {
	BindingID string `json:"bindingId,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
	DataPath  string `json:"dataPath,omitempty"`
}

type taskTimelineProps struct {
	BindingID string `json:"bindingId,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
}

type chartPanelProps struct {
	BindingID    string `json:"bindingId,omitempty"`
	StateKey     string `json:"stateKey,omitempty"`
	DataPath     string `json:"dataPath"`
	ChartType    string `json:"chartType"`
	CategoryPath string `json:"categoryPath,omitempty"`
	SeriesPath   string `json:"seriesPath,omitempty"`
	ValuePath    string `json:"valuePath,omitempty"`
}

// GenerateForResource creates PageSpec candidates from explicit contracts.
// Resource-level entity pages are generated only when a list/table contract is
// present. Other operations remain standalone operation/task/report candidates.
func GenerateForResource(resource spec.ResourceSpec, opts GenerateOptions) []spec.GeneratedPageSpec {
	if len(resource.Operations) == 0 {
		return nil
	}
	opts = normalizeOptions(opts)
	ops := sortedOperations(resource.Operations)
	consumed := map[string]struct{}{}
	pages := make([]spec.GeneratedPageSpec, 0, len(ops))
	if entity, ok, entityConsumed := GenerateEntityPageForResource(resource, ops, opts); ok {
		pages = append(pages, entity)
		for _, functionID := range entityConsumed {
			consumed[functionID] = struct{}{}
		}
	}
	for _, op := range ops {
		if _, ok := consumed[op.FunctionID]; ok {
			continue
		}
		pages = append(pages, GenerateForOperation(op, opts))
	}
	return pages
}

// GenerateEntityPageForResource creates a resource management page only from a
// concrete pagination + table PageContract. It never infers CRUD from names.
func GenerateEntityPageForResource(resource spec.ResourceSpec, ops []spec.OperationSpec, opts GenerateOptions) (spec.GeneratedPageSpec, bool, []string) {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(resource.Key)
	if resourceKey == "" {
		return spec.GeneratedPageSpec{}, false, nil
	}
	queryOp, ok := firstEntityQueryOperation(ops)
	if !ok {
		return spec.GeneratedPageSpec{}, false, nil
	}

	pageKey := opts.PageKeyPrefix + sanitizePageKey(FormatPageKey(resourceKey, "manage"))
	category := categoryForPage(resourceKey, pageKey, opts.DefaultLocale)
	title := localizedFrom(resource.Labels, opts.DefaultLocale, resourceKey+" management")
	queryBinding := pageBinding(queryOp, "query", spec.BindingUsageQuery, spec.PageExecutionModeSync)
	bindings := []spec.PageFunctionBinding{queryBinding}
	rowActions, actionBindings, actionDiags, consumedActions := rowActionsForEntity(queryOp, ops)
	bindings = append(bindings, actionBindings...)

	diags := assessEntityCandidate(resourceKey, queryOp)
	diags = append(diags, actionDiags...)
	root := consolePage(pageKey, resourceKey)
	root.Properties["table"] = rawNode(componentNode("DataTable", dataTablePropsJSON(dataTablePropsFromContract(queryBinding.ID, queryOp.PageContract, rowActions))))

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeEntity,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    category,
			Schema:      marshalSchema(root),
			Bindings:    bindings,
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
	}, true, append([]string{queryOp.FunctionID}, consumedActions...)
}

// GenerateForOperation creates an operation, task, or report candidate from a
// single capability. Page type is selected only from explicit PageContract data.
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

// GenerateOperationPageForOperation creates a standalone Operation Page.
func GenerateOperationPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := opts.PageKeyPrefix + sanitizePageKey(firstNonEmpty(resourceKey, op.FunctionID))
	if operation := strings.TrimSpace(op.Operation); operation != "" {
		pageKey += "." + sanitizePageKey(operation)
	}
	if strings.TrimSpace(pageKey) == "" {
		pageKey = opts.PageKeyPrefix + sanitizePageKey(op.FunctionID)
	}
	if strings.TrimSpace(pageKey) == "" {
		pageKey = opts.PageKeyPrefix + "operation"
	}

	title := spec.LocalizedText{
		opts.DefaultLocale: firstNonEmpty(op.Operation, op.FunctionID, pageKey),
	}
	binding := pageBinding(op, "main", spec.BindingUsageAction, spec.PageExecutionModeSync)
	diags := assessOperationCandidate(op)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForPage(resourceKey, pageKey, opts.DefaultLocale),
			Schema:      buildOperationPageSchema(pageKey, resourceKey, binding, opts),
			Bindings:    []spec.PageFunctionBinding{binding},
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
	}
}

// GenerateTaskPageForOperation creates an async task page candidate.
func GenerateTaskPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := opts.PageKeyPrefix + sanitizePageKey(firstNonEmpty(resourceKey, op.FunctionID, "task"))
	if operation := strings.TrimSpace(op.Operation); operation != "" {
		pageKey += "." + sanitizePageKey(operation)
	}
	binding := pageBinding(op, "task", spec.BindingUsageTask, spec.PageExecutionModeTask)
	diags := assessTaskCandidate(op)

	root := consolePage(pageKey, resourceKey)
	root.Properties["form"] = rawNode(queryFormNode(binding, opts, ""))
	root.Properties["timeline"] = rawNode(componentNode("TaskTimeline", taskTimelinePropsJSON(taskTimelineProps{BindingID: binding.ID})))
	root.Properties["result"] = rawNode(componentNode("ResultPanel", resultPanelPropsJSON(resultPanelProps{BindingID: binding.ID})))

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeTask,
			ResourceKey: resourceKey,
			Title:       localizedTitle(op, pageKey, opts.DefaultLocale),
			Category:    categoryForPage(resourceKey, pageKey, opts.DefaultLocale),
			Schema:      marshalSchema(root),
			Bindings:    []spec.PageFunctionBinding{binding},
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
	}
}

// GenerateReportPageForOperation creates a report page candidate when chart
// paths are explicitly declared in PageContract.
func GenerateReportPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := opts.PageKeyPrefix + sanitizePageKey(firstNonEmpty(resourceKey, op.FunctionID, "report"))
	if operation := strings.TrimSpace(op.Operation); operation != "" {
		pageKey += "." + sanitizePageKey(operation)
	}
	binding := pageBinding(op, "report", spec.BindingUsageReport, spec.PageExecutionModeSync)
	diags := assessReportCandidate(op)

	root := consolePage(pageKey, resourceKey)
	root.Properties["form"] = rawNode(queryFormNode(binding, opts, ""))
	root.Properties["chart"] = rawNode(componentNode("ChartPanel", chartPanelPropsJSON(chartPanelPropsFromContract(binding.ID, op.PageContract))))
	if hasCompleteTableContract(op.PageContract) && hasCompletePaginationContract(op.PageContract) {
		root.Properties["table"] = rawNode(componentNode("DataTable", dataTablePropsJSON(dataTablePropsFromContract(binding.ID, op.PageContract, nil))))
	}

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeReport,
			ResourceKey: resourceKey,
			Title:       localizedTitle(op, pageKey, opts.DefaultLocale),
			Category:    categoryForPage(resourceKey, pageKey, opts.DefaultLocale),
			Schema:      marshalSchema(root),
			Bindings:    []spec.PageFunctionBinding{binding},
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
	}
}

func buildOperationPageSchema(pageKey, resourceKey string, binding spec.PageFunctionBinding, opts GenerateOptions) spec.FormilySchema {
	root := consolePage(pageKey, resourceKey)
	root.Properties["form"] = rawNode(queryFormNode(binding, opts, ""))
	root.Properties["result"] = rawNode(componentNode("ResultPanel", resultPanelPropsJSON(resultPanelProps{BindingID: binding.ID})))
	return marshalSchema(root)
}

func normalizeOptions(opts GenerateOptions) GenerateOptions {
	if strings.TrimSpace(opts.DefaultLocale) == "" {
		opts.DefaultLocale = "zh-CN"
	}
	return opts
}

func assessOperationCandidate(op spec.OperationSpec) []spec.Diagnostic {
	diags := assessBaseCandidate(op)
	if op.PageContract == nil {
		return diags
	}
	diags = append(diags, requireMappingDiagnostics(op, true, true)...)
	return diags
}

func assessEntityCandidate(resourceKey string, queryOp spec.OperationSpec) []spec.Diagnostic {
	diags := assessBaseCandidate(queryOp)
	if strings.TrimSpace(resourceKey) == "" {
		diags = append(diags, diagnostic("resource_missing", spec.SeverityError, "entity page requires an explicit resource", queryOp.FunctionID, "resourceKey"))
	}
	if !hasCompletePaginationContract(queryOp.PageContract) {
		diags = append(diags, diagnostic("page_contract_pagination_incomplete", spec.SeverityError, "entity page requires explicit pagination fields", queryOp.FunctionID, "pageContract.pagination"))
	}
	if !hasCompleteTableContract(queryOp.PageContract) {
		diags = append(diags, diagnostic("page_contract_table_incomplete", spec.SeverityError, "entity page requires explicit table columns or columnsPath", queryOp.FunctionID, "pageContract.table"))
	}
	diags = append(diags, requireMappingDiagnostics(queryOp, true, true)...)
	return diags
}

func assessTaskCandidate(op spec.OperationSpec) []spec.Diagnostic {
	diags := assessBaseCandidate(op)
	if op.PageContract == nil {
		return diags
	}
	if op.PageContract.ExecutionMode != spec.PageExecutionModeTask {
		diags = append(diags, diagnostic("page_contract_task_mode_missing", spec.SeverityWarning, "task page requires executionMode=task", op.FunctionID, "pageContract.executionMode"))
	}
	if op.PageContract.Task == nil {
		diags = append(diags, diagnostic("page_contract_task_missing", spec.SeverityWarning, "task page requires explicit task tracking contract", op.FunctionID, "pageContract.task"))
	} else {
		if strings.TrimSpace(op.PageContract.Task.TaskIDPath) == "" {
			diags = append(diags, diagnostic("page_contract_task_id_missing", spec.SeverityWarning, "task page requires taskIdPath", op.FunctionID, "pageContract.task.taskIdPath"))
		}
		if strings.TrimSpace(op.PageContract.Task.StatusPath) == "" {
			diags = append(diags, diagnostic("page_contract_task_status_missing", spec.SeverityWarning, "task page requires statusPath", op.FunctionID, "pageContract.task.statusPath"))
		}
	}
	diags = append(diags, requireMappingDiagnostics(op, true, true)...)
	return diags
}

func assessReportCandidate(op spec.OperationSpec) []spec.Diagnostic {
	diags := assessBaseCandidate(op)
	if op.PageContract == nil {
		return diags
	}
	if op.PageContract.Report == nil {
		diags = append(diags, diagnostic("page_contract_report_missing", spec.SeverityWarning, "report page requires explicit chart contract", op.FunctionID, "pageContract.report"))
	} else {
		report := op.PageContract.Report
		if strings.TrimSpace(report.ChartType) == "" {
			diags = append(diags, diagnostic("page_contract_chart_type_missing", spec.SeverityWarning, "report page requires chartType", op.FunctionID, "pageContract.report.chartType"))
		}
		if strings.TrimSpace(report.CategoryPath) == "" {
			diags = append(diags, diagnostic("page_contract_chart_category_missing", spec.SeverityWarning, "report page requires categoryPath", op.FunctionID, "pageContract.report.categoryPath"))
		}
		if strings.TrimSpace(report.SeriesPath) == "" {
			diags = append(diags, diagnostic("page_contract_chart_series_missing", spec.SeverityWarning, "report page requires seriesPath", op.FunctionID, "pageContract.report.seriesPath"))
		}
		if strings.TrimSpace(report.ValuePath) == "" {
			diags = append(diags, diagnostic("page_contract_chart_value_missing", spec.SeverityWarning, "report page requires valuePath", op.FunctionID, "pageContract.report.valuePath"))
		}
	}
	diags = append(diags, requireMappingDiagnostics(op, true, true)...)
	return diags
}

func assessBaseCandidate(op spec.OperationSpec) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if strings.TrimSpace(op.FunctionID) == "" {
		diags = append(diags, diagnostic("function_id_missing", spec.SeverityError, "functionId is required", "", "functionId"))
	}
	if strings.TrimSpace(op.ResourceKey) == "" {
		diags = append(diags, diagnostic("resource_missing", spec.SeverityWarning, "resource is missing; Page Studio must choose page grouping before publishing", op.FunctionID, "resourceKey"))
	}
	if strings.TrimSpace(op.Operation) == "" {
		diags = append(diags, diagnostic("operation_missing", spec.SeverityWarning, "operation is missing; Page Studio must name how this capability is used", op.FunctionID, "operation"))
	}
	if op.PageContract == nil {
		diags = append(diags, diagnostic("page_contract_missing", spec.SeverityWarning, "pageContract is missing; generated page is only a draft candidate", op.FunctionID, "pageContract"))
	}
	return diags
}

func requireMappingDiagnostics(op spec.OperationSpec, input bool, output bool) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if op.PageContract == nil {
		return diags
	}
	if input && !hasJSONMapping(op.PageContract.InputMapping) {
		diags = append(diags, diagnostic("binding_input_mapping_missing", spec.SeverityWarning, "inputMapping is missing; Page Studio must confirm request mapping", op.FunctionID, "pageContract.inputMapping"))
	}
	if input && hasJSONMapping(op.PageContract.InputMapping) && !isJSONObject(op.PageContract.InputMapping) {
		diags = append(diags, diagnostic("binding_input_mapping_invalid", spec.SeverityError, "inputMapping must be a JSON object", op.FunctionID, "pageContract.inputMapping"))
	}
	if output && !hasJSONMapping(op.PageContract.OutputMapping) {
		diags = append(diags, diagnostic("binding_output_mapping_missing", spec.SeverityWarning, "outputMapping is missing; Page Studio must confirm response mapping", op.FunctionID, "pageContract.outputMapping"))
	}
	if output && hasJSONMapping(op.PageContract.OutputMapping) && !isJSONObject(op.PageContract.OutputMapping) {
		diags = append(diags, diagnostic("binding_output_mapping_invalid", spec.SeverityError, "outputMapping must be a JSON object", op.FunctionID, "pageContract.outputMapping"))
	}
	return diags
}

func componentNode(component string, props json.RawMessage) pageSchemaNode {
	return pageSchemaNode{
		Type:           "void",
		Component:      component,
		ComponentProps: props,
		Properties:     map[string]json.RawMessage{},
	}
}

func consolePage(pageKey, resourceKey string) pageSchemaNode {
	return componentNode("ConsolePage", consolePagePropsJSON(consolePageProps{
		SchemaVersion: "formily-page:1",
		PageKey:       strings.TrimSpace(pageKey),
		ResourceKey:   strings.TrimSpace(resourceKey),
	}))
}

func queryFormNode(binding spec.PageFunctionBinding, opts GenerateOptions, resultStateKey string) pageSchemaNode {
	node := componentNode("QueryForm", queryFormPropsJSON(queryFormProps{
		BindingID:      binding.ID,
		InputMapping:   binding.InputMapping,
		ResultStateKey: strings.TrimSpace(resultStateKey),
	}))
	for key, field := range functionFormProperties(opts.Functions[binding.FunctionID]) {
		node.Properties[key] = field
	}
	return node
}

func marshalSchema(root pageSchemaNode) spec.FormilySchema {
	b, _ := json.Marshal(root)
	return spec.FormilySchema(b)
}

func rawNode(node pageSchemaNode) json.RawMessage {
	b, _ := json.Marshal(node)
	return json.RawMessage(b)
}

func consolePagePropsJSON(value consolePageProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func queryFormPropsJSON(value queryFormProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func dataTablePropsJSON(value dataTableProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func resultPanelPropsJSON(value resultPanelProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func taskTimelinePropsJSON(value taskTimelineProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func chartPanelPropsJSON(value chartPanelProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
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

func sortedOperations(ops []spec.OperationSpec) []spec.OperationSpec {
	out := append([]spec.OperationSpec(nil), ops...)
	sort.SliceStable(out, func(i, j int) bool {
		left := firstNonEmpty(out[i].ResourceKey, "") + "." + firstNonEmpty(out[i].Operation, "") + "." + out[i].FunctionID
		right := firstNonEmpty(out[j].ResourceKey, "") + "." + firstNonEmpty(out[j].Operation, "") + "." + out[j].FunctionID
		return left < right
	})
	return out
}

func firstEntityQueryOperation(ops []spec.OperationSpec) (spec.OperationSpec, bool) {
	for _, op := range ops {
		if isEntityQueryOperation(op) {
			return op, true
		}
	}
	return spec.OperationSpec{}, false
}

func isEntityQueryOperation(op spec.OperationSpec) bool {
	return hasCompletePaginationContract(op.PageContract) && hasCompleteTableContract(op.PageContract) && !isTaskOperation(op) && !isReportOperation(op)
}

func isTaskOperation(op spec.OperationSpec) bool {
	if op.PageContract == nil {
		return false
	}
	return op.PageContract.ExecutionMode == spec.PageExecutionModeTask || op.PageContract.Task != nil
}

func isReportOperation(op spec.OperationSpec) bool {
	return op.PageContract != nil && op.PageContract.Report != nil
}

func hasCompletePaginationContract(contract *spec.PageContract) bool {
	if contract == nil || contract.Pagination == nil {
		return false
	}
	pagination := contract.Pagination
	return strings.TrimSpace(pagination.PageField) != "" &&
		strings.TrimSpace(pagination.PageSizeField) != "" &&
		strings.TrimSpace(pagination.ItemsPath) != "" &&
		strings.TrimSpace(pagination.TotalPath) != ""
}

func hasCompleteTableContract(contract *spec.PageContract) bool {
	if contract == nil || contract.Table == nil {
		return false
	}
	table := contract.Table
	if strings.TrimSpace(table.ColumnsPath) != "" {
		return true
	}
	if len(table.Columns) == 0 {
		return false
	}
	for _, column := range table.Columns {
		if strings.TrimSpace(column.Key) == "" || strings.TrimSpace(column.ValuePath) == "" {
			return false
		}
	}
	return true
}

func rowActionsForEntity(queryOp spec.OperationSpec, ops []spec.OperationSpec) ([]rowAction, []spec.PageFunctionBinding, []spec.Diagnostic, []string) {
	var actions []rowAction
	var bindings []spec.PageFunctionBinding
	var diags []spec.Diagnostic
	var consumed []string
	for _, op := range ops {
		if op.FunctionID == queryOp.FunctionID {
			continue
		}
		if isEntityQueryOperation(op) || isTaskOperation(op) || isReportOperation(op) {
			continue
		}
		if op.PageContract == nil || !hasJSONMapping(op.PageContract.InputMapping) {
			diags = append(diags, diagnostic("entity_action_mapping_missing", spec.SeverityWarning, "entity action requires explicit row or selection inputMapping", op.FunctionID, "pageContract.inputMapping"))
			continue
		}
		if !mappingReferencesRowOrSelection(op.PageContract.InputMapping) {
			diags = append(diags, diagnostic("entity_action_mapping_context_missing", spec.SeverityWarning, "entity action inputMapping must reference row.* or selection.*", op.FunctionID, "pageContract.inputMapping"))
			continue
		}
		if !hasJSONMapping(op.PageContract.OutputMapping) {
			diags = append(diags, diagnostic("entity_action_output_mapping_missing", spec.SeverityWarning, "entity action requires explicit outputMapping before it can be added to an entity page", op.FunctionID, "pageContract.outputMapping"))
			continue
		}
		if !isJSONObject(op.PageContract.OutputMapping) {
			diags = append(diags, diagnostic("entity_action_output_mapping_invalid", spec.SeverityError, "entity action outputMapping must be a JSON object", op.FunctionID, "pageContract.outputMapping"))
			continue
		}
		binding := pageBinding(op, sanitizePageKey(firstNonEmpty(op.Operation, "action")), spec.BindingUsageAction, spec.PageExecutionModeSync)
		bindings = append(bindings, binding)
		actions = append(actions, rowAction{
			BindingID:    binding.ID,
			Label:        firstNonEmpty(op.Operation, op.FunctionID),
			Risk:         string(op.Risk),
			InputMapping: op.PageContract.InputMapping,
		})
		consumed = append(consumed, op.FunctionID)
	}
	return actions, bindings, diags, consumed
}

func dataTablePropsFromContract(bindingID string, contract *spec.PageContract, actions []rowAction) dataTableProps {
	props := dataTableProps{BindingID: bindingID}
	if contract == nil {
		return props
	}
	if pagination := contract.Pagination; pagination != nil {
		props.ItemsPath = strings.TrimSpace(pagination.ItemsPath)
		props.TotalPath = strings.TrimSpace(pagination.TotalPath)
		props.PageField = strings.TrimSpace(pagination.PageField)
		props.PageSizeField = strings.TrimSpace(pagination.PageSizeField)
	}
	if table := contract.Table; table != nil {
		props.ColumnsPath = strings.TrimSpace(table.ColumnsPath)
		props.Columns = columnsFromContract(table.Columns)
	}
	props.RowActions = actions
	return props
}

func columnsFromContract(columns []spec.PageTableColumnContract) []dataTableColumn {
	if len(columns) == 0 {
		return nil
	}
	out := make([]dataTableColumn, 0, len(columns))
	for _, column := range columns {
		key := strings.TrimSpace(column.Key)
		valuePath := strings.TrimSpace(column.ValuePath)
		if key == "" || valuePath == "" {
			continue
		}
		out = append(out, dataTableColumn{
			Title:     localizedFallback(column.Title, "zh-CN", key),
			DataIndex: valuePath,
			Key:       key,
		})
	}
	return out
}

func chartPanelPropsFromContract(bindingID string, contract *spec.PageContract) chartPanelProps {
	props := chartPanelProps{BindingID: bindingID}
	if contract == nil || contract.Report == nil {
		return props
	}
	report := contract.Report
	props.ChartType = strings.TrimSpace(report.ChartType)
	props.CategoryPath = strings.TrimSpace(report.CategoryPath)
	props.SeriesPath = strings.TrimSpace(report.SeriesPath)
	props.ValuePath = strings.TrimSpace(report.ValuePath)
	props.DataPath = firstNonEmpty(props.SeriesPath, props.ValuePath, props.CategoryPath)
	return props
}

func categoryForPage(resourceKey, pageKey, locale string) spec.PageCategorySpec {
	categoryKey := InferCategoryFromKey(firstNonEmpty(resourceKey, pageKey))
	return spec.PageCategorySpec{
		Key: categoryKey,
		Labels: spec.LocalizedText{
			locale: categoryKey,
		},
	}
}

func localizedTitle(op spec.OperationSpec, pageKey, locale string) spec.LocalizedText {
	return spec.LocalizedText{
		locale: firstNonEmpty(op.Operation, op.FunctionID, pageKey),
	}
}

func localizedFrom(labels spec.LocalizedText, locale string, fallback string) spec.LocalizedText {
	value := localizedFallback(labels, locale, fallback)
	return spec.LocalizedText{locale: value}
}

func localizedFallback(labels spec.LocalizedText, locale string, fallback string) string {
	if labels != nil {
		if value := strings.TrimSpace(labels[locale]); value != "" {
			return value
		}
		if value := strings.TrimSpace(labels["zh-CN"]); value != "" {
			return value
		}
		if value := strings.TrimSpace(labels["en-US"]); value != "" {
			return value
		}
		for _, value := range labels {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func functionFormProperties(fn spec.FunctionSpec) map[string]json.RawMessage {
	raw := []byte(fn.InputFormilySchema)
	if len(raw) == 0 {
		return nil
	}
	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Properties) == 0 {
		return nil
	}
	return parsed.Properties
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

func hasJSONMapping(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func isJSONObject(raw json.RawMessage) bool {
	var parsed map[string]json.RawMessage
	return json.Unmarshal(raw, &parsed) == nil
}

func mappingReferencesRowOrSelection(raw json.RawMessage) bool {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false
	}
	for _, value := range parsed {
		var path string
		if err := json.Unmarshal(value, &path); err != nil {
			continue
		}
		parts := normalizePath(path)
		if len(parts) > 0 && (parts[0] == "row" || parts[0] == "selection") {
			return true
		}
	}
	return false
}

func normalizePath(path string) []string {
	path = strings.TrimSpace(strings.TrimPrefix(path, "$."))
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

// FormatPageKey creates a page key from resource and page name.
func FormatPageKey(resourceKey, pageName string) string {
	return fmt.Sprintf("%s.%s", resourceKey, pageName)
}

// InferPageType returns the strongest page shape supported by explicit
// PageContract data. It never reads function names.
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
	for _, op := range ops {
		if isEntityQueryOperation(op) {
			return spec.PageTypeEntity
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
