package openapi

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Validator validates OpenAPI 3.0.3 specifications
type Validator struct {
	loader *openapi3.Loader
}

// NewValidator creates a new OpenAPI validator
func NewValidator() *Validator {
	return &Validator{
		loader: openapi3.NewLoader(),
	}
}

// ValidateSpec validates an OpenAPI 3.0.3 specification document
func (v *Validator) ValidateSpec(data []byte) (*openapi3.T, error) {
	doc, err := v.loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	// Validate OpenAPI version
	if doc.OpenAPI == "" {
		return nil, fmt.Errorf("missing openapi version field")
	}

	if !strings.HasPrefix(doc.OpenAPI, "3.0.") {
		return nil, fmt.Errorf("unsupported OpenAPI version: %s (required 3.0.x)", doc.OpenAPI)
	}

	// Basic validation
	if doc.Info == nil {
		return nil, fmt.Errorf("missing info field")
	}

	if doc.Info.Title == "" {
		return nil, fmt.Errorf("missing info.title field")
	}

	if doc.Paths == nil || len(doc.Paths.Map()) == 0 {
		return nil, fmt.Errorf("no paths defined in spec")
	}

	return doc, nil
}

// ValidateOperation validates a single OpenAPI operation
func (v *Validator) ValidateOperation(op *openapi3.Operation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}

	if op.OperationID == "" {
		return fmt.Errorf("operationId is required")
	}

	// Validate request body
	if op.RequestBody != nil {
		reqBody := op.RequestBody.Value
		if reqBody == nil {
			return fmt.Errorf("request body value is nil")
		}

		if len(reqBody.Content) == 0 {
			return fmt.Errorf("request body must have at least one content type")
		}

		// Check for JSON content type
		if _, ok := reqBody.Content["application/json"]; !ok {
			return fmt.Errorf("request body must support application/json")
		}
	}

	// Validate responses
	if op.Responses == nil || len(op.Responses.Map()) == 0 {
		return fmt.Errorf("operation must have at least one response")
	}

	// Check for 200 response
	if response200 := op.Responses.Value("200"); response200 == nil {
		return fmt.Errorf("operation must have a 200 response")
	}

	return nil
}
