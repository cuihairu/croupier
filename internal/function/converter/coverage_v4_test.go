package converter

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- ToJSONSchema: ExclusiveMin/Max with Value (not Bool) ----

func TestToJSONSchema_ExclusiveBounds_Value_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()
	intType := openapi3.Types{"integer"}

	min := 0.0
	max := 100.0
	exclMinVal := 0.0
	exclMaxVal := 100.0

	schema := &openapi3.Schema{
		Type:         &intType,
		Min:          &min,
		Max:          &max,
		ExclusiveMin: openapi3.ExclusiveBound{Value: &exclMinVal},
		ExclusiveMax: openapi3.ExclusiveBound{Value: &exclMaxVal},
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)
	assert.Equal(t, 0.0, result["exclusiveMinimum"])
	assert.Equal(t, 100.0, result["exclusiveMaximum"])
}

// ---- ToJSONSchema: AdditionalProperties ----

func TestToJSONSchema_AdditionalProperties_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()
	objectType := openapi3.Types{"object"}
	stringType := openapi3.Types{"string"}

	additionalSchema := openapi3.AdditionalProperties{
		Schema: &openapi3.SchemaRef{
			Value: &openapi3.Schema{Type: &stringType},
		},
	}

	schema := &openapi3.Schema{
		Type:                 &objectType,
		AdditionalProperties: additionalSchema,
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)
	additional, ok := result["additionalProperties"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", additional["type"])
}

// ---- ToJSONSchema: allOf, anyOf, oneOf ----

func TestToJSONSchema_Combinators_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()
	objectType := openapi3.Types{"object"}
	stringType := openapi3.Types{"string"}

	schema := &openapi3.Schema{
		AllOf: openapi3.SchemaRefs{
			{Value: &openapi3.Schema{Type: &objectType}},
			{Value: &openapi3.Schema{Type: &stringType}},
		},
		AnyOf: openapi3.SchemaRefs{
			{Value: &openapi3.Schema{Type: &stringType}},
		},
		OneOf: openapi3.SchemaRefs{
			{Value: &openapi3.Schema{Type: &objectType}},
		},
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)

	allOf, ok := result["allOf"].([]interface{})
	require.True(t, ok)
	assert.Len(t, allOf, 2)

	anyOf, ok := result["anyOf"].([]interface{})
	require.True(t, ok)
	assert.Len(t, anyOf, 1)

	oneOf, ok := result["oneOf"].([]interface{})
	require.True(t, ok)
	assert.Len(t, oneOf, 1)
}

// ---- ToJSONSchema: allOf with nil ref entries ----

func TestToJSONSchema_Combinators_NilRefs_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()

	schema := &openapi3.Schema{
		AllOf: openapi3.SchemaRefs{
			nil,
			{Value: nil},
		},
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)
	// Nil refs are skipped, so allOf should be empty
	allOf, ok := result["allOf"].([]interface{})
	if ok {
		assert.Len(t, allOf, 0)
	}
}

// ---- ToJSONSchema: default, nullable, readOnly, writeOnly ----

func TestToJSONSchema_DefaultNullableReadWrite_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()
	stringType := openapi3.Types{"string"}
	defaultVal := "hello"

	schema := &openapi3.Schema{
		Type:      &stringType,
		Default:   defaultVal,
		Nullable:  true,
		ReadOnly:  true,
		WriteOnly: true,
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)
	assert.Equal(t, "hello", result["default"])
	assert.Equal(t, true, result["nullable"])
	assert.Equal(t, true, result["readOnly"])
	assert.Equal(t, true, result["writeOnly"])
}

// ---- ToJSONSchema: property with nil ref ----

func TestToJSONSchema_PropertyNilRef_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()
	objectType := openapi3.Types{"object"}

	schema := &openapi3.Schema{
		Type: &objectType,
		Properties: map[string]*openapi3.SchemaRef{
			"valid":   {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
			"nil_ref": nil,
		},
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)
	props := result["properties"].(map[string]interface{})
	_, hasValid := props["valid"]
	assert.True(t, hasValid)
}

// ---- ExtractExtension: without x- prefix ----

func TestExtractExtension_WithoutPrefix_V4(t *testing.T) {
	t.Parallel()

	t.Run("finds key without x- prefix", func(t *testing.T) {
		extensions := map[string]interface{}{
			"resource": "player",
		}
		value, exists := ExtractExtension(extensions, "resource")
		assert.True(t, exists)
		assert.Equal(t, "player", value)
	})

	t.Run("x- prefix takes precedence", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-resource": "from-x",
			"resource":   "from-plain",
		}
		value, exists := ExtractExtension(extensions, "resource")
		assert.True(t, exists)
		assert.Equal(t, "from-x", value)
	})

	t.Run("not found", func(t *testing.T) {
		extensions := map[string]interface{}{
			"other": "value",
		}
		_, exists := ExtractExtension(extensions, "resource")
		assert.False(t, exists)
	})
}

// ---- ToOpenAPIOperation: capability, execution, permission extensions ----

func TestToOpenAPIOperation_AllExtensions_V4(t *testing.T) {
	t.Parallel()
	descriptor := ProviderFunctionDescriptorDesc{
		OperationID:  "player.ban",
		Summary:      "Ban Player",
		Resource:     "player",
		Risk:         "high",
		Operation:    "ban",
		Capability:   "lifecycle",
		Execution:    "async",
		Permission:   "player.ban",
		InputSchema:  `{"type": "object"}`,
		OutputSchema: `{"type": "object"}`,
	}

	op, err := ToOpenAPIOperation(descriptor)
	require.NoError(t, err)
	assert.Equal(t, "player", op.Extensions["x-resource"])
	assert.Equal(t, "high", op.Extensions["x-risk"])
	assert.Equal(t, "ban", op.Extensions["x-operation"])
	assert.Equal(t, "lifecycle", op.Extensions["x-capability"])
	assert.Equal(t, "async", op.Extensions["x-execution"])
	assert.Equal(t, "player.ban", op.Extensions["x-permission"])
}

// ---- ToOpenAPIOperation: no extensions ----

func TestToOpenAPIOperation_NoExtensions_V4(t *testing.T) {
	t.Parallel()
	descriptor := ProviderFunctionDescriptorDesc{
		OperationID: "test.fn",
		Summary:     "Simple",
	}

	op, err := ToOpenAPIOperation(descriptor)
	require.NoError(t, err)
	assert.Nil(t, op.Extensions)
	assert.Nil(t, op.RequestBody)
	assert.Nil(t, op.Responses)
}

// ---- ToOpenAPIOperation: only capability ----

func TestToOpenAPIOperation_OnlyCapability_V4(t *testing.T) {
	t.Parallel()
	descriptor := ProviderFunctionDescriptorDesc{
		OperationID: "test.cap",
		Capability:  "query",
	}

	op, err := ToOpenAPIOperation(descriptor)
	require.NoError(t, err)
	assert.Equal(t, "query", op.Extensions["x-capability"])
}

// ---- ToOpenAPIOperation: only execution ----

func TestToOpenAPIOperation_OnlyExecution_V4(t *testing.T) {
	t.Parallel()
	descriptor := ProviderFunctionDescriptorDesc{
		OperationID: "test.exec",
		Execution:   "sync",
	}

	op, err := ToOpenAPIOperation(descriptor)
	require.NoError(t, err)
	assert.Equal(t, "sync", op.Extensions["x-execution"])
}

// ---- ToOpenAPIOperation: only permission ----

func TestToOpenAPIOperation_OnlyPermission_V4(t *testing.T) {
	t.Parallel()
	descriptor := ProviderFunctionDescriptorDesc{
		OperationID: "test.perm",
		Permission:  "admin.write",
	}

	op, err := ToOpenAPIOperation(descriptor)
	require.NoError(t, err)
	assert.Equal(t, "admin.write", op.Extensions["x-permission"])
}

// ---- ToJSONSchema: nil type ----

func TestToJSONSchema_NilType_V4(t *testing.T) {
	t.Parallel()
	converter := NewOpenAPIConverter()

	schema := &openapi3.Schema{
		Description: "no type",
	}

	result, err := converter.ToJSONSchema(schema)
	require.NoError(t, err)
	assert.Equal(t, "no type", result["description"])
	assert.Nil(t, result["type"])
}

// ---- GetStringExtension: without x- prefix ----

func TestGetStringExtension_WithoutPrefix_V4(t *testing.T) {
	t.Parallel()
	extensions := map[string]interface{}{
		"resource": "player",
	}
	value, exists := GetStringExtension(extensions, "resource")
	assert.True(t, exists)
	assert.Equal(t, "player", value)
}

// ---- GetBoolExtension: without x- prefix ----

func TestGetBoolExtension_WithoutPrefix_V4(t *testing.T) {
	t.Parallel()
	extensions := map[string]interface{}{
		"enabled": true,
	}
	value, exists := GetBoolExtension(extensions, "enabled")
	assert.True(t, exists)
	assert.True(t, value)
}
