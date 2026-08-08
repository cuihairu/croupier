// Package converter provides utilities for converting between different function descriptor formats
package converter

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPIConverter handles OpenAPI 3.0.3 to JSON Schema conversions
type OpenAPIConverter struct{}

// NewOpenAPIConverter creates a new OpenAPI converter instance
func NewOpenAPIConverter() *OpenAPIConverter {
	return &OpenAPIConverter{}
}

// ToJSONSchema converts an OpenAPI 3.0.3 Schema to JSON Schema format
func (c *OpenAPIConverter) ToJSONSchema(schema *openapi3.Schema) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Type
	if schema.Type != nil {
		if len(*schema.Type) > 0 {
			result["type"] = (*schema.Type)[0]
		}
	}

	// Description
	if schema.Description != "" {
		result["description"] = schema.Description
	}

	// Title
	if schema.Title != "" {
		result["title"] = schema.Title
	}

	// Format
	if schema.Format != "" {
		result["format"] = schema.Format
	}

	// Enum
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	// Constraints
	// MinLength is uint64, MaxLength is *uint64
	if schema.MinLength > 0 {
		result["minLength"] = schema.MinLength
	}
	if schema.MaxLength != nil && *schema.MaxLength > 0 {
		result["maxLength"] = *schema.MaxLength
	}
	if schema.Min != nil {
		result["minimum"] = *schema.Min
	}
	if schema.Max != nil {
		result["maximum"] = *schema.Max
	}
	if schema.ExclusiveMin.Bool != nil {
		result["exclusiveMinimum"] = *schema.ExclusiveMin.Bool
	} else if schema.ExclusiveMin.Value != nil {
		result["exclusiveMinimum"] = *schema.ExclusiveMin.Value
	}
	if schema.ExclusiveMax.Bool != nil {
		result["exclusiveMaximum"] = *schema.ExclusiveMax.Bool
	} else if schema.ExclusiveMax.Value != nil {
		result["exclusiveMaximum"] = *schema.ExclusiveMax.Value
	}
	if schema.MultipleOf != nil {
		result["multipleOf"] = *schema.MultipleOf
	}
	if schema.Pattern != "" {
		result["pattern"] = schema.Pattern
	}
	// MinItems is uint64, MaxItems is *uint64
	if schema.MinItems > 0 {
		result["minItems"] = schema.MinItems
	}
	if schema.MaxItems != nil && *schema.MaxItems > 0 {
		result["maxItems"] = *schema.MaxItems
	}
	if schema.UniqueItems {
		result["uniqueItems"] = true
	}

	// Required
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Properties
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for name, propRef := range schema.Properties {
			if propRef != nil && propRef.Value != nil {
				propSchema, err := c.ToJSONSchema(propRef.Value)
				if err != nil {
					return nil, err
				}
				properties[name] = propSchema
			}
		}
		result["properties"] = properties
	}

	// Items (for arrays)
	if schema.Items != nil && schema.Items.Value != nil {
		itemsSchema, err := c.ToJSONSchema(schema.Items.Value)
		if err != nil {
			return nil, err
		}
		result["items"] = itemsSchema
	}

	// AdditionalProperties - check if it has a Schema
	// AdditionalProperties is a struct, not a pointer
	if schema.AdditionalProperties.Schema != nil {
		additionalSchema, err := c.ToJSONSchema(schema.AdditionalProperties.Schema.Value)
		if err != nil {
			return nil, err
		}
		result["additionalProperties"] = additionalSchema
	}

	// AllOf, AnyOf, OneOf
	if len(schema.AllOf) > 0 {
		allOf := make([]interface{}, 0, len(schema.AllOf))
		for _, ref := range schema.AllOf {
			if ref != nil && ref.Value != nil {
				s, err := c.ToJSONSchema(ref.Value)
				if err != nil {
					return nil, err
				}
				allOf = append(allOf, s)
			}
		}
		result["allOf"] = allOf
	}
	if len(schema.AnyOf) > 0 {
		anyOf := make([]interface{}, 0, len(schema.AnyOf))
		for _, ref := range schema.AnyOf {
			if ref != nil && ref.Value != nil {
				s, err := c.ToJSONSchema(ref.Value)
				if err != nil {
					return nil, err
				}
				anyOf = append(anyOf, s)
			}
		}
		result["anyOf"] = anyOf
	}
	if len(schema.OneOf) > 0 {
		oneOf := make([]interface{}, 0, len(schema.OneOf))
		for _, ref := range schema.OneOf {
			if ref != nil && ref.Value != nil {
				s, err := c.ToJSONSchema(ref.Value)
				if err != nil {
					return nil, err
				}
				oneOf = append(oneOf, s)
			}
		}
		result["oneOf"] = oneOf
	}

	// Default
	if schema.Default != nil {
		result["default"] = schema.Default
	}

	// Extensions (x-* fields)
	if len(schema.Extensions) > 0 {
		for key, value := range schema.Extensions {
			result[key] = value
		}
	}

	// Nullable
	if schema.Nullable {
		result["nullable"] = true
	}

	// ReadOnly & WriteOnly
	if schema.ReadOnly {
		result["readOnly"] = true
	}
	if schema.WriteOnly {
		result["writeOnly"] = true
	}

	return result, nil
}

// ToOpenAPIOperation converts a ProviderFunctionDescriptor to an OpenAPI 3.0.3 Operation object
func ToOpenAPIOperation(descriptor ProviderFunctionDescriptorDesc) (*openapi3.Operation, error) {
	op := &openapi3.Operation{
		OperationID: descriptor.OperationID,
		Summary:     descriptor.Summary,
		Description: descriptor.Description,
		Tags:        descriptor.Tags,
		Deprecated:  descriptor.Deprecated,
	}

	// Parse and set request body
	if descriptor.InputSchema != "" {
		var schemaBody json.RawMessage
		if err := json.Unmarshal([]byte(descriptor.InputSchema), &schemaBody); err != nil {
			return nil, err
		}

		requestSchema := &openapi3.SchemaRef{
			Value: &openapi3.Schema{},
		}
		if err := requestSchema.Value.UnmarshalJSON(schemaBody); err != nil {
			return nil, err
		}

		op.RequestBody = &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: requestSchema,
					},
				},
			},
		}
	}

	// Parse and set response body
	if descriptor.OutputSchema != "" {
		var schemaBody json.RawMessage
		if err := json.Unmarshal([]byte(descriptor.OutputSchema), &schemaBody); err != nil {
			return nil, err
		}

		responseSchema := &openapi3.SchemaRef{
			Value: &openapi3.Schema{},
		}
		if err := responseSchema.Value.UnmarshalJSON(schemaBody); err != nil {
			return nil, err
		}

		if op.Responses == nil {
			op.Responses = openapi3.NewResponses()
		}

		desc := "Success"
		op.Responses.Set("200", &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: &desc,
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: responseSchema,
					},
				},
			},
		})
	}

	// Set only executable capability extensions. Dashboard UI, labels,
	// menu, placement, and page metadata are not part of function registration.
	if descriptor.Resource != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-resource"] = descriptor.Resource
	}
	if descriptor.Risk != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-risk"] = descriptor.Risk
	}
	if descriptor.Operation != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-operation"] = descriptor.Operation
	}
	if descriptor.Capability != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-capability"] = descriptor.Capability
	}
	if descriptor.Execution != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-execution"] = descriptor.Execution
	}
	if descriptor.Permission != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-permission"] = descriptor.Permission
	}

	return op, nil
}

// ProviderFunctionDescriptorDesc represents a ProviderFunctionDescriptor for conversion
type ProviderFunctionDescriptorDesc struct {
	ID           string
	Version      string
	Tags         []string
	Summary      string
	Description  string
	OperationID  string
	Deprecated   bool
	InputSchema  string
	OutputSchema string
	Resource     string
	Operation    string
	Capability   string
	Execution    string
	Risk         string
	Permission   string
}

// ExtractExtension extracts an extension value without the x- prefix
func ExtractExtension(extensions map[string]interface{}, key string) (interface{}, bool) {
	if extensions == nil {
		return nil, false
	}

	// Try with x- prefix first
	fullKey := "x-" + key
	if value, exists := extensions[fullKey]; exists {
		return value, true
	}

	// Try without prefix
	if value, exists := extensions[key]; exists {
		return value, true
	}

	return nil, false
}

// GetStringExtension extracts a string extension value
func GetStringExtension(extensions map[string]interface{}, key string) (string, bool) {
	value, exists := ExtractExtension(extensions, key)
	if !exists {
		return "", false
	}

	if str, ok := value.(string); ok {
		return str, true
	}

	return "", false
}

// GetBoolExtension extracts a boolean extension value
func GetBoolExtension(extensions map[string]interface{}, key string) (bool, bool) {
	value, exists := ExtractExtension(extensions, key)
	if !exists {
		return false, false
	}

	if b, ok := value.(bool); ok {
		return b, true
	}

	return false, false
}
