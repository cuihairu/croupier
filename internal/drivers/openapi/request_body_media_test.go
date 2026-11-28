package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-007 验收：请求体媒体类型按执行器能力确定性选择；声明但无一可执行时
// 投影记录原因（永不静默丢弃）；执行器把选中的媒体类型原样作为 Content-Type
// 发送，存量不可执行契约以同样原因拒绝。

// swaggerMediaDoc 声明三种请求体媒体类型组合：可执行 application/json 与
// application/vnd.api+json（+json 后缀）、不可执行 form-urlencoded 与 xml。
const swaggerMediaDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "media", "version": "1.0.0"},
  "paths": {
    "/things": {
      "post": {
        "operationId": "thing.create",
        "requestBody": {"required": true, "content": {
          "application/json": {"schema": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}},
          "application/x-www-form-urlencoded": {"schema": {"type": "object", "properties": {"legacy": {"type": "string"}}}},
          "application/xml": {"schema": {"type": "string"}}
        }},
        "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}}}
      }
    },
    "/docs": {
      "post": {
        "operationId": "doc.create",
        "requestBody": {"content": {
          "application/vnd.api+json": {"schema": {"type": "object", "properties": {"sku": {"type": "string"}}}},
          "application/x-www-form-urlencoded": {"schema": {"type": "object"}}
        }},
        "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}}}
      }
    },
    "/forms": {
      "post": {
        "operationId": "form.create",
        "requestBody": {"content": {
          "application/x-www-form-urlencoded": {"schema": {"type": "object", "properties": {"nickname": {"type": "string"}}}},
          "application/xml": {"schema": {"type": "string"}}
        }},
        "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}}}
      }
    }
  }
}`

func TestIsJSONMediaType(t *testing.T) {
	assert.True(t, IsJSONMediaType("application/json"))
	assert.True(t, IsJSONMediaType("application/vnd.api+json"))
	assert.True(t, IsJSONMediaType("Application/JSON; charset=utf-8"))
	assert.False(t, IsJSONMediaType("application/x-www-form-urlencoded"))
	assert.False(t, IsJSONMediaType("application/xml"))
	assert.False(t, IsJSONMediaType("*/*"))
	assert.False(t, IsJSONMediaType(""))
}

func TestIsExecutableRequestBodyMediaType(t *testing.T) {
	// 执行器只序列化 JSON 载荷，能力集 = application/json 与 +json 后缀。
	assert.True(t, IsExecutableRequestBodyMediaType("application/json"))
	assert.True(t, IsExecutableRequestBodyMediaType("application/hal+json"))
	assert.False(t, IsExecutableRequestBodyMediaType("multipart/form-data"))
	assert.False(t, IsExecutableRequestBodyMediaType("text/plain"))
}

func TestSelectRequestBodyMediaType_PreferenceAndTieBreak(t *testing.T) {
	// application/json 优先于 +json 后缀。
	media, reason := SelectRequestBodyMediaType(map[string]RequestMedia{
		"application/json":         {},
		"application/vnd.api+json": {},
	})
	assert.Equal(t, "application/json", media)
	assert.Empty(t, reason)

	// 无 application/json 时 +json 后缀可执行，多条目按字典序决胜。
	media, reason = SelectRequestBodyMediaType(map[string]RequestMedia{
		"application/hal+json":     {},
		"application/vnd.api+json": {},
	})
	assert.Equal(t, "application/hal+json", media)
	assert.Empty(t, reason)

	// 参数与大小写不改变可执行性。
	media, reason = SelectRequestBodyMediaType(map[string]RequestMedia{
		"application/json; charset=utf-8": {},
	})
	assert.Equal(t, "application/json; charset=utf-8", media)
	assert.Empty(t, reason)

	// 全部不可执行：返回原因，指明声明集合与可执行集合。
	media, reason = SelectRequestBodyMediaType(map[string]RequestMedia{
		"application/xml": {},
		"text/plain":      {},
	})
	assert.Empty(t, media)
	assert.Contains(t, reason, "application/xml")
	assert.Contains(t, reason, executableMediaTypeSet)

	// 无任何声明：同样返回原因（调用方按各自语义决定是否记录）。
	media, reason = SelectRequestBodyMediaType(map[string]RequestMedia{})
	assert.Empty(t, media)
	assert.NotEmpty(t, reason)
}

func TestSelectRequestBodyMediaType_Deterministic(t *testing.T) {
	content := map[string]RequestMedia{
		"application/xml":                   {},
		"application/vnd.api+json":          {},
		"application/x-www-form-urlencoded": {},
		"application/merge-patch+json":      {},
	}
	want, _ := SelectRequestBodyMediaType(content)
	require.Equal(t, "application/merge-patch+json", want)
	for i := 0; i < 20; i++ {
		got, reason := SelectRequestBodyMediaType(content)
		assert.Equal(t, want, got, "selection must not drift")
		assert.Empty(t, reason)
	}
}

func TestSelectExecutableMediaType_ConsumesOrder(t *testing.T) {
	// Swagger 2.0 consumes：声明顺序有意义，第一个可执行条目胜出。
	media, reason := SelectExecutableMediaType([]string{"application/xml", "application/json"})
	assert.Equal(t, "application/json", media)
	assert.Empty(t, reason)

	// 空声明：无指导，调用方自行回退默认。
	media, reason = SelectExecutableMediaType(nil)
	assert.Empty(t, media)
	assert.Empty(t, reason)

	// 全部不可执行：返回原因。
	media, reason = SelectExecutableMediaType([]string{"application/xml"})
	assert.Empty(t, media)
	assert.Contains(t, reason, "application/xml")
}

func TestRequestBodyMediaDiagnostic(t *testing.T) {
	diag := RequestBodyMediaDiagnostic("no executable request body media type")
	assert.Equal(t, DiagRequestBodyNoExecutableMedia, diag.Code)
	assert.Equal(t, spec.SeverityWarning, diag.Severity)
	assert.Equal(t, "body", diag.Field)
}

// mediaContract 构造仅含请求体的契约（kin 提取产物的最小形态）。
func mediaContract(content map[string]RequestMedia) *RequestContract {
	return &RequestContract{
		Body: &RequestBody{
			Required: true,
			Content:  content,
			Origin:   RequestOrigin{Pointer: "#/paths/~1things/post/requestBody"},
		},
	}
}

func TestProjectInput_ExecutableMediaTypeSelected(t *testing.T) {
	projection := ProjectInput("thing.create", mediaContract(map[string]RequestMedia{
		"application/json":                  {Schema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)},
		"application/x-www-form-urlencoded": {Schema: json.RawMessage(`{"type":"object"}`)},
		"application/xml":                   {Schema: json.RawMessage(`{"type":"string"}`)},
	}), map[string]any{})
	assert.False(t, projection.Grouped)
	assert.Empty(t, projection.Diagnostics)
	assert.Contains(t, projection.Schema, `"name"`)
	assert.NotContains(t, projection.Schema, "legacy", "form entry must not leak into the projection")
}

func TestProjectInput_SuffixMediaTypeSelected(t *testing.T) {
	// 无 application/json：+json 后缀条目可执行并胜出。
	projection := ProjectInput("doc.create", mediaContract(map[string]RequestMedia{
		"application/vnd.api+json":          {Schema: json.RawMessage(`{"type":"object","properties":{"sku":{"type":"string"}}}`)},
		"application/x-www-form-urlencoded": {Schema: json.RawMessage(`{"type":"object"}`)},
	}), map[string]any{})
	assert.False(t, projection.Grouped)
	assert.Empty(t, projection.Diagnostics)
	assert.Contains(t, projection.Schema, `"sku"`)
}

func TestProjectInput_NoExecutableMediaTypeRecordsReason(t *testing.T) {
	projection := ProjectInput("form.create", mediaContract(map[string]RequestMedia{
		"application/x-www-form-urlencoded": {Schema: json.RawMessage(`{"type":"object"}`)},
		"application/xml":                   {Schema: json.RawMessage(`{"type":"string"}`)},
	}), map[string]any{})
	var diag *spec.Diagnostic
	for _, item := range projection.Diagnostics {
		if item.Code == DiagRequestBodyNoExecutableMedia {
			copy := item
			diag = &copy
			break
		}
	}
	require.NotNil(t, diag, "non-executable-only body must record the reason")
	assert.Equal(t, "body", diag.Field)
	assert.Equal(t, spec.SeverityWarning, diag.Severity)
	assert.Contains(t, diag.Message, "application/x-www-form-urlencoded")
	assert.Contains(t, diag.Message, "executable")
	assert.True(t, projection.Grouped, "diagnostics force the grouped input")
}

// swagger20Doc 构造最小 Swagger 2.0 文档（kin 拒载，走 map 回退前端）。
func extractSwagger20Input(t *testing.T, doc string) (string, []spec.Diagnostic) {
	t.Helper()
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	paths, _ := root["paths"].(map[string]interface{})
	pathItem, _ := paths["/x"].(map[string]interface{})
	methodObj, _ := pathItem["post"].(map[string]interface{})
	return extractInputSchemaWithIssues(methodObj, root)
}

func TestExtractInputSchema_Swagger20ConsumesCapability(t *testing.T) {
	t.Run("non-executable consumes records reason", func(t *testing.T) {
		schema, diagnostics := extractSwagger20Input(t,
			`{"swagger":"2.0","consumes":["application/xml"],"paths":{"/x":{"post":{"operationId":"x.create","parameters":[{"name":"payload","in":"body","required":true,"schema":{"type":"object","properties":{"a":{"type":"string"}}}}]}}}}`)
		require.Len(t, diagnostics, 1)
		assert.Equal(t, DiagRequestBodyNoExecutableMedia, diagnostics[0].Code)
		assert.Contains(t, diagnostics[0].Message, "application/xml")
		// body 槽不落地：in:body 参数不得伪装成扁平字段或 body 属性。
		assert.NotContains(t, schema, `"a"`)
		assert.NotContains(t, schema, `"body"`)
	})

	t.Run("first executable consumes wins", func(t *testing.T) {
		schema, diagnostics := extractSwagger20Input(t,
			`{"swagger":"2.0","consumes":["application/xml","application/json"],"paths":{"/x":{"post":{"operationId":"x.create","parameters":[{"name":"payload","in":"body","schema":{"type":"object","properties":{"b":{"type":"integer"}}}}]}}}}`)
		assert.Empty(t, diagnostics)
		assert.Contains(t, schema, `"b"`, "object body flattens into the flat properties")
	})

	t.Run("body parameter never joins flat fields", func(t *testing.T) {
		schema, diagnostics := extractSwagger20Input(t,
			`{"swagger":"2.0","paths":{"/x":{"post":{"operationId":"x.create","parameters":[{"name":"game_id","in":"query","type":"string"},{"name":"payload","in":"body","schema":{"type":"object","properties":{"b":{"type":"integer"}}}}]}}}}`)
		assert.Empty(t, diagnostics)
		assert.Contains(t, schema, `"b"`)
		assert.NotContains(t, schema, `"payload"`, "the in:body slot is the body, not a flat field")
	})
}

func TestProviderCall_BodyMediaTypeSentAsContentType(t *testing.T) {
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	p := NewProvider()
	p.config = provider.ProviderConfig{Enabled: true}
	p.openapiConfig = &Config{BaseURL: server.URL}
	p.httpClient = &http.Client{}
	require.NoError(t, p.parseOpenAPISpec([]byte(swaggerMediaDoc)))

	// +json 后缀条目选中的媒体类型原样发送，不改写为 application/json。
	_, err := p.Call(context.Background(), "doc.create", []byte(`{"sku":"sku-1"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.api+json", gotContentType)

	_, err = p.Call(context.Background(), "thing.create", []byte(`{"name":"n"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", gotContentType)
}

func TestProviderCall_StaleNonExecutableBodyMediaTypeRefused(t *testing.T) {
	// 存量契约记录不可执行媒体类型（旧选择器产物）：执行器以同样原因拒绝，
	// 绝不把 JSON 载荷错标 Content-Type 送出去。
	p := NewProvider()
	p.config = provider.ProviderConfig{Enabled: true}
	p.openapiConfig = &Config{BaseURL: "http://127.0.0.1:1"}
	p.methodMap["stale.create"] = &APIMethod{
		Name:   "stale.create",
		Path:   "/forms",
		Method: "POST",
		Request: &RequestContract{
			Body: &RequestBody{
				Content: map[string]RequestMedia{
					"application/x-www-form-urlencoded": {},
				},
			},
		},
	}
	_, err := p.Call(context.Background(), "stale.create", []byte(`{"nickname":"n"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no executable request body media type")
}
