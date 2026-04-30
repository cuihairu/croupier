package openapi

import (
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaMapper converts between OpenAPI Schema and JSON Schema.
type SchemaMapper struct{}

// NewSchemaMapper creates a new SchemaMapper instance.
func NewSchemaMapper() *SchemaMapper {
	return &SchemaMapper{}
}

// OpenAPIToJSONSchema converts an OpenAPI Schema to JSON Schema string.
// Since OpenAPI 3.0.3 is a subset of JSON Schema Draft 2020-12, this is mostly a serialization.
func (m *SchemaMapper) OpenAPIToJSONSchema(schema *openapi3.Schema) (string, error) {
	if schema == nil {
		return "{}", nil
	}

	// Convert to JSON
	bytes, err := schema.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("marshal schema failed: %w", err)
	}

	return string(bytes), nil
}

// JSONSchemaToOpenAPI converts a JSON Schema string to OpenAPI Schema.
func (m *SchemaMapper) JSONSchemaToOpenAPI(jsonSchema string) *openapi3.SchemaRef {
	if jsonSchema == "" {
		return nil
	}

	// Parse JSON Schema
	var schemaData interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &schemaData); err != nil {
		// Return a simple object schema on parse error
		objectType := openapi3.Types{"object"}
		return &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type: &objectType,
			},
		}
	}

	// Build OpenAPI Schema from JSON Schema
	schema := m.buildSchemaFromData(schemaData)
	return &openapi3.SchemaRef{Value: &schema}
}

// buildSchemaFromData builds an OpenAPI Schema from JSON Schema data.
func (m *SchemaMapper) buildSchemaFromData(data interface{}) openapi3.Schema {
	schema := openapi3.Schema{}

	switch v := data.(type) {
	case map[string]interface{}:
		return m.buildSchemaFromObject(v)
	case []interface{}:
		// Array type schema
		if len(v) > 0 {
			itemsSchema := m.buildSchemaFromData(v[0])
			arrayType := openapi3.Types{"array"}
			schema.Type = &arrayType
			schema.Items = &openapi3.SchemaRef{Value: &itemsSchema}
		}
	default:
		// Primitive type
		typeName := m.inferType(v)
		typeSlice := openapi3.Types{typeName}
		schema.Type = &typeSlice
	}

	return schema
}

// buildSchemaFromObject builds an OpenAPI Schema from a JSON Schema object.
func (m *SchemaMapper) buildSchemaFromObject(obj map[string]interface{}) openapi3.Schema {
	schema := openapi3.Schema{}

	// Extract type
	if typeVal, ok := obj["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			typeSlice := openapi3.Types{typeStr}
			schema.Type = &typeSlice
		}
	}

	// Extract description
	if descVal, ok := obj["description"]; ok {
		if descStr, ok := descVal.(string); ok {
			schema.Description = descStr
		}
	}

	// Extract title
	if titleVal, ok := obj["title"]; ok {
		if titleStr, ok := titleVal.(string); ok {
			schema.Title = titleStr
		}
	}

	// Extract format
	if formatVal, ok := obj["format"]; ok {
		if formatStr, ok := formatVal.(string); ok {
			schema.Format = formatStr
		}
	}

	// Extract enum values
	if enumVal, ok := obj["enum"]; ok {
		if enumArr, ok := enumVal.([]interface{}); ok {
			schema.Enum = enumArr
		}
	}

	// Extract default value
	if defaultVal, ok := obj["default"]; ok {
		schema.Default = defaultVal
	}

	// Extract required array
	if requiredVal, ok := obj["required"]; ok {
		if requiredArr, ok := requiredVal.([]interface{}); ok {
			for _, item := range requiredArr {
				if str, ok := item.(string); ok {
					schema.Required = append(schema.Required, str)
				}
			}
		}
	}

	// Extract properties (for object type)
	if propsVal, ok := obj["properties"]; ok {
		if propsObj, ok := propsVal.(map[string]interface{}); ok {
			if schema.Properties == nil {
				schema.Properties = make(map[string]*openapi3.SchemaRef)
			}
			for propName, propData := range propsObj {
				propSchema := m.buildSchemaFromData(propData)
				schema.Properties[propName] = &openapi3.SchemaRef{Value: &propSchema}
			}
		}
	}

	// Extract items (for array type)
	if itemsVal, ok := obj["items"]; ok {
		itemsSchema := m.buildSchemaFromData(itemsVal)
		schema.Items = &openapi3.SchemaRef{Value: &itemsSchema}
	}

	// Note: $ref handling is done at SchemaRef level, not Schema level

	// Extract additionalProperties
	if addPropsVal, ok := obj["additionalProperties"]; ok {
		switch addProps := addPropsVal.(type) {
		case bool:
			boolType := openapi3.Types{"boolean"}
			schema.AdditionalProperties = openapi3.AdditionalProperties{
				Has:    &addProps,
				Schema: openapi3.NewSchemaRef("", &openapi3.Schema{Type: &boolType}),
			}
		case map[string]interface{}:
			addPropsSchema := m.buildSchemaFromObject(addProps)
			trueVal := true
			schema.AdditionalProperties = openapi3.AdditionalProperties{
				Has:    &trueVal,
				Schema: openapi3.NewSchemaRef("", &addPropsSchema),
			}
		}
	}

	// Extract allOf, anyOf, oneOf
	if allOfVal, ok := obj["allOf"]; ok {
		if allOfArr, ok := allOfVal.([]interface{}); ok {
			for _, item := range allOfArr {
				itemSchema := m.buildSchemaFromData(item)
				schema.AllOf = append(schema.AllOf, &openapi3.SchemaRef{Value: &itemSchema})
			}
		}
	}

	if anyOfVal, ok := obj["anyOf"]; ok {
		if anyOfArr, ok := anyOfVal.([]interface{}); ok {
			for _, item := range anyOfArr {
				itemSchema := m.buildSchemaFromData(item)
				schema.AnyOf = append(schema.AnyOf, &openapi3.SchemaRef{Value: &itemSchema})
			}
		}
	}

	if oneOfVal, ok := obj["oneOf"]; ok {
		if oneOfArr, ok := oneOfVal.([]interface{}); ok {
			for _, item := range oneOfArr {
				itemSchema := m.buildSchemaFromData(item)
				schema.OneOf = append(schema.OneOf, &openapi3.SchemaRef{Value: &itemSchema})
			}
		}
	}

	// Extract minimum/maximum for numeric types
	if minVal, ok := obj["minimum"]; ok {
		if num, ok := minVal.(float64); ok {
			schema.Min = &num
		}
	}

	if maxVal, ok := obj["maximum"]; ok {
		if num, ok := maxVal.(float64); ok {
			schema.Max = &num
		}
	}

	// Extract minLength/maxLength for string types
	if minLenVal, ok := obj["minLength"]; ok {
		if num, ok := minLenVal.(float64); ok {
			schema.MinLength = uint64(num)
		}
	}

	if maxLenVal, ok := obj["maxLength"]; ok {
		if num, ok := maxLenVal.(float64); ok {
			val := uint64(num)
			schema.MaxLength = &val
		}
	}

	// Extract pattern for string types
	if patternVal, ok := obj["pattern"]; ok {
		if patternStr, ok := patternVal.(string); ok {
			schema.Pattern = patternStr
		}
	}

	return schema
}

// inferType infers the JSON type from a Go value.
func (m *SchemaMapper) inferType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case int, int64, uint, uint64:
		return "integer"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "string"
	}
}

// ValidateJSONSchema validates if a string is a valid JSON Schema.
func (m *SchemaMapper) ValidateJSONSchema(jsonSchema string) error {
	if jsonSchema == "" {
		return fmt.Errorf("schema is empty")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Basic validation: must be an object (JSON Schema root)
	obj, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("JSON Schema root must be an object")
	}

	// Should have at least a "type" field or "$ref" or be an allOf/anyOf/oneOf
	_, hasType := obj["type"]
	_, hasRef := obj["$ref"]
	_, hasAllOf := obj["allOf"]
	_, hasAnyOf := obj["anyOf"]
	_, hasOneOf := obj["oneOf"]

	if !hasType && !hasRef && !hasAllOf && !hasAnyOf && !hasOneOf {
		return fmt.Errorf("JSON Schema must have type, $ref, allOf, anyOf, or oneOf")
	}

	return nil
}

// MergeSchemas merges multiple JSON Schema objects into one.
func (m *SchemaMapper) MergeSchemas(schemas ...string) (string, error) {
	if len(schemas) == 0 {
		return "{}", nil
	}

	if len(schemas) == 1 {
		return schemas[0], nil
	}

	// Parse all schemas
	var parsedSchemas []map[string]interface{}
	for _, schema := range schemas {
		var data interface{}
		if err := json.Unmarshal([]byte(schema), &data); err != nil {
			return "", fmt.Errorf("parse schema failed: %w", err)
		}
		obj, ok := data.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("schema root must be an object")
		}
		parsedSchemas = append(parsedSchemas, obj)
	}

	// Merge schemas
	merged := make(map[string]interface{})
	merged["allOf"] = make([]interface{}, 0)

	for _, schema := range parsedSchemas {
		merged["allOf"] = append(merged["allOf"].([]interface{}), schema)
	}

	result, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged schema failed: %w", err)
	}

	return string(result), nil
}

// GetSchemaType extracts the type from a JSON Schema string.
func (m *SchemaMapper) GetSchemaType(jsonSchema string) (string, error) {
	if jsonSchema == "" {
		return "", fmt.Errorf("schema is empty")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	obj, ok := data.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("schema root must be an object")
	}

	if typeVal, ok := obj["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			return typeStr, nil
		}
	}

	// Check for $ref
	if _, hasRef := obj["$ref"]; hasRef {
		return "$ref", nil
	}

	// Check for composite schemas
	if _, hasAllOf := obj["allOf"]; hasAllOf {
		return "allOf", nil
	}
	if _, hasAnyOf := obj["anyOf"]; hasAnyOf {
		return "anyOf", nil
	}
	if _, hasOneOf := obj["oneOf"]; hasOneOf {
		return "oneOf", nil
	}

	return "", fmt.Errorf("schema type not found")
}

// IsObjectSchema checks if a JSON Schema represents an object type.
func (m *SchemaMapper) IsObjectSchema(jsonSchema string) bool {
	schemaType, err := m.GetSchemaType(jsonSchema)
	if err != nil {
		return false
	}
	return schemaType == "object"
}

// IsArraySchema checks if a JSON Schema represents an array type.
func (m *SchemaMapper) IsArraySchema(jsonSchema string) bool {
	schemaType, err := m.GetSchemaType(jsonSchema)
	if err != nil {
		return false
	}
	return schemaType == "array"
}

// GetObjectProperties extracts property names from an object schema.
func (m *SchemaMapper) GetObjectProperties(jsonSchema string) ([]string, error) {
	if jsonSchema == "" {
		return nil, fmt.Errorf("schema is empty")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	obj, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("schema root must be an object")
	}

	if propsVal, ok := obj["properties"]; ok {
		if propsObj, ok := propsVal.(map[string]interface{}); ok {
			properties := make([]string, 0, len(propsObj))
			for propName := range propsObj {
				properties = append(properties, propName)
			}
			return properties, nil
		}
	}

	return nil, fmt.Errorf("schema is not an object or has no properties")
}
