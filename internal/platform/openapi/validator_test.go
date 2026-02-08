package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ValidateSpec(t *testing.T) {
	validator := NewValidator()

	t.Run("valid minimal spec", func(t *testing.T) {
		spec := []byte(`{
			"openapi": "3.0.3",
			"info": {
				"title": "Test API",
				"version": "1.0.0"
			},
			"paths": {}
		}`)

		doc, err := validator.ValidateSpec(spec)
		require.NoError(t, err)
		assert.Equal(t, "3.0.3", doc.OpenAPI)
		assert.Equal(t, "Test API", doc.Info.Title)
	})

	t.Run("invalid spec", func(t *testing.T) {
		spec := []byte(`{"invalid": "spec"}`)
		_, err := validator.ValidateSpec(spec)
		assert.Error(t, err)
	})
}

func TestValidator_ValidateOperation(t *testing.T) {
	validator := NewValidator()

	t.Run("valid operation", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		operation := &openapi3.Operation{
			OperationID: "testOperation",
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &stringType,
								},
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(),
		}
		desc := "OK"
		operation.Responses.Set("200", &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: &desc,
			},
		})
		// Add default response with description
		defaultDesc := "Default response"
		operation.Responses.Set("default", &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: &defaultDesc,
			},
		})

		err := validator.ValidateOperation(operation)
		assert.NoError(t, err)
	})

	t.Run("missing operation_id", func(t *testing.T) {
		operation := &openapi3.Operation{}
		err := validator.ValidateOperation(operation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation_id is required")
	})

	t.Run("missing responses", func(t *testing.T) {
		operation := &openapi3.Operation{
			OperationID: "test",
		}
		err := validator.ValidateOperation(operation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one response is required")
	})
}

func TestValidator_ValidateExtensionFields(t *testing.T) {
	validator := NewValidator()

	t.Run("valid extensions", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-risk":      "safe",
			"x-category":  "player",
			"x-operation": "create",
		}
		err := validator.ValidateExtensionFields(extensions)
		assert.NoError(t, err)
	})

	t.Run("invalid risk value", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-risk": "invalid",
		}
		err := validator.ValidateExtensionFields(extensions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid x-risk value")
	})

	t.Run("invalid operation value", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-operation": "invalid",
		}
		err := validator.ValidateExtensionFields(extensions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid x-operation value")
	})

	t.Run("risk must be string", func(t *testing.T) {
		extensions := map[string]interface{}{
			"x-risk": 123,
		}
		err := validator.ValidateExtensionFields(extensions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "x-risk must be a string")
	})
}

func TestValidator_validateRequestBody(t *testing.T) {
	validator := NewValidator()

	t.Run("valid JSON body", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		body := &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &stringType,
						},
					},
				},
			},
		}
		err := validator.validateRequestBody(body)
		assert.NoError(t, err)
	})

	t.Run("missing content", func(t *testing.T) {
		body := &openapi3.RequestBody{}
		err := validator.validateRequestBody(body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at least one content type")
	})
}
