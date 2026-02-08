package openapi

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// ProviderConverter converts various function representations to OpenAPI Operations
type ProviderConverter struct {
	validator *Validator
}

// NewProviderConverter creates a new Provider converter
func NewProviderConverter() *ProviderConverter {
	return &ProviderConverter{
		validator: NewValidator(),
	}
}

// FunctionDescriptor represents a generic function descriptor
type FunctionDescriptor struct {
	ID           string
	Name         string
	Summary      string
	Description  string
	InputSchema  string
	OutputSchema string
	Category     string
	Risk         string
	Entity       string
	Operation    string
}

// ToOpenAPIOperation converts a FunctionDescriptor to OpenAPI Operation
func (c *ProviderConverter) ToOpenAPIOperation(desc *FunctionDescriptor) (*openapi3.Operation, error) {
	if desc == nil {
		return nil, fmt.Errorf("descriptor is nil")
	}

	op := &openapi3.Operation{
		OperationID: desc.ID,
		Summary:     desc.Summary,
		Description: desc.Description,
	}

	// Add extensions
	if desc.Category != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-category"] = desc.Category
	}

	if desc.Risk != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-risk"] = desc.Risk
	}

	if desc.Entity != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-entity"] = desc.Entity
	}

	if desc.Operation != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-operation"] = desc.Operation
	}

	// Create request body
	objectType := openapi3.Types{"object"}
	requestSchema := openapi3.NewSchema()
	requestSchema.Type = &objectType
	requestSchema.Description = "Request payload"

	op.RequestBody = &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{
			Description: "Request payload",
			Required:    true,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{Value: requestSchema},
				},
			},
		},
	}

	// Create response
	responseSchema := openapi3.NewSchema()
	responseSchema.Type = &objectType
	responseSchema.Description = "Response payload"

	respDesc := "Response payload"
	response := &openapi3.Response{
		Description: &respDesc,
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{
				Schema: &openapi3.SchemaRef{Value: responseSchema},
			},
		},
	}

	if op.Responses == nil {
		op.Responses = openapi3.NewResponses()
	}
	op.Responses.Set("200", &openapi3.ResponseRef{Value: response})

	// Validate the operation
	if err := c.validator.ValidateOperation(op); err != nil {
		return nil, fmt.Errorf("invalid operation: %w", err)
	}

	return op, nil
}
