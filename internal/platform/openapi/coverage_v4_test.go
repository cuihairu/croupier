package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openapi3Types(s string) openapi3.Types { return openapi3.Types{s} }
func strPtr(s string) *string               { return &s }

func TestDuration_UnmarshalJSON_V4(t *testing.T) {
	t.Parallel()

	t.Run("integer nanoseconds", func(t *testing.T) {
		var d Duration
		err := json.Unmarshal([]byte(`5000000000`), &d)
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, d.Duration())
	})

	t.Run("float seconds", func(t *testing.T) {
		var d Duration
		err := json.Unmarshal([]byte(`2.5`), &d)
		require.NoError(t, err)
		assert.Equal(t, 2500*time.Millisecond, d.Duration())
	})

	t.Run("invalid string duration", func(t *testing.T) {
		var d Duration
		err := json.Unmarshal([]byte(`"not-a-duration"`), &d)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid duration string")
	})

	t.Run("invalid type", func(t *testing.T) {
		var d Duration
		err := json.Unmarshal([]byte(`{"key":"val"}`), &d)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duration must be a string")
	})
}

func TestParseOpenAPISpec_MergePaths_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	// First spec
	spec1 := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List users"
				}
			}
		}
	}`
	err := p.parseOpenAPISpec([]byte(spec1))
	require.NoError(t, err)
	assert.Len(t, p.methods, 1)

	// Second spec — merge should add new path and keep existing
	spec2 := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/items": {
				"get": {
					"operationId": "listItems",
					"summary": "List items"
				}
			}
		}
	}`
	err = p.parseOpenAPISpec([]byte(spec2))
	require.NoError(t, err)
	assert.Len(t, p.methods, 2)
	assert.Contains(t, p.methodMap, "listUsers")
	assert.Contains(t, p.methodMap, "listItems")
}

func TestParseOpenAPISpec_Swagger20_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	spec := `{
		"swagger": "2.0",
		"apis": {
			"/api/books": {
				"get": {
					"operationId": "listBooks",
					"summary": "List books"
				}
			}
		}
	}`
	err := p.parseOpenAPISpec([]byte(spec))
	require.NoError(t, err)
	assert.Len(t, p.methods, 1)
	assert.Contains(t, p.methodMap, "listBooks")
}

func TestParseOpenAPISpec_SkipsParameters_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	spec := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List users"
				},
				"PARAMETERS": {
					"name": "limit"
				},
				"$ref": "#/definitions/Shared"
			}
		}
	}`
	err := p.parseOpenAPISpec([]byte(spec))
	require.NoError(t, err)
	// PARAMETERS and $ref should be skipped
	assert.Len(t, p.methods, 1)
}

func TestParseOpenAPISpec_NonOperationItem_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	// Path item that isn't a map (non-pathItem entry)
	spec := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/users": "not-a-map",
			"/api/items": {
				"get": {
					"operationId": "listItems",
					"summary": "List items"
				}
			}
		}
	}`
	err := p.parseOpenAPISpec([]byte(spec))
	require.NoError(t, err)
	assert.Len(t, p.methods, 1)
}

func TestParseOpenAPISpec_MethodNotMap_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	// Method value that isn't a map
	spec := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/users": {
				"get": "not-a-map"
			}
		}
	}`
	err := p.parseOpenAPISpec([]byte(spec))
	require.NoError(t, err)
	assert.Len(t, p.methods, 0)
}

func TestParseOpenAPISpec_DescriptionFallback_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	// No operationId, no summary, has description — should fall through to description for desc
	spec := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/items": {
				"get": {
					"description": "Long description of the endpoint"
				}
			}
		}
	}`
	err := p.parseOpenAPISpec([]byte(spec))
	require.NoError(t, err)
	assert.Len(t, p.methods, 1)
}

func TestExtractTags_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()

	t.Run("non-array tags", func(t *testing.T) {
		methodObj := map[string]interface{}{"tags": "not-an-array"}
		result := p.extractTags(methodObj)
		assert.Nil(t, result)
	})

	t.Run("mixed tag types", func(t *testing.T) {
		methodObj := map[string]interface{}{
			"tags": []interface{}{"valid", 42, "also-valid"},
		}
		result := p.extractTags(methodObj)
		assert.Equal(t, []string{"valid", "also-valid"}, result)
	})
}

func TestExtractParameters_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()

	t.Run("no parameters key", func(t *testing.T) {
		methodObj := map[string]interface{}{}
		result := p.extractParameters(methodObj)
		assert.Empty(t, result)
	})

	t.Run("non-array parameters", func(t *testing.T) {
		methodObj := map[string]interface{}{"parameters": "not-array"}
		result := p.extractParameters(methodObj)
		assert.Empty(t, result)
	})

	t.Run("non-map parameter entry", func(t *testing.T) {
		methodObj := map[string]interface{}{
			"parameters": []interface{}{"not-a-map", 42},
		}
		result := p.extractParameters(methodObj)
		assert.Empty(t, result)
	})

	t.Run("parameter with schema type/format/enum", func(t *testing.T) {
		methodObj := map[string]interface{}{
			"parameters": []interface{}{
				map[string]interface{}{
					"name":        "status",
					"in":          "query",
					"required":    true,
					"description": "Filter status",
					"deprecated":  true,
					"default":     "active",
					"schema": map[string]interface{}{
						"type":   "string",
						"format": "enum",
						"enum":   []interface{}{"active", "inactive"},
					},
				},
			},
		}
		result := p.extractParameters(methodObj)
		require.Len(t, result, 1)
		assert.Equal(t, "status", result[0].Name)
		assert.Equal(t, "query", result[0].In)
		assert.True(t, result[0].Required)
		assert.True(t, result[0].Deprecated)
		assert.Equal(t, "active", result[0].Default)
		assert.Equal(t, "string", result[0].Type)
		assert.Equal(t, "enum", result[0].Format)
		assert.Len(t, result[0].Enum, 2)
	})
}

func TestValidateSchema_V4(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	t.Run("nil schema", func(t *testing.T) {
		err := v.validateSchema(nil)
		assert.NoError(t, err)
	})

	t.Run("array without items", func(t *testing.T) {
		arrayType := openapi3Types("array")
		schema := &openapi3.Schema{Type: &arrayType}
		err := v.validateSchema(schema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "array type must have items defined")
	})

	t.Run("object with properties (no error)", func(t *testing.T) {
		objectType := openapi3Types("object")
		schema := &openapi3.Schema{Type: &objectType}
		err := v.validateSchema(schema)
		assert.NoError(t, err)
	})
}

func TestValidateOperation_V4(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	t.Run("nil operation", func(t *testing.T) {
		err := v.ValidateOperation(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation is nil")
	})

	t.Run("nil response ref", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "test",
			Responses:   openapi3.NewResponses(),
		}
		op.Responses.Set("200", nil)
		err := v.ValidateOperation(op)
		// nil response ref should be skipped (continue), no error from validation
		assert.NoError(t, err)
	})

	t.Run("nil response value", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "test",
			Responses:   openapi3.NewResponses(),
		}
		op.Responses.Set("200", &openapi3.ResponseRef{Value: nil})
		err := v.ValidateOperation(op)
		assert.NoError(t, err)
	})

	t.Run("invalid request body schema", func(t *testing.T) {
		arrayType := openapi3Types("array")
		op := &openapi3.Operation{
			OperationID: "test",
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{Type: &arrayType},
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(),
		}
		op.Responses.Set("200", &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: strPtr("OK"),
			},
		})
		err := v.ValidateOperation(op)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON schema")
	})

	t.Run("invalid response schema", func(t *testing.T) {
		arrayType := openapi3Types("array")
		op := &openapi3.Operation{
			OperationID: "test",
			Responses:   openapi3.NewResponses(),
		}
		op.Responses.Set("200", &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: strPtr("OK"),
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &arrayType},
						},
					},
				},
			},
		})
		err := v.ValidateOperation(op)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON schema")
	})

	t.Run("request body with empty content", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "test",
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{},
				},
			},
			Responses: openapi3.NewResponses(),
		}
		op.Responses.Set("200", &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: strPtr("OK"),
			},
		})
		err := v.ValidateOperation(op)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at least one content type")
	})
}

func TestValidateRequestBody_V4(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	t.Run("JSON content without schema", func(t *testing.T) {
		body := &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{},
			},
		}
		err := v.validateRequestBody(body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have a schema")
	})

	t.Run("non-JSON content type is accepted", func(t *testing.T) {
		body := &openapi3.RequestBody{
			Content: openapi3.Content{
				"text/plain": &openapi3.MediaType{},
			},
		}
		err := v.validateRequestBody(body)
		assert.NoError(t, err)
	})
}

func TestValidateResponse_V4(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	t.Run("empty description", func(t *testing.T) {
		desc := ""
		resp := &openapi3.Response{Description: &desc}
		err := v.validateResponse(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "response description is required")
	})

	t.Run("nil description", func(t *testing.T) {
		resp := &openapi3.Response{Description: nil}
		err := v.validateResponse(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "response description is required")
	})

	t.Run("response with content but no JSON schema", func(t *testing.T) {
		desc := "OK"
		resp := &openapi3.Response{
			Description: &desc,
			Content: openapi3.Content{
				"text/plain": &openapi3.MediaType{},
			},
		}
		err := v.validateResponse(resp)
		assert.NoError(t, err)
	})

	t.Run("response with JSON content and nil schema", func(t *testing.T) {
		desc := "OK"
		resp := &openapi3.Response{
			Description: &desc,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{},
			},
		}
		err := v.validateResponse(resp)
		assert.NoError(t, err)
	})
}

func TestValidateExtensionFields_V4(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	t.Run("nil extensions", func(t *testing.T) {
		err := v.ValidateExtensionFields(nil)
		assert.NoError(t, err)
	})

	t.Run("x-risk with valid values", func(t *testing.T) {
		for _, val := range []string{"safe", "warning", "high", "danger"} {
			exts := map[string]interface{}{"x-risk": val}
			err := v.ValidateExtensionFields(exts)
			assert.NoError(t, err, "x-risk=%s", val)
		}
	})

	t.Run("x-resource not string", func(t *testing.T) {
		exts := map[string]interface{}{"x-resource": 123}
		err := v.ValidateExtensionFields(exts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "x-resource must be a string")
	})

	t.Run("x-operation not string", func(t *testing.T) {
		exts := map[string]interface{}{"x-operation": 123}
		err := v.ValidateExtensionFields(exts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "x-operation must be a string")
	})

	t.Run("x-enabled not bool", func(t *testing.T) {
		exts := map[string]interface{}{"x-enabled": "yes"}
		err := v.ValidateExtensionFields(exts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "x-enabled must be a boolean")
	})

	t.Run("x-permission not string", func(t *testing.T) {
		exts := map[string]interface{}{"x-permission": 42}
		err := v.ValidateExtensionFields(exts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "x-permission must be a string")
	})

	t.Run("all valid extensions", func(t *testing.T) {
		exts := map[string]interface{}{
			"x-risk":       "safe",
			"x-resource":   "player",
			"x-operation":  "ban",
			"x-enabled":    true,
			"x-permission": "player:ban",
		}
		err := v.ValidateExtensionFields(exts)
		assert.NoError(t, err)
	})
}

func TestTransformResponse_NoDataField_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{
		BaseURL: "http://example.com",
		Transform: &TransformConfig{
			SuccessField: "code",
			SuccessValue: float64(0),
			ErrorField:   "message",
			// No DataField — should wrap entire response as data
		},
	}

	body := []byte(`{"code": 0, "message": "ok", "extra": "value"}`)
	result, err := p.transformResponse(body)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(result, &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	// Without DataField, data should be the entire original response
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ok", data["message"])
}

func TestTransformResponse_NonJSON_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{
		BaseURL: "http://example.com",
		Transform: &TransformConfig{
			SuccessField: "code",
		},
	}

	body := []byte(`not json at all`)
	result, err := p.transformResponse(body)
	require.NoError(t, err)
	assert.Equal(t, body, result)
}

func TestTransformResponse_NoTransform_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}

	body := []byte(`{"key": "value"}`)
	result, err := p.transformResponse(body)
	require.NoError(t, err)
	assert.Equal(t, body, result)
}

func TestCall_RetryAnd4xx_V4(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl":    server.URL,
			"retryCount": 3,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "retry_test",
					"path":   "/test",
					"method": "GET",
				},
			},
		},
	})

	_, err := p.Call(context.Background(), "retry_test", nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestCall_4xxError_V4(t *testing.T) {
	t.Parallel()
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	})
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "err_test",
					"path":   "/test",
					"method": "GET",
				},
			},
		},
	})

	_, err := p.Call(context.Background(), "err_test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestCall_InvalidJSON_V4(t *testing.T) {
	t.Parallel()
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "json_test",
					"path":   "/test",
					"method": "POST",
				},
			},
		},
	})

	_, err := p.Call(context.Background(), "json_test", []byte(`{invalid json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request JSON")
}

func TestBuildRequest_PUTPatch_V4(t *testing.T) {
	t.Parallel()
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	})
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "put_test",
					"path":   "/test",
					"method": "PUT",
					"requestBody": map[string]interface{}{
						"type": "json",
						"fields": map[string]interface{}{
							"name": "name",
						},
					},
				},
				map[string]interface{}{
					"name":   "patch_no_body",
					"path":   "/test",
					"method": "PATCH",
				},
				map[string]interface{}{
					"name":       "header_param",
					"path":       "/test",
					"method":     "GET",
					"parameters": []interface{}{map[string]interface{}{"name": "X-Req-ID", "in": "header"}},
				},
			},
		},
	})

	// PUT with body
	reqData, _ := json.Marshal(map[string]interface{}{"name": "test"})
	_, err := p.Call(context.Background(), "put_test", reqData)
	assert.NoError(t, err)

	// PATCH without body config but with data
	reqData2, _ := json.Marshal(map[string]interface{}{"foo": "bar"})
	_, err = p.Call(context.Background(), "patch_no_body", reqData2)
	assert.NoError(t, err)

	// GET with header param
	reqData3, _ := json.Marshal(map[string]interface{}{"X-Req-ID": "abc"})
	_, err = p.Call(context.Background(), "header_param", reqData3)
	assert.NoError(t, err)
}

func TestBuildBody_NonJSONType_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}

	body, ct := p.buildBody(&RequestBodyMapping{Type: "text"}, nil)
	assert.Nil(t, body)
	assert.Equal(t, "", ct)
}

func TestGetParamValue_NilValue_V4(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}

	// Request data has key but value is nil
	reqData := map[string]interface{}{"key": nil}
	val := p.getParamValue(reqData, ParameterMapping{Name: "key"})
	assert.Equal(t, "", val)

	// Request data missing key, has default
	val = p.getParamValue(map[string]interface{}{}, ParameterMapping{Name: "key", Default: "fallback"})
	assert.Equal(t, "fallback", val)
}

func TestDiscoverMethodsFromSpec_File_V4(t *testing.T) {
	// Create a temporary OpenAPI spec file
	specJSON := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/test": {
				"get": {
					"operationId": "testOp",
					"summary": "Test operation"
				}
			}
		}
	}`
	tmpFile := t.TempDir() + "/spec.json"
	err := os.WriteFile(tmpFile, []byte(specJSON), 0644)
	require.NoError(t, err)

	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	err = p.discoverMethodsFromSpec(context.Background(), tmpFile)
	require.NoError(t, err)
	assert.Len(t, p.methods, 1)
	assert.Contains(t, p.methodMap, "testOp")
}

func TestDiscoverMethodsFromSpec_YAMLFile_V4(t *testing.T) {
	specYAML := `
openapi: "3.0.3"
paths:
  /api/yaml-test:
    get:
      operationId: yamlOp
      summary: YAML operation
`
	tmpFile := t.TempDir() + "/spec.yaml"
	err := os.WriteFile(tmpFile, []byte(specYAML), 0644)
	require.NoError(t, err)

	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	err = p.discoverMethodsFromSpec(context.Background(), tmpFile)
	require.NoError(t, err)
	assert.Len(t, p.methods, 1)
	assert.Contains(t, p.methodMap, "yamlOp")
}

func TestDiscoverMethodsFromSpec_FileNotFound_V4(t *testing.T) {
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	err := p.discoverMethodsFromSpec(context.Background(), "/nonexistent/spec.json")
	assert.Error(t, err)
}

func TestInit_OpenAPISpec_File_V4(t *testing.T) {
	specJSON := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/discover": {
				"get": {
					"operationId": "discoverOp",
					"summary": "Discover operation"
				}
			}
		}
	}`
	tmpFile := t.TempDir() + "/spec.json"
	err := os.WriteFile(tmpFile, []byte(specJSON), 0644)
	require.NoError(t, err)

	p := NewProvider()
	err = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl":     "http://example.com",
			"openapiSpec": tmpFile,
		},
	})
	require.NoError(t, err)
	assert.Contains(t, p.methodMap, "discoverOp")
}

func TestInit_OpenAPISpecs_Multiple_V4(t *testing.T) {
	spec1 := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/a": { "get": { "operationId": "opA" } }
		}
	}`
	spec2 := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/b": { "get": { "operationId": "opB" } }
		}
	}`
	f1 := t.TempDir() + "/a.json"
	f2 := t.TempDir() + "/b.json"
	require.NoError(t, os.WriteFile(f1, []byte(spec1), 0644))
	require.NoError(t, os.WriteFile(f2, []byte(spec2), 0644))

	p := NewProvider()
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl":      "http://example.com",
			"openapiSpecs": []interface{}{f1, f2},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, p.methodMap, "opA")
	assert.Contains(t, p.methodMap, "opB")
}

func TestGetOpenAPIDoc_AfterInit_V4(t *testing.T) {
	specJSON := `{
		"openapi": "3.0.3",
		"paths": {}
	}`
	p := NewProvider()
	p.openapiDoc = json.RawMessage(specJSON)
	doc := p.GetOpenAPIDoc()
	assert.NotNil(t, doc)
	assert.Contains(t, string(doc), "3.0.3")
}

// Helper: create a simple test HTTP server
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}
