package openapi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/cuihairu/croupier/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-009 验收（驱动层）：3.0 `nullable` 与 3.1 `type: [T, "null"]` 经契约归一
// 后折叠为同一 `type: [T, "null"]`（"null" 恒在末位），条件 schema / 元组 /
// 多类型联合记录能力诊断而不静默。前端（web/src/utils/schemaDialect.ts）对
// 同一批样例的判定与 spec.ClassifySchemaSupport 一致。

const dialect30Doc = `{
  "openapi": "3.0.3",
  "info": {"title": "dialect", "version": "1.0.0"},
  "paths": {
    "/items": {"post": {"operationId": "item.create",
      "parameters": [
        {"name": "note", "in": "query", "schema": {"type": "string", "nullable": true}}
      ],
      "requestBody": {"required": true, "content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "label": {"type": "string", "nullable": true},
          "count": {"type": "integer", "nullable": true},
          "tags": {"type": "array", "items": {"type": "string", "nullable": true}, "nullable": true}
        },
        "required": ["label"]
      }}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
}`

const dialect31Doc = `{
  "openapi": "3.1.0",
  "info": {"title": "dialect", "version": "1.0.0"},
  "paths": {
    "/items": {"post": {"operationId": "item.create",
      "parameters": [
        {"name": "note", "in": "query", "schema": {"type": ["string", "null"]}}
      ],
      "requestBody": {"required": true, "content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "label": {"type": ["string", "null"]},
          "count": {"type": ["null", "integer"]},
          "tags": {"type": ["array", "null"], "items": {"type": ["string", "null"]}}
        },
        "required": ["label"]
      }}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
}`

// dialectDegradedDoc：条件 schema + 3.1 元组（prefixItems）+ 多类型联合。
const dialectDegradedDoc = `{
  "openapi": "3.1.0",
  "info": {"title": "dialect", "version": "1.0.0"},
  "paths": {
    "/items": {"post": {"operationId": "item.create",
      "requestBody": {"content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "mode": {"type": ["string", "integer"]},
          "pair": {"type": "array", "prefixItems": [{"type": "string"}, {"type": "integer"}]},
          "choice": {"type": "object", "properties": {"x": {"type": "string"}},
            "if": {"properties": {"x": {"const": "a"}}}, "then": {"required": ["x"]}}
        }
      }}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
}`

// dialectTupleDoc：draft-07 元组（items 为数组）——kin 拒载，生产走 map 回落。
const dialectTupleDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "dialect", "version": "1.0.0"},
  "paths": {
    "/items": {"post": {"operationId": "item.create",
      "requestBody": {"content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "pair": {"type": "array", "items": [{"type": "string"}, {"type": "integer"}]},
          "note": {"type": "string", "nullable": true}
        }
      }}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
}`

// dialectOutputDoc：3.0 响应可空字段——输出 schema 须与输入同机归一。
const dialectOutputDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "dialect", "version": "1.0.0"},
  "paths": {
    "/items": {"get": {"operationId": "item.read",
      "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "label": {"type": "string", "nullable": true},
          "tags": {"type": "array", "items": {"type": "string", "nullable": true}}
        },
        "required": ["label"]
      }}}}}
    }}
  }
}`

func dialectRoot(t *testing.T, doc string) map[string]any {
	t.Helper()
	var root map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	return root
}

func dialectMethodObj(t *testing.T, doc string, method string) map[string]any {
	t.Helper()
	root := dialectRoot(t, doc)
	paths := root["paths"].(map[string]any)
	pathItem := paths["/items"].(map[string]any)
	return pathItem[method].(map[string]any)
}

func dialectProjection(t *testing.T, doc string) InputProjection {
	t.Helper()
	root := dialectRoot(t, doc)
	requests := kinRequestContracts([]byte(doc))
	require.NotNil(t, requests, "the kin front-end must accept this document")
	request, ok := requests[requestOperation{path: "/items", method: "POST"}]
	require.True(t, ok, "the operation must carry an extracted contract")
	return ProjectInput("item.create", request, root)
}

func dialectProjectedProps(t *testing.T, projection InputProjection) map[string]any {
	t.Helper()
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(projection.Schema), &schema))
	return schema["properties"].(map[string]any)
}

func TestDialect30_NullableFolded(t *testing.T) {
	projection := dialectProjection(t, dialect30Doc)
	assert.Empty(t, projection.Diagnostics, "nullable single types are form-ready")
	props := dialectProjectedProps(t, projection)
	assert.Equal(t, `{"type":["string","null"]}`, mustJSON(t, props["note"]))
	assert.Equal(t, `{"type":["string","null"]}`, mustJSON(t, props["label"]))
	assert.Equal(t, `{"type":["integer","null"]}`, mustJSON(t, props["count"]))
	assert.Equal(t, `{"items":{"type":["string","null"]},"type":["array","null"]}`, mustJSON(t, props["tags"]))
	assert.False(t, projection.Grouped)
}

func TestDialect31_UnionNormalized(t *testing.T) {
	projection := dialectProjection(t, dialect31Doc)
	assert.Empty(t, projection.Diagnostics)
	props := dialectProjectedProps(t, projection)
	assert.Equal(t, `{"type":["string","null"]}`, mustJSON(t, props["note"]))
	// "null" 移到末位
	assert.Equal(t, `{"type":["integer","null"]}`, mustJSON(t, props["count"]))
	assert.Equal(t, `{"items":{"type":["string","null"]},"type":["array","null"]}`, mustJSON(t, props["tags"]))
}

func TestDialect30And31_SameCanonicalForm(t *testing.T) {
	// 3.0 nullable 与 3.1 联合类型描述同一语义 → 归一后逐字节同形。
	assert.Equal(t, dialectProjection(t, dialect30Doc).Schema, dialectProjection(t, dialect31Doc).Schema)
}

func TestDialect_ProductionByteStable(t *testing.T) {
	// 同一文档重复投影须产出字节稳定的 schema（重导入幂等）。
	first := dialectProjection(t, dialect31Doc).Schema
	for i := 0; i < 5; i++ {
		assert.Equal(t, first, dialectProjection(t, dialect31Doc).Schema)
	}
}

func TestDialect_DegradedKeywordsDiagnosed(t *testing.T) {
	projection := dialectProjection(t, dialectDegradedDoc)
	require.NotEmpty(t, projection.Diagnostics)
	paths := map[string]bool{}
	for _, diagnostic := range projection.Diagnostics {
		assert.Equal(t, spec.DiagSchemaDialectKeyword, diagnostic.Code, diagnostic.Field)
		assert.Equal(t, spec.SeverityWarning, diagnostic.Severity)
		assert.Equal(t, "item.create", diagnostic.FunctionID)
		paths[diagnostic.Field] = true
	}
	assert.True(t, paths["/properties/mode"], "multi-type union must be noted")
	assert.True(t, paths["/properties/pair"], "3.1 tuple must be noted")
	assert.True(t, paths["/properties/choice"], "conditional schema must be noted")
	// 能力诊断不丢槽位：schema 仍可用
	require.NotEmpty(t, projection.Schema)
}

// TestDialect_Draft07TupleUsesFallbackPath：draft-07 元组 kin 拒载，生产走 map
// 回落；回落侧同样归一方言并记录元组能力诊断（不静默）。
func TestDialect_Draft07TupleUsesFallbackPath(t *testing.T) {
	require.Nil(t, kinRequestContracts([]byte(dialectTupleDoc)), "kin must refuse the tuple document")
	schema, diagnostics := extractInputSchemaWithIssues(dialectMethodObj(t, dialectTupleDoc, "post"), dialectRoot(t, dialectTupleDoc))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))
	props := parsed["properties"].(map[string]any)
	// 元组原样保留（只归一子 schema 的 type），可空字段折叠为 type 数组
	assert.Contains(t, mustJSON(t, props["pair"]), `"items":[`)
	assert.Equal(t, `{"type":["string","null"]}`, mustJSON(t, props["note"]))
	require.Len(t, diagnostics, 1)
	assert.Equal(t, spec.DiagSchemaDialectKeyword, diagnostics[0].Code)
	assert.Equal(t, "/properties/pair", diagnostics[0].Field)
	assert.Contains(t, diagnostics[0].Message, "tuple")
}

// TestDialect_OutputSchemaNormalized：响应 schema 与输入同机归一，Draft 7 校验
// 器接受显式 null、拒绝缺失。
func TestDialect_OutputSchemaNormalized(t *testing.T) {
	schema := extractOutputSchema(dialectMethodObj(t, dialectOutputDoc, "get"), dialectRoot(t, dialectOutputDoc))
	require.NotEmpty(t, schema)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))
	props := parsed["properties"].(map[string]any)
	assert.Equal(t, `{"type":["string","null"]}`, mustJSON(t, props["label"]))
	assert.Contains(t, mustJSON(t, props["tags"]), `"items":{"type":["string","null"]}`)
	assert.NotContains(t, schema, "nullable", "the raw 3.0 keyword must not survive")
	require.Error(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"q":"x"}`)),
		"missing required label must be rejected")
	require.NoError(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"label":null}`)),
		"explicit null must be accepted by the normalized schema")
	require.Error(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"label":"ok","tags":null}`)),
		"a non-nullable array must still reject null")
}

// TestDialect_FallbackDiscoveryRecordsDialectDiagnostic：map 回落经 provider
// 发现链把方言诊断带入方法详情。
func TestDialect_FallbackDiscoveryRecordsDialectDiagnostic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tuple.json")
	require.NoError(t, os.WriteFile(path, []byte(dialectTupleDoc), 0o644))
	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config:  map[string]any{"openapiSpec": path},
	}))
	method, ok := p.GetMethodDetails()["item.create"]
	require.True(t, ok)
	found := false
	for _, diagnostic := range method.Diagnostics {
		if diagnostic.Code == spec.DiagSchemaDialectKeyword {
			assert.Equal(t, "item.create", diagnostic.FunctionID)
			found = true
		}
	}
	assert.True(t, found, "fallback discovery must record the dialect diagnostic")
}
