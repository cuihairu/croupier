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

type jsonObject map[string]interface{}

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
	Risk       string `json:"risk,omitempty"`
	Permission string `json:"permission,omitempty"`
	Enabled    bool   `json:"enabled"`

	SummaryMap     map[string]string `json:"summary_map,omitempty"`
	DescriptionMap map[string]string `json:"description_map,omitempty"`

	// Tags
	Tags []string `json:"tags,omitempty"`

	// Optional data contract for generator quality. This is not UI.
	PageContract *spec.PageContract `json:"page_contract,omitempty"`
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
	var inputFormilySchema spec.FormilySchema
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
			// Derive Formily schema from JSON Schema
			inputFormilySchema = deriveFormilySchema(parsed)
		}
	} else {
		diags = append(diags, spec.Diagnostic{
			Code:     "input_schema_missing",
			Severity: spec.SeverityWarning,
			Message:  "No input_schema defined; function form will be a single payload field",
		})
		// Generate minimal Formily schema with single payload field
		inputFormilySchema = minimalPayloadFormilySchema()
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
		ID:                 input.ID,
		Version:            input.Version,
		Enabled:            input.Enabled,
		InputSchema:        inputSchema,
		InputFormilySchema: inputFormilySchema,
		OutputSchema:       outputSchema,
		Summary:            summaryMap,
		Description:        descriptionMap,
		Resource:           resourceKey,
		Operation:          operationKey,
		Risk:               normalizeRisk(input.Risk),
		Permission:         strings.TrimSpace(input.Permission),
		Tags:               input.Tags,
		Diagnostics:        diags,
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
			FunctionID:   fn.ID,
			ResourceKey:  fn.Resource,
			Operation:    fn.Operation,
			Risk:         fn.Risk,
			Permission:   fn.Permission,
			Enabled:      fn.Enabled,
			PageContract: input.PageContract,
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

// inferCategoryFromKey extracts the category from a key like "player.ban" -> "player".
func inferCategoryFromKey(key string) string {
	key = strings.TrimSpace(key)
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}

// deriveFormilySchema creates a basic Formily schema from a JSON Schema.
// This is a minimal implementation; a full implementation would recursively
// convert JSON Schema properties to Formily fields.
func deriveFormilySchema(jsonSchema jsonObject) spec.FormilySchema {
	// Create a Formily-compatible schema wrapper
	formily := jsonObject{
		"type":       "object",
		"properties": jsonObject{},
	}

	// Extract properties from JSON Schema
	if props, ok := asJSONObject(jsonSchema["properties"]); ok {
		formilyProps := jsonObject{}
		for name, prop := range props {
			if propMap, ok := asJSONObject(prop); ok {
				field := jsonObject{
					"type":        getOrDefault(propMap, "type", "string"),
					"title":       getOrDefault(propMap, "title", name),
					"x-component": mapTypeToComponent(getOrDefault(propMap, "type", "string")),
				}
				if desc, ok := propMap["description"].(string); ok {
					field["description"] = desc
				}
				if enum, ok := propMap["enum"].([]any); ok {
					field["enum"] = enum
					field["x-component"] = "Select"
				}
				formilyProps[name] = field
			}
		}
		formily["properties"] = formilyProps
	}

	// Handle required fields
	if required, ok := jsonSchema["required"].([]any); ok {
		formily["required"] = required
	}

	b, _ := json.Marshal(formily)
	return spec.FormilySchema(b)
}

// minimalPayloadFormilySchema returns a Formily schema with a single "payload" field.
func minimalPayloadFormilySchema() spec.FormilySchema {
	schema := jsonObject{
		"type": "object",
		"properties": jsonObject{
			"payload": jsonObject{
				"type":        "object",
				"title":       "Payload",
				"x-component": "Input.TextArea",
				"x-component-props": jsonObject{
					"rows": 6,
				},
			},
		},
	}
	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

// getOrDefault extracts a string value from a map or returns a default.
func getOrDefault(m jsonObject, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

func asJSONObject(value interface{}) (jsonObject, bool) {
	switch v := value.(type) {
	case jsonObject:
		return v, true
	case map[string]any:
		return jsonObject(v), true
	default:
		return nil, false
	}
}

// mapTypeToComponent maps JSON Schema types to Formily component names.
func mapTypeToComponent(typeStr string) string {
	switch typeStr {
	case "string":
		return "Input"
	case "number", "integer":
		return "NumberPicker"
	case "boolean":
		return "Switch"
	case "array":
		return "ArrayTable"
	case "object":
		return "ObjectContainer"
	default:
		return "Input"
	}
}
