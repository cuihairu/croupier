// Package converter tests the proto schema conversion.
package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/descriptorpb"
)

// TestProtoConverter_ProtoSchemaToOpenAPISchema tests converting proto descriptors to OpenAPI schemas.
func TestProtoConverter_ProtoSchemaToOpenAPISchema(t *testing.T) {
	converter := NewProtoConverter()

	t.Run("basic message with fields", func(t *testing.T) {
		descriptor := &descriptorpb.DescriptorProto{
			Name: protoString("Player"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     protoString("id"),
					Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					JsonName: protoString("id"),
					Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				},
				{
					Name:     protoString("score"),
					Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_INT32),
					JsonName: protoString("score"),
					Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				},
			},
		}

		schema, err := converter.ProtoSchemaToOpenAPISchema(descriptor)

		assert.Nil(t, err)
		assert.NotNil(t, schema)
		assert.Equal(t, "Player", schema.Description)
		assert.NotNil(t, schema.Type)
		assert.Equal(t, "object", (*schema.Type)[0])
		assert.NotNil(t, schema.Properties)
		assert.Equal(t, 2, len(schema.Properties))
		_, hasID := schema.Properties["id"]
		_, hasScore := schema.Properties["score"]
		assert.True(t, hasID)
		assert.True(t, hasScore)
	})

	t.Run("message with required fields", func(t *testing.T) {
		descriptor := &descriptorpb.DescriptorProto{
			Name: protoString("CreatePlayerRequest"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     protoString("name"),
					Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					JsonName: protoString("name"),
					Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REQUIRED),
				},
				{
					Name:     protoString("email"),
					Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					JsonName: protoString("email"),
					Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				},
			},
		}

		schema, err := converter.ProtoSchemaToOpenAPISchema(descriptor)

		assert.Nil(t, err)
		assert.NotNil(t, schema.Required)
		assert.Equal(t, 1, len(schema.Required))
		assert.Equal(t, "name", schema.Required[0])
	})

	t.Run("empty message", func(t *testing.T) {
		descriptor := &descriptorpb.DescriptorProto{
			Name:  protoString("EmptyMessage"),
			Field: []*descriptorpb.FieldDescriptorProto{},
		}

		schema, err := converter.ProtoSchemaToOpenAPISchema(descriptor)

		assert.Nil(t, err)
		assert.NotNil(t, schema)
		assert.Equal(t, "EmptyMessage", schema.Description)
		assert.Equal(t, 0, len(schema.Properties))
	})
}

// TestProtoConverter_ProtoFieldToSchema_Types tests proto field type mapping.
func TestProtoConverter_ProtoFieldToSchema_Types(t *testing.T) {
	converter := NewProtoConverter()

	tests := []struct {
		name        string
		fieldType   descriptorpb.FieldDescriptorProto_Type
		expected    string
		expectedFmt string
	}{
		{"double", descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "number", "float"},
		{"float", descriptorpb.FieldDescriptorProto_TYPE_FLOAT, "number", "float"},
		{"int64", descriptorpb.FieldDescriptorProto_TYPE_INT64, "integer", ""},
		{"uint64", descriptorpb.FieldDescriptorProto_TYPE_UINT64, "integer", ""},
		{"int32", descriptorpb.FieldDescriptorProto_TYPE_INT32, "integer", ""},
		{"fixed64", descriptorpb.FieldDescriptorProto_TYPE_FIXED64, "integer", ""},
		{"fixed32", descriptorpb.FieldDescriptorProto_TYPE_FIXED32, "integer", ""},
		{"uint32", descriptorpb.FieldDescriptorProto_TYPE_UINT32, "integer", ""},
		{"sfixed32", descriptorpb.FieldDescriptorProto_TYPE_SFIXED32, "integer", ""},
		{"sfixed64", descriptorpb.FieldDescriptorProto_TYPE_SFIXED64, "integer", ""},
		{"sint32", descriptorpb.FieldDescriptorProto_TYPE_SINT32, "integer", ""},
		{"sint64", descriptorpb.FieldDescriptorProto_TYPE_SINT64, "integer", ""},
		{"bool", descriptorpb.FieldDescriptorProto_TYPE_BOOL, "boolean", ""},
		{"string", descriptorpb.FieldDescriptorProto_TYPE_STRING, "string", ""},
		{"bytes", descriptorpb.FieldDescriptorProto_TYPE_BYTES, "string", ""},
		{"enum", descriptorpb.FieldDescriptorProto_TYPE_ENUM, "string", ""},
		{"message", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "object", ""},
		{"group", descriptorpb.FieldDescriptorProto_TYPE_GROUP, "object", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &descriptorpb.FieldDescriptorProto{
				Type: &tt.fieldType,
			}

			schema := converter.protoFieldToSchema(field)

			assert.NotNil(t, schema.Type)
			assert.Equal(t, tt.expected, (*schema.Type)[0])
			if tt.expectedFmt != "" {
				assert.Equal(t, tt.expectedFmt, schema.Format)
			}
		})
	}
}

// TestProtoConverter_ProtoFieldToSchema_Repeated tests repeated field handling.
func TestProtoConverter_ProtoFieldToSchema_Repeated(t *testing.T) {
	converter := NewProtoConverter()

	field := &descriptorpb.FieldDescriptorProto{
		Type:  typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
		Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
	}

	schema := converter.protoFieldToSchema(field)

	assert.NotNil(t, schema.Type)
	assert.Equal(t, "array", (*schema.Type)[0])
	assert.NotNil(t, schema.Items)
	assert.NotNil(t, schema.Items.Value)
}

// TestProtoConverter_ProtoFieldToSchema_DefaultType tests unknown type defaults to string.
func TestProtoConverter_ProtoFieldToSchema_DefaultType(t *testing.T) {
	converter := NewProtoConverter()

	// Unknown type defaults to string
	unknownType := descriptorpb.FieldDescriptorProto_Type(9999)
	field := &descriptorpb.FieldDescriptorProto{
		Type: &unknownType,
	}

	schema := converter.protoFieldToSchema(field)

	assert.NotNil(t, schema.Type)
	assert.Equal(t, "string", (*schema.Type)[0])
}

// Helper functions

func protoString(s string) *string {
	return &s
}

func typePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}

func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &l
}
