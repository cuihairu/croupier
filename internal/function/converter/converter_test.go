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
				"x-category": "player",
				"x-risk":     "high",
			},
		}

		result, err := converter.ToJSONSchema(schema)
		require.NoError(t, err)
		assert.Equal(t, "player", result["x-category"])
		assert.Equal(t, "high", result["x-risk"])
	})
}

func TestPackConverter_PackToOpenAPI(t *testing.T) {
	converter := NewPackConverter()

	t.Run("convert simple function", func(t *testing.T) {
		manifest := &PackManifest{
			ID:      "test-pack",
			Version: "1.0.0",
			Functions: []PackFunction{
				{
					ID:          "player.ban",
					Name:        "Ban Player",
					Summary:     "Ban a player",
					Description: "Permanently ban a player from the game",
					Params: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"player_id": map[string]interface{}{
								"type":        "string",
								"description": "Player ID",
							},
							"reason": map[string]interface{}{
								"type":        "string",
								"description": "Ban reason",
							},
						},
						"required": []string{"player_id"},
					},
					Returns: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"success": map[string]interface{}{
								"type": "boolean",
							},
						},
					},
					Category:  "player",
					Risk:      "high",
					Operation: "update",
				},
			},
		}

		operations, err := converter.PackToOpenAPI(manifest)
		require.NoError(t, err)
		assert.Contains(t, operations, "player.ban")

		op := operations["player.ban"]
		assert.Equal(t, "player.ban", op.OperationID)
		assert.Equal(t, "Ban a player", op.Summary) // Description is used as Summary if Summary is empty
		assert.NotNil(t, op.RequestBody)
		assert.NotNil(t, op.Responses)

		// Check extensions
		assert.Equal(t, "player", op.Extensions["x-category"])
		assert.Equal(t, "high", op.Extensions["x-risk"])
		assert.Equal(t, "update", op.Extensions["x-operation"])
	})

	t.Run("convert entity operations", func(t *testing.T) {
		manifest := &PackManifest{
			ID:      "test-pack",
			Version: "1.0.0",
			Entities: []PackEntity{
				{
					ID:   "session",
					Name: "Game Session",
					Schema: map[string]interface{}{
						"type": "object",
					},
					Operations: []PackEntityOperation{
						{
							OP:   "create",
							Name: "Create Session",
							Params: map[string]interface{}{
								"type":     "object",
								"required": []string{"player_id"},
							},
							Returns: map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
			},
		}

		operations, err := converter.PackToOpenAPI(manifest)
		require.NoError(t, err)
		assert.Contains(t, operations, "session.create")

		op := operations["session.create"]
		assert.Equal(t, "session.create", op.OperationID)
		assert.Equal(t, "Create Session", op.Summary)
		assert.Equal(t, "session", op.Extensions["x-entity"])
		assert.Equal(t, "create", op.Extensions["x-operation"])
	})
}

func TestExtractExtension(t *testing.T) {
	t.Run("extract string extension", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-category": "player",
			"x-risk":     "high",
		}

		value, exists := ExtractExtension(extensions, "category")
		assert.True(t, exists)
		assert.Equal(t, "player", value)

		value, exists = ExtractExtension(extensions, "risk")
		assert.True(t, exists)
		assert.Equal(t, "high", value)
	})

	t.Run("extract non-existent extension", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-category": "player",
		}

		value, exists := ExtractExtension(extensions, "nonexistent")
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("extract with string helper", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-category": "player",
		}

		value, exists := GetStringExtension(extensions, "category")
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
			"x-category": "player",
			"x-risk":     "low",
		})
		require.NoError(t, err)
		assert.Equal(t, "PlayerService.GetPlayer", op.OperationID)
		assert.Equal(t, "GetPlayer", op.Summary)
		assert.Equal(t, "player", op.Extensions["x-category"])
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
