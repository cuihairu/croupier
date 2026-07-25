// Package normalizer converts raw function descriptors from SDK, OpenAPI,
// or DB templates into the canonical strong-typed spec models defined in
// internal/dashboard/spec.
//
// The normalizer is the single place where:
//   - locale keys are normalized (e.g. "zh" -> "zh-CN")
//   - missing fields produce diagnostics
//   - Formily schema is derived from JSON Schema when needed
//   - ResourceSpec and OperationSpec are extracted from function metadata
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

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// DescriptorInput is the raw input to the normalizer. It can come from
// SDK registration, OpenAPI import, or DB descriptor templates.
type DescriptorInput struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
	Description string `json:"description"`

	// JSON Schema strings (may be empty)
	InputSchema  string `json:"input_schema,omitempty"`
	OutputSchema string `json:"output_schema,omitempty"`

	// v2 semantic fields
	Category      string `json:"category,omitempty"`
	Entity        string `json:"entity,omitempty"`
	Operation     string `json:"operation,omitempty"`
	OperationKind string `json:"operation_kind,omitempty"`
	Placement     string `json:"placement,omitempty"`
	PageHint      string `json:"page_hint,omitempty"`
	Risk          string `json:"risk,omitempty"`
	Enabled       bool   `json:"enabled"`

	// Multi-language display fields (may use short keys like "zh", "en")
	CategoryDisplay  map[string]string `json:"category_display,omitempty"`
	EntityDisplay    map[string]string `json:"entity_display,omitempty"`
	OperationDisplay map[string]string `json:"operation_display,omitempty"`
	DisplayName      map[string]string `json:"display_name,omitempty"`
	SummaryMap       map[string]string `json:"summary_map,omitempty"`
	DescriptionMap   map[string]string `json:"description_map,omitempty"`

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

	// 2. Normalize locale keys
	categoryDisplay := normalizeLocaleKeys(input.CategoryDisplay)
	entityDisplay := normalizeLocaleKeys(input.EntityDisplay)
	operationDisplay := normalizeLocaleKeys(input.OperationDisplay)
	displayName := normalizeLocaleKeys(input.DisplayName)
	summaryMap := normalizeLocaleKeys(input.SummaryMap)
	descriptionMap := normalizeLocaleKeys(input.DescriptionMap)
	if displayName == nil {
		displayName = localizedFallback(firstNonEmpty(input.Summary, input.Description, input.ID))
	}
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
		var parsed map[string]interface{}
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
		var parsed map[string]interface{}
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

	// 4. Validate v2 semantic fields
	operationKind := normalizeOperationKind(input.OperationKind)
	placement := normalizePlacement(input.Placement)

	if operationKind == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "operation_kind_missing",
			Severity:   spec.SeverityWarning,
			Message:    "operation_kind is missing; page cannot be auto-generated",
			FunctionID: input.ID,
			Field:      "operation_kind",
		})
	}

	if placement == "" {
		diags = append(diags, spec.Diagnostic{
			Code:       "placement_missing",
			Severity:   spec.SeverityWarning,
			Message:    "placement is missing; page cannot be auto-generated",
			FunctionID: input.ID,
			Field:      "placement",
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
		DisplayName:        displayName,
		Summary:            summaryMap,
		Description:        descriptionMap,
		Category:           strings.TrimSpace(input.Category),
		CategoryDisplay:    categoryDisplay,
		Entity:             strings.TrimSpace(input.Entity),
		EntityDisplay:      entityDisplay,
		Operation:          strings.TrimSpace(input.Operation),
		OperationDisplay:   operationDisplay,
		OperationKind:      operationKind,
		Placement:          placement,
		PageHint:           strings.TrimSpace(input.PageHint),
		Risk:               normalizeRisk(input.Risk),
		Tags:               input.Tags,
		Diagnostics:        diags,
	}

	// 6. Build ResourceSpec if entity is present
	var resource *spec.ResourceSpec
	if fn.Entity != "" {
		resource = &spec.ResourceSpec{
			Key:    fn.Entity,
			Labels: entityDisplay,
			Category: spec.ResourceCategorySpec{
				Key:    fn.Category,
				Labels: categoryDisplay,
			},
		}
		// Infer category from entity key if not explicitly set
		if resource.Category.Key == "" {
			resource.Category.Key = inferCategoryFromKey(fn.Entity)
		}
	}

	// 7. Build OperationSpec if we have enough info
	var operation *spec.OperationSpec
	if fn.Entity != "" || fn.Operation != "" {
		operation = &spec.OperationSpec{
			FunctionID:   fn.ID,
			ResourceKey:  fn.Entity,
			Operation:    fn.Operation,
			Kind:         operationKind,
			Placement:    placement,
			Labels:       operationDisplay,
			Risk:         fn.Risk,
			Enabled:      fn.Enabled,
			PageContract: input.PageContract,
		}
		// Add diagnostics to operation if fields missing
		if operationKind == "" {
			operation.Diagnostics = append(operation.Diagnostics, spec.Diagnostic{
				Code:     "operation_kind_missing",
				Severity: spec.SeverityWarning,
				Message:  "operation_kind is required for page generation",
			})
		}
		if placement == "" {
			operation.Diagnostics = append(operation.Diagnostics, spec.Diagnostic{
				Code:     "placement_missing",
				Severity: spec.SeverityWarning,
				Message:  "placement is required for page generation",
			})
		}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

// normalizeOperationKind validates and normalizes the operation kind.
func normalizeOperationKind(kind string) spec.OperationKind {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch spec.OperationKind(kind) {
	case spec.OperationKindList, spec.OperationKindGet, spec.OperationKindCreate,
		spec.OperationKindUpdate, spec.OperationKindDelete, spec.OperationKindAction,
		spec.OperationKindTask, spec.OperationKindReport:
		return spec.OperationKind(kind)
	default:
		return ""
	}
}

// normalizePlacement validates and normalizes the placement.
// Supports canonical camelCase and common aliases.
func normalizePlacement(p string) spec.OperationPlacement {
	p = strings.TrimSpace(p)
	// Direct match first (case-sensitive for camelCase)
	switch spec.OperationPlacement(p) {
	case spec.PlacementQuery, spec.PlacementTableData, spec.PlacementDetailData,
		spec.PlacementRowAction, spec.PlacementDetailAction, spec.PlacementToolbarAction,
		spec.PlacementBatchAction, spec.PlacementStandalone:
		return spec.OperationPlacement(p)
	}

	// Case-insensitive aliases
	switch strings.ToLower(strings.ReplaceAll(p, "_", "")) {
	case "query":
		return spec.PlacementQuery
	case "tabledata":
		return spec.PlacementTableData
	case "detaildata":
		return spec.PlacementDetailData
	case "rowaction":
		return spec.PlacementRowAction
	case "detailaction":
		return spec.PlacementDetailAction
	case "toolbaraction":
		return spec.PlacementToolbarAction
	case "batchaction":
		return spec.PlacementBatchAction
	case "standalone":
		return spec.PlacementStandalone
	}

	// snake_case aliases
	switch strings.ToLower(p) {
	case "table_data":
		return spec.PlacementTableData
	case "detail_data":
		return spec.PlacementDetailData
	case "row_action":
		return spec.PlacementRowAction
	case "detail_action":
		return spec.PlacementDetailAction
	case "toolbar_action":
		return spec.PlacementToolbarAction
	case "batch_action":
		return spec.PlacementBatchAction
	}

	// kebab-case aliases
	switch strings.ToLower(p) {
	case "table-data":
		return spec.PlacementTableData
	case "detail-data":
		return spec.PlacementDetailData
	case "row-action":
		return spec.PlacementRowAction
	case "detail-action":
		return spec.PlacementDetailAction
	case "toolbar-action":
		return spec.PlacementToolbarAction
	case "batch-action":
		return spec.PlacementBatchAction
	}

	return ""
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
func deriveFormilySchema(jsonSchema map[string]interface{}) spec.FormilySchema {
	// Create a Formily-compatible schema wrapper
	formily := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	// Extract properties from JSON Schema
	if props, ok := jsonSchema["properties"].(map[string]interface{}); ok {
		formilyProps := map[string]interface{}{}
		for name, prop := range props {
			if propMap, ok := prop.(map[string]interface{}); ok {
				field := map[string]interface{}{
					"type":        getOrDefault(propMap, "type", "string"),
					"title":       getOrDefault(propMap, "title", name),
					"x-component": mapTypeToComponent(getOrDefault(propMap, "type", "string")),
				}
				if desc, ok := propMap["description"].(string); ok {
					field["description"] = desc
				}
				if enum, ok := propMap["enum"].([]interface{}); ok {
					field["enum"] = enum
					field["x-component"] = "Select"
				}
				formilyProps[name] = field
			}
		}
		formily["properties"] = formilyProps
	}

	// Handle required fields
	if required, ok := jsonSchema["required"].([]interface{}); ok {
		formily["required"] = required
	}

	b, _ := json.Marshal(formily)
	return spec.FormilySchema(b)
}

// minimalPayloadFormilySchema returns a Formily schema with a single "payload" field.
func minimalPayloadFormilySchema() spec.FormilySchema {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"payload": map[string]interface{}{
				"type":        "object",
				"title":       "Payload",
				"x-component": "Input.TextArea",
				"x-component-props": map[string]interface{}{
					"rows": 6,
				},
			},
		},
	}
	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

// getOrDefault extracts a string value from a map or returns a default.
func getOrDefault(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
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
