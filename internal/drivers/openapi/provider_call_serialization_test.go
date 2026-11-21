package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
)

// specSerializationDoc 覆盖 SUI-006 的支持矩阵：path（simple/label/matrix）、
// query（form/deepObject/pipeDelimited/spaceDelimited）、header（simple）、
// cookie（form 原始值），以及一个不可执行的 cookie 数组组合。
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
    }
  }
}`

// wireCapture 记录一次请求的线上形态供断言。
type wireCapture struct {
	path        string
	escapedPath string
	rawQuery    string
	trace       string
	src         string
	cookie      string
}

// serializationTestServer 起一个捕获型测试服务器。
func serializationTestServer(cap *wireCapture) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.path = r.URL.Path
		cap.escapedPath = r.URL.EscapedPath()
		cap.rawQuery = r.URL.RawQuery
		cap.trace = r.Header.Get("X-Trace")
		cap.src = r.Header.Get("X-Src")
		cap.cookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
}

// serializationCall 发起一次调用并断言成功。
func serializationCall(t *testing.T, p *Provider, method string, payload map[string]any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := p.Call(context.Background(), method, raw); err != nil {
		t.Fatalf("Call(%s): %v", method, err)
	}
}

// TestProviderCall_ParameterSerializationWire 断言实际 HTTP 线上的 path 替换、
// query 编码、header 与 cookie 形态（SUI-006）。
func TestProviderCall_ParameterSerializationWire(t *testing.T) {
	var cap wireCapture
	server := serializationTestServer(&cap)
	defer server.Close()

	p := specCallProvider(t, specSerializationDoc)
	p.openapiConfig.BaseURL = server.URL
	p.httpClient = &http.Client{}

	// path simple：值原样替换，含斜杠时百分号编码。
	serializationCall(t, p, "user.get", map[string]any{"id": "123"})
	if cap.escapedPath != "/users/123" {
		t.Errorf("user.get path = %q, want %q", cap.escapedPath, "/users/123")
	}
	serializationCall(t, p, "user.get", map[string]any{"id": "a/b"})
	if cap.escapedPath != "/users/a%2Fb" {
		t.Errorf("user.get escaped path = %q, want %q", cap.escapedPath, "/users/a%2Fb")
	}

	// path matrix 与 label。
	serializationCall(t, p, "file.matrix", map[string]any{"id": 5})
	if cap.escapedPath != "/files/;id=5" {
		t.Errorf("file.matrix path = %q, want %q", cap.escapedPath, "/files/;id=5")
	}
	serializationCall(t, p, "ver.label", map[string]any{"ver": "1.2"})
	if cap.escapedPath != "/versions/.1.2" {
		t.Errorf("ver.label path = %q, want %q", cap.escapedPath, "/versions/.1.2")
	}

	// query 数组（form explode=true）、deepObject 对象、pipeDelimited、
	// spaceDelimited，以及 header 数组与 cookie 原始值。
	serializationCall(t, p, "search.run", map[string]any{
		"ids":     []any{1, 2},
		"meta":    map[string]any{"a": "1", "b": "2"},
		"tags":    []any{"x", "y"},
		"spaced":  []any{"a b", "c"},
		"X-Trace": []any{"p", "q"},
		"token":   "8",
	})
	if cap.rawQuery != "ids=1&ids=2&meta%5Ba%5D=1&meta%5Bb%5D=2&tags=x|y&spaced=a+b+c" {
		t.Errorf("search.run raw query = %q", cap.rawQuery)
	}
	if cap.trace != "p,q" {
		t.Errorf("X-Trace header = %q, want p,q", cap.trace)
	}
	if cap.cookie != "token=8" {
		t.Errorf("Cookie header = %q, want token=8", cap.cookie)
	}
	if cap.path != "/search" {
		t.Errorf("search.run path = %q, want /search", cap.path)
	}

	// 不可执行的组合（cookie 数组）返回错误，不静默错序列化。
	if _, err := p.Call(context.Background(), "strict.cookieArray", []byte(`{"bad":["a","b"]}`)); err == nil {
		t.Errorf("Call(strict.cookieArray) succeeded, want error")
	} else if !strings.Contains(err.Error(), "cookie") {
		t.Errorf("Call(strict.cookieArray) error = %q, want mention cookie", err.Error())
	}
}

// TestProviderCall_LegacyConfigSerialization 覆盖无契约的 config 声明方法：
// 沿用 OpenAPI 默认样式（query form explode=true），不回归既有行为。
func TestProviderCall_LegacyConfigSerialization(t *testing.T) {
	var cap wireCapture
	server := serializationTestServer(&cap)
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
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
						map[string]interface{}{"name": "X-Src", "in": "header"},
					},
				},
			},
		},
	})
	p.httpClient = &http.Client{}

	if _, err := p.Call(context.Background(), "list_all", []byte(`{"names":["a","b"],"X-Src":"ops"}`)); err != nil {
		t.Fatalf("Call(list_all): %v", err)
	}
	if cap.rawQuery != "names=a&names=b" {
		t.Errorf("legacy query = %q, want names=a&names=b", cap.rawQuery)
	}
	if cap.src != "ops" {
		t.Errorf("legacy header = %q, want ops", cap.src)
	}
}

// TestRequestContract_SerializationDiagnostics 断言不可执行的序列化组合在两个
// 提取前端都以同一诊断代码落库（SUI-006 诊断面）。
func TestRequestContract_SerializationDiagnostics(t *testing.T) {
	loaded, root := mustLoadKin(t, specSerializationDoc)
	rootMap := decodeMap(t, spec.JSONSchema(specSerializationDoc))
	strict := rootMap["paths"].(map[string]interface{})["/strict"].(map[string]interface{})["get"].(map[string]interface{})

	kinContract := ExtractOperationRequestContractKin(loaded.Paths.Find("/strict").Get, nil, root)
	mapContract := ExtractOperationRequestContractMap(strict, root, nil)

	for name, contract := range map[string]spec.OperationRequestContract{"kin": kinContract, "map": mapContract} {
		found := false
		for _, d := range contract.Diagnostics {
			if d.Code == spec.DiagParameterSerializationUnsupported {
				found = true
				if d.Field != "bad" {
					t.Errorf("%s diagnostic field = %q, want bad", name, d.Field)
				}
				if d.Severity != spec.SeverityError {
					t.Errorf("%s diagnostic severity = %q, want error", name, d.Severity)
				}
			}
		}
		if !found {
			t.Errorf("%s diagnostics = %+v, want %s", name, contract.Diagnostics, spec.DiagParameterSerializationUnsupported)
		}
	}
}
