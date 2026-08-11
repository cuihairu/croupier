package generator

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// isRemoteRef
// ---------------------------------------------------------------------------

func TestIsRemoteRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want bool
	}{
		{"http", "http://example.com/schema.json", true},
		{"https", "https://example.com/schema.json", true},
		{"https upper", "HTTPS://EXAMPLE.COM", true},
		{"local ref", "#/definitions/Foo", false},
		{"relative", "./schema.json", false},
		{"empty", "", false},
		{"spaces", "  https://x.com  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRemoteRef(tt.ref))
		})
	}
}

// ---------------------------------------------------------------------------
// schemaSubsetDiagnostic
// ---------------------------------------------------------------------------

func TestSchemaSubsetDiagnostic(t *testing.T) {
	d := schemaSubsetDiagnostic("player.getList", "input.schema", "unsupported feature")
	assert.Equal(t, "json_schema_generation_subset_unsupported", d.Code)
	assert.Equal(t, "player.getList", d.FunctionID)
	assert.Equal(t, "input.schema", d.Field)
	assert.Contains(t, d.Message, "unsupported feature")
}

// ---------------------------------------------------------------------------
// schemaSubsetDiagnostics
// ---------------------------------------------------------------------------

func TestSchemaSubsetDiagnostics_EmptySchema(t *testing.T) {
	assert.Nil(t, schemaSubsetDiagnostics("fn", "field", nil))
	assert.Nil(t, schemaSubsetDiagnostics("fn", "field", []byte{}))
}

func TestSchemaSubsetDiagnostics_InvalidJSON(t *testing.T) {
	diags := schemaSubsetDiagnostics("fn", "field", []byte(`{invalid`))
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "invalid")
}

func TestSchemaSubsetDiagnostics_SupportedSchema(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Nil(t, diags)
}

func TestSchemaSubsetDiagnostics_OneOf(t *testing.T) {
	schema := []byte(`{"oneOf":[{"type":"string"},{"type":"number"}]}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "oneOf")
}

func TestSchemaSubsetDiagnostics_AnyOf(t *testing.T) {
	schema := []byte(`{"anyOf":[{"type":"string"}]}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "anyOf")
}

func TestSchemaSubsetDiagnostics_Discriminator(t *testing.T) {
	schema := []byte(`{"discriminator":{"propertyName":"type"}}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "discriminator")
}

func TestSchemaSubsetDiagnostics_RemoteRef(t *testing.T) {
	schema := []byte(`{"$ref":"https://example.com/schema.json"}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "remote_$ref")
}

func TestSchemaSubsetDiagnostics_LocalRef(t *testing.T) {
	schema := []byte(`{"$ref":"#/definitions/Foo"}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Nil(t, diags)
}

func TestSchemaSubsetDiagnostics_NestedUnsupported(t *testing.T) {
	schema := []byte(`{"properties":{"x":{"oneOf":[{"type":"string"}]}}}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "oneOf")
}

func TestSchemaSubsetDiagnostics_MultipleUnsupported(t *testing.T) {
	schema := []byte(`{"oneOf":[],"anyOf":[]}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "anyOf")
	assert.Contains(t, diags[0].Message, "oneOf")
}

func TestSchemaSubsetDiagnostics_ArrayItems(t *testing.T) {
	schema := []byte(`{"type":"array","items":{"oneOf":[{"type":"string"}]}}`)
	diags := schemaSubsetDiagnostics("fn", "field", schema)
	assert.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "oneOf")
}

func TestSchemaSubsetDiagnostics_NullRaw(t *testing.T) {
	diags := schemaSubsetDiagnostics("fn", "field", spec.JSONSchema(`null`))
	assert.Nil(t, diags)
}
