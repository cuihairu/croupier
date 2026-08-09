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
	if !json.Valid(raw) {
		reasons["invalid_json"] = struct{}{}
		return
	}
	collectUnsupportedSchemaValue(raw, reasons)
}

func collectUnsupportedSchemaValue(raw json.RawMessage, reasons map[string]struct{}) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		if object == nil {
			return
		}
		if _, ok := object["oneOf"]; ok {
			reasons["oneOf"] = struct{}{}
		}
		if _, ok := object["anyOf"]; ok {
			reasons["anyOf"] = struct{}{}
		}
		if _, ok := object["discriminator"]; ok {
			reasons["discriminator"] = struct{}{}
		}
		var ref string
		if refRaw, ok := object["$ref"]; ok && json.Unmarshal(refRaw, &ref) == nil && isRemoteRef(ref) {
			reasons["remote_$ref"] = struct{}{}
		}
		for _, child := range object {
			collectUnsupportedSchemaValue(child, reasons)
		}
		return
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		for _, child := range items {
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
