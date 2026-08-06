package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func schemaSubsetDiagnostics(functionID string, field string, schema spec.JSONSchema) []spec.Diagnostic {
	if len(schema) == 0 {
		return nil
	}
	var root json.RawMessage = json.RawMessage(schema)
	if !json.Valid(root) {
		return []spec.Diagnostic{schemaSubsetDiagnostic(functionID, field, "schema JSON is invalid")}
	}
	reasons := map[string]struct{}{}
	collectUnsupportedSchemaFeatures(root, reasons)
	if len(reasons) == 0 {
		return nil
	}
	items := make([]string, 0, len(reasons))
	for reason := range reasons {
		items = append(items, reason)
	}
	sort.Strings(items)
	return []spec.Diagnostic{schemaSubsetDiagnostic(functionID, field, "JSON Schema uses unsupported generation features: "+strings.Join(items, ", "))}
}

func collectUnsupportedSchemaFeatures(raw json.RawMessage, reasons map[string]struct{}) {
	if len(raw) == 0 {
		return
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		reasons["invalid_json"] = struct{}{}
		return
	}
	collectUnsupportedSchemaValue(value, reasons)
}

func collectUnsupportedSchemaValue(value interface{}, reasons map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		if _, ok := typed["oneOf"]; ok {
			reasons["oneOf"] = struct{}{}
		}
		if _, ok := typed["anyOf"]; ok {
			reasons["anyOf"] = struct{}{}
		}
		if _, ok := typed["discriminator"]; ok {
			reasons["discriminator"] = struct{}{}
		}
		if ref, ok := typed["$ref"].(string); ok && isRemoteRef(ref) {
			reasons["remote_$ref"] = struct{}{}
		}
		for _, child := range typed {
			collectUnsupportedSchemaValue(child, reasons)
		}
	case []interface{}:
		for _, child := range typed {
			collectUnsupportedSchemaValue(child, reasons)
		}
	}
}

func isRemoteRef(ref string) bool {
	ref = strings.TrimSpace(strings.ToLower(ref))
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

func schemaSubsetDiagnostic(functionID string, field string, message string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:       "json_schema_generation_subset_unsupported",
		Severity:   spec.SeverityWarning,
		Message:    message,
		FunctionID: strings.TrimSpace(functionID),
		Field:      field,
	}
}
