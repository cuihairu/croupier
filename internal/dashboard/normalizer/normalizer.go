// Package normalizer converts raw function descriptors from SDK, OpenAPI,
// or DB templates into the canonical strong-typed spec models defined in
// internal/dashboard/spec.
//
// The normalizer is the single place where:
//   - missing fields produce diagnostics
//   - Formily schema is derived from JSON Schema when needed
//   - ResourceSpec and OperationSpec candidates are extracted from capability metadata
//
// The normalizer does NOT:
//   - persist anything to the database
//   - generate PageSpec (that is the generator's job)
//   - infer menu structure (that is the console API's job)
package normalizer

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

type jsonObject map[string]json.RawMessage

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// DescriptorInput is the raw input to the normalizer. It can come from
// SDK registration, OpenAPI Source parsing, or DB descriptor templates.
type DescriptorInput struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
	Description string `json:"description"`

	// JSON Schema strings (may be empty)
	InputSchema  string `json:"input_schema,omitempty"`
	OutputSchema string `json:"output_schema,omitempty"`

	Resource   string `json:"resource,omitempty"`
	Operation  string `json:"operation,omitempty"`
	Capability string `json:"capability,omitempty"`
	Execution  string `json:"execution,omitempty"`
	Risk       string `json:"risk,omitempty"`
	Permission string `json:"permission,omitempty"`
	Enabled    bool   `json:"enabled"`

	SummaryMap     map[string]string `json:"summary_map,omitempty"`
	DescriptionMap map[string]string `json:"description_map,omitempty"`

	// Tags
	Tags []string `json:"tags,omitempty"`
}

// NormalizerResult holds the normalized specs and diagnostics.
type NormalizerResult struct {
	Function    spec.FunctionSpec   `json:"function"`
	Resource    *spec.ResourceSpec  `json:"resource,omitempty"`
	Operation   *spec.OperationSpec `json:"operation,omitempty"`
	Diagnostics []spec.Diagnostic   `json:"diagnostics,omitempty"`
}

// Normalize converts a single DescriptorInput into normalized specs.
// It returns diagnostics for missing or invalid fields.
func Normalize(input DescriptorInput) NormalizerResult {
	var diags []spec.Diagnostic

	// 1. Validate required fields
	if strings.TrimSpace(input.ID) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "id_missing",
			Severity: spec.SeverityError,
			Message:  "Function ID is required",
		})
		return NormalizerResult{Diagnostics: diags}
	}

	// 2. Normalize catalog text keys. These are not runtime menu labels.
	summaryMap := normalizeLocaleKeys(input.SummaryMap)
	descriptionMap := normalizeLocaleKeys(input.DescriptionMap)
	if summaryMap == nil {
		summaryMap = localizedFallback(input.Summary)
	}
	if descriptionMap == nil {
		descriptionMap = localizedFallback(input.Description)
	}

	// 3. Parse and validate JSON Schema
	var inputSchema spec.JSONSchema
	if input.InputSchema != "" {
		var parsed jsonObject
		if err := json.Unmarshal([]byte(input.InputSchema), &parsed); err != nil {
			diags = append(diags, spec.Diagnostic{
				Code:     "input_schema_invalid",
				Severity: spec.SeverityError,
				Message:  "input_schema is not valid JSON: " + err.Error(),
			})
		} else {
			inputSchema = spec.JSONSchema(input.InputSchema)
		}
	} else {
		diags = append(diags, spec.Diagnostic{
			Code:     "input_schema_missing",
			Severity: spec.SeverityWarning,
			Message:  "No input_schema defined; function form will be a single payload field",
		})
	}

	var outputSchema spec.JSONSchema
	if input.OutputSchema != "" {
		var parsed jsonObject
		if err := json.Unmarshal([]byte(input.OutputSchema), &parsed); err != nil {
			diags = append(diags, spec.Diagnostic{
				Code:     "output_schema_invalid",
				Severity: spec.SeverityError,
				Message:  "output_schema is not valid JSON: " + err.Error(),
			})
		} else {
			outputSchema = spec.JSONSchema(input.OutputSchema)
		}
	}

	resourceKey := strings.TrimSpace(input.Resource)
	operationKey := strings.TrimSpace(input.Operation)
	capability, capabilityDiagnostics := normalizeCapability(input.Capability, input.ID)
	diags = append(diags, capabilityDiagnostics...)
	execution, executionDiagnostics := normalizeExecution(input.Execution, capability, input.ID)
	diags = append(diags, executionDiagnostics...)
	if resourceKey == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "resource_missing",
			Severity:   spec.SeverityWarning,
			Message:    "resource is missing; the function remains executable but cannot be grouped into a resource candidate",
			FunctionID: input.ID,
			Field:      "resource",
		})
	}
	if operationKey == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "operation_missing",
			Severity:   spec.SeverityWarning,
			Message:    "operation is missing; Page Studio must name how this capability is used",
			FunctionID: input.ID,
			Field:      "operation",
		})
	}

	// 5. Build FunctionSpec
	fn := spec.FunctionSpec{
		ID:           input.ID,
		Version:      input.Version,
		Enabled:      input.Enabled,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Summary:      summaryMap,
		Description:  descriptionMap,
		Resource:     resourceKey,
		Operation:    operationKey,
		Capability:   capability,
		Execution:    execution,
		Risk:         normalizeRisk(input.Risk),
		Permission:   strings.TrimSpace(input.Permission),
		Tags:         input.Tags,
		Diagnostics:  diags,
	}

	// 6. Build ResourceSpec candidate if resource is present.
	var resource *spec.ResourceSpec
	if fn.Resource != "" {
		categoryKey := inferCategoryFromKey(fn.Resource)
		resource = &spec.ResourceSpec{
			Key:    fn.Resource,
			Labels: localizedFallback(fn.Resource),
			Category: spec.ResourceCategorySpec{
				Key:    categoryKey,
				Labels: localizedFallback(categoryKey),
			},
		}
	}

	// 7. Build OperationSpec candidate if we have resource or operation info.
	var operation *spec.OperationSpec
	if fn.Resource != "" || fn.Operation != "" {
		operation = &spec.OperationSpec{
			FunctionID:  fn.ID,
			ResourceKey: fn.Resource,
			Operation:   fn.Operation,
			Capability:  fn.Capability,
			Execution:   fn.Execution,
			Risk:        fn.Risk,
			Permission:  fn.Permission,
			Enabled:     fn.Enabled,
		}
		operation.Diagnostics = append(operation.Diagnostics, diags...)
	}

	return NormalizerResult{
		Function:    fn,
		Resource:    resource,
		Operation:   operation,
		Diagnostics: diags,
	}
}

// NormalizeBatch normalizes multiple inputs and groups them by resource.
func NormalizeBatch(inputs []DescriptorInput) ([]NormalizerResult, map[string]*spec.ResourceSpec) {
	results := make([]NormalizerResult, 0, len(inputs))
	resources := map[string]*spec.ResourceSpec{}

	for _, input := range inputs {
		result := Normalize(input)
		results = append(results, result)

		// Merge into resource map
		if result.Resource != nil {
			key := result.Resource.Key
			if existing, ok := resources[key]; ok {
				// Merge operations
				if result.Operation != nil {
					existing.Operations = append(existing.Operations, *result.Operation)
				}
				// Merge diagnostics
				existing.Diagnostics = append(existing.Diagnostics, result.Diagnostics...)
			} else {
				// Create new resource copy
				r := *result.Resource
				if result.Operation != nil {
					r.Operations = []spec.OperationSpec{*result.Operation}
				}
				r.Diagnostics = result.Diagnostics
				resources[key] = &r
			}
		}
	}

	return results, resources
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// normalizeLocaleKeys converts short locale keys to full keys.
// "zh" -> "zh-CN", "en" -> "en-US". Keys already in full form are kept.
func normalizeLocaleKeys(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch strings.ToLower(k) {
		case "zh", "zh-cn", "zh_cn":
			out["zh-CN"] = v
		case "en", "en-us", "en_us":
			out["en-US"] = v
		default:
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func localizedFallback(value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return map[string]string{
		"zh-CN": value,
		"en-US": value,
	}
}

// normalizeRisk validates and normalizes the risk level.
func normalizeRisk(r string) spec.RiskLevel {
	r = strings.TrimSpace(strings.ToLower(r))
	switch spec.RiskLevel(r) {
	case spec.RiskSafe, spec.RiskWarning, spec.RiskHigh, spec.RiskDanger:
		return spec.RiskLevel(r)
	default:
		return ""
	}
}

func normalizeCapability(value string, functionID string) (spec.CapabilityKind, []spec.Diagnostic) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", nil
	}
	capability := spec.CapabilityKind(value)
	if spec.IsValidCapabilityKind(capability) {
		return capability, nil
	}
	return "", []spec.Diagnostic{{
		Code:       "capability_invalid",
		Severity:   spec.SeverityError,
		Message:    "capability must be one of collection_query, item_query, create, update, delete, action, task, report",
		FunctionID: functionID,
		Field:      "capability",
	}}
}

func normalizeExecution(value string, capability spec.CapabilityKind, functionID string) (spec.FunctionExecution, []spec.Diagnostic) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		if capability == spec.CapabilityTask {
			return spec.FunctionExecutionTask, nil
		}
		return spec.FunctionExecutionSync, nil
	}
	execution := spec.FunctionExecution(value)
	if spec.IsValidFunctionExecution(execution) {
		return execution, nil
	}
	return "", []spec.Diagnostic{{
		Code:       "execution_invalid",
		Severity:   spec.SeverityError,
		Message:    "execution must be one of sync, task, approval",
		FunctionID: functionID,
		Field:      "execution",
	}}
}

// inferCategoryFromKey extracts the category from a key like "player.ban" -> "player".
func inferCategoryFromKey(key string) string {
	key = strings.TrimSpace(key)
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}

// getOrDefault extracts a string value from a map or returns a default.
func getOrDefault(m jsonObject, key, defaultVal string) string {
	if v := getString(m, key); v != "" {
		return v
	}
	return defaultVal
}

func asJSONObject(raw json.RawMessage) (jsonObject, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var out jsonObject
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func getString(m jsonObject, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func rawJSONString(value string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func rawJSONNumber(value int) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func rawJSONObject(value jsonObject) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
