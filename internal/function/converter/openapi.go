package converter

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPIConverter provides helper functions for OpenAPI 3.0.3 conversions
type OpenAPIConverter struct{}

// NewOpenAPIConverter creates a new OpenAPI converter instance
func NewOpenAPIConverter() *OpenAPIConverter {
	return &OpenAPIConverter{}
}

// ToJSONSchema converts an OpenAPI Schema to JSON Schema format
func (c *OpenAPIConverter) ToJSONSchema(schema *openapi3.Schema) (map[string]interface{}, error) {
	if schema == nil {
		return nil, nil
	}

	result := make(map[string]interface{})

	// Type
	if schema.Type != nil {
		result["type"] = schema.Type
	}

	// Format
	if schema.Format != "" {
		result["format"] = schema.Format
	}

	// Description
	if schema.Description != "" {
		result["description"] = schema.Description
	}

	// Title
	if schema.Title != "" {
		result["title"] = schema.Title
	}

	// Default value
	if schema.Default != nil {
		result["default"] = schema.Default
	}

	// Enum
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	// Multiple of (for numbers)
	if schema.MultipleOf != nil {
		result["multipleOf"] = *schema.MultipleOf
	}

	// Minimum / Maximum
	if schema.Min != nil {
		result["minimum"] = *schema.Min
	}
	if schema.Max != nil {
		result["maximum"] = *schema.Max
	}

	// Exclusive Minimum / Maximum
	if schema.ExclusiveMin {
		result["exclusiveMinimum"] = true
	}
	if schema.ExclusiveMax {
		result["exclusiveMaximum"] = true
	}

	// MinLength / MaxLength (for strings)
	if schema.MinLength > 0 {
		result["minLength"] = schema.MinLength
	}
	if schema.MaxLength != nil {
		result["maxLength"] = *schema.MaxLength
	}

	// Pattern (for strings)
	if schema.Pattern != "" {
		result["pattern"] = schema.Pattern
	}

	// MinItems / MaxItems (for arrays)
	if schema.MinItems > 0 {
		result["minItems"] = schema.MinItems
	}
	if schema.MaxItems != nil {
		result["maxItems"] = *schema.MaxItems
	}

	// UniqueItems (for arrays)
	if schema.UniqueItems {
		result["uniqueItems"] = true
	}

	// Required fields (for objects)
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Properties (for objects)
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for name, propRef := range schema.Properties {
			propSchema := propRef.Value
			if propSchema != nil {
				propJSON, err := c.ToJSONSchema(propSchema)
				if err != nil {
					return nil, fmt.Errorf("failed to convert property %s: %w", name, err)
				}
				properties[name] = propJSON
			}
		}
		result["properties"] = properties
	}

	// Items (for arrays)
	if schema.Items != nil {
		itemsJSON, err := c.ToJSONSchema(schema.Items.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert items: %w", err)
		}
		result["items"] = itemsJSON
	}

	// AllOf, AnyOf, OneOf
	if len(schema.AllOf) > 0 {
		allOf := make([]interface{}, len(schema.AllOf))
		for i, subRef := range schema.AllOf {
			subSchema := subRef.Value
			if subSchema != nil {
				subJSON, err := c.ToJSONSchema(subSchema)
				if err != nil {
					return nil, fmt.Errorf("failed to convert allOf[%d]: %w", i, err)
				}
				allOf[i] = subJSON
			}
		}
		result["allOf"] = allOf
	}

	if len(schema.AnyOf) > 0 {
		anyOf := make([]interface{}, len(schema.AnyOf))
		for i, subRef := range schema.AnyOf {
			subSchema := subRef.Value
			if subSchema != nil {
				subJSON, err := c.ToJSONSchema(subSchema)
				if err != nil {
					return nil, fmt.Errorf("failed to convert anyOf[%d]: %w", i, err)
				}
				anyOf[i] = subJSON
			}
		}
		result["anyOf"] = anyOf
	}

	if len(schema.OneOf) > 0 {
		oneOf := make([]interface{}, len(schema.OneOf))
		for i, subRef := range schema.OneOf {
			subSchema := subRef.Value
			if subSchema != nil {
				subJSON, err := c.ToJSONSchema(subSchema)
				if err != nil {
					return nil, fmt.Errorf("failed to convert oneOf[%d]: %w", i, err)
				}
				oneOf[i] = subJSON
			}
		}
		result["oneOf"] = oneOf
	}

	// Custom extensions (x-*)
	for key, value := range schema.Extensions {
		keyStr := fmt.Sprintf("%v", key)
		if len(keyStr) > 2 && keyStr[0:2] == "x-" {
			result[keyStr] = value
		}
	}

	return result, nil
}

// ExtractExtension extracts a custom extension value from an OpenAPI object
func ExtractExtension(extensions map[string]interface{}, key string) (interface{}, bool) {
	if extensions == nil {
		return nil, false
	}
	fullKey := "x-" + key
	value, exists := extensions[fullKey]
	return value, exists
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
