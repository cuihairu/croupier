package spec

import (
	"encoding/json"
	"strings"
)

// SelectorAST maps function input JSON Pointer targets to typed value sources.
type SelectorAST struct {
	Assignments []InputAssignment `json:"assignments"`
}

// InputAssignment maps a function input field to a source expression.
type InputAssignment struct {
	// Target is a JSON Pointer inside the function input JSON Schema.
	Target string      `json:"target"`
	Source ValueSource `json:"source"`
}

// OutputAssignment maps a function output JSON Pointer to page state.
type OutputAssignment struct {
	StateKey string            `json:"stateKey"`
	Source   string            `json:"source"`
	Shape    OutputResultShape `json:"shape"`
}

// OutputResultShape is the expected page-state shape for an output mapping.
type OutputResultShape string

const (
	OutputShapeScalar     OutputResultShape = "scalar"
	OutputShapeObject     OutputResultShape = "object"
	OutputShapeCollection OutputResultShape = "collection"
	OutputShapeTask       OutputResultShape = "task"
	OutputShapeDataset    OutputResultShape = "dataset"
)

// ValueSource defines where an input value comes from.
type ValueSource struct {
	Kind      ValueSourceKind `json:"kind"`
	Path      string          `json:"path,omitempty"`
	Key       string          `json:"key,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	Transform *TransformSpec  `json:"transform,omitempty"`
}

// ValueSourceKind defines allowed selector source kinds.
type ValueSourceKind string

const (
	SourceForm      ValueSourceKind = "form"
	SourceRow       ValueSourceKind = "row"
	SourceSelection ValueSourceKind = "selection"
	SourceDetail    ValueSourceKind = "detail"
	SourcePageState ValueSourceKind = "page_state"
	SourceLiteral   ValueSourceKind = "literal"
)

// TransformSpec defines a controlled value transformation.
type TransformSpec struct {
	Type   TransformType              `json:"type"`
	Params map[string]json.RawMessage `json:"params,omitempty"`
}

// TransformType defines available transforms.
type TransformType string

const (
	// TransformPick extracts one declared field from every selected row. It is
	// the only transform implemented by the controlled execute boundary.
	TransformPick TransformType = "pick"
)

// SelectorValidationResult holds validation results for a selector.
type SelectorValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []SelectorError   `json:"errors,omitempty"`
	Warnings []SelectorWarning `json:"warnings,omitempty"`
}

// SelectorError is a selector validation error.
type SelectorError struct {
	Field   string `json:"field"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// SelectorWarning is a selector validation warning.
type SelectorWarning struct {
	Field   string `json:"field"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

const (
	ErrCodeInvalidPath     = "invalid_path"
	ErrCodeTypeMismatch    = "type_mismatch"
	ErrCodeMissingRequired = "missing_required"
	ErrCodeInvalidSource   = "invalid_source"
	ErrCodeAmbiguousSource = "ambiguous_source"
	ErrCodeStaleSelector   = "stale_selector"
)

// SelectorContext provides page context for selector validation.
type SelectorContext struct {
	PageType PageType

	HasListView   bool
	HasDetailView bool
	IsRowAction   bool
	IsBatchAction bool

	FormSchema   JSONSchema
	RowSchema    JSONSchema
	DetailSchema JSONSchema
	PageState    map[string]JSONSchema
}

// SelectorContextForBinding builds the runtime selector context for a concrete
// page binding. It must stay aligned with PageRenderer execution contexts.
func SelectorContextForBinding(page PageSpec, binding PageFunctionBinding) SelectorContext {
	// composite 页：组合区块无独立 form/row schema——输入来源受限为
	// page_state（联动键）与 literal；输出为 OutputAssignment 写 state。
	if page.Type == PageTypeComposite && page.Composite != nil {
		return SelectorContext{
			PageType:   page.Type,
			PageState:  compositePageStateSchema(page.Composite),
			FormSchema: JSONSchema(""),
		}
	}
	ctx := SelectorContext{
		PageType:      page.Type,
		HasListView:   page.Resource != nil && page.Resource.ListView != nil,
		HasDetailView: page.Resource != nil && page.Resource.DetailView != nil,
		FormSchema:    FormSchemaForBinding(page, binding),
	}
	if page.Task != nil {
		taskIDStateKey := "taskId"
		if page.Task.TaskView != nil && strings.TrimSpace(page.Task.TaskView.TaskIDStateKey) != "" {
			taskIDStateKey = strings.TrimSpace(page.Task.TaskView.TaskIDStateKey)
		}
		ctx.PageState = map[string]JSONSchema{
			taskIDStateKey: JSONSchema(`{"type":"string"}`),
			"taskStatus":   JSONSchema(`{"type":"object","properties":{}}`),
			"taskEvents":   JSONSchema(`{"type":"array","items":{"type":"object","properties":{}}}`),
			"taskResult":   JSONSchema(`{"type":"object","properties":{}}`),
		}
	}
	if page.Resource != nil && page.Resource.ListView != nil {
		ctx.RowSchema = page.Resource.ListView.RowSchema
	}
	if page.Type == PageTypeResource && binding.Usage == BindingUsageAction {
		ctx.IsRowAction = bindingUsesRowSource(binding)
		ctx.IsBatchAction = bindingUsesSelectionSource(binding)
	}
	return ctx
}

// compositePageStateSchema 汇总组合页全部区块的联动 stateKey（宽松
// object schema：键存在即可，具体形状由函数契约校验兜底）。
func compositePageStateSchema(comp *CompositePageSpec) map[string]JSONSchema {
	out := map[string]JSONSchema{}
	if comp == nil {
		return out
	}
	seen := map[string]bool{}
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out[key] = JSONSchema(`{"type":"object"}`)
	}
	for i := range comp.Sections {
		sec := &comp.Sections[i]
		for _, dep := range sec.RefreshOn {
			add(dep)
		}
		add(sec.BindingID)
	}
	return out
}

// FormSchemaForBinding returns the PageSpec form that supplies SourceForm
// values for a binding.
func FormSchemaForBinding(page PageSpec, binding PageFunctionBinding) JSONSchema {
	switch {
	case page.Operation != nil && binding.Usage == BindingUsageAction && page.Operation.Form != nil:
		return page.Operation.Form.JSONSchema
	case page.Task != nil && binding.Usage == BindingUsageTask && page.Task.Form != nil:
		return page.Task.Form.JSONSchema
	case page.Report != nil && binding.Usage == BindingUsageReport && page.Report.QueryForm != nil:
		return page.Report.QueryForm.JSONSchema
	case page.Resource != nil && binding.ID == "create" && page.Resource.CreateForm != nil:
		return page.Resource.CreateForm.JSONSchema
	case page.Resource != nil && binding.ID == "update" && page.Resource.UpdateForm != nil:
		return page.Resource.UpdateForm.JSONSchema
	case page.Resource != nil && binding.Usage == BindingUsageQuery:
		return listQuerySchema(page.Resource)
	default:
		return nil
	}
}

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

func isSupportedTransform(source ValueSource) bool {
	if source.Transform == nil {
		return true
	}
	return source.Kind == SourceSelection && source.Transform.Type == TransformPick
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
		if strings.TrimSpace(source.Path) == "" {
			result.addError(assignment.Target, ErrCodeMissingRequired, "source path is required")
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

// SchemaFieldSnapshot is a comparable field view extracted from the supported
// JSON Schema subset.
type SchemaFieldSnapshot struct {
	Path     string `json:"path"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// SchemaFieldChangeType describes a field-level schema change.
type SchemaFieldChangeType string

const (
	SchemaFieldAdded           SchemaFieldChangeType = "added"
	SchemaFieldRemoved         SchemaFieldChangeType = "removed"
	SchemaFieldTypeChanged     SchemaFieldChangeType = "type_changed"
	SchemaFieldRequiredChanged SchemaFieldChangeType = "required_changed"
)

// SchemaFieldChange is a deterministic field-level schema diff item.
type SchemaFieldChange struct {
	Path       string                `json:"path"`
	ChangeType SchemaFieldChangeType `json:"changeType"`
	Old        *SchemaFieldSnapshot  `json:"old,omitempty"`
	New        *SchemaFieldSnapshot  `json:"new,omitempty"`
}

// FieldRenameCandidate is a conservative rename hint for Page Studio.
type FieldRenameCandidate struct {
	OldPath    string `json:"oldPath"`
	NewPath    string `json:"newPath"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

// SchemaDiffResult contains supported-subset schema diff output.
type SchemaDiffResult struct {
	Changes          []SchemaFieldChange    `json:"changes"`
	RenameCandidates []FieldRenameCandidate `json:"renameCandidates,omitempty"`
	Diagnostics      []Diagnostic           `json:"diagnostics,omitempty"`
}

// DiffJSONSchemaFields compares the supported object/property subset of JSON Schema.
func DiffJSONSchemaFields(oldSchema JSONSchema, newSchema JSONSchema) SchemaDiffResult {
	oldFields, oldOK := schemaFieldSnapshots(oldSchema)
	newFields, newOK := schemaFieldSnapshots(newSchema)
	result := SchemaDiffResult{}
	if !oldOK || !newOK {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code:     "schema_diff_invalid_schema",
			Severity: SeverityError,
			Message:  "schema diff requires valid JSON Schema objects",
		})
		return result
	}

	allPaths := make(map[string]struct{}, len(oldFields)+len(newFields))
	for path := range oldFields {
		allPaths[path] = struct{}{}
	}
	for path := range newFields {
		allPaths[path] = struct{}{}
	}
	paths := sortedMapKeys(allPaths)

	var removed []SchemaFieldSnapshot
	var added []SchemaFieldSnapshot
	for _, path := range paths {
		oldField, oldExists := oldFields[path]
		newField, newExists := newFields[path]
		switch {
		case !oldExists && newExists:
			newCopy := newField
			added = append(added, newField)
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldAdded,
				New:        &newCopy,
			})
		case oldExists && !newExists:
			oldCopy := oldField
			removed = append(removed, oldField)
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldRemoved,
				Old:        &oldCopy,
			})
		case oldField.Type != newField.Type:
			oldCopy := oldField
			newCopy := newField
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldTypeChanged,
				Old:        &oldCopy,
				New:        &newCopy,
			})
		case oldField.Required != newField.Required:
			oldCopy := oldField
			newCopy := newField
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldRequiredChanged,
				Old:        &oldCopy,
				New:        &newCopy,
			})
		}
	}
	result.RenameCandidates = fieldRenameCandidates(removed, added)
	return result
}

// SelectorStaleDiagnostics explains how selector paths are affected by schema changes.
func SelectorStaleDiagnostics(
	selector SelectorAST,
	outputAssignments []OutputAssignment,
	oldInputSchema JSONSchema,
	newInputSchema JSONSchema,
	oldOutputSchema JSONSchema,
	newOutputSchema JSONSchema,
) []Diagnostic {
	inputDiff := DiffJSONSchemaFields(oldInputSchema, newInputSchema)
	outputDiff := DiffJSONSchemaFields(oldOutputSchema, newOutputSchema)
	diags := append([]Diagnostic{}, inputDiff.Diagnostics...)
	diags = append(diags, outputDiff.Diagnostics...)
	if len(inputDiff.Diagnostics) > 0 || len(outputDiff.Diagnostics) > 0 {
		return diags
	}

	assignedTargets := map[string]struct{}{}
	for _, assignment := range selector.Assignments {
		assignedTargets[assignment.Target] = struct{}{}
		if !schemaHasPath(newInputSchema, assignment.Target) {
			diags = append(diags, staleDiagnostic("selector_target_stale", "selector target no longer exists in input schema", "input."+assignment.Target))
			continue
		}
		if hasChange(inputDiff.Changes, assignment.Target, SchemaFieldTypeChanged) {
			diags = append(diags, staleDiagnostic("selector_target_type_stale", "selector target type changed in input schema", "input."+assignment.Target))
		}
	}

	required, err := requiredPointers(newInputSchema)
	if err == nil {
		for _, path := range sortedMapKeys(required) {
			if _, ok := assignedTargets[path]; !ok {
				diags = append(diags, staleDiagnostic("selector_required_stale", "new required input field is not assigned", "input."+path))
			}
		}
	}

	for _, assignment := range outputAssignments {
		if !schemaHasPath(newOutputSchema, assignment.Source) {
			diags = append(diags, staleDiagnostic("selector_output_source_stale", "output source no longer exists in output schema", "output."+assignment.Source))
			continue
		}
		if hasChange(outputDiff.Changes, assignment.Source, SchemaFieldTypeChanged) {
			diags = append(diags, staleDiagnostic("selector_output_type_stale", "output source type changed in output schema", "output."+assignment.Source))
		}
	}

	for _, candidate := range inputDiff.RenameCandidates {
		if _, ok := assignedTargets[candidate.OldPath]; ok {
			diags = append(diags, Diagnostic{
				Code:     "selector_field_rename_candidate",
				Severity: SeverityInfo,
				Message:  "input selector may be updated from " + candidate.OldPath + " to " + candidate.NewPath,
				Field:    "input." + candidate.OldPath,
			})
		}
	}
	for _, candidate := range outputDiff.RenameCandidates {
		for _, assignment := range outputAssignments {
			if assignment.Source == candidate.OldPath {
				diags = append(diags, Diagnostic{
					Code:     "selector_output_rename_candidate",
					Severity: SeverityInfo,
					Message:  "output selector may be updated from " + candidate.OldPath + " to " + candidate.NewPath,
					Field:    "output." + candidate.OldPath,
				})
			}
		}
	}
	return diags
}

// DefaultSelector creates a default selector that maps each top-level form
// field to the same function input JSON Pointer.
func DefaultSelector(schema JSONSchema) SelectorAST {
	var schemaObj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return SelectorAST{}
	}

	properties, ok := rawObject(schemaObj["properties"])
	if !ok {
		return SelectorAST{}
	}

	keys := make([]string, 0, len(properties))
	for field := range properties {
		keys = append(keys, field)
	}
	sortStrings(keys)

	assignments := make([]InputAssignment, 0, len(keys))
	for _, field := range keys {
		pointer := "/" + escapeJSONPointerToken(field)
		assignments = append(assignments, InputAssignment{
			Target: pointer,
			Source: ValueSource{
				Kind: SourceForm,
				Path: pointer,
			},
		})
	}
	return SelectorAST{Assignments: assignments}
}

func isOutputShape(shape OutputResultShape) bool {
	switch shape {
	case OutputShapeScalar, OutputShapeObject, OutputShapeCollection, OutputShapeTask, OutputShapeDataset:
		return true
	default:
		return false
	}
}

func isJSONPointer(path string) bool {
	return path == "" || strings.HasPrefix(path, "/")
}

func jsonPointerTokens(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}

func rawObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func schemaFieldSnapshots(schema JSONSchema) (map[string]SchemaFieldSnapshot, bool) {
	if len(schema) == 0 {
		return map[string]SchemaFieldSnapshot{}, true
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(schema, &root); err != nil {
		return nil, false
	}
	out := map[string]SchemaFieldSnapshot{}
	collectSchemaFieldSnapshots("", root, requiredSet(root), out)
	return out, true
}

func collectSchemaFieldSnapshots(prefix string, node map[string]json.RawMessage, required map[string]struct{}, out map[string]SchemaFieldSnapshot) {
	properties, ok := rawObject(node["properties"])
	if !ok {
		return
	}
	for _, name := range sortedRawMapKeys(properties) {
		propRaw := properties[name]
		prop, ok := rawObject(propRaw)
		if !ok {
			continue
		}
		path := prefix + "/" + escapeJSONPointerToken(name)
		_, isRequired := required[name]
		out[path] = SchemaFieldSnapshot{
			Path:     path,
			Type:     schemaTypeFromNode(prop),
			Required: isRequired,
		}
		collectSchemaFieldSnapshots(path, prop, requiredSet(prop), out)
	}
}

func requiredSet(node map[string]json.RawMessage) map[string]struct{} {
	out := map[string]struct{}{}
	var required []string
	if raw := node["required"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &required); err == nil {
			for _, field := range required {
				out[field] = struct{}{}
			}
		}
	}
	return out
}

func schemaTypeFromNode(node map[string]json.RawMessage) string {
	rawType, ok := node["type"]
	if !ok {
		return ""
	}
	var schemaType string
	if err := json.Unmarshal(rawType, &schemaType); err == nil {
		return schemaType
	}
	var schemaTypes []string
	if err := json.Unmarshal(rawType, &schemaTypes); err == nil && len(schemaTypes) == 1 {
		return schemaTypes[0]
	}
	return ""
}

func fieldRenameCandidates(removed []SchemaFieldSnapshot, added []SchemaFieldSnapshot) []FieldRenameCandidate {
	candidates := []FieldRenameCandidate{}
	for _, oldField := range removed {
		for _, newField := range added {
			if oldField.Type == "" || oldField.Type != newField.Type {
				continue
			}
			if oldField.Required != newField.Required {
				continue
			}
			if parentPointer(oldField.Path) != parentPointer(newField.Path) {
				continue
			}
			candidates = append(candidates, FieldRenameCandidate{
				OldPath:    oldField.Path,
				NewPath:    newField.Path,
				Confidence: "low",
				Reason:     "same parent, type, and required flag",
			})
		}
	}
	return candidates
}

func hasChange(changes []SchemaFieldChange, path string, changeType SchemaFieldChangeType) bool {
	for _, change := range changes {
		if change.Path == path && change.ChangeType == changeType {
			return true
		}
	}
	return false
}

func staleDiagnostic(code string, message string, field string) Diagnostic {
	return Diagnostic{
		Code:     code,
		Severity: SeverityError,
		Message:  message,
		Field:    field,
	}
}

func parentPointer(path string) string {
	if path == "" {
		return ""
	}
	index := strings.LastIndex(path, "/")
	if index <= 0 {
		return ""
	}
	return path[:index]
}

func sortedMapKeys[T any](input map[string]T) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func sortedRawMapKeys(input map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// ---------------------------------------------------------------------------
// CapabilitySemantics semantic validation
// ---------------------------------------------------------------------------

// SemanticContext provides semantic context for selector validation.
type SemanticContext struct {
	// IdentityField is the resource identity field name (e.g., "player_id")
	IdentityField string

	// PageFieldName is the pagination page field name (e.g., "page")
	PageFieldName string

	// PageSizeFieldName is the pagination page size field name (e.g., "page_size")
	PageSizeFieldName string

	// ItemsFieldName is the items array field name (e.g., "items")
	ItemsFieldName string

	// TotalFieldName is the total count field name (e.g., "total")
	TotalFieldName string
}

// ValidateSelectorSemantics validates that selector assignments are consistent
// with the resource's CapabilitySemantics. This ensures that:
// - Identity fields are properly mapped
// - Collection query fields are properly mapped
// - CRUD operations use correct identity inputs
func ValidateSelectorSemantics(
	selector SelectorAST,
	semantics *SemanticContext,
	pageType PageType,
	bindingUsage PageBindingUsage,
) SelectorValidationResult {
	result := SelectorValidationResult{Valid: true}

	if semantics == nil {
		return result
	}

	// Check identity field mapping for resource operations
	if pageType == PageTypeResource && semantics.IdentityField != "" {
		validateIdentityMapping(selector, semantics, bindingUsage, &result)
	}

	// Check collection query mapping
	if bindingUsage == BindingUsageQuery {
		validateCollectionMapping(selector, semantics, &result)
	}

	return result
}

func validateIdentityMapping(
	selector SelectorAST,
	semantics *SemanticContext,
	bindingUsage PageBindingUsage,
	result *SelectorValidationResult,
) {
	// For row actions (update, delete), identity must come from row source
	if bindingUsage == BindingUsageAction {
		hasRowIdentity := false
		for _, assignment := range selector.Assignments {
			if assignment.Source.Kind == SourceRow && assignment.Source.Path != "" {
				// Check if this maps to the identity field
				if isIdentityPath(assignment.Source.Path, semantics.IdentityField) {
					hasRowIdentity = true
					break
				}
			}
		}
		if !hasRowIdentity && semantics.IdentityField != "" {
			result.addWarning("", ErrCodeMissingRequired,
				"row action should map identity field from row source")
		}
	}
}

func validateCollectionMapping(
	selector SelectorAST,
	semantics *SemanticContext,
	result *SelectorValidationResult,
) {
	// Collection query should map pagination fields
	if semantics.PageFieldName != "" || semantics.PageSizeFieldName != "" {
		hasPagination := false
		for _, assignment := range selector.Assignments {
			if assignment.Source.Kind == SourceForm {
				if assignment.Target == "/"+semantics.PageFieldName ||
					assignment.Target == "/"+semantics.PageSizeFieldName {
					hasPagination = true
					break
				}
			}
		}
		if !hasPagination && semantics.PageFieldName != "" {
			result.addWarning("", ErrCodeMissingRequired,
				"collection query should map pagination fields")
		}
	}
}

func isIdentityPath(sourcePath string, identityField string) bool {
	if identityField == "" {
		return false
	}
	// Check if source path points to identity field
	// e.g., "/player_id" or "/id"
	return sourcePath == "/"+identityField
}
