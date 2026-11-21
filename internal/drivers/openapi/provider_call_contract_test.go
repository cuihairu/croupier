package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

// specContractCallDoc 覆盖 SUI-005 的四种投影形态：根数组 + query（structured）、
// 标量根 + query（structured）、根数组无参数（body_only）、对象根 + query（flat）。
const specContractCallDoc = `{
  "openapi": "3.0.3",
  "paths": {
    "/batch": {
      "post": {
        "operationId": "batch.send",
        "parameters": [
          {"name": "page", "in": "query", "schema": {"type": "integer"}}
        ],
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "array", "items": {"type": "string"}}}}
        }
      }
    },
    "/ping": {
      "post": {
        "operationId": "ping.say",
        "parameters": [
          {"name": "tag", "in": "query", "schema": {"type": "string"}}
        ],
        "requestBody": {
          "content": {"application/json": {"schema": {"type": "string"}}}
        }
      }
    },
    "/bulk": {
      "post": {
        "operationId": "bulk.run",
        "requestBody": {
          "content": {"application/json": {"schema": {"type": "array", "items": {"type": "integer"}}}}
        }
      }
    },
    "/obj": {
      "post": {
        "operationId": "obj.set",
        "parameters": [
          {"name": "id", "in": "query", "schema": {"type": "integer"}}
        ],
        "requestBody": {
          "content": {"application/json": {"schema": {"type": "object", "properties": {"name": {"type": "string"}}}}}
        }
      }
    }
  }
}`

// specCallProvider 从 spec 建 provider（与运行时发现同路径），供执行器用例复用。
func specCallProvider(t *testing.T, doc string) *Provider {
	t.Helper()
	p := NewProvider()
	p.config = provider.ProviderConfig{Enabled: true}
	p.openapiConfig = &Config{}
	p.methodMap = make(map[string]*APIMethod)
	if err := p.parseOpenAPISpec([]byte(doc)); err != nil {
		t.Fatalf("parseOpenAPISpec: %v", err)
	}
	return p
}

// SUI-005 验收：根数组与标量根 body 在存在 query 参数时仍经请求映射送达上游，
// 并断言实际 HTTP body 的 JSON 类型与值。
func TestProviderCall_ContractRootShapePreserved(t *testing.T) {
	var gotPath, gotQuery, gotRaw, gotContentType string
	var gotValue interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotPath, gotQuery, gotRaw = r.URL.Path, r.URL.RawQuery, string(raw)
		gotContentType = r.Header.Get("Content-Type")
		_ = json.Unmarshal(raw, &gotValue)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	p := specCallProvider(t, specContractCallDoc)
	p.openapiConfig.BaseURL = server.URL
	p.httpClient = &http.Client{}

	// 根数组 + query：命名空间入参拆分后 body 原样发送（JSON 数组）。
	pay, _ := json.Marshal(map[string]interface{}{
		"params": map[string]interface{}{"page": float64(2)},
		"body":   []interface{}{"a", "b"},
	})
	if _, err := p.Call(context.Background(), "batch.send", pay); err != nil {
		t.Fatalf("Call(batch.send): %v", err)
	}
	if gotQuery != "page=2" {
		t.Errorf("batch.send query = %q, want %q", gotQuery, "page=2")
	}
	if gotRaw != `["a","b"]` {
		t.Errorf("batch.send body = %s, want array JSON", gotRaw)
	}
	if _, isArr := gotValue.([]interface{}); !isArr {
		t.Errorf("batch.send body JSON type = %T, want []interface{}", gotValue)
	}
	if gotContentType != "application/json" {
		t.Errorf("batch.send content-type = %q, want application/json", gotContentType)
	}

	// 标量根 + query：body 为 JSON 字符串。
	pay, _ = json.Marshal(map[string]interface{}{
		"params": map[string]interface{}{"tag": "x"},
		"body":   "hello",
	})
	if _, err := p.Call(context.Background(), "ping.say", pay); err != nil {
		t.Fatalf("Call(ping.say): %v", err)
	}
	if gotQuery != "tag=x" {
		t.Errorf("ping.say query = %q, want %q", gotQuery, "tag=x")
	}
	if gotRaw != `"hello"` {
		t.Errorf("ping.say body = %s, want scalar JSON string", gotRaw)
	}
	if _, isStr := gotValue.(string); !isStr {
		t.Errorf("ping.say body JSON type = %T, want string", gotValue)
	}

	// 根数组无参数（body_only）：payload 本身就是 body。
	if _, err := p.Call(context.Background(), "bulk.run", []byte(`[1,2]`)); err != nil {
		t.Fatalf("Call(bulk.run): %v", err)
	}
	if gotRaw != `[1,2]` {
		t.Errorf("bulk.run body = %s, want [1,2]", gotRaw)
	}
	if gotQuery != "" {
		t.Errorf("bulk.run query = %q, want empty", gotQuery)
	}

	// 对象根 + query（flat 投影）：寻址参数不泄漏进 body（既有语义不变）。
	if _, err := p.Call(context.Background(), "obj.set", []byte(`{"id":1,"name":"n"}`)); err != nil {
		t.Fatalf("Call(obj.set): %v", err)
	}
	if gotQuery != "id=1" {
		t.Errorf("obj.set query = %q, want %q", gotQuery, "id=1")
	}
	if gotRaw != `{"name":"n"}` {
		t.Errorf("obj.set body = %s, want {\"name\":\"n\"}", gotRaw)
	}
	if gotPath != "/obj" {
		t.Errorf("obj.set path = %q, want /obj", gotPath)
	}
}
