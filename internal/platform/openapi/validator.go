// Package openapi provides OpenAPI 3.0.3 specification validation and management
package openapi

import (
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// Validator validates OpenAPI 3.0.3 specifications
type Validator struct {
	loader *openapi3.Loader
}

// NewValidator creates a new OpenAPI validator instance
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

	// Validate the document
	if err := doc.Validate(v.loader.Context); err != nil {
		return nil, fmt.Errorf("OpenAPI validation failed: %w", err)
	}

	return doc, nil
}

// ValidateOperation validates a single OpenAPI 3.0.3 operation
func (v *Validator) ValidateOperation(operation *openapi3.Operation) error {
	if operation == nil {
		return errors.New("operation is nil")
	}

	// Check required fields
	if operation.OperationID == "" {
		return errors.New("operation_id is required")
	}

	// Validate request body
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if err := v.validateRequestBody(operation.RequestBody.Value); err != nil {
			return fmt.Errorf("invalid request body: %w", err)
		}
	}

	// Validate responses
	if operation.Responses == nil || len(operation.Responses.Map()) == 0 {
		return errors.New("at least one response is required")
	}

	for statusCode, responseRef := range operation.Responses.Map() {
		if responseRef == nil || responseRef.Value == nil {
			continue
		}
		if err := v.validateResponse(responseRef.Value); err != nil {
			return fmt.Errorf("invalid response %s: %w", statusCode, err)
		}
	}

	return nil
}

// validateRequestBody validates an OpenAPI request body
func (v *Validator) validateRequestBody(body *openapi3.RequestBody) error {
	if len(body.Content) == 0 {
		return errors.New("request body must have at least one content type")
	}

	// Check for JSON schema
	if jsonContent, exists := body.Content["application/json"]; exists {
		if jsonContent.Schema == nil || jsonContent.Schema.Value == nil {
			return errors.New("application/json content must have a schema")
		}
		if err := v.validateSchema(jsonContent.Schema.Value); err != nil {
			return fmt.Errorf("invalid JSON schema: %w", err)
		}
	}

	return nil
}

// validateResponse validates an OpenAPI response
func (v *Validator) validateResponse(response *openapi3.Response) error {
	if response.Description == nil || *response.Description == "" {
		return errors.New("response description is required")
	}

	if len(response.Content) > 0 {
		// Check for JSON schema
		if jsonContent, exists := response.Content["application/json"]; exists {
			if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
				if err := v.validateSchema(jsonContent.Schema.Value); err != nil {
					return fmt.Errorf("invalid JSON schema: %w", err)
				}
			}
		}
	}

	return nil
}

// validateSchema validates an OpenAPI schema
func (v *Validator) validateSchema(schema *openapi3.Schema) error {
	if schema == nil {
		return nil
	}

	// If type is specified, validate it
	if schema.Type != nil {
		switch (*schema.Type)[0] {
		case "object":
			if len(schema.Properties) == 0 && schema.AdditionalProperties.Schema == nil {
				// Empty object is allowed, but warn
			}
		case "array":
			if schema.Items == nil || schema.Items.Value == nil {
				return errors.New("array type must have items defined")
			}
		}
	}

	return nil
}

// ValidateExtensionFields validates Croupier-specific extension fields
func (v *Validator) ValidateExtensionFields(extensions map[string]interface{}) error {
	if extensions == nil {
		return nil
	}

	// Validate x-risk if present
	if risk, exists := extensions["x-risk"]; exists {
		riskStr, ok := risk.(string)
		if !ok {
			return errors.New("x-risk must be a string")
		}
		if riskStr != "safe" && riskStr != "warning" && riskStr != "high" && riskStr != "danger" {
			return fmt.Errorf("invalid x-risk value: %s (must be safe, warning, high, or danger)", riskStr)
		}
	}

	// Validate x-resource if present
	if resource, exists := extensions["x-resource"]; exists {
		if _, ok := resource.(string); !ok {
			return errors.New("x-resource must be a string")
		}
	}

	// Validate x-operation if present
	if operation, exists := extensions["x-operation"]; exists {
		if _, ok := operation.(string); !ok {
			return errors.New("x-operation must be a string")
		}
	}

	// Validate x-enabled if present
	if enabled, exists := extensions["x-enabled"]; exists {
		if _, ok := enabled.(bool); !ok {
			return errors.New("x-enabled must be a boolean")
		}
	}

	// Validate x-permission if present
	if permission, exists := extensions["x-permission"]; exists {
		if _, ok := permission.(string); !ok {
			return errors.New("x-permission must be a string")
		}
	}

	return nil
}
