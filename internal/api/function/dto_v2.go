package function

import "encoding/json"

// FunctionUIV2Response represents the function UI configuration.
// This is the new canonical response that only includes Formily Schema.
type FunctionUIV2Response struct {
	// Schema is the Formily JSON Schema for the function's input form.
	Schema json.RawMessage `json:"schema"`

	// UISource indicates where the schema came from.
	UISource string `json:"uiSource"` // custom_metadata/config_file_override/generated_default/none

	// UISourceDetail provides a human-readable description of the source.
	UISourceDetail string `json:"uiSourceDetail,omitempty"`

	// HasDefault indicates whether a default schema was derived.
	HasDefault bool `json:"hasDefault"`

	// Diagnostics contains validation warnings/info.
	Diagnostics []FunctionUIDiagnostic `json:"diagnostics,omitempty"`
}

// FunctionUIV2UpdateRequest represents a request to update function UI.
type FunctionUIV2UpdateRequest struct {
	ID     string          `json:"id" binding:"required"`
	Schema json.RawMessage `json:"schema" binding:"required"`
}

// FunctionUIV2RollbackRequest represents a request to rollback function UI.
type FunctionUIV2RollbackRequest struct {
	ID      string `json:"id" binding:"required"`
	Version int    `json:"version" binding:"required"`
}

// FunctionUIDiagnostic represents a diagnostic message for function UI.
type FunctionUIDiagnostic struct {
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
