package registrationguard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var forbiddenPresentationFields = map[string]struct{}{
	"categorydisplay":     {},
	"category-display":    {},
	"category_display":    {},
	"displayname":         {},
	"display-name":        {},
	"display_name":        {},
	"entitydisplay":       {},
	"entity-display":      {},
	"entity_display":      {},
	"formily":             {},
	"inputmapping":        {},
	"input-mapping":       {},
	"input_mapping":       {},
	"layout":              {},
	"menu":                {},
	"operationdisplay":    {},
	"operation-display":   {},
	"operation_display":   {},
	"operationkind":       {},
	"operation-kind":      {},
	"operation_kind":      {},
	"outputmapping":       {},
	"output-mapping":      {},
	"output_mapping":      {},
	"pagecontract":        {},
	"page-contract":       {},
	"page_contract":       {},
	"pagehint":            {},
	"page-hint":           {},
	"page_hint":           {},
	"pageschema":          {},
	"page-schema":         {},
	"page_schema":         {},
	"placement":           {},
	"pagination":          {},
	"route":               {},
	"routes":              {},
	"table":               {},
	"tablecolumns":        {},
	"table-columns":       {},
	"table_columns":       {},
	"ui":                  {},
	"x-category-display":  {},
	"x_category_display":  {},
	"x-components":        {},
	"x_components":        {},
	"x-columns":           {},
	"x-display-name":      {},
	"x_display_name":      {},
	"x-entity-display":    {},
	"x_entity_display":    {},
	"x-formily":           {},
	"x_formily":           {},
	"x-layout":            {},
	"x_layout":            {},
	"x-labels":            {},
	"x_labels":            {},
	"x-menu":              {},
	"x-operation-display": {},
	"x_operation_display": {},
	"x-operation-kind":    {},
	"x_operation_kind":    {},
	"x-output-mapping":    {},
	"x_output_mapping":    {},
	"x-page":              {},
	"x-page-contract":     {},
	"x_page_contract":     {},
	"x-page-hint":         {},
	"x_page_hint":         {},
	"x-page-schema":       {},
	"x_page_schema":       {},
	"x-pagination":        {},
	"x_pagination":        {},
	"x-placement":         {},
	"x_placement":         {},
	"x-route":             {},
	"x-routes":            {},
	"x-table":             {},
	"x-table-columns":     {},
	"x_table":             {},
	"x_table_columns":     {},
	"x-title":             {},
	"x_title":             {},
	"x-ui":                {},
	"x_ui":                {},
	"x-input-mapping":     {},
	"x_input_mapping":     {},
}

var forbiddenRegistrationExtensionFields = map[string]struct{}{
	"columns": {},
	"labels":  {},
	"title":   {},
}

// PresentationViolation identifies a rejected registration field and its
// request-relative location.
type PresentationViolation struct {
	Field    string
	Location string
}

// ForbiddenPresentationField reports whether key tries to attach presentation,
// navigation, or page-composition concerns to a function capability contract.
func ForbiddenPresentationField(key string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(key))
	if normalized == "" {
		return "", false
	}
	normalized = strings.ReplaceAll(normalized, " ", "")
	if _, ok := forbiddenPresentationFields[normalized]; ok {
		return normalized, true
	}
	return normalized, false
}

// FindPresentationViolation checks an entire FunctionContract registration
// payload. Extensions are checked as registration metadata, while schemas are
// recursively scanned only for presentation extensions so ordinary payload
// property names remain valid.
func FindPresentationViolation(extensions map[string]string, inputSchema, outputSchema string) (PresentationViolation, bool) {
	extensionKeys := make([]string, 0, len(extensions))
	for key := range extensions {
		extensionKeys = append(extensionKeys, key)
	}
	sort.Strings(extensionKeys)
	for _, key := range extensionKeys {
		if field, ok := ForbiddenRegistrationExtensionField(key); ok {
			return PresentationViolation{Field: field, Location: "extensions." + key}, true
		}
	}
	if field, path, ok := ScanJSON(inputSchema); ok {
		return PresentationViolation{Field: field, Location: "input_schema" + path[1:]}, true
	}
	if field, path, ok := ScanJSON(outputSchema); ok {
		return PresentationViolation{Field: field, Location: "output_schema" + path[1:]}, true
	}
	return PresentationViolation{}, false
}

// ForbiddenRegistrationExtensionField applies the stricter registration
// boundary to free-form extension maps. Bare title, labels, and columns are
// invalid there, while a standard OpenAPI field such as info.title remains
// valid when the source document is scanned structurally.
func ForbiddenRegistrationExtensionField(key string) (string, bool) {
	if field, ok := ForbiddenPresentationField(key); ok {
		return field, true
	}
	normalized := strings.TrimSpace(strings.ToLower(key))
	normalized = strings.ReplaceAll(normalized, " ", "")
	_, forbidden := forbiddenRegistrationExtensionFields[normalized]
	return normalized, forbidden
}

// ScanJSON decodes a raw JSON document and reports the first forbidden
// presentation extension found anywhere in it. Empty or non-JSON input
// reports nothing — malformed schemas are rejected by their own validators.
func ScanJSON(raw string) (field string, path string, found bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false
	}
	var value interface{}
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return "", "", false
	}
	return ScanJSONValue(value, "$")
}

// ScanJSONValue walks a decoded JSON value and reports the first key that
// smuggles presentation, navigation, or page-composition concerns into a
// capability contract, together with its JSON path.
//
// Only extension-style keys (x-*/x_*) and unambiguous UI keywords such as
// "formily" are enforced: plain data property names like "menu" or "table"
// may legitimately appear in payload schemas and are left alone. Keys are
// visited in sorted order so the reported field is deterministic.
func ScanJSONValue(value interface{}, path string) (field string, location string, found bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := path + "." + key
			if field, ok := jsonPresentationExtension(key); ok {
				return field, childPath, true
			}
			if field, p, ok := ScanJSONValue(v[key], childPath); ok {
				return field, p, true
			}
		}
	case []interface{}:
		for i, child := range v {
			if field, p, ok := ScanJSONValue(child, fmt.Sprintf("%s[%d]", path, i)); ok {
				return field, p, true
			}
		}
	}
	return "", "", false
}

// jsonPresentationExtension reports whether a JSON key is a presentation
// extension smuggled into contract data. Bare payload property names are not
// flagged here; only extension-style keys and known UI keywords are.
func jsonPresentationExtension(key string) (string, bool) {
	trimmed := strings.TrimSpace(key)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "x-") && !strings.HasPrefix(lower, "x_") && lower != "formily" {
		return "", false
	}
	return ForbiddenPresentationField(trimmed)
}
