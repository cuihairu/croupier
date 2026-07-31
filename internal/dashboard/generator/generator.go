// Package generator creates conservative default PageSpec suggestions from
// normalized function capability contracts. It does not read registration-side
// UI, menu, mapping, pagination, column, task view, or chart view extensions.
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
	return GenerateOptions{DefaultLocale: "zh-CN"}
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
	BindingID string `json:"bindingId,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
}

type approvalStatusProps struct {
	BindingID string `json:"bindingId,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
}

// GenerateForResource creates default PageSpec candidates for every operation
// under a resource. CRUD ResourcePage generation waits for CapabilitySemantics;
// this function never guesses CRUD, columns, pagination, or row actions from
// names or raw schemas.
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
// CapabilitySemantics exists. Function registration cannot provide enough UI
// semantics to safely generate Resource CRUD pages.
func GenerateEntityPageForResource(spec.ResourceSpec, []spec.OperationSpec, GenerateOptions) (spec.GeneratedPageSpec, bool, []string) {
	return spec.GeneratedPageSpec{}, false, nil
}

// GenerateForOperation creates an operation, task, or report candidate from a
// single executable capability.
func GenerateForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	switch {
	case isApprovalOperation(op):
		return GenerateApprovalPageForOperation(op, opts)
	case isReportOperation(op):
		return GenerateReportPageForOperation(op, opts)
	case isTaskOperation(op):
		return GenerateTaskPageForOperation(op, opts)
	default:
		return GenerateOperationPageForOperation(op, opts)
	}
}

// GenerateOperationPageForOperation creates a standalone OperationPage that can
// be directly published when the executable contract is complete.
func GenerateOperationPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "main", spec.BindingUsageAction, executionModeForOperation(op))
	applyDefaultBindingMappings(&binding, opts.Functions[op.FunctionID])
	diags := assessBaseCandidate(op)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: resourceKey,
			Title:       localizedTitle(op, pageKey, opts.DefaultLocale),
			Category:    categoryForPage(resourceKey, pageKey, opts.DefaultLocale),
			Schema:      buildOperationPageSchema(pageKey, resourceKey, binding, opts),
			Bindings:    []spec.PageFunctionBinding{binding},
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
	applyDefaultBindingMappings(&binding, opts.Functions[op.FunctionID])
	diags := append(assessBaseCandidate(op), diagnostic(
		"task_semantics_missing",
		spec.SeverityWarning,
		"task capability requires status/events/result semantics before it can be safely published",
		op.FunctionID,
		"capability",
	))

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

// GenerateReportPageForOperation creates a report candidate. Without dataset,
// dimension, metric, and chart semantics it must be reviewed before publication.
func GenerateReportPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "report", spec.BindingUsageReport, spec.PageExecutionModeSync)
	applyDefaultBindingMappings(&binding, opts.Functions[op.FunctionID])
	diags := append(assessBaseCandidate(op), diagnostic(
		"report_semantics_missing",
		spec.SeverityWarning,
		"report capability requires dataset, dimension, metric, and chart semantics before it can be safely published",
		op.FunctionID,
		"capability",
	))

	root := consolePage(pageKey, resourceKey)
	root.Properties["form"] = rawNode(queryFormNode(binding, opts, ""))
	root.Properties["chart"] = rawNode(componentNode("ChartPanel", chartPanelPropsJSON(chartPanelProps{BindingID: binding.ID})))

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

// GenerateApprovalPageForOperation creates an approval candidate.
// Approval pages require explicit waiting state and status refresh rules.
func GenerateApprovalPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "approval", spec.BindingUsageAction, spec.PageExecutionModeSync)
	applyDefaultBindingMappings(&binding, opts.Functions[op.FunctionID])
	diags := append(assessBaseCandidate(op), diagnostic(
		"approval_semantics_missing",
		spec.SeverityWarning,
		"approval capability requires pending/approved/rejected/expired status semantics before it can be safely published",
		op.FunctionID,
		"capability",
	))

	root := consolePage(pageKey, resourceKey)
	root.Properties["form"] = rawNode(queryFormNode(binding, opts, ""))
	root.Properties["status"] = rawNode(componentNode("ApprovalStatus", approvalStatusPropsJSON(approvalStatusProps{BindingID: binding.ID})))
	root.Properties["result"] = rawNode(componentNode("ResultPanel", resultPanelPropsJSON(resultPanelProps{BindingID: binding.ID})))

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
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

func buildOperationPageSchema(pageKey string, resourceKey string, binding spec.PageFunctionBinding, opts GenerateOptions) spec.FormilySchema {
	root := consolePage(pageKey, resourceKey)
	root.Properties["form"] = rawNode(queryFormNode(binding, opts, ""))
	root.Properties["result"] = rawNode(componentNode("ResultPanel", resultPanelPropsJSON(resultPanelProps{BindingID: binding.ID})))
	return marshalSchema(root)
}

func operationPageKey(op spec.OperationSpec, opts GenerateOptions) string {
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
	return pageKey
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
	if strings.TrimSpace(op.ResourceKey) == "" {
		diags = append(diags, diagnostic("resource_missing", spec.SeverityWarning, "resource is missing; Page Studio must choose page grouping before publishing", op.FunctionID, "resourceKey"))
	}
	if strings.TrimSpace(op.Operation) == "" {
		diags = append(diags, diagnostic("operation_missing", spec.SeverityWarning, "operation is missing; Page Studio must name how this capability is used", op.FunctionID, "operation"))
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

func consolePage(pageKey string, resourceKey string) pageSchemaNode {
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

func approvalStatusPropsJSON(value approvalStatusProps) json.RawMessage {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func pageBinding(op spec.OperationSpec, suffix string, usage spec.PageBindingUsage, mode spec.PageExecutionMode) spec.PageFunctionBinding {
	return spec.PageFunctionBinding{
		ID:         bindingIDForOperationWithSuffix(op, suffix),
		FunctionID: op.FunctionID,
		Usage:      usage,
		Execution:  spec.PageBindingExecution{Mode: mode},
	}
}

func applyDefaultBindingMappings(binding *spec.PageFunctionBinding, fn spec.FunctionSpec) {
	binding.InputMapping = defaultInputMapping(fn)
	binding.OutputMapping = json.RawMessage(`{}`)
}

func defaultInputMapping(fn spec.FunctionSpec) json.RawMessage {
	properties := functionFormProperties(fn)
	if len(properties) == 0 {
		return json.RawMessage(`{}`)
	}
	mapping := make(map[string]string, len(properties))
	for key := range properties {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		mapping[key] = "values." + key
	}
	if len(mapping) == 0 {
		return json.RawMessage(`{}`)
	}
	raw, err := json.Marshal(mapping)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
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

func isApprovalOperation(op spec.OperationSpec) bool {
	return op.Execution == spec.FunctionExecutionApproval
}

func categoryForPage(resourceKey string, pageKey string, locale string) spec.PageCategorySpec {
	categoryKey := InferCategoryFromKey(firstNonEmpty(resourceKey, pageKey))
	return spec.PageCategorySpec{
		Key: categoryKey,
		Labels: spec.LocalizedText{
			locale: categoryKey,
		},
	}
}

func localizedTitle(op spec.OperationSpec, pageKey string, locale string) spec.LocalizedText {
	return spec.LocalizedText{
		locale: firstNonEmpty(op.Operation, op.FunctionID, pageKey),
	}
}

// functionFormProperties extracts form field properties from a function's schema.
// TODO(P2-2): Replace with JSON Schema + FormPresentationSpec based form generation.
func functionFormProperties(fn spec.FunctionSpec) map[string]json.RawMessage {
	raw := []byte(fn.InputSchema)
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

func qualityFromDiagnostics(diags []spec.Diagnostic) spec.GeneratedPageQuality {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return spec.GeneratedPageQualityBlocked
		}
	}
	if len(diags) > 0 {
		return spec.GeneratedPageQualityNeedsReview
	}
	return spec.GeneratedPageQualityReady
}

func operationQuality(op spec.OperationSpec, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	quality := qualityFromDiagnostics(diags)
	if quality == spec.GeneratedPageQualityReady {
		return spec.GeneratedPageQualityBasic
	}
	if quality == spec.GeneratedPageQualityNeedsReview && strings.TrimSpace(op.FunctionID) != "" {
		return spec.GeneratedPageQualityBasic
	}
	return quality
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
func FormatPageKey(resourceKey string, pageName string) string {
	return fmt.Sprintf("%s.%s", resourceKey, pageName)
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
