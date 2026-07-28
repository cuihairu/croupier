package function

import "encoding/json"

// FunctionFormV2Response represents the function form configuration.
// This is the new canonical response that only includes Formily Schema.
type FunctionFormV2Response struct {
	// Schema is the Formily JSON Schema for the function's input form.
	Schema json.RawMessage `json:"schema"`

	// FormSource indicates where the schema came from.
	FormSource string `json:"formSource"` // custom_metadata/config_file_override/generated_default/none

	// FormSourceDetail provides a human-readable description of the source.
	FormSourceDetail string `json:"formSourceDetail,omitempty"`

	// HasDefault indicates whether a default schema was derived.
	HasDefault bool `json:"hasDefault"`

	// Diagnostics contains validation warnings/info.
	Diagnostics []FunctionFormDiagnostic `json:"diagnostics,omitempty"`
}

// FunctionFormV2UpdateRequest represents a request to update function form.
type FunctionFormV2UpdateRequest struct {
	ID     string          `json:"id" binding:"required"`
	Schema json.RawMessage `json:"schema" binding:"required"`
}

// FunctionFormV2RollbackRequest represents a request to rollback function form.
type FunctionFormV2RollbackRequest struct {
	ID      string `json:"id" binding:"required"`
	Version int    `json:"version" binding:"required"`
}

// FunctionFormDiagnostic represents a diagnostic message for function form.
type FunctionFormDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error/warning/info
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
}

// IsFormilySchema performs basic validation on a Formily schema.
func IsFormilySchema(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return false
	}
	// Must have at least "type" or "properties" or "x-component"
	_, hasType := parsed["type"]
	_, hasProps := parsed["properties"]
	_, hasXComp := parsed["x-component"]
	return hasType || hasProps || hasXComp
}
