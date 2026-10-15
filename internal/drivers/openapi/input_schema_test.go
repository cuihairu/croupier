package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

// mustParseSpec 走真实 parseOpenAPISpec 链，返回 methodMap 副本断言用。
func mustParseSpec(t *testing.T, doc string) map[string]*APIMethod {
	t.Helper()
	p := NewProvider()
	if err := p.parseOpenAPISpec([]byte(doc)); err != nil {
		t.Fatalf("parseOpenAPISpec() error = %v", err)
	}
	return p.methodMap
}

func TestExtractInputSchema_ParametersAndInlineBody(t *testing.T) {
	doc := `{
	  "openapi": "3.0.3",
	  "paths": {"/players/{id}": {"put": {
	    "operationId": "player.update",
	    "parameters": [
	      {"name": "id", "in": "path", "required": true,
	       "schema": {"type": "string"}, "description": "player id"},
	      {"name": "dry_run", "in": "query", "schema": {"type": "boolean"}}
	    ],
	    "requestBody": {"required": true, "content": {"application/json": {"schema": {"type": "object",
	      "properties": {"name": {"type": "string"}, "level": {"type": "integer"}},
	      "required": ["name"]}}}}
	  }}}
	}`
	methods := mustParseSpec(t, doc)

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(methods["player.update"].InputSchema), &schema); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if len(props) != 4 {
		t.Fatalf("expected 4 properties (id/dry_run/name/level merged), got %v", props)
	}
	for _, key := range []string{"id", "dry_run", "name", "level"} {
		if _, ok := props[key]; !ok {
			t.Errorf("missing property %q", key)
		}
	}
	required, _ := schema["required"].([]interface{})
	if len(required) != 2 {
		t.Fatalf("expected required [id name], got %v", required)
	}
	if id, _ := props["id"].(map[string]interface{}); id["description"] != "player id" {
		t.Errorf("parameter description should ride along, got %v", id)
	}
}

func TestExtractInputSchema_RefResolution(t *testing.T) {
	doc := `{
	  "openapi": "3.0.3",
	  "components": {
	    "schemas": {"PlayerInput": {
	      "type": "object",
	      "properties": {"name": {"type": "string"}},
	      "required": ["name"]
	    }}
	  },
	  "paths": {"/players": {"post": {
	    "operationId": "player.create",
	    "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/PlayerInput"}}}}
	  }}}}
	`
	methods := mustParseSpec(t, doc)

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(methods["player.create"].InputSchema), &schema); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if _, ok := props["name"]; !ok {
		t.Fatalf("$ref body properties should merge into top level, got %v", schema)
	}
	required, _ := schema["required"].([]interface{})
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("body required should merge, got %v", required)
	}
}

func TestExtractInputSchema_CyclicRefTerminates(t *testing.T) {
	doc := `{
	  "openapi": "3.0.3",
	  "components": {"schemas": {"Node": {
	    "type": "object",
	    "properties": {"child": {"$ref": "#/components/schemas/Node"}}
	  }}},
	  "paths": {"/nodes": {"post": {
	    "operationId": "node.create",
	    "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Node"}}}}
	  }}}}
	`
	methods := mustParseSpec(t, doc)
	if methods["node.create"].InputSchema == "" {
		t.Fatal("cyclic $ref must still produce a schema")
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(methods["node.create"].InputSchema), &schema); err != nil {
		t.Fatalf("cyclic $ref must produce valid JSON: %v", err)
	}
}

func TestExtractInputSchema_NonObjectBodyKeptAsProperty(t *testing.T) {
	doc := `{
	  "openapi": "3.0.3",
	  "paths": {"/batch": {"post": {
	    "operationId": "batch.import",
	    "requestBody": {"content": {"application/json": {"schema": {
	      "type": "array", "items": {"type": "string"}
	    }}}}
	  }}}}
	`
	methods := mustParseSpec(t, doc)

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(methods["batch.import"].InputSchema), &schema); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	body, ok := props["body"].(map[string]interface{})
	if !ok || body["type"] != "array" {
		t.Fatalf("non-object body should be kept under properties.body, got %v", schema)
	}
}

func TestExtractInputSchema_NoInputIsEmptyObject(t *testing.T) {
	doc := `{
	  "openapi": "3.0.3",
	  "paths": {"/players": {"get": {"operationId": "player.list"}}}
	}`
	methods := mustParseSpec(t, doc)

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(methods["player.list"].InputSchema), &schema); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("root type should be object, got %v", schema)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if len(props) != 0 {
		t.Errorf("expected empty properties, got %v", props)
	}
}

func TestExtractInputSchema_DanglingAndExternalRefs(t *testing.T) {
	doc := `{
	  "openapi": "3.0.3",
	  "paths": {"/x": {"post": {
	    "operationId": "x.create",
	    "parameters": [
	      {"name": "a", "in": "query", "schema": {"$ref": "#/components/schemas/Missing"}},
	      {"name": "b", "in": "query", "schema": {"$ref": "https://example.com/other.json#/schemas/B"}}
	    ]
	  }}}}
	`
	methods := mustParseSpec(t, doc)

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(methods["x.create"].InputSchema), &schema); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if len(props) != 2 {
		t.Fatalf("both params must survive, got %v", props)
	}
	if a, _ := props["a"].(map[string]interface{}); a["type"] != "object" && a["type"] != "string" {
		// 悬空引用退化为 object；外层参数兜底只在完全没有 schema 时给 string
		t.Errorf("dangling ref should collapse to empty/object schema, got %v", a)
	}
	if b, _ := props["b"].(map[string]interface{}); b["$ref"] == "" {
		t.Errorf("external ref must be preserved, got %v", b)
	}
}

// 端到端：Init（拉取 spec）→ GetMethodDetails 透传 InputSchema。
func TestProviderInit_DerivesInputSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "openapi": "3.0.3",
		  "info": {"version": "1.0.0"},
		  "paths": {"/players/{id}": {"delete": {
		    "operationId": "player.delete",
		    "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}]
		  }}}
		}`))
	}))
	defer server.Close()

	p := NewProvider()
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config:  map[string]interface{}{"openapiSpec": server.URL},
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	details := p.GetMethodDetails()
	d, ok := details["player.delete"]
	if !ok {
		t.Fatal("player.delete not discovered")
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(d.InputSchema), &schema); err != nil {
		t.Fatalf("MethodDetails.InputSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if _, ok := props["id"]; !ok {
		t.Errorf("path parameter should be in input schema, got %v", schema)
	}
}
