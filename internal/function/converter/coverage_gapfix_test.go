package converter

import (
	"errors"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// failSubSchemaConversion 将 convertSubSchema 缝隙替换为恒定失败实现，
// 用于驱动 ToJSONSchema 各嵌套结构的错误传播分支。
func failSubSchemaConversion(t *testing.T) {
	t.Helper()
	orig := convertSubSchema
	convertSubSchema = func(c *OpenAPIConverter, schema *openapi3.Schema) (map[string]interface{}, error) {
		return nil, errSubSchemaBoom
	}
	t.Cleanup(func() { convertSubSchema = orig })
}

var errSubSchemaBoom = errors.New("subschema conversion boom")

func gapfixStringSchema() *openapi3.Schema {
	st := openapi3.Types{"string"}
	return &openapi3.Schema{Type: &st}
}

func TestToJSONSchema_NestedConversionErrors(t *testing.T) {
	objectType := openapi3.Types{"object"}
	cases := []struct {
		name   string
		schema *openapi3.Schema
	}{
		{
			name: "properties",
			schema: &openapi3.Schema{
				Type: &objectType,
				Properties: openapi3.Schemas{
					"name": {Value: gapfixStringSchema()},
				},
			},
		},
		{
			name:   "items",
			schema: &openapi3.Schema{Items: &openapi3.SchemaRef{Value: gapfixStringSchema()}},
		},
		{
			name: "additionalProperties",
			schema: &openapi3.Schema{
				AdditionalProperties: openapi3.AdditionalProperties{
					Schema: &openapi3.SchemaRef{Value: gapfixStringSchema()},
				},
			},
		},
		{
			name:   "allOf",
			schema: &openapi3.Schema{AllOf: openapi3.SchemaRefs{{Value: gapfixStringSchema()}}},
		},
		{
			name:   "anyOf",
			schema: &openapi3.Schema{AnyOf: openapi3.SchemaRefs{{Value: gapfixStringSchema()}}},
		},
		{
			name:   "oneOf",
			schema: &openapi3.Schema{OneOf: openapi3.SchemaRefs{{Value: gapfixStringSchema()}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failSubSchemaConversion(t)

			result, err := NewOpenAPIConverter().ToJSONSchema(tc.schema)

			require.Error(t, err)
			require.Nil(t, result)
			assert.ErrorIs(t, err, errSubSchemaBoom)
		})
	}
}

// TestProtoSchemaToOpenAPISchema_FieldWithUninterpretedOptions 覆盖
// protoFieldToSchema 中 field.Options.UninterpretedOption 非空的分支。
func TestProtoSchemaToOpenAPISchema_FieldWithUninterpretedOptions(t *testing.T) {
	field := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String("label"),
		Number: proto.Int32(1),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		Options: &descriptorpb.FieldOptions{
			UninterpretedOption: []*descriptorpb.UninterpretedOption{{
				Name: []*descriptorpb.UninterpretedOption_NamePart{{
					NamePart:    proto.String("x-note"),
					IsExtension: proto.Bool(false),
				}},
			}},
		},
	}

	schema, err := NewProtoConverter().ProtoSchemaToOpenAPISchema(&descriptorpb.DescriptorProto{
		Name:  proto.String("Tag"),
		Field: []*descriptorpb.FieldDescriptorProto{field},
	})

	require.NoError(t, err)
	require.NotNil(t, schema.Properties["label"])
	require.NotNil(t, schema.Properties["label"].Value)
	require.NotNil(t, schema.Properties["label"].Value.Type)
	assert.Equal(t, "string", (*schema.Properties["label"].Value.Type)[0])
}
