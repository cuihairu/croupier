package converter

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIConverter_ToJSONSchema(t *testing.T) {
	converter := NewOpenAPIConverter()

	t.Run("basic string schema", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type:        &stringType,
			Description: "test field",
			MinLength:   1,
			MaxLength:   ptr(uint64(100)),
		}

		result, err := converter.ToJSONSchema(schema)
		require.NoError(t, err)
		// Type is stored as *openapi3.Types in result
		assert.NotNil(t, result["type"])
		assert.Equal(t, "test field", result["description"])
		assert.Equal(t, uint64(1), result["minLength"])
		assert.Equal(t, uint64(100), result["maxLength"])
	})

	t.Run("object with properties", func(t *testing.T) {
		objectType := openapi3.Types{"object"}
		stringType := openapi3.Types{"string"}
		intType := openapi3.Types{"integer"}
		min := 0.0
		max := 150.0
		schema := &openapi3.Schema{
			Type: &objectType,
			Properties: map[string]*openapi3.SchemaRef{
				"name": {
					Value: &openapi3.Schema{
						Type:        &stringType,
						Description: "User name",
					},
				},
				"age": {
					Value: &openapi3.Schema{
						Type: &intType,
						Min:  &min,
						Max:  &max,
					},
				},
			},
			Required: []string{"name"},
		}

		result, err := converter.ToJSONSchema(schema)
		require.NoError(t, err)
		assert.NotNil(t, result["type"])

		props, ok := result["properties"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, props, "name")
		assert.Contains(t, props, "age")

		required, ok := result["required"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"name"}, required)
	})

	t.Run("array schema", func(t *testing.T) {
		arrayType := openapi3.Types{"array"}
		stringType := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type:     &arrayType,
			Items:    &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &stringType}},
			MinItems: 1,
			MaxItems: ptr(uint64(10)),
		}

		result, err := converter.ToJSONSchema(schema)
		require.NoError(t, err)
		assert.NotNil(t, result["type"])
		assert.Equal(t, uint64(1), result["minItems"])
		assert.Equal(t, uint64(10), result["maxItems"])
	})

	t.Run("schema with extensions", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type: &stringType,
			Extensions: map[string]interface{}{
				"x-resource": "player",
				"x-risk":     "high",
			},
		}

		result, err := converter.ToJSONSchema(schema)
		require.NoError(t, err)
		assert.Equal(t, "player", result["x-resource"])
		assert.Equal(t, "high", result["x-risk"])
	})
}

func TestExtractExtension(t *testing.T) {
	t.Run("extract string extension", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-resource": "player",
			"x-risk":     "high",
		}

		value, exists := ExtractExtension(extensions, "resource")
		assert.True(t, exists)
		assert.Equal(t, "player", value)

		value, exists = ExtractExtension(extensions, "risk")
		assert.True(t, exists)
		assert.Equal(t, "high", value)
	})

	t.Run("extract non-existent extension", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-resource": "player",
		}

		value, exists := ExtractExtension(extensions, "nonexistent")
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("extract with string helper", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-resource": "player",
		}

		value, exists := GetStringExtension(extensions, "resource")
		assert.True(t, exists)
		assert.Equal(t, "player", value)
	})

	t.Run("extract with bool helper", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-deprecated": true,
		}

		value, exists := GetBoolExtension(extensions, "deprecated")
		assert.True(t, exists)
		assert.True(t, value)
	})
}

func TestProtoConverter_ProtoToOpenAPI(t *testing.T) {
	converter := NewProtoConverter()

	t.Run("convert simple method", func(t *testing.T) {
		method := &ProtoMethodInfo{
			Name:       "GetPlayer",
			Package:    "croupier.player.v1",
			Service:    "PlayerService",
			InputType:  ".croupier.player.v1.GetPlayerRequest",
			OutputType: ".croupier.player.v1.GetPlayerResponse",
		}

		op, err := converter.ProtoToOpenAPI(method, map[string]interface{}{
			"x-resource": "player",
			"x-risk":     "low",
		})
		require.NoError(t, err)
		assert.Equal(t, "PlayerService.GetPlayer", op.OperationID)
		assert.Equal(t, "GetPlayer", op.Summary)
		assert.Equal(t, "player", op.Extensions["x-resource"])
		assert.Equal(t, "low", op.Extensions["x-risk"])
	})

	t.Run("convert streaming method", func(t *testing.T) {
		method := &ProtoMethodInfo{
			Name:            "StreamPlayerEvents",
			Package:         "croupier.player.v1",
			Service:         "PlayerService",
			InputType:       ".croupier.player.v1.StreamEventsRequest",
			OutputType:      ".croupier.player.v1.PlayerEvent",
			ServerStreaming: true,
		}

		op, err := converter.ProtoToOpenAPI(method, nil)
		require.NoError(t, err)
		streaming, ok := op.Extensions["x-server-streaming"].(bool)
		require.True(t, ok)
		assert.True(t, streaming)
	})
}

func TestJSONSchemaRoundTrip(t *testing.T) {
	converter := NewOpenAPIConverter()

	// Original JSON Schema
	originalJSON := `{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"minLength": 1,
				"maxLength": 100
			},
			"age": {
				"type": "integer",
				"minimum": 0,
				"maximum": 150
			},
			"tags": {
				"type": "array",
				"items": {
					"type": "string"
				},
				"uniqueItems": true
			}
		},
		"required": ["name"]
	}`

	var originalSchema map[string]interface{}
	err := json.Unmarshal([]byte(originalJSON), &originalSchema)
	require.NoError(t, err)

	// Convert to OpenAPI Schema
	var openapiSchema openapi3.Schema
	schemaBytes, _ := json.Marshal(originalSchema)
	err = json.Unmarshal(schemaBytes, &openapiSchema)
	require.NoError(t, err)

	// Convert back to JSON Schema
	result, err := converter.ToJSONSchema(&openapiSchema)
	require.NoError(t, err)

	// Verify key properties are preserved
	assert.NotNil(t, result["type"])
	assert.Equal(t, []string{"name"}, result["required"])

	props, ok := result["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "age")
	assert.Contains(t, props, "tags")
}

// Helper function
func ptr[T any](v T) *T {
	return &v
}

func TestToOpenAPIOperation(t *testing.T) {
	t.Run("convert descriptor with input and output schemas", func(t *testing.T) {
		descriptor := LocalFunctionDescriptorDesc{
			OperationID: "player.ban",
			Summary:     "Ban Player",
			Description: "Permanently ban a player from the game",
			Tags:        []string{"player", "moderation"},
			Deprecated:  false,
			InputSchema: `{
				"type": "object",
				"properties": {
					"player_id": {"type": "string"},
					"reason": {"type": "string"}
				},
				"required": ["player_id"]
			}`,
			OutputSchema: `{
				"type": "object",
				"properties": {
					"success": {"type": "boolean"}
				}
			}`,
			Resource:          "player",
			Risk:              "high",
			Operation:         "ban",
			Permission:        "player.ban",
			ApprovalRequired:  true,
			ApprovalPolicyKey: "two_person",
		}

		op, err := ToOpenAPIOperation(descriptor)
		require.NoError(t, err)

		assert.Equal(t, "player.ban", op.OperationID)
		assert.Equal(t, "Ban Player", op.Summary)
		assert.Equal(t, "Permanently ban a player from the game", op.Description)
		assert.Equal(t, []string{"player", "moderation"}, op.Tags)
		assert.False(t, op.Deprecated)

		// Check request body
		assert.NotNil(t, op.RequestBody)
		assert.Contains(t, op.RequestBody.Value.Content, "application/json")
		mediaType := op.RequestBody.Value.Content["application/json"]
		assert.NotNil(t, mediaType.Schema)
		assert.NotNil(t, mediaType.Schema.Value)

		// Check response
		assert.NotNil(t, op.Responses)
		responsesMap := op.Responses.Map()
		response200, ok := responsesMap["200"]
		assert.True(t, ok)
		assert.NotNil(t, response200)
		assert.NotNil(t, response200.Value)
		desc := "Success"
		assert.Equal(t, &desc, response200.Value.Description)

		// Check extensions
		assert.Equal(t, "player", op.Extensions["x-resource"])
		assert.Equal(t, "high", op.Extensions["x-risk"])
		assert.Equal(t, "ban", op.Extensions["x-operation"])
		assert.Equal(t, "player.ban", op.Extensions["x-permission"])
		assert.Equal(t, map[string]interface{}{
			"required":  true,
			"policyKey": "two_person",
		}, op.Extensions["x-approval"])
	})

	t.Run("convert minimal descriptor", func(t *testing.T) {
		descriptor := LocalFunctionDescriptorDesc{
			OperationID: "test.simple",
			Summary:     "Simple Test",
		}

		op, err := ToOpenAPIOperation(descriptor)
		require.NoError(t, err)

		assert.Equal(t, "test.simple", op.OperationID)
		assert.Equal(t, "Simple Test", op.Summary)
		assert.Nil(t, op.RequestBody)
		assert.Nil(t, op.Responses)
	})

	t.Run("convert descriptor with invalid input schema", func(t *testing.T) {
		descriptor := LocalFunctionDescriptorDesc{
			OperationID: "test.invalid",
			InputSchema: `{invalid json`,
		}

		_, err := ToOpenAPIOperation(descriptor)
		assert.Error(t, err)
	})

	t.Run("convert descriptor with invalid output schema", func(t *testing.T) {
		descriptor := LocalFunctionDescriptorDesc{
			OperationID:  "test.invalid",
			OutputSchema: `{invalid json`,
		}

		_, err := ToOpenAPIOperation(descriptor)
		assert.Error(t, err)
	})

	t.Run("convert deprecated descriptor", func(t *testing.T) {
		descriptor := LocalFunctionDescriptorDesc{
			OperationID: "legacy.endpoint",
			Summary:     "Legacy Endpoint",
			Deprecated:  true,
		}

		op, err := ToOpenAPIOperation(descriptor)
		require.NoError(t, err)

		assert.True(t, op.Deprecated)
	})

	t.Run("convert descriptor with only resource extension", func(t *testing.T) {
		descriptor := LocalFunctionDescriptorDesc{
			OperationID: "game.create",
			Resource:    "game",
		}

		op, err := ToOpenAPIOperation(descriptor)
		require.NoError(t, err)

		assert.NotNil(t, op.Extensions)
		assert.Equal(t, "game", op.Extensions["x-resource"])
		_, hasRisk := op.Extensions["x-risk"]
		assert.False(t, hasRisk)
	})
}

func TestOpenAPIConverter_ToJSONSchema_Complete(t *testing.T) {
	converter := NewOpenAPIConverter()

	objectType := openapi3.Types{"object"}
	min := 0.0
	max := 100.0
	multipleOf := 5.0
	exclusiveMin := true
	exclusiveMax := true

	schema := &openapi3.Schema{
		Type:         &objectType,
		Title:        "TestSchema",
		Description:  "A test schema",
		Format:       "int64",
		Enum:         []interface{}{"a", "b", "c"},
		Min:          &min,
		Max:          &max,
		MultipleOf:   &multipleOf,
		ExclusiveMin: openapi3.ExclusiveBound{Bool: &exclusiveMin},
		ExclusiveMax: openapi3.ExclusiveBound{Bool: &exclusiveMax},
		Pattern:      "^[a-z]+$",
		MinLength:    1,
		MaxLength:    ptr(uint64(50)),
		MinItems:     1,
		MaxItems:     ptr(uint64(10)),
		UniqueItems:  true,
		Required:     []string{"id", "name"},
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)

	assert.Equal(t, "object", result["type"])
	assert.Equal(t, "TestSchema", result["title"])
	assert.Equal(t, "A test schema", result["description"])
	assert.Equal(t, "int64", result["format"])
	assert.NotNil(t, result["enum"])
	assert.Equal(t, 0.0, result["minimum"])
	assert.Equal(t, 100.0, result["maximum"])
	assert.Equal(t, 5.0, result["multipleOf"])
	assert.True(t, result["exclusiveMinimum"].(bool))
	assert.True(t, result["exclusiveMaximum"].(bool))
	assert.Equal(t, "^[a-z]+$", result["pattern"])
	assert.Equal(t, uint64(1), result["minLength"])
	assert.Equal(t, uint64(50), result["maxLength"])
	assert.Equal(t, uint64(1), result["minItems"])
	assert.Equal(t, uint64(10), result["maxItems"])
	assert.True(t, result["uniqueItems"].(bool))
	assert.Equal(t, []string{"id", "name"}, result["required"])
}

func TestExtractExtension_ErrorCases(t *testing.T) {
	t.Run("nil extensions map", func(t *testing.T) {
		value, exists := ExtractExtension(nil, "resource")
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("extension value is number", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-max": 42.0,
		}

		value, exists := ExtractExtension(extensions, "max")
		assert.True(t, exists)
		assert.Equal(t, 42.0, value)
	})

	t.Run("extension value is bool", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-enabled": true,
		}

		value, exists := ExtractExtension(extensions, "enabled")
		assert.True(t, exists)
		assert.True(t, value.(bool))
	})
}

func TestGetStringExtension_ErrorCases(t *testing.T) {
	t.Run("nil extensions", func(t *testing.T) {
		value, exists := GetStringExtension(nil, "key")
		assert.False(t, exists)
		assert.Empty(t, value)
	})

	t.Run("non-string value", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-count": 42,
		}

		value, exists := GetStringExtension(extensions, "count")
		assert.False(t, exists)
		assert.Empty(t, value)
	})
}

func TestGetBoolExtension_ErrorCases(t *testing.T) {
	t.Run("nil extensions", func(t *testing.T) {
		value, exists := GetBoolExtension(nil, "key")
		assert.False(t, exists)
		assert.False(t, value)
	})

	t.Run("non-bool value", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-value": "true",
		}

		value, exists := GetBoolExtension(extensions, "value")
		assert.False(t, exists)
		assert.False(t, value)
	})

	t.Run("bool value false", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-enabled": false,
		}

		value, exists := GetBoolExtension(extensions, "enabled")
		assert.True(t, exists)
		assert.False(t, value)
	})
}

func TestToOpenAPIOperation_WithCompleteSchema(t *testing.T) {
	descriptor := LocalFunctionDescriptorDesc{
		OperationID: "test.complex",
		Summary:     "Complex Test",
		Description: "A complex test function",
		Tags:        []string{"test", "complex"},
		Deprecated:  true,
		InputSchema: `{
			"type": "object",
			"title": "Input",
			"required": ["field1"]
		}`,
		OutputSchema: `{
			"type": "object",
			"title": "Output"
		}`,
		Resource:  "test",
		Risk:      "warning",
		Operation: "update",
	}

	op, err := ToOpenAPIOperation(descriptor)
	require.NoError(t, err)

	assert.Equal(t, "test.complex", op.OperationID)
	assert.Equal(t, "Complex Test", op.Summary)
	assert.Equal(t, "A complex test function", op.Description)
	assert.Equal(t, []string{"test", "complex"}, op.Tags)
	assert.True(t, op.Deprecated)
	assert.NotNil(t, op.RequestBody)
	assert.NotNil(t, op.Responses)
	assert.Equal(t, "test", op.Extensions["x-resource"])
	assert.Equal(t, "warning", op.Extensions["x-risk"])
	assert.Equal(t, "update", op.Extensions["x-operation"])
}
