package generator

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func requiredFieldNames(root map[string]json.RawMessage) []string {
	if len(root) == 0 || len(root["required"]) == 0 {
		return nil
	}
	var required []string
	if err := json.Unmarshal(root["required"], &required); err != nil {
		return nil
	}
	return required
}

func schemaHasNoRequiredFields(schema spec.JSONSchema) bool {
	root := parseJSONObject(json.RawMessage(schema))
	return len(requiredFieldNames(root)) == 0
}

func schemaCanBindBySingleRequiredField(schema spec.JSONSchema, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	root := parseJSONObject(json.RawMessage(schema))
	if len(root) == 0 {
		return false
	}
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return false
	}
	if _, ok := properties[field]; !ok {
		return false
	}
	for _, required := range requiredFieldNames(root) {
		if required != field {
			return false
		}
	}
	return true
}

func schemaCanBindBySingleRequiredArrayField(schema spec.JSONSchema, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	root := parseJSONObject(json.RawMessage(schema))
	if len(root) == 0 {
		return false
	}
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return false
	}
	prop, ok := properties[field]
	if !ok || schemaTypeFromRaw(prop) != "array" {
		return false
	}
	for _, required := range requiredFieldNames(root) {
		if required != field {
			return false
		}
	}
	return true
}

func schemaTypeFromRaw(raw json.RawMessage) string {
	obj := parseJSONObject(raw)
	return rawString(obj["type"])
}

func topLevelPointerToken(pointer string) string {
	if !strings.HasPrefix(pointer, "/") {
		return ""
	}
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	if len(tokens) != 1 || strings.TrimSpace(tokens[0]) == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(tokens[0], "~1", "/"), "~0", "~")
}

func replaceSelectorSource(selector *spec.SelectorAST, target string, source spec.ValueSource) bool {
	if selector == nil {
		return false
	}
	target = strings.TrimSpace(target)
	for i := range selector.Assignments {
		if selector.Assignments[i].Target == target {
			selector.Assignments[i].Source = source
			return true
		}
	}
	return false
}

func inferDataType(raw json.RawMessage) string {
	prop := parseJSONObject(raw)
	typeStr := rawString(prop["type"])
	format := rawString(prop["format"])
	switch typeStr {
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "string":
		switch format {
		case "date-time", "date":
			return "datetime"
		default:
			return "string"
		}
	default:
		return "string"
	}
}

func inferFilterType(raw json.RawMessage) string {
	prop := parseJSONObject(raw)
	typeStr := rawString(prop["type"])
	format := rawString(prop["format"])
	if len(prop["enum"]) > 0 {
		return "select"
	}
	switch typeStr {
	case "integer", "number":
		return "number"
	case "string":
		switch format {
		case "date", "date-time":
			return "date"
		default:
			return "text"
		}
	default:
		return "text"
	}
}

func enumOptionsFromSchema(raw json.RawMessage) []spec.EnumOption {
	prop := parseJSONObject(raw)
	if len(prop["enum"]) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(prop["enum"], &values); err != nil {
		return nil
	}
	options := make([]spec.EnumOption, 0, len(values))
	for _, value := range values {
		options = append(options, spec.EnumOption{
			Value: value,
			Label: spec.LocalizedText{"zh-CN": value},
		})
	}
	return options
}
