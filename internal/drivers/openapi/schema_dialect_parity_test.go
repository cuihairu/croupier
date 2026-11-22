package openapi

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-009 验收（驱动层）：同一文档经 kin 前端与 map 前端提取的请求契约在
// 方言归一后逐 JSON 同形——3.0 `nullable` 与 3.1 `type: [T, "null"]` 都折叠
// 为 `type: [T, "null"]`（"null" 恒在末位），条件 schema / 元组 / 多类型联合
// 在两侧记录同一能力诊断。前端（web/src/utils/schemaDialect.ts）对同一批
// 样例的判定与 spec.ClassifySchemaSupport 一致。

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
// kin 可载入（type 数组 / prefixItems / if-then 均为 kin Schema 字段），
// 两侧对拍同形并记录同一能力诊断。
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

// dialectTupleDoc：draft-07 元组（items 为数组）——kin 拒载（SchemaBis 无法
// 解组数组），生产走 map 回落；map 前端记录元组能力诊断。
const dialectTupleDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "dialect", "version": "1.0.0"},
  "paths": {
    "/items": {"post": {"operationId": "item.create",
      "requestBody": {"content": {"application/json": {"schema": {
        "type": "object",
        "properties": {
          "pair": {"type": "array", "items": [{"type": "string"}, {"type": "integer"}]}
        }
      }}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
}`

func kinMapContracts(t *testing.T, doc string) (spec.OperationRequestContract, spec.OperationRequestContract) {
	t.Helper()
	loaded, root := mustLoadKin(t, doc)
	item := loaded.Paths.Find("/items")
	require.NotNil(t, item)
	op := item.Post
	require.NotNil(t, op)
	kin := ExtractOperationRequestContractKin(op, item.Parameters, root)

	var rootMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &rootMap))
	paths := rootMap["paths"].(map[string]interface{})
	pathItem := paths["/items"].(map[string]interface{})
	methodObj := pathItem["post"].(map[string]interface{})
	m := ExtractOperationRequestContractMap(methodObj, rootMap, nil)
	return kin, m
}

func TestDialect30_NullableNormalized(t *testing.T) {
	kin, m := kinMapContracts(t, dialect30Doc)
	// 两个前端对拍同形
	assert.Equal(t, jsonText(t, kin), jsonText(t, m))

	// 参数 note：nullable string → type:["string","null"]
	require.Len(t, kin.Parameters, 1)
	assert.Equal(t, `{"type":["string","null"]}`, jsonText(t, kin.Parameters[0].Schema))

	// body 属性：label 必填可空、count 可空、tags 可空数组
	body := parseObject(kin.Body.Schema)
	props := body["properties"].(map[string]interface{})
	assert.Equal(t, `{"type":["string","null"]}`, jsonText(t, props["label"]))
	assert.Equal(t, `{"type":["integer","null"]}`, jsonText(t, props["count"]))
	assert.Equal(t, `{"items":{"type":["string","null"]},"type":["array","null"]}`, jsonText(t, props["tags"]))
	assert.Equal(t, `["label"]`, jsonText(t, body["required"]))
	// 无诊断：可空单类型是直接可表单的
	assert.Empty(t, kin.Diagnostics)
}

func TestDialect31_UnionNormalized(t *testing.T) {
	kin, m := kinMapContracts(t, dialect31Doc)
	assert.Equal(t, jsonText(t, kin), jsonText(t, m))

	require.Len(t, kin.Parameters, 1)
	assert.Equal(t, `{"type":["string","null"]}`, jsonText(t, kin.Parameters[0].Schema))

	body := parseObject(kin.Body.Schema)
	props := body["properties"].(map[string]interface{})
	// 3.1 `["null","integer"]` → "null" 移到末位
	assert.Equal(t, `{"type":["integer","null"]}`, jsonText(t, props["count"]))
	// 嵌套 items 联合也归一
	assert.Equal(t, `{"items":{"type":["string","null"]},"type":["array","null"]}`, jsonText(t, props["tags"]))
	assert.Empty(t, kin.Diagnostics)
}

func TestDialect30And31_ProduceSameCanonicalForm(t *testing.T) {
	// 3.0 nullable 与 3.1 联合类型描述同一语义 → 归一后逐字节同形。
	kin30, _ := kinMapContracts(t, dialect30Doc)
	kin31, _ := kinMapContracts(t, dialect31Doc)
	assert.Equal(t, jsonText(t, kin30), jsonText(t, kin31))
}

func TestDialect_DegradedKeywordsNoted(t *testing.T) {
	kin, m := kinMapContracts(t, dialectDegradedDoc)
	assert.Equal(t, jsonText(t, kin), jsonText(t, m))

	// 多类型联合（type 数组两个非 null）+ 3.1 元组（prefixItems）+ 条件 if/then
	// 应产生能力诊断；body 槽位仍在（schema 仍可用）。
	require.NotNil(t, kin.Body)
	fields := map[string]int{}
	for _, diag := range kin.Diagnostics {
		if diag.Code == spec.DiagSchemaDialectKeyword {
			fields[diag.Field]++
			assert.Equal(t, spec.SeverityWarning, diag.Severity)
		}
	}
	// mode 的多类型联合在 /properties/mode
	assert.Contains(t, fields, "/properties/mode")
	// pair 的 3.1 元组在 /properties/pair
	assert.Contains(t, fields, "/properties/pair")
	// choice 的 if/then 在 /properties/choice
	assert.Contains(t, fields, "/properties/choice")
}

// TestDialect_Draft07TupleFallsBackToMap：draft-07 元组（items 为数组）kin
// 拒载，生产走 map 回落；回落侧记录元组能力诊断（不静默）。
func TestDialect_Draft07TupleFallsBackToMap(t *testing.T) {
	kinContracts := kinOperationContracts([]byte(dialectTupleDoc), mustMapRoot(t, dialectTupleDoc))
	// kin 拒载 → nil，调用方回落 map
	require.Nil(t, kinContracts)

	contract := ExtractOperationRequestContractMap(mapMethodObj(t, dialectTupleDoc), mustMapRoot(t, dialectTupleDoc), nil)
	require.NotNil(t, contract.Body)
	found := false
	for _, diag := range contract.Diagnostics {
		if diag.Code == spec.DiagSchemaDialectKeyword && diag.Field == "/properties/pair" {
			found = true
			assert.Contains(t, diag.Message, "tuple")
		}
	}
	assert.True(t, found, "draft-07 tuple must be noted by the map front-end fallback")
}

// TestDialect_ValidationAcceptsNullButNotMissing：归一后的 schema 经服务端
// 校验器（Draft 7）验证缺失值 vs 显式 null 的区分——required 可空字段缺
// 失被拒、显式 null 通过；可选非空字段缺失通过、显式 null 被拒。
func TestDialect_ValidationAcceptsNullButNotMissing(t *testing.T) {
	schema := `{
	  "type": "object",
	  "properties": {
	    "label": {"type": ["string", "null"]},
	    "q": {"type": "string"}
	  },
	  "required": ["label"]
	}`
	// label 缺失 → 拒绝（required 只查存在性）
	require.Error(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"q":"x"}`)))
	// label 显式 null → 通过（type 含 null）
	require.NoError(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"label":null}`)))
	// label 非空 → 通过
	require.NoError(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"label":"ok"}`)))
	// q 缺失 → 通过（非 required）
	require.NoError(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"label":"ok"}`)))
	// q 显式 null → 拒绝（type 无 null）
	require.Error(t, validation.ValidateJSONRaw(json.RawMessage(schema), []byte(`{"label":"ok","q":null}`)))
}

func jsonText(t *testing.T, v interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	return string(encoded)
}

func mustMapRoot(t *testing.T, doc string) map[string]interface{} {
	t.Helper()
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	return root
}

func mapMethodObj(t *testing.T, doc string) map[string]interface{} {
	t.Helper()
	root := mustMapRoot(t, doc)
	paths := root["paths"].(map[string]interface{})
	pathItem := paths["/items"].(map[string]interface{})
	return pathItem["post"].(map[string]interface{})
}
