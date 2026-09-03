package openapi

import (
	"encoding/json"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverterImportFromSpecNilPathItemV9(t *testing.T) {
	converter := NewConverter()

	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "T", Version: "1"},
		Paths:   openapi3.NewPaths(),
	}
	spec.Paths.Set("/valid", &openapi3.PathItem{
		Post: &openapi3.Operation{OperationID: "valid.op"},
	})
	// nil PathItem 必须被跳过而不是 panic。
	spec.Paths.Set("/nil-entry", nil)

	metadatas, err := converter.ImportFromSpec(spec, nil)
	require.NoError(t, err)
	require.Len(t, metadatas, 1)
	assert.Equal(t, "valid.op", metadatas[0].Id)
}

func TestConverterMetadataToOperationNilV9(t *testing.T) {
	_, err := NewConverter().MetadataToOperation(nil)
	require.Error(t, err)
}

func TestConverterDeriveBehaviorModeV9(t *testing.T) {
	converter := NewConverter()

	cases := []struct {
		mode string
		want functionv1.FunctionBehavior_Mode
	}{
		{"command", functionv1.FunctionBehavior_MODE_COMMAND},
		{"write", functionv1.FunctionBehavior_MODE_COMMAND},
		{"COMMAND", functionv1.FunctionBehavior_MODE_COMMAND},
		{"query", functionv1.FunctionBehavior_MODE_QUERY},
		{"read", functionv1.FunctionBehavior_MODE_QUERY},
		{"bogus", functionv1.FunctionBehavior_MODE_QUERY},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			op := &openapi3.Operation{
				Extensions: map[string]interface{}{"x-mode": tc.mode},
			}
			behavior := converter.deriveBehavior(op, nil)
			assert.Equal(t, tc.want, behavior.Mode)
		})
	}
}

func TestConverterDeriveBehaviorIdempotentV9(t *testing.T) {
	converter := NewConverter()

	op := &openapi3.Operation{
		Extensions: map[string]interface{}{"x-idempotent": "true"},
	}
	behavior := converter.deriveBehavior(op, nil)
	assert.True(t, behavior.Idempotent)

	op = &openapi3.Operation{
		Extensions: map[string]interface{}{"x-idempotent": "TRUE"},
	}
	behavior = converter.deriveBehavior(op, nil)
	assert.True(t, behavior.Idempotent)

	op = &openapi3.Operation{
		Extensions: map[string]interface{}{"x-idempotent": "false"},
	}
	behavior = converter.deriveBehavior(op, nil)
	assert.False(t, behavior.Idempotent)
}

func TestConverterDeriveBehaviorOptionsV9(t *testing.T) {
	converter := NewConverter()
	op := &openapi3.Operation{}

	behavior := converter.deriveBehavior(op, &ImportOptions{
		DefaultTimeoutMs:     5000,
		DefaultRouteStrategy: "broadcast",
	})
	assert.Equal(t, int32(5000), behavior.TimeoutMs)
	assert.Equal(t, functionv1.FunctionBehavior_ROUTE_STRATEGY_BROADCAST, behavior.RouteStrategy)

	behavior = converter.deriveBehavior(op, &ImportOptions{DefaultRouteStrategy: "targeted"})
	assert.Equal(t, functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED, behavior.RouteStrategy)

	behavior = converter.deriveBehavior(op, &ImportOptions{DefaultRouteStrategy: "hash"})
	assert.Equal(t, functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH, behavior.RouteStrategy)

	behavior = converter.deriveBehavior(op, &ImportOptions{DefaultRouteStrategy: "unknown"})
	assert.Equal(t, functionv1.FunctionBehavior_ROUTE_STRATEGY_LB, behavior.RouteStrategy)
}

func TestConverterExtractExtensionTypesV9(t *testing.T) {
	converter := NewConverter()

	assert.Equal(t, "", converter.extractExtension(nil, "x-resource"))
	assert.Equal(t, "", converter.extractExtension(map[string]interface{}{}, "missing"))

	ext := map[string]interface{}{
		"number": json.Number("42"),
		"bool":   true,
		"float":  2.5,
		"slice":  []string{"a"},
		"object": map[string]interface{}{"k": "v"},
	}
	assert.Equal(t, "42", converter.extractExtension(ext, "number"))
	assert.Equal(t, "true", converter.extractExtension(ext, "bool"))
	assert.Equal(t, "2.5", converter.extractExtension(ext, "float"))
	assert.Equal(t, "[a]", converter.extractExtension(ext, "slice"))
	assert.Equal(t, "map[k:v]", converter.extractExtension(ext, "object"))
}

func TestSchemaMapperOpenAPIToJSONSchemaMarshalErrorV9(t *testing.T) {
	mapper := NewSchemaMapper()

	// Extensions 携带不可序列化值 → MarshalJSON 失败。
	schema := &openapi3.Schema{
		Extensions: map[string]interface{}{"x-bad": make(chan int)},
	}
	out, err := mapper.OpenAPIToJSONSchema(schema)
	require.Error(t, err)
	assert.Empty(t, out)
}

func TestSchemaMapperBuildSchemaFromObjectBoundsV9(t *testing.T) {
	mapper := NewSchemaMapper()

	obj := map[string]interface{}{
		"type":        "number",
		"description": "a numeric field",
		"minimum":     1.5,
		"maximum":     99.5,
		"minLength":   2.0,
		"maxLength":   10.0,
		"pattern":     "^[a-z]+$",
	}
	schema := mapper.buildSchemaFromObject(obj)

	assert.Equal(t, "a numeric field", schema.Description)
	require.NotNil(t, schema.Min)
	assert.Equal(t, 1.5, *schema.Min)
	require.NotNil(t, schema.Max)
	assert.Equal(t, 99.5, *schema.Max)
	assert.Equal(t, uint64(2), schema.MinLength)
	require.NotNil(t, schema.MaxLength)
	assert.Equal(t, uint64(10), *schema.MaxLength)
	assert.Equal(t, "^[a-z]+$", schema.Pattern)
}

func TestSchemaMapperBuildSchemaAdditionalPropertiesBoolV9(t *testing.T) {
	mapper := NewSchemaMapper()

	schema := mapper.buildSchemaFromObject(map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
	})
	require.NotNil(t, schema.AdditionalProperties.Has)
	assert.False(t, *schema.AdditionalProperties.Has)
	require.NotNil(t, schema.AdditionalProperties.Schema)
	require.NotNil(t, schema.AdditionalProperties.Schema.Value.Type)
	assert.Equal(t, "boolean", (*schema.AdditionalProperties.Schema.Value.Type)[0])
}

func TestSchemaMapperValidateJSONSchemaRootNotObjectV9(t *testing.T) {
	mapper := NewSchemaMapper()

	err := mapper.ValidateJSONSchema(`42`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root must be an object")

	err = mapper.ValidateJSONSchema(`["a"]`)
	require.Error(t, err)

	// 缺少 type/$ref/allOf/anyOf/oneOf。
	err = mapper.ValidateJSONSchema(`{"title":"x"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have type")

	// 复合关键字通过校验。
	assert.NoError(t, mapper.ValidateJSONSchema(`{"allOf":[{"type":"object"}]}`))
}

func TestSchemaMapperMergeSchemasEdgeCasesV9(t *testing.T) {
	mapper := NewSchemaMapper()

	merged, err := mapper.MergeSchemas()
	require.NoError(t, err)
	assert.Equal(t, "{}", merged)

	single := `{"type":"string"}`
	merged, err = mapper.MergeSchemas(single)
	require.NoError(t, err)
	assert.Equal(t, single, merged)

	_, err = mapper.MergeSchemas("42", `{"type":"object"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root must be an object")
}

func TestSchemaMapperGetSchemaTypeEdgesV9(t *testing.T) {
	mapper := NewSchemaMapper()

	_, err := mapper.GetSchemaType(`42`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root must be an object")

	cases := []struct {
		schema string
		want   string
	}{
		{`{"allOf":[{"type":"string"}]}`, "allOf"},
		{`{"anyOf":[{"type":"string"}]}`, "anyOf"},
		{`{"oneOf":[{"type":"string"}]}`, "oneOf"},
		{`{"type": 42}`, "allOf-does-not-match"}, // type 非字符串 → 落入复合检查
	}
	for _, tc := range cases[:3] {
		got, err := mapper.GetSchemaType(tc.schema)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}

	// type 是数字而非字符串：GetSchemaType 返回错误。
	_, err = mapper.GetSchemaType(`{"type": 42}`)
	require.Error(t, err)
}

func TestSchemaMapperGetObjectPropertiesEdgesV9(t *testing.T) {
	mapper := NewSchemaMapper()

	_, err := mapper.GetObjectProperties("")
	require.Error(t, err)

	_, err = mapper.GetObjectProperties(`42`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root must be an object")

	_, err = mapper.GetObjectProperties(`["x"]`)
	require.Error(t, err)
}

func TestConverterOperationExtensionsThroughImportV9(t *testing.T) {
	converter := NewConverter()

	op := &openapi3.Operation{
		OperationID: "ext.op",
		Extensions: map[string]interface{}{
			"x-resource":    "player",
			"x-risk":        "danger",
			"x-permission":  "player.write",
			"x-mode":        "command",
			"x-idempotent":  "true",
			"x-unknown-num": json.Number("7"),
		},
	}
	md, err := converter.ImportToMetadata("ext.op", op)
	require.NoError(t, err)

	assert.Equal(t, "player", md.Resource)
	assert.Equal(t, functionv1.FunctionSecurity_RISK_LEVEL_DANGER, md.Security.RiskLevel)
	assert.True(t, md.Security.RequiresApproval)
	assert.Equal(t, "player.write", md.Security.Permission)
	assert.Equal(t, functionv1.FunctionBehavior_MODE_COMMAND, md.Behavior.Mode)
	assert.True(t, md.Behavior.Idempotent)
}
