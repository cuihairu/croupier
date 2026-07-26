// Package generator creates conservative PageSpec suggestions from normalized
// resource capabilities. It does not infer CRUD pages, task pages, reports,
// menu categories, labels, or component placement from function registration.
package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	DefaultLocale string
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

// GenerateForResource creates one conservative operation-page candidate per
// capability. Page Studio must decide final page type, layout, labels, and
// binding usage before publishing.
func GenerateForResource(resource spec.ResourceSpec, opts GenerateOptions) []spec.GeneratedPageSpec {
	if len(resource.Operations) == 0 {
		return nil
	}
	pages := make([]spec.GeneratedPageSpec, 0, len(resource.Operations))
	for _, op := range resource.Operations {
		pages = append(pages, GenerateForOperation(op, opts))
	}
	return pages
}

// GenerateForOperation creates a conservative Operation Page candidate.
func GenerateForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
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
	categoryKey := InferCategoryFromKey(firstNonEmpty(resourceKey, pageKey))
	binding := pageBinding(op, "main", spec.BindingUsageAction, spec.PageExecutionModeSync)
	quality, diags := assessConservativeCandidate(op)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeOperation,
			ResourceKey: resourceKey,
			Title:       title,
			Category: spec.PageCategorySpec{
				Key: categoryKey,
				Labels: spec.LocalizedText{
					opts.DefaultLocale: categoryKey,
				},
			},
			Schema:   buildOperationPageSchema(resourceKey, binding.ID),
			Bindings: []spec.PageFunctionBinding{binding},
		},
		Quality:     quality,
		Diagnostics: diags,
	}
}

func buildOperationPageSchema(resourceKey, bindingID string) spec.FormilySchema {
	root := componentNode("ConsolePage", consolePageProps{
		SchemaVersion: "formily-page:1",
		ResourceKey:   strings.TrimSpace(resourceKey),
	})
	root.Properties["form"] = componentNode("QueryForm", bindingComponentProps{BindingID: bindingID})
	root.Properties["result"] = emptyComponentNode("ResultPanel")
	return marshalSchema(root)
}

func assessConservativeCandidate(op spec.OperationSpec) (string, []spec.Diagnostic) {
	var diags []spec.Diagnostic
	if strings.TrimSpace(op.FunctionID) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "function_id_missing",
			Severity: spec.SeverityError,
			Message:  "functionId is required",
			Field:    "functionId",
		})
	}
	if strings.TrimSpace(op.ResourceKey) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "resource_missing",
			Severity:   spec.SeverityWarning,
			Message:    "resource is missing; Page Studio must choose page grouping before publishing",
			FunctionID: op.FunctionID,
			Field:      "resourceKey",
		})
	}
	if strings.TrimSpace(op.Operation) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "operation_missing",
			Severity:   spec.SeverityWarning,
			Message:    "operation is missing; Page Studio must name how this capability is used",
			FunctionID: op.FunctionID,
			Field:      "operation",
		})
	}
	if op.PageContract == nil {
		diags = append(diags, spec.Diagnostic{
			Code:       "page_contract_missing",
			Severity:   spec.SeverityWarning,
			Message:    "pageContract is missing; generated page is only a draft candidate",
			FunctionID: op.FunctionID,
			Field:      "pageContract",
		})
	} else {
		if !hasJSONMapping(op.PageContract.InputMapping) {
			diags = append(diags, spec.Diagnostic{
				Code:       "binding_input_mapping_missing",
				Severity:   spec.SeverityWarning,
				Message:    "inputMapping is missing; Page Studio must confirm request mapping",
				FunctionID: op.FunctionID,
				Field:      "pageContract.inputMapping",
			})
		}
		if !hasJSONMapping(op.PageContract.OutputMapping) {
			diags = append(diags, spec.Diagnostic{
				Code:       "binding_output_mapping_missing",
				Severity:   spec.SeverityWarning,
				Message:    "outputMapping is missing; Page Studio must confirm response mapping",
				FunctionID: op.FunctionID,
				Field:      "pageContract.outputMapping",
			})
		}
	}
	return qualityFromDiagnostics(diags), diags
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

func qualityFromDiagnostics(diags []spec.Diagnostic) string {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return "blocked"
		}
	}
	if len(diags) > 0 {
		return "needs_review"
	}
	return "needs_review"
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

// InferPageType intentionally returns Operation by default. Final page type is
// a Page Studio decision, not a function-registration inference.
func InferPageType(ops []spec.OperationSpec) spec.PageType {
	if len(ops) == 0 {
		return spec.PageTypeOperation
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
