package registry

import (
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaSource identifies where a schema originated from.
type SchemaSource int

const (
	SourcePack SchemaSource = iota
	SourceProto
	SourceOpenAPI
)

// SchemaNormalizer normalizes schemas from different sources into OpenAPI 3.0.3 format.
type SchemaNormalizer struct{}

// NewSchemaNormalizer creates a new schema normalizer.
func NewSchemaNormalizer() *SchemaNormalizer {
	return &SchemaNormalizer{}
}

// NormalizeSchema converts a schema from any source to OpenAPI 3.0.3 Schema.
func (n *SchemaNormalizer) NormalizeSchema(source SchemaSource, schema interface{}) (*openapi3.Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	switch source {
	case SourcePack:
		return n.normalizePackSchema(schema)
	case SourceProto:
		return n.normalizeProtoSchema(schema)
	case SourceOpenAPI:
		return n.normalizeOpenAPISchema(schema)
	default:
		return nil, fmt.Errorf("unknown schema source: %d", source)
	}
}

// normalizePackSchema converts Pack manifest schema to OpenAPI 3.0.3.
func (n *SchemaNormalizer) normalizePackSchema(schema interface{}) (*openapi3.Schema, error) {
	// Pack schemas are already JSON Schema compatible
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pack schema: %w", err)
	}

	var openapiSchema openapi3.Schema
	if err := json.Unmarshal(data, &openapiSchema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	return &openapiSchema, nil
}

// normalizeProtoSchema converts Proto descriptor to OpenAPI 3.0.3.
func (n *SchemaNormalizer) normalizeProtoSchema(schema interface{}) (*openapi3.Schema, error) {
	// Proto descriptors need more complex conversion
	// For now, return a basic object schema
	objectType := openapi3.Types{"object"}
	result := &openapi3.Schema{
		Type: &objectType,
	}

	// TODO: Implement full proto-to-openapi conversion using FileDescriptorSet
	return result, nil
}

// normalizeOpenAPISchema validates and returns OpenAPI schema.
func (n *SchemaNormalizer) normalizeOpenAPISchema(schema interface{}) (*openapi3.Schema, error) {
	// Try to convert to openapi3.Schema
	switch s := schema.(type) {
	case *openapi3.Schema:
		return s, nil
	case map[string]interface{}:
		data, err := json.Marshal(s)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema map: %w", err)
		}

		var openapiSchema openapi3.Schema
		if err := json.Unmarshal(data, &openapiSchema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
		}

		return &openapiSchema, nil
	default:
		return nil, fmt.Errorf("unsupported schema type: %T", schema)
	}
}

// MergeSchemas merges multiple schemas into one using OpenAPI 3.0.3 composition.
// For conflicting fields, later schemas override earlier ones.
func (n *SchemaNormalizer) MergeSchemas(schemas ...*openapi3.Schema) (*openapi3.Schema, error) {
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no schemas to merge")
	}

	result := &openapi3.Schema{}

	for _, schema := range schemas {
		if schema == nil {
			continue
		}

		// Merge type (first non-empty wins)
		if result.Type == nil && schema.Type != nil {
			result.Type = schema.Type
		}

		// Merge format
		if result.Format == "" && schema.Format != "" {
			result.Format = schema.Format
		}

		// Merge description (last wins)
		if schema.Description != "" {
			result.Description = schema.Description
		}

		// Merge properties (deep merge)
		if len(schema.Properties) > 0 {
			if result.Properties == nil {
				result.Properties = make(map[string]*openapi3.SchemaRef)
			}
			for name, prop := range schema.Properties {
				result.Properties[name] = prop
			}
		}

		// Merge required fields (union)
		if len(schema.Required) > 0 {
			if result.Required == nil {
				result.Required = []string{}
			}
			result.Required = append(result.Required, schema.Required...)
		}

		// Merge extensions (last wins for conflicts)
		for key, value := range schema.Extensions {
			if result.Extensions == nil {
				result.Extensions = make(map[string]interface{})
			}
			result.Extensions[key] = value
		}
	}

	return result, nil
}
