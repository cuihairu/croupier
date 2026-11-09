package spec

import (
	"encoding/json"
	"strings"
)

// ValidateOutputAssignments validates output selectors against a function output schema.
func ValidateOutputAssignments(assignments []OutputAssignment, schema JSONSchema) SelectorValidationResult {
	result := SelectorValidationResult{Valid: true}
	for _, assignment := range assignments {
		stateKey := strings.TrimSpace(assignment.StateKey)
		if stateKey == "" {
			result.addError(assignment.Source, ErrCodeMissingRequired, "output stateKey is required")
		}
		if !isJSONPointer(assignment.Source) {
			result.addError(assignment.Source, ErrCodeInvalidPath, "output source must be a JSON Pointer")
			continue
		}
		if len(schema) > 0 && !schemaHasPath(schema, assignment.Source) {
			result.addError(assignment.Source, ErrCodeInvalidPath, "output source not found in schema")
		}
		if !isOutputShape(assignment.Shape) {
			result.addError(stateKey, ErrCodeInvalidSource, "output shape is invalid")
			continue
		}
		if len(schema) > 0 && !outputShapeMatchesSchema(assignment.Shape, schema, assignment.Source) {
			result.addError(assignment.Source, ErrCodeTypeMismatch, "output source schema does not match output shape")
		}
	}
	return result
}

func validateInputSource(assignment InputAssignment, ctx SelectorContext, result *SelectorValidationResult) {
	source := assignment.Source
	switch source.Kind {
	case SourceLiteral:
		return
	case SourcePageState:
		if strings.TrimSpace(source.Key) == "" {
			result.addError(assignment.Target, ErrCodeMissingRequired, "page_state source key is required")
			return
		}
		if source.Path != "" && !isJSONPointer(source.Path) {
			result.addError(assignment.Target, ErrCodeInvalidPath, "page_state source path must be a JSON Pointer")
			return
		}
		if !validatePageStatePath(source, ctx) {
			result.addError(assignment.Target, ErrCodeInvalidPath, "page_state source path does not exist")
		}
	default:
		// rename consumes the whole source object (or every selected row);
		// an empty path is its contract, not a missing one (U8).
		if strings.TrimSpace(source.Path) == "" && !isWholeObjectTransform(source.Transform) {
			result.addError(assignment.Target, ErrCodeMissingRequired, "source path is required")
			return
		}
		if isWholeObjectTransform(source.Transform) {
			return
		}
		if !isJSONPointer(source.Path) {
			result.addError(assignment.Target, ErrCodeInvalidPath, "source path must be a JSON Pointer")
			return
		}
		if !validateSourcePath(source.Kind, source.Path, ctx) {
			result.addError(assignment.Target, ErrCodeInvalidPath, "source path does not exist")
		}
	}
}

func invalidSelectorResult(message string) SelectorValidationResult {
	return SelectorValidationResult{
		Valid: false,
		Errors: []SelectorError{{
			Code:    ErrCodeInvalidPath,
			Message: message,
		}},
	}
}

func (result *SelectorValidationResult) addError(field string, code string, message string) {
	result.Valid = false
	result.Errors = append(result.Errors, SelectorError{
		Field:   field,
		Code:    code,
		Message: message,
	})
}

func (result *SelectorValidationResult) addWarning(field string, code string, message string) {
	result.Warnings = append(result.Warnings, SelectorWarning{
		Field:   field,
		Code:    code,
		Message: message,
	})
}

func isSourceAllowed(sourceKind ValueSourceKind, ctx SelectorContext) bool {
	switch sourceKind {
	case SourceForm:
		return true
	case SourceRow:
		return ctx.IsRowAction || ctx.HasDetailView
	case SourceSelection:
		return ctx.IsBatchAction
	case SourceDetail:
		return ctx.HasDetailView
	case SourcePageState:
		return true
	case SourceLiteral:
		return true
	default:
		return false
	}
}

func validateSourcePath(sourceKind ValueSourceKind, path string, ctx SelectorContext) bool {
	var schema JSONSchema
	switch sourceKind {
	case SourceForm:
		schema = ctx.FormSchema
	case SourceRow, SourceSelection:
		schema = ctx.RowSchema
	case SourceDetail:
		schema = ctx.DetailSchema
	default:
		return true
	}
	if len(schema) == 0 {
		return true
	}
	return schemaHasPath(schema, path)
}

func validatePageStatePath(source ValueSource, ctx SelectorContext) bool {
	if len(ctx.PageState) == 0 {
		return true
	}
	schema, ok := ctx.PageState[strings.TrimSpace(source.Key)]
	if !ok {
		return false
	}
	if source.Path == "" {
		return true
	}
	return schemaHasPath(schema, source.Path)
}

func isAssignable(targetSchema JSONSchema, targetPath string, source ValueSource, ctx SelectorContext) bool {
	if source.Kind == SourceLiteral {
		return true
	}
	targetType, ok := schemaTypeAtPath(targetSchema, targetPath)
	if !ok || targetType == "" {
		return true
	}
	if source.Kind == SourceSelection && source.Transform != nil && source.Transform.Type == TransformPick {
		return targetType == "array"
	}
	// rename reshapes the whole source object (selection → array of objects);
	// default wraps the resolved value without changing its shape.
	if source.Transform != nil && source.Transform.Type == TransformRename {
		if source.Kind == SourceSelection {
			return targetType == "array"
		}
		return targetType == "object"
	}
	if source.Transform != nil {
		return true
	}
	sourceSchema, sourcePath, ok := sourceSchemaAndPath(source, ctx)
	if !ok || len(sourceSchema) == 0 {
		return true
	}
	sourceType, ok := schemaTypeAtPath(sourceSchema, sourcePath)
	if !ok || sourceType == "" {
		return true
	}
	return jsonSchemaTypeAssignable(targetType, sourceType)
}

func sourceSchemaAndPath(source ValueSource, ctx SelectorContext) (JSONSchema, string, bool) {
	switch source.Kind {
	case SourceForm:
		return ctx.FormSchema, source.Path, true
	case SourceRow, SourceSelection:
		return ctx.RowSchema, source.Path, true
	case SourceDetail:
		return ctx.DetailSchema, source.Path, true
	case SourcePageState:
		schema, ok := ctx.PageState[strings.TrimSpace(source.Key)]
		return schema, source.Path, ok
	default:
		return nil, "", false
	}
}

func requiredPointers(schema JSONSchema) (map[string]struct{}, error) {
	var schemaObj map[string]json.RawMessage
	if len(schema) == 0 {
		return map[string]struct{}{}, nil
	}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return nil, err
	}

	var required []string
	if raw := schemaObj["required"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &required); err != nil {
			return nil, err
		}
	}

	out := make(map[string]struct{}, len(required))
	for _, field := range required {
		out["/"+escapeJSONPointerToken(field)] = struct{}{}
	}
	return out, nil
}

// schemaHasPath checks if a JSON Schema contains a property at the JSON Pointer.
func schemaHasPath(schema JSONSchema, path string) bool {
	if !isJSONPointer(path) {
		return false
	}
	if path == "" {
		return true
	}

	var schemaObj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return true
	}
	if len(schemaObj) == 0 {
		return true
	}

	current := schemaObj
	for _, token := range jsonPointerTokens(path) {
		properties, ok := rawObject(current["properties"])
		if !ok {
			return false
		}
		nextRaw, ok := properties[token]
		if !ok {
			return false
		}
		nextObj, ok := rawObject(nextRaw)
		if !ok {
			return false
		}
		current = nextObj
	}
	return true
}

func schemaTypeAtPath(schema JSONSchema, path string) (string, bool) {
	node, ok := schemaNodeAtPath(schema, path)
	if !ok {
		return "", false
	}
	rawType, ok := node["type"]
	if !ok {
		return "", true
	}
	var schemaType string
	if err := json.Unmarshal(rawType, &schemaType); err == nil {
		return schemaType, true
	}
	var schemaTypes []string
	if err := json.Unmarshal(rawType, &schemaTypes); err == nil && len(schemaTypes) == 1 {
		return schemaTypes[0], true
	}
	return "", true
}

func schemaNodeAtPath(schema JSONSchema, path string) (map[string]json.RawMessage, bool) {
	if !isJSONPointer(path) {
		return nil, false
	}
	var current map[string]json.RawMessage
	if err := json.Unmarshal(schema, &current); err != nil {
		return nil, false
	}
	if path == "" {
		return current, true
	}
	for _, token := range jsonPointerTokens(path) {
		properties, ok := rawObject(current["properties"])
		if !ok {
			return nil, false
		}
		nextRaw, ok := properties[token]
		if !ok {
			return nil, false
		}
		nextObj, ok := rawObject(nextRaw)
		if !ok {
			return nil, false
		}
		current = nextObj
	}
	return current, true
}

func jsonSchemaTypeAssignable(targetType string, sourceType string) bool {
	if targetType == sourceType {
		return true
	}
	return targetType == "number" && sourceType == "integer"
}

func outputShapeMatchesSchema(shape OutputResultShape, schema JSONSchema, source string) bool {
	sourceType, ok := schemaTypeAtPath(schema, source)
	if !ok || sourceType == "" {
		return true
	}
	switch shape {
	case OutputShapeCollection, OutputShapeDataset:
		return sourceType == "array"
	case OutputShapeObject, OutputShapeTask:
		return sourceType == "object"
	case OutputShapeScalar:
		return sourceType != "array" && sourceType != "object"
	default:
		return true
	}
}
