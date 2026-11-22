package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-007 验收：请求体媒体类型按执行器能力确定性选择；同一份多媒体类型文档
// 重复导入结果一致；声明但无一可执行时提案记录原因（kin/map 两个前端同码）。

// specMediaDoc 声明三种请求体媒体类型：可执行 application/json 与
// application/vnd.api+json（+json 后缀）、不可执行 form-urlencoded 与 xml。
const specMediaDoc = `{
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

// mediaContracts 用 kin 与 map 前端各提取一次，返回契约集（键 = path.method 与
// path.map.method，便于对同操作的前端产物逐对断言）。
func mediaContracts(t *testing.T, doc string) map[string]spec.OperationRequestContract {
	t.Helper()
	loaded, root := mustLoadKin(t, doc)
	rootMap := decodeMap(t, spec.JSONSchema(doc))
	pathsRaw := rootMap["paths"].(map[string]interface{})
	out := map[string]spec.OperationRequestContract{}
	for path, pathItemRaw := range pathsRaw {
		pathObj := pathItemRaw.(map[string]interface{})
		pathItemParams, _ := pathObj["parameters"].([]interface{})
		item, ok := loaded.Paths.Map()[path]
		require.True(t, ok, "kin must load path %q", path)
		for methodRaw, methodItem := range pathObj {
			if _, isMethod := methodItem.(map[string]interface{}); !isMethod {
				continue
			}
			op := item.Operations()[strings.ToUpper(methodRaw)]
			require.NotNil(t, op, "kin missing operation %s.%s", methodRaw, path)
			out[path+"."+methodRaw] = ExtractOperationRequestContractKin(op, item.Parameters, root)
			out[path+".map."+methodRaw] = ExtractOperationRequestContractMap(methodItem.(map[string]interface{}), root, pathItemParams)
		}
	}
	return out
}

func TestRequestContract_ExecutableMediaTypeSelected(t *testing.T) {
	contracts := mediaContracts(t, specMediaDoc)
	for _, name := range []string{"/things.post", "/things.map.post"} {
		contract, ok := contracts[name]
		require.True(t, ok, "contract missing for %s", name)
		require.NotNil(t, contract.Body, "%s: json entry must yield a body slot", name)
		assert.Equal(t, "application/json", contract.Body.MediaType, "%s: executable json wins over form/xml", name)
		assert.Nil(t, diagnosticByCode(contract.Diagnostics, spec.DiagRequestBodyNoExecutableMedia), "%s: no spurious media diagnostic", name)
	}
}

func TestRequestContract_ExecutableMediaTypeSuffix(t *testing.T) {
	contracts := mediaContracts(t, specMediaDoc)
	for _, name := range []string{"/docs.post", "/docs.map.post"} {
		contract, ok := contracts[name]
		require.True(t, ok, "contract missing for %s", name)
		require.NotNil(t, contract.Body, "%s: +json entry must yield a body slot", name)
		assert.Equal(t, "application/vnd.api+json", contract.Body.MediaType, "%s: +json suffix is executable", name)
		assert.Nil(t, diagnosticByCode(contract.Diagnostics, spec.DiagRequestBodyNoExecutableMedia))
	}
}

func TestRequestContract_NoExecutableMediaTypeRecordsReason(t *testing.T) {
	contracts := mediaContracts(t, specMediaDoc)
	for _, name := range []string{"/forms.post", "/forms.map.post"} {
		contract, ok := contracts[name]
		require.True(t, ok, "contract missing for %s", name)
		assert.Nil(t, contract.Body, "%s: non-executable body must not become an executable slot", name)
		diag := diagnosticByCode(contract.Diagnostics, spec.DiagRequestBodyNoExecutableMedia)
		require.NotNil(t, diag, "%s: no executable media type must be recorded", name)
		assert.Equal(t, "body", diag.Field)
		assert.Equal(t, spec.SeverityWarning, diag.Severity)
		assert.Contains(t, diag.Message, "application/x-www-form-urlencoded")
		assert.Contains(t, diag.Message, "executable")
	}
}

func TestRequestContract_RepeatedImportDeterministic(t *testing.T) {
	// 同一份多媒体类型文档重复导入：契约逐方法 JSONEq（map 迭代与顺序不影响
	// 选择结果——重复导入结果一致）。
	first := contractsByMethod(t, specMediaDoc)
	require.NotEmpty(t, first)
	for i := 0; i < 20; i++ {
		again := contractsByMethod(t, specMediaDoc)
		require.Len(t, again, len(first))
		for name, want := range first {
			got, ok := again[name]
			require.True(t, ok, "import %d lost method %s", i, name)
			assert.JSONEq(t, want, got, "import %d drifted for %s", i, name)
		}
	}
}

func TestRequestContract_Swagger20ConsumesCapability(t *testing.T) {
	t.Run("non-executable consumes records reason", func(t *testing.T) {
		doc := `{"swagger":"2.0","consumes":["application/xml"],"paths":{"/x":{"post":{"operationId":"x.create","parameters":[{"name":"payload","in":"body","required":true,"schema":{"type":"object","properties":{"a":{"type":"string"}}}}]}}}}`
		contract := ExtractOperationRequestContractMap(methodObjOf(t, doc, "/x", "post"), mustLoadRoot(t, doc), nil)
		assert.Nil(t, contract.Body, "xml-only consumes must not become an executable body slot")
		diag := diagnosticByCode(contract.Diagnostics, spec.DiagRequestBodyNoExecutableMedia)
		require.NotNil(t, diag)
		assert.Contains(t, diag.Message, "application/xml")
	})

	t.Run("first executable consumes wins", func(t *testing.T) {
		doc := `{"swagger":"2.0","consumes":["application/xml","application/json"],"paths":{"/y":{"post":{"operationId":"y.create","parameters":[{"name":"payload","in":"body","schema":{"type":"object","properties":{"b":{"type":"integer"}}}}]}}}}`
		contract := ExtractOperationRequestContractMap(methodObjOf(t, doc, "/y", "post"), mustLoadRoot(t, doc), nil)
		require.NotNil(t, contract.Body)
		assert.Equal(t, "application/json", contract.Body.MediaType)
		assert.Nil(t, diagnosticByCode(contract.Diagnostics, spec.DiagRequestBodyNoExecutableMedia))
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

	p := specCallProvider(t, specMediaDoc)
	p.openapiConfig.BaseURL = server.URL
	p.httpClient = &http.Client{}

	_, err := p.Call(context.Background(), "doc.create", []byte(`{"sku":"sku-1"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.api+json", gotContentType, "the selected executable media type is sent verbatim")

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
	p.methodMap = make(map[string]*APIMethod)
	p.methodMap["stale.create"] = &APIMethod{
		Name:   "stale.create",
		Path:   "/forms",
		Method: "POST",
		RequestContract: &spec.OperationRequestContract{
			Body: &spec.RequestBodySpec{
				MediaType: "application/x-www-form-urlencoded",
				Schema:    spec.JSONSchema(`{"type":"object"}`),
			},
		},
	}
	_, err := p.Call(context.Background(), "stale.create", []byte(`{"nickname":"n"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not executable")
}

// contractsByMethod 用运行时发现路径（同生产 parseOpenAPISpec）导出每方法契约
// 的 JSON 文本，供重复导入确定性断言。
func contractsByMethod(t *testing.T, doc string) map[string]string {
	t.Helper()
	p := NewProvider()
	p.config = provider.ProviderConfig{Enabled: true}
	p.openapiConfig = &Config{}
	p.methodMap = make(map[string]*APIMethod)
	require.NoError(t, p.parseOpenAPISpec([]byte(doc)))
	out := map[string]string{}
	for name, m := range p.methodMap {
		if m.RequestContract == nil {
			continue
		}
		encoded, err := json.Marshal(m.RequestContract)
		require.NoError(t, err)
		out[name] = string(encoded)
	}
	return out
}

func keysOf(m map[string]spec.OperationRequestContract) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
