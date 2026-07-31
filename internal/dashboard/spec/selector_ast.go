package spec

import "encoding/json"

// SelectorAST represents a typed selector for binding input/output mapping.
// It replaces the raw JSON object mapping with a structured, validated AST.
type SelectorAST struct {
	// Assignments maps target field paths to source expressions
	Assignments []Assignment `json:"assignments"`
}

// Assignment maps a target field to a source expression.
type Assignment struct {
	// Target is the field path in the function's JSON Schema
	Target string `json:"target"`

	// Source defines where to get the value
	Source SelectorSource `json:"source"`
}

// SelectorSource defines the source of a value.
type SelectorSource struct {
	// Type is the source type
	Type SelectorSourceType `json:"type"`

	// Path is the field path within the source (for form, row, selection, detail)
	Path string `json:"path,omitempty"`

	// Value is the literal value (for literal type)
	Value json.RawMessage `json:"value,omitempty"`

	// Transform optionally transforms the value
	Transform *TransformSpec `json:"transform,omitempty"`
}

// SelectorSourceType defines allowed source types.
type SelectorSourceType string

const (
	// SourceForm reads from form input fields
	SourceForm SelectorSourceType = "form"

	// SourceRow reads from the current row data (for row actions)
	SourceRow SelectorSourceType = "row"

	// SourceSelection reads from selected rows (for batch actions)
	SourceSelection SelectorSourceType = "selection"

	// SourceDetail reads from detail view data
	SourceDetail SelectorSourceType = "detail"

	// SourcePageState reads from page-level state
	SourcePageState SelectorSourceType = "page_state"

	// SourceLiteral uses a fixed literal value
	SourceLiteral SelectorSourceType = "literal"
)

// TransformSpec defines a value transformation.
type TransformSpec struct {
	// Type of transform
	Type TransformType `json:"type"`

	// Params for the transform
	Params map[string]json.RawMessage `json:"params,omitempty"`
}

// TransformType defines available transforms.
type TransformType string

const (
	// TransformDefault provides a default value if source is empty
	TransformDefault TransformType = "default"

	// TransformFormat formats a string with params
	TransformFormat TransformType = "format"

	// TransformConvert converts between types
	TransformConvert TransformType = "convert"

	// TransformPick picks a field from an object
	TransformPick TransformType = "pick"

	// TransformMap maps array items
	TransformMap TransformType = "map"
)

// SelectorValidationResult holds validation results for a selector.
type SelectorValidationResult struct {
	Valid       bool                `json:"valid"`
	Errors      []SelectorError     `json:"errors,omitempty"`
	Warnings    []SelectorWarning   `json:"warnings,omitempty"`
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

// Common selector error codes
const (
	ErrCodeInvalidPath      = "invalid_path"
	ErrCodeTypeMismatch     = "type_mismatch"
	ErrCodeMissingRequired  = "missing_required"
	ErrCodeInvalidSource    = "invalid_source"
	ErrCodeAmbiguousSource  = "ambiguous_source"
	ErrCodeStaleSelector    = "stale_selector"
)

// ValidateSelector validates a selector against a function's JSON Schema.
func ValidateSelector(selector SelectorAST, schema JSONSchema, context SelectorContext) SelectorValidationResult {
	result := SelectorValidationResult{Valid: true}

	// Parse schema
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, SelectorError{
			Code:    ErrCodeInvalidPath,
			Message: "invalid JSON Schema",
		})
		return result
	}

	// Get required fields
	required := make(map[string]bool)
	if req, ok := schemaObj["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}

	// Get properties
	properties, _ := schemaObj["properties"].(map[string]interface{})

	// Track which required fields are assigned
	assignedRequired := make(map[string]bool)

	for _, assignment := range selector.Assignments {
		// Validate target path exists in schema
		if properties != nil {
			if _, ok := properties[assignment.Target]; !ok {
				result.Valid = false
				result.Errors = append(result.Errors, SelectorError{
					Field:   assignment.Target,
					Code:    ErrCodeInvalidPath,
					Message: "target field not found in schema",
				})
				continue
			}
		}

		// Track required fields
		if required[assignment.Target] {
			assignedRequired[assignment.Target] = true
		}

		// Validate source type is allowed in context
		if !isSourceAllowed(assignment.Source.Type, context) {
			result.Valid = false
			result.Errors = append(result.Errors, SelectorError{
				Field:   assignment.Target,
				Code:    ErrCodeInvalidSource,
				Message: "source type not allowed in this context",
			})
		}

		// Validate path exists for source type
		if assignment.Source.Type != SourceLiteral && assignment.Source.Path != "" {
			if !validateSourcePath(assignment.Source.Type, assignment.Source.Path, context) {
				result.Warnings = append(result.Warnings, SelectorWarning{
					Field:   assignment.Target,
					Code:    ErrCodeStaleSelector,
					Message: "source path may not exist",
				})
			}
		}
	}

	// Check all required fields are assigned
	for field := range required {
		if !assignedRequired[field] {
			result.Valid = false
			result.Errors = append(result.Errors, SelectorError{
				Field:   field,
				Code:    ErrCodeMissingRequired,
				Message: "required field not assigned",
			})
		}
	}

	return result
}

// SelectorContext provides context for selector validation.
type SelectorContext struct {
	// PageType determines allowed source types
	PageType PageType

	// HasListView indicates if page has a list view
	HasListView bool

	// HasDetailView indicates if page has a detail view
	HasDetailView bool

	// IsRowAction indicates if this is a row-level action
	IsRowAction bool

	// IsBatchAction indicates if this is a batch action
	IsBatchAction bool
}

// isSourceAllowed checks if a source type is allowed in the given context.
func isSourceAllowed(sourceType SelectorSourceType, ctx SelectorContext) bool {
	switch sourceType {
	case SourceForm:
		return true // always allowed
	case SourceRow:
		return ctx.IsRowAction || ctx.HasDetailView
	case SourceSelection:
		return ctx.IsBatchAction
	case SourceDetail:
		return ctx.HasDetailView
	case SourcePageState:
		return true // always allowed
	case SourceLiteral:
		return true // always allowed
	default:
		return false
	}
}

// validateSourcePath checks if a source path is valid.
func validateSourcePath(sourceType SelectorSourceType, path string, ctx SelectorContext) bool {
	// Basic validation - in a real implementation this would check against
	// actual form schema, row schema, etc.
	return path != ""
}

// DefaultSelector creates a default selector that maps all form fields to function inputs.
func DefaultSelector(schema JSONSchema) SelectorAST {
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return SelectorAST{}
	}

	properties, ok := schemaObj["properties"].(map[string]interface{})
	if !ok {
		return SelectorAST{}
	}

	assignments := make([]Assignment, 0, len(properties))
	for field := range properties {
		assignments = append(assignments, Assignment{
			Target: field,
			Source: SelectorSource{
				Type: SourceForm,
				Path: field,
			},
		})
	}

	return SelectorAST{Assignments: assignments}
}
