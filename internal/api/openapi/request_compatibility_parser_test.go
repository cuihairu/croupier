package openapi

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-011 end to end through the source pipeline: the tier reaches the source
// operation DTO so the detail view can say why an operation is not directly
// callable, and the field-level findings reach the source diagnostics.

const sui011SourceSpec = `{
  "openapi": "3.1.0",
  "info": {"title": "compatibility", "version": "1"},
  "paths": {
    "/things": {
      "post": {
        "operationId": "createThing",
        "requestBody": {"required": true, "content": {
          "application/json": {"schema": {"type": "object", "properties": {"name": {"type": "string"}}}}}}
      },
      "put": {
        "operationId": "uploadThing",
        "requestBody": {"required": true, "content": {
          "multipart/form-data": {"schema": {"type": "object"}}}}
      },
      "delete": {
        "operationId": "removeThing",
        "requestBody": {"content": {
          "application/json": {"schema": {
            "type": "object",
            "oneOf": [{"type": "object"}, {"type": "object"}],
            "discriminator": {"propertyName": "kind"}}}}}
      }
    }
  }
}`

func TestParseOpenAPISource_AssessesRequestCompatibility(t *testing.T) {
	parsed := parseOpenAPISource([]byte(sui011SourceSpec))
	require.Len(t, parsed.Operations, 3)

	byID := map[string]OpenAPISourceOperation{}
	for _, operation := range parsed.Operations {
		byID[operation.OperationID] = operation
	}

	require.Contains(t, byID, "createThing")
	require.Contains(t, byID, "uploadThing")
	require.Contains(t, byID, "removeThing")
	assert.Equal(t, spec.RequestCompatibilityDirect, byID["createThing"].RequestCompatibility)
	assert.Equal(t, spec.RequestCompatibilityManual, byID["uploadThing"].RequestCompatibility)
	assert.Equal(t, spec.RequestCompatibilityManual, byID["removeThing"].RequestCompatibility)

	codes := map[string]bool{}
	for _, diagnostic := range parsed.Diagnostics {
		codes[diagnostic.Code] = true
	}
	assert.True(t, codes[spec.DiagSchemaCompatibilityManual],
		"expected the manual finding on the source diagnostics, got %+v", parsed.Diagnostics)

	var multipart, discriminator bool
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Field == "$.body.content" {
			multipart = true
		}
		if diagnostic.Field == "$.body.content.application~1json/schema/discriminator" {
			discriminator = true
		}
	}
	assert.True(t, multipart, "the multipart operation must name its media type")
	assert.True(t, discriminator, "the discriminator must name its field path")
}
