package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specSerializationDoc covers the SUI-006 support matrix: path
// (simple/label/matrix), query (form/deepObject/pipeDelimited/spaceDelimited),
// header (simple), cookie (form primitive), plus an unexecutable cookie-array
// combo and a same-name path+query collision (grouped projection).
const specSerializationDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "serialization", "version": "1.0.0"},
  "paths": {
    "/users/{id}": {
      "get": {
        "operationId": "user.get",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}]
      }
    },
    "/files/{id}": {
      "get": {
        "operationId": "file.matrix",
        "parameters": [{"name": "id", "in": "path", "required": true, "style": "matrix", "schema": {"type": "integer"}}]
      }
    },
    "/versions/{ver}": {
      "get": {
        "operationId": "ver.label",
        "parameters": [{"name": "ver", "in": "path", "required": true, "style": "label", "schema": {"type": "string"}}]
      }
    },
    "/search": {
      "get": {
        "operationId": "search.run",
        "parameters": [
          {"name": "ids", "in": "query", "schema": {"type": "array", "items": {"type": "integer"}}},
          {"name": "meta", "in": "query", "style": "deepObject", "explode": true, "schema": {"type": "object"}},
          {"name": "tags", "in": "query", "style": "pipeDelimited", "explode": false, "schema": {"type": "array", "items": {"type": "string"}}},
          {"name": "spaced", "in": "query", "style": "spaceDelimited", "explode": false, "schema": {"type": "array", "items": {"type": "string"}}},
          {"name": "X-Trace", "in": "header", "schema": {"type": "array", "items": {"type": "string"}}},
          {"name": "token", "in": "cookie", "schema": {"type": "string"}}
        ]
      }
    },
    "/strict": {
      "get": {
        "operationId": "strict.cookieArray",
        "parameters": [{"name": "bad", "in": "cookie", "schema": {"type": "array", "items": {"type": "string"}}}]
      }
    },
    "/invites/{token}": {
      "delete": {
        "operationId": "invite.revoke",
        "parameters": [
          {"name": "token", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "token", "in": "query", "schema": {"type": "string"}}
        ]
      }
    }
  }
}`

// wireCapture records one request's wire form for assertions.
type wireCapture struct {
	path        string
	escapedPath string
	rawQuery    string
	trace       string
	cookie      string
}

func serializationTestServer(cap *wireCapture) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.path = r.URL.Path
		cap.escapedPath = r.URL.EscapedPath()
		cap.rawQuery = r.URL.RawQuery
		cap.trace = r.Header.Get("X-Trace")
		cap.cookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
}

// serializationCall invokes one method and fails the test on error.
func serializationCall(t *testing.T, p *Provider, method string, payload map[string]any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = p.Call(context.Background(), method, raw)
	require.NoError(t, err)
}

// serializationSpecProvider discovers methods from the matrix document via a
// temp file (the kin path, so Request contracts carry style/explode).
func serializationSpecProvider(t *testing.T) *Provider {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "spec.json")
	require.NoError(t, os.WriteFile(file, []byte(specSerializationDoc), 0o600))
	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"openapiSpec": file,
		},
	}))
	p.httpClient = &http.Client{}
	return p
}

// TestProviderCall_ParameterSerializationWire asserts the actual HTTP wire:
// path replacement with escaping, query encodings, header and cookie forms.
func TestProviderCall_ParameterSerializationWire(t *testing.T) {
	var cap wireCapture
	server := serializationTestServer(&cap)
	defer server.Close()

	p := serializationSpecProvider(t)
	p.openapiConfig.BaseURL = server.URL

	// path simple: verbatim, percent-encoded when it contains a slash.
	serializationCall(t, p, "user.get", map[string]any{"id": "123"})
	assert.Equal(t, "/users/123", cap.escapedPath)
	serializationCall(t, p, "user.get", map[string]any{"id": "a/b"})
	assert.Equal(t, "/users/a%2Fb", cap.escapedPath)

	// path matrix and label.
	serializationCall(t, p, "file.matrix", map[string]any{"id": float64(5)})
	assert.Equal(t, "/files/;id=5", cap.escapedPath)
	serializationCall(t, p, "ver.label", map[string]any{"ver": "1.2"})
	assert.Equal(t, "/versions/.1.2", cap.escapedPath)

	// query array (form explode=true), deepObject, pipeDelimited,
	// spaceDelimited, plus header array and cookie primitive.
	serializationCall(t, p, "search.run", map[string]any{
		"ids":     []any{1, 2},
		"meta":    map[string]any{"a": "1", "b": "2"},
		"tags":    []any{"x", "y"},
		"spaced":  []any{"a b", "c"},
		"X-Trace": []any{"p", "q"},
		"token":   "8",
	})
	assert.Equal(t, "ids=1&ids=2&meta%5Ba%5D=1&meta%5Bb%5D=2&tags=x|y&spaced=a+b+c", cap.rawQuery)
	assert.Equal(t, "p,q", cap.trace)
	assert.Equal(t, "token=8", cap.cookie)
	assert.Equal(t, "/search", cap.path)

	// Unexecutable combo (cookie array) errors instead of mis-serializing.
	_, err := p.Call(context.Background(), "strict.cookieArray", []byte(`{"bad":["a","b"]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cookie")
}

// TestProviderCall_LegacyConfigSerialization covers config-declared methods
// without a contract: OpenAPI default styles apply (query form explode=true)
// and existing behavior does not regress.
func TestProviderCall_LegacyConfigSerialization(t *testing.T) {
	var cap wireCapture
	server := serializationTestServer(&cap)
	defer server.Close()

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "list_all",
					"path":   "/api/items",
					"method": "GET",
					"parameters": []interface{}{
						map[string]interface{}{"name": "names", "in": "query"},
						map[string]interface{}{"name": "X-Trace", "in": "header"},
					},
				},
			},
		},
	}))
	p.httpClient = &http.Client{}

	raw, err := json.Marshal(map[string]any{"names": []any{"a", "b"}, "X-Trace": "ops"})
	require.NoError(t, err)
	_, err = p.Call(context.Background(), "list_all", raw)
	require.NoError(t, err)
	assert.Equal(t, "names=a&names=b", cap.rawQuery)
	assert.Equal(t, "ops", cap.trace)
}

// TestProjectInput_SerializationDiagnostics asserts unexecutable combos land
// as error diagnostics on the projected method (proposal quality gate input).
func TestProjectInput_SerializationDiagnostics(t *testing.T) {
	p := serializationSpecProvider(t)
	p.mu.RLock()
	method, exists := p.methodMap["strict.cookieArray"]
	p.mu.RUnlock()
	require.True(t, exists)

	found := false
	for _, d := range method.projection.Diagnostics {
		if d.Code == diagParameterSerializationUnsupported {
			found = true
			assert.Equal(t, "bad", d.Field)
			assert.Equal(t, spec.SeverityError, d.Severity)
		}
	}
	assert.True(t, found, "diagnostics = %+v, want %s", method.projection.Diagnostics, diagParameterSerializationUnsupported)
}

// TestCall_GroupedProjectionRefused asserts the fail-closed guard: a
// location-grouped contract (same-name path+query collision) is refused at
// Call entry instead of silently losing the parameter values.
func TestCall_GroupedProjectionRefused(t *testing.T) {
	p := serializationSpecProvider(t)
	p.mu.RLock()
	method, exists := p.methodMap["invite.revoke"]
	p.mu.RUnlock()
	require.True(t, exists)
	assert.True(t, method.projection.Grouped, "same-name path+query collision must project grouped")

	_, err := p.Call(context.Background(), "invite.revoke", []byte(`{"token":"t1"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires location-grouped input")
}

// TestBuildURL_MissingPathParamStaysEmpty asserts a missing path parameter
// replaces as empty string instead of leaking the placeholder (both contract
// and legacy sources).
func TestBuildURL_MissingPathParamStaysEmpty(t *testing.T) {
	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://x"}

	legacy := &APIMethod{Path: "/players/{id}", Method: "GET", Parameters: []ParameterMapping{{Name: "id", In: "path"}}}
	assert.Equal(t, "http://x/players/", p.buildURL(legacy, map[string]any{}))

	explode := false
	contract := &APIMethod{
		Path:   "/players/{id}",
		Method: "GET",
		Request: &RequestContract{Parameters: []RequestParameter{
			{In: "path", Name: "id", Required: true, Style: "simple", Explode: &explode},
		}},
	}
	assert.Equal(t, "http://x/players/", p.buildURL(contract, map[string]any{"other": "1"}))

	// Provided value substitutes (and body-filter keys cover contract params).
	assert.Equal(t, "http://x/players/7", p.buildURL(contract, map[string]any{"id": "7"}))
	keys := consumedKeysFromMethod(contract)
	assert.Equal(t, []string{"id"}, keys)
	assert.NotContains(t, filterConsumedParamsByKeys(keys, map[string]any{"id": "7", "keep": 1}), "id")
}

// TestResolveContractParamValue asserts the contract parameter resolution:
// caller key = parameter name, empty string counts as absent, schema default
// fills an absent value.
func TestResolveContractParamValue(t *testing.T) {
	p := NewProvider()
	explode := false
	param := func(schema string) RequestParameter {
		return RequestParameter{In: "query", Name: "limit", Style: "form", Explode: &explode, Schema: json.RawMessage(schema)}
	}

	val, present := p.resolveContractParamValue(param(`{"type":"integer"}`), map[string]any{"limit": float64(20)})
	assert.True(t, present)
	assert.Equal(t, float64(20), val)

	val, present = p.resolveContractParamValue(param(`{"type":"integer"}`), map[string]any{"limit": ""})
	assert.False(t, present)
	assert.Nil(t, val)

	val, present = p.resolveContractParamValue(param(`{"type":"integer","default":10}`), map[string]any{})
	assert.True(t, present)
	assert.Equal(t, float64(10), val)

	_, present = p.resolveContractParamValue(param(`{"type":"integer"}`), nil)
	assert.False(t, present)
}
