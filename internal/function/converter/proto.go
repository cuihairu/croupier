package converter

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ProtoConverter converts Proto descriptors to OpenAPI 3.0.3 Operations
type ProtoConverter struct {
	openapiConverter *OpenAPIConverter
}

// NewProtoConverter creates a new Proto converter instance
func NewProtoConverter() *ProtoConverter {
	return &ProtoConverter{
		openapiConverter: NewOpenAPIConverter(),
	}
}

// ProtoMethodInfo contains information about a Proto method
type ProtoMethodInfo struct {
	Name            string
	Package         string
	Service         string
	InputType       string
	OutputType      string
	ClientStreaming bool
	ServerStreaming bool
	Options         map[string]string
}

// ProtoToOpenAPI converts a Proto method descriptor to OpenAPI 3.0.3 Operation
func (c *ProtoConverter) ProtoToOpenAPI(method *ProtoMethodInfo, extensions map[string]interface{}) (*openapi3.Operation, error) {
	op := &openapi3.Operation{
		OperationID: fmt.Sprintf("%s.%s", method.Service, method.Name),
		Summary:     method.Name,
		Description: fmt.Sprintf("gRPC method: %s.%s/%s", method.Package, method.Service, method.Name),
	}

	// Extensions
	if extensions != nil {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		for key, value := range extensions {
			op.Extensions[key] = value
		}
	}

	// Set streaming info in extensions
	if method.ClientStreaming || method.ServerStreaming {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-client-streaming"] = method.ClientStreaming
		op.Extensions["x-server-streaming"] = method.ServerStreaming
	}

	// Note: Full schema conversion requires FileDescriptorSet
	// This is a simplified version that creates placeholders for request/response
	objectType := openapi3.Types{"object"}
	requestSchema := openapi3.NewSchema()
	requestSchema.Type = &objectType
	requestSchema.Description = fmt.Sprintf("Request: %s", method.InputType)

	responseSchema := openapi3.NewSchema()
	responseSchema.Type = &objectType
	responseSchema.Description = fmt.Sprintf("Response: %s", method.OutputType)

	// Request body
	op.RequestBody = &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{
			Description: "Request payload",
			Required:    true,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{Value: requestSchema},
				},
			},
		},
	}

	// Response body
	desc := "Response payload"
	response := &openapi3.Response{
		Description: &desc,
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{
				Schema: &openapi3.SchemaRef{Value: responseSchema},
			},
		},
	}

	op.Responses = openapi3.NewResponses()
	op.Responses.Set("200", &openapi3.ResponseRef{Value: response})

	return op, nil
}

// ProtoSchemaToOpenAPISchema converts a Proto message descriptor to OpenAPI Schema
// This is a placeholder for full proto-to-json-schema conversion
func (c *ProtoConverter) ProtoSchemaToOpenAPISchema(descriptor *descriptorpb.DescriptorProto) (*openapi3.Schema, error) {
	objectType := openapi3.Types{"object"}
	schema := openapi3.NewSchema()
	schema.Type = &objectType
	schema.Description = descriptor.GetName()

	// Convert fields
	if len(descriptor.Field) > 0 {
		properties := make(map[string]*openapi3.SchemaRef)
		required := make([]string, 0)

		for _, field := range descriptor.Field {
			fieldSchema := c.protoFieldToSchema(field)
			properties[field.GetName()] = &openapi3.SchemaRef{Value: fieldSchema}

			// Mark required fields (proto3 doesn't have explicit required, but labels can indicate)
			if field.Label != nil && *field.Label == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
				required = append(required, field.GetName())
			}
		}

		schema.Properties = properties
		if len(required) > 0 {
			schema.Required = required
		}
	}

	return schema, nil
}

// protoFieldToSchema converts a Proto field to OpenAPI Schema
func (c *ProtoConverter) protoFieldToSchema(field *descriptorpb.FieldDescriptorProto) *openapi3.Schema {
	schema := openapi3.NewSchema()

	// Set type based on Proto type
	if field.Type != nil {
		switch *field.Type {
		case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
			descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
			t := openapi3.Types{"number"}
			schema.Type = &t
			schema.Format = "float"
		case descriptorpb.FieldDescriptorProto_TYPE_INT64,
			descriptorpb.FieldDescriptorProto_TYPE_UINT64,
			descriptorpb.FieldDescriptorProto_TYPE_INT32,
			descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
			descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
			descriptorpb.FieldDescriptorProto_TYPE_UINT32,
			descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
			descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
			descriptorpb.FieldDescriptorProto_TYPE_SINT32,
			descriptorpb.FieldDescriptorProto_TYPE_SINT64:
			t := openapi3.Types{"integer"}
			schema.Type = &t
		case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
			t := openapi3.Types{"boolean"}
			schema.Type = &t
		case descriptorpb.FieldDescriptorProto_TYPE_STRING,
			descriptorpb.FieldDescriptorProto_TYPE_BYTES:
			t := openapi3.Types{"string"}
			schema.Type = &t
		case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
			t := openapi3.Types{"string"}
			schema.Type = &t
		case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
			descriptorpb.FieldDescriptorProto_TYPE_GROUP:
			t := openapi3.Types{"object"}
			schema.Type = &t
		default:
			t := openapi3.Types{"string"}
			schema.Type = &t
		}
	}

	// Set label (repeated -> array)
	if field.Label != nil && *field.Label == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		t := openapi3.Types{"array"}
		schema.Type = &t
		itemSchema := openapi3.NewSchema()
		t2 := openapi3.Types{"string"}
		itemSchema.Type = &t2 // Placeholder, would need recursion
		schema.Items = &openapi3.SchemaRef{Value: itemSchema}
	}

	// Set description from proto comments (if available)
	if field.Options != nil && field.Options.UninterpretedOption != nil {
		// Could parse comments from uninterpreted options
	}

	return schema
}
