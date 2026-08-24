package registry

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaNormalizer_NormalizeSchema_Nil(t *testing.T) {
	n := NewSchemaNormalizer()
	_, err := n.NormalizeSchema(SourcePack, nil)
	assert.Error(t, err)
}

func TestSchemaNormalizer_NormalizePackSchema_MarshalError(t *testing.T) {
	n := NewSchemaNormalizer()
	_, err := n.NormalizeSchema(SourcePack, make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal pack schema")
}

func TestSchemaNormalizer_NormalizePackSchema_UnmarshalError(t *testing.T) {
	n := NewSchemaNormalizer()
	_, err := n.NormalizeSchema(SourcePack, "definitely-not-an-object")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal schema")
}

func TestSchemaNormalizer_NormalizeOpenAPISchema_MarshalError(t *testing.T) {
	n := NewSchemaNormalizer()
	_, err := n.NormalizeSchema(SourceOpenAPI, map[string]interface{}{"bad": make(chan int)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal schema map")
}

func TestSchemaNormalizer_MergeSchemas_SkipsNilEntries(t *testing.T) {
	n := NewSchemaNormalizer()

	merged, err := n.MergeSchemas(
		nil,
		&openapi3.Schema{
			Type:     &openapi3.Types{"object"},
			Required: []string{"name"},
			Properties: openapi3.Schemas{
				"name": {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
			},
		},
		nil,
		&openapi3.Schema{
			Format:      "int64",
			Description: "overridden later",
		},
		&openapi3.Schema{Description: "final description"},
	)
	require.NoError(t, err)
	require.NotNil(t, merged)
	assert.Equal(t, openapi3.Types{"object"}, *merged.Type)
	assert.Equal(t, "int64", merged.Format)
	assert.Equal(t, "final description", merged.Description)
	assert.Len(t, merged.Properties, 1)
	assert.Contains(t, merged.Required, "name")
}

func TestSchemaNormalizer_MergeSchemas_MergesExtensionsAndRequiredUnion(t *testing.T) {
	n := NewSchemaNormalizer()

	merged, err := n.MergeSchemas(
		&openapi3.Schema{
			Extensions: map[string]interface{}{"x-source": "a"},
			Required:   []string{"a"},
		},
		&openapi3.Schema{
			Extensions: map[string]interface{}{"x-source": "b", "x-extra": 1},
			Required:   []string{"b"},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "b", merged.Extensions["x-source"])
	assert.Equal(t, 1, merged.Extensions["x-extra"])
	assert.ElementsMatch(t, []string{"a", "b"}, merged.Required)
}
