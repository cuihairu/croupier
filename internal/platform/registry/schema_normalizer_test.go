package registry

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestSchemaNormalizer_NormalizeSchema(t *testing.T) {
	normalizer := NewSchemaNormalizer()

	t.Run("normalize pack schema", func(t *testing.T) {
		packSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
		}

		schema, err := normalizer.NormalizeSchema(SourcePack, packSchema)
		require.NoError(t, err)

		objectType := openapi3.Types{"object"}
		assert.Equal(t, &objectType, schema.Type)
		assert.Contains(t, schema.Properties, "name")
	})

	t.Run("normalize proto schema", func(t *testing.T) {
		nameFieldType := descriptorpb.FieldDescriptorProto_TYPE_STRING
		protoSchema := &descriptorpb.DescriptorProto{
			Name: protoString("TestMessage"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name: protoString("name"),
					Type: &nameFieldType,
				},
			},
		}

		schema, err := normalizer.NormalizeSchema(SourceProto, protoSchema)
		require.NoError(t, err)

		objectType := openapi3.Types{"object"}
		assert.Equal(t, &objectType, schema.Type)
		assert.Contains(t, schema.Properties, "name")
	})

	t.Run("normalize openapi schema", func(t *testing.T) {
		openapiSchema := &openapi3.Schema{}
		stringType := openapi3.Types{"string"}
		openapiSchema.Type = &stringType

		schema, err := normalizer.NormalizeSchema(SourceOpenAPI, openapiSchema)
		require.NoError(t, err)

		stringTypeCheck := openapi3.Types{"string"}
		assert.Equal(t, &stringTypeCheck, schema.Type)
	})

	t.Run("nil schema returns error", func(t *testing.T) {
		_, err := normalizer.NormalizeSchema(SourcePack, nil)
		assert.Error(t, err)
	})

	t.Run("unknown source returns error", func(t *testing.T) {
		_, err := normalizer.NormalizeSchema(99, map[string]interface{}{})
		assert.Error(t, err)
	})
}

func protoString(value string) *string {
	return &value
}

func TestSchemaNormalizer_MergeSchemas(t *testing.T) {
	normalizer := NewSchemaNormalizer()

	t.Run("merge multiple schemas", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		intType := openapi3.Types{"integer"}
		objectType := openapi3.Types{"object"}

		schema1 := &openapi3.Schema{
			Type: &objectType,
			Properties: map[string]*openapi3.SchemaRef{
				"name": {Value: &openapi3.Schema{Type: &stringType}},
			},
			Required: []string{"name"},
		}

		schema2 := &openapi3.Schema{
			Description: "Test schema",
			Properties: map[string]*openapi3.SchemaRef{
				"age": {Value: &openapi3.Schema{Type: &intType}},
			},
			Required: []string{"age"},
		}

		merged, err := normalizer.MergeSchemas(schema1, schema2)
		require.NoError(t, err)

		assert.Equal(t, &objectType, merged.Type)
		assert.Equal(t, "Test schema", merged.Description)
		assert.Contains(t, merged.Properties, "name")
		assert.Contains(t, merged.Properties, "age")
		assert.Contains(t, merged.Required, "name")
		assert.Contains(t, merged.Required, "age")
	})

	t.Run("merge empty schemas list returns error", func(t *testing.T) {
		_, err := normalizer.MergeSchemas()
		assert.Error(t, err)
	})

	t.Run("merge with nil schemas", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		schema1 := &openapi3.Schema{Type: &stringType}

		merged, err := normalizer.MergeSchemas(nil, schema1, nil)
		require.NoError(t, err)

		assert.Equal(t, &stringType, merged.Type)
	})
}
