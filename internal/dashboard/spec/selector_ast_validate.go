package spec

import (
	"encoding/json"
	"strings"
)

// ValidateSelector validates input selector assignments against JSON Schema.
func ValidateSelector(selector SelectorAST, schema JSONSchema, context SelectorContext) SelectorValidationResult {
	result := SelectorValidationResult{Valid: true}

	required, err := requiredPointers(schema)
	if err != nil {
		return invalidSelectorResult("invalid JSON Schema")
	}

	for _, assignment := range selector.Assignments {
		if !isJSONPointer(assignment.Target) {
			result.addError(assignment.Target, ErrCodeInvalidPath, "target must be a JSON Pointer")
			continue
		}
		if !schemaHasPath(schema, assignment.Target) {
			result.addError(assignment.Target, ErrCodeInvalidPath, "target field not found in schema")
			continue
		}
		delete(required, assignment.Target)

		if !isSourceAllowed(assignment.Source.Kind, context) {
			result.addError(assignment.Target, ErrCodeInvalidSource, "source kind not allowed in this context")
			continue
		}
		if !isSupportedTransform(assignment.Source) {
			result.addError(assignment.Target, ErrCodeInvalidSource, "selector transform is not supported for this source")
			continue
		}
		if t := assignment.Source.Transform; t != nil {
			if t.Type == TransformRename && !validRenameParams(t) {
				result.addError(assignment.Target, ErrCodeInvalidSource, "rename transform requires a non-empty field mapping of non-empty names")
				continue
			}
			if t.Type == TransformDefault && !validDefaultParams(t) {
				result.addError(assignment.Target, ErrCodeInvalidSource, "default transform requires a \"value\" param")
				continue
			}
		}
		validateInputSource(assignment, context, &result)
		if !isAssignable(schema, assignment.Target, assignment.Source, context) {
			result.addError(assignment.Target, ErrCodeTypeMismatch, "source type is not assignable to target")
		}
	}

	for field := range required {
		result.addError(field, ErrCodeMissingRequired, "required field not assigned")
	}
	return result
}

// isSupportedTransform gates transform/source combinations (U8):
//   - pick:      selection only (existing batch extraction)
//   - rename:    row/selection/page_state with empty path — the transform maps
//     the whole object's field names; a path would narrow the value to a
//     scalar and leave nothing to rename
//   - default:   any kind; it only wraps the resolved value with a fallback
func isSupportedTransform(source ValueSource) bool {
	if source.Transform == nil {
		return true
	}
	switch source.Transform.Type {
	case TransformPick:
		return source.Kind == SourceSelection
	case TransformRename:
		return (source.Kind == SourceRow || source.Kind == SourceSelection || source.Kind == SourcePageState) &&
			strings.TrimSpace(source.Path) == ""
	case TransformDefault:
		return true
	default:
		return false
	}
}

// isWholeObjectTransform reports whether the transform consumes the entire
// source object (path must stay empty for these).
func isWholeObjectTransform(t *TransformSpec) bool {
	return t != nil && t.Type == TransformRename
}

// validRenameParams requires a non-empty mapping of non-empty identifier-ish
// strings — empty tables or blank keys would silently produce empty objects.
func validRenameParams(t *TransformSpec) bool {
	if t == nil || len(t.Params) == 0 {
		return false
	}
	for from, raw := range t.Params {
		if strings.TrimSpace(from) == "" {
			return false
		}
		var to string
		if err := json.Unmarshal(raw, &to); err != nil || strings.TrimSpace(to) == "" {
			return false
		}
	}
	return true
}

// validDefaultParams requires the literal fallback under the "value" key.
func validDefaultParams(t *TransformSpec) bool {
	if t == nil {
		return false
	}
	_, ok := t.Params["value"]
	return ok
}

func bindingUsesRowSource(binding PageFunctionBinding) bool {
	if binding.Selectors == nil {
		return false
	}
	for _, assignment := range binding.Selectors.Input.Assignments {
		if assignment.Source.Kind == SourceRow {
			return true
		}
	}
	return false
}

func bindingUsesSelectionSource(binding PageFunctionBinding) bool {
	if binding.Selectors == nil {
		return false
	}
	for _, assignment := range binding.Selectors.Input.Assignments {
		if assignment.Source.Kind == SourceSelection {
			return true
		}
	}
	return false
}

func listQuerySchema(resource *ResourcePageSpec) JSONSchema {
	if resource == nil || resource.ListView == nil {
		return nil
	}
	properties := map[string]json.RawMessage{}
	for _, filter := range resource.ListView.Filters {
		if key := strings.TrimSpace(filter.Key); key != "" {
			properties[key] = schemaForFilter(filter)
		}
	}
	if resource.ListView.Pagination != nil && resource.ListView.Pagination.Enabled {
		properties["current"] = json.RawMessage(`{"type":"integer"}`)
		properties["pageSize"] = json.RawMessage(`{"type":"integer"}`)
	}
	if len(properties) == 0 {
		return nil
	}
	root := map[string]json.RawMessage{
		"type":       json.RawMessage(`"object"`),
		"properties": marshalRawObject(properties),
	}
	return JSONSchema(marshalRawObject(root))
}

func schemaForFilter(filter FilterSpec) json.RawMessage {
	switch strings.TrimSpace(filter.Type) {
	case "number":
		return json.RawMessage(`{"type":"number"}`)
	case "date", "daterange":
		return json.RawMessage(`{"type":"string"}`)
	default:
		return json.RawMessage(`{"type":"string"}`)
	}
}

func marshalRawObject(value map[string]json.RawMessage) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return raw
}
