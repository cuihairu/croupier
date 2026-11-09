package spec

import (
	"encoding/json"
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
	// TransformPick extracts one declared field from every selected row.
	TransformPick TransformType = "pick"
	// TransformRename maps the source object's field names onto the target
	// vocabulary via an explicit mapping table (U8). The output object contains
	// only mapped fields — a controlled whitelist, mirroring pick. For
	// selection sources the mapping applies to every selected row.
	TransformRename TransformType = "rename"
	// TransformDefault provides a fallback literal when the source resolves to
	// a missing value or null (U8). params must carry a "value" key.
	TransformDefault TransformType = "default"
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
