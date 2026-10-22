package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 批次 D 对拍集（能力矩阵 #3 验收）：同一 spec 双实现推导 inputSchema，
// 逐 operation JSONEq 对齐。差异必须逐个人工裁定——旧对/新错→修新实现，
// 旧错/新对→记录为修复并移入 expectedDiffs（带理由）。风险门控：对拍差异
// 率 >5% 或出现不可裁定项即叫停本批（拍板口径，见矩阵文档）。
//
// 旧实现 = extractInputSchema（map 遍历，兼容回落路径）；新实现 =
// kinInputSchemas（kin-openapi 解析层，主路径）。

func deriveInputSchemasOld(doc string) map[string]string {
	var root map[string]interface{}
	_ = json.Unmarshal([]byte(doc), &root)
	paths, _ := root["paths"].(map[string]interface{})
	out := map[string]string{}
	for path, pathItem := range paths {
		pathObj, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}
		for httpMethod, methodItem := range pathObj {
			httpMethod = strings.ToUpper(httpMethod)
			if httpMethod == "PARAMETERS" || httpMethod == "$REF" {
				continue
			}
			methodObj, ok := methodItem.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := methodObj["operationId"].(string)
			if name == "" {
				name = pathToMethodName(path, httpMethod)
			}
			out[name] = extractInputSchema(methodObj, root)
		}
	}
	return out
}

func deriveInputSchemasKin(doc string) map[string]string {
	var root map[string]interface{}
	_ = json.Unmarshal([]byte(doc), &root)
	return kinInputSchemas([]byte(doc), root)
}

type parityDoc struct {
	name string
	doc  string
	// kinNil 期望 kinInputSchemas 返回 nil（旧路径兜底覆盖，不做逐 op 对拍）
	kinNil bool
}

func parityCorpus() []parityDoc {
	return []parityDoc{
		// ---- 既有 fixture（input_schema_test.go 起底）----
		{"params+inline-body-merge", `{
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
		}`, false},
		{"ref-body-merge", `{
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
		`, false},
		{"cyclic-ref", `{
		  "openapi": "3.0.3",
		  "components": {"schemas": {"Node": {
		    "type": "object",
		    "properties": {"child": {"$ref": "#/components/schemas/Node"}}
		  }}},
		  "paths": {"/nodes": {"post": {
		    "operationId": "node.create",
		    "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Node"}}}}
		  }}}}
		`, false},
		{"non-object-body", `{
		  "openapi": "3.0.3",
		  "paths": {"/batch": {"post": {
		    "operationId": "batch.import",
		    "requestBody": {"content": {"application/json": {"schema": {
		      "type": "array", "items": {"type": "string"}
		    }}}}
		  }}}}
		`, false},
		{"no-input", `{
		  "openapi": "3.0.3",
		  "paths": {"/players": {"get": {"operationId": "player.list"}}}
		}`, false},
		// 悬空/外链 $ref：kin loader 拒载 → kinNil，走旧路径（行为保真即对拍通过）。
		{"dangling-and-external-refs", `{
		  "openapi": "3.0.3",
		  "paths": {"/x": {"post": {
		    "operationId": "x.create",
		    "parameters": [
		      {"name": "a", "in": "query", "schema": {"$ref": "#/components/schemas/Missing"}},
		      {"name": "b", "in": "query", "schema": {"$ref": "https://example.com/other.json#/schemas/B"}}
		    ]
		  }}}}
		`, true},
		// ---- 角病例（矩阵点名的 OpenAPI 边角）----
		{"allof-body", `{
		  "openapi": "3.0.3",
		  "paths": {"/compose": {"post": {
		    "operationId": "compose.create",
		    "requestBody": {"content": {"application/json": {"schema": {"allOf": [
		      {"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]},
		      {"type": "object", "properties": {"b": {"type": "integer"}}}
		    ]}}}}
		  }}}}
		`, false},
		{"nullable-and-vendor-ext", `{
		  "openapi": "3.0.3",
		  "components": {"schemas": {"Name": {
		    "type": "string", "nullable": true, "x-vendor-note": "kept"
		  }}},
		  "paths": {"/n": {"post": {
		    "operationId": "n.create",
		    "parameters": [{"name": "name", "in": "query", "schema": {"$ref": "#/components/schemas/Name"}}]
		  }}}}
		`, false},
		{"param-without-schema-defaults-string", `{
		  "openapi": "3.0.3",
		  "paths": {"/s": {"get": {
		    "operationId": "s.list",
		    "parameters": [
		      {"name": "q", "in": "query"},
		      {"name": "limit", "in": "query", "required": true}
		    ]
		  }}}}
		`, false},
		{"header-cookie-params-required", `{
		  "openapi": "3.0.3",
		  "paths": {"/h": {"get": {
		    "operationId": "h.call",
		    "parameters": [
		      {"name": "X-Trace", "in": "header", "schema": {"type": "string"}},
		      {"name": "session", "in": "cookie", "required": true, "schema": {"type": "string", "format": "uuid"}},
		      {"name": "ver", "in": "path", "required": true, "schema": {"type": "integer", "enum": [1, 2]}}
		    ]
		  }}}}
		`, false},
		{"body-props-enum-format-default-desc", `{
		  "openapi": "3.0.3",
		  "paths": {"/e": {"post": {
		    "operationId": "e.create",
		    "requestBody": {"content": {"application/json": {"schema": {"type": "object",
		      "properties": {
		        "kind": {"type": "string", "enum": ["pvp", "pve"], "default": "pve", "description": "battle kind"},
		        "ratio": {"type": "number", "format": "double", "minimum": 0, "maximum": 1}
		      },
		      "required": ["kind"]}}}}
		  }}}}
		`, false},
		{"no-operationid-name-derived", `{
		  "openapi": "3.0.3",
		  "paths": {"/things/{tid}": {"delete": {
		    "parameters": [{"name": "tid", "in": "path", "required": true, "schema": {"type": "string"}}]
		  }}}}
		`, false},
		{"nested-property-ref", `{
		  "openapi": "3.0.3",
		  "components": {"schemas": {"Item": {"type": "object", "properties": {"sku": {"type": "string"}}}}},
		  "paths": {"/cart": {"post": {
		    "operationId": "cart.add",
		    "requestBody": {"content": {"application/json": {"schema": {"type": "object",
		      "properties": {"item": {"$ref": "#/components/schemas/Item"}}, "required": ["item"]}}}}
		  }}}}
		`, false},
		{"openapi31-type-array", `{
		  "openapi": "3.1.0",
		  "paths": {"/t": {"post": {
		    "operationId": "t.create",
		    "requestBody": {"content": {"application/json": {"schema": {"type": "object",
		      "properties": {"maybe": {"type": ["string", "null"]}}}}}}
		  }}}}
		`, false},
		// Swagger 2.0：kin 拒载 → 旧 map 遍历路径（openapiSpec 形态兼容边界）。
		{"swagger20-falls-back", `{
		  "swagger": "2.0",
		  "info": {"version": "1.0.0"},
		  "paths": {"/legacy": {"get": {
		    "operationId": "legacy.list",
		    "parameters": [{"name": "q", "in": "query", "type": "string"}],
		    "responses": {"200": {"description": "ok"}}
		  }}}
		}`, true},
	}
}

func TestInputSchemaParity_KinMatchesLegacy(t *testing.T) {
	var diffs []string
	for _, tc := range parityCorpus() {
		newSchemas := deriveInputSchemasKin(tc.doc)
		if tc.kinNil {
			require.Nil(t, newSchemas, "%s: expected kin loader rejection (fallback covers)", tc.name)
			continue
		}
		require.NotNil(t, newSchemas, "%s: kin must parse this corpus doc", tc.name)
		oldSchemas := deriveInputSchemasOld(tc.doc)
		require.Len(t, newSchemas, len(oldSchemas),
			"%s: operation count mismatch old=%d new=%d", tc.name, len(oldSchemas), len(newSchemas))
		for name, oldSchema := range oldSchemas {
			newSchema, ok := newSchemas[name]
			if !ok {
				diffs = append(diffs, fmt.Sprintf("%s/%s: operation missing in kin result", tc.name, name))
				continue
			}
			if !jsonEqual(oldSchema, newSchema) {
				diffs = append(diffs, fmt.Sprintf("%s/%s:\n  old: %s\n  new: %s", tc.name, name, oldSchema, newSchema))
			}
		}
	}
	// 拍板门控：此处必须为空。任何残留差异即对拍未收敛，差异率 >5% 叫停本批。
	require.Empty(t, diffs, "对拍未收敛（逐条裁定后修新实现或记录为修复）:\n%s", strings.Join(diffs, "\n---\n"))
}

// jsonEqual 语义等价比较（两侧均 map 编码、键序无关；数值经 json.Number
// 解码后字符串形态必须一致，避免 5 与 5.0 误判等价）。
func jsonEqual(a, b string) bool {
	var av, bv interface{}
	decA := json.NewDecoder(strings.NewReader(a))
	decA.UseNumber()
	decB := json.NewDecoder(strings.NewReader(b))
	decB.UseNumber()
	if err := decA.Decode(&av); err != nil {
		return false
	}
	if err := decB.Decode(&bv); err != nil {
		return false
	}
	return jsonNumbersEqual(av, bv)
}

func jsonNumbersEqual(a, b interface{}) bool {
	switch at := a.(type) {
	case map[string]interface{}:
		bt, ok := b.(map[string]interface{})
		if !ok || len(at) != len(bt) {
			return false
		}
		for k, av := range at {
			bv, ok := bt[k]
			if !ok || !jsonNumbersEqual(av, bv) {
				return false
			}
		}
		return true
	case []interface{}:
		bt, ok := b.([]interface{})
		if !ok || len(at) != len(bt) {
			return false
		}
		for i := range at {
			if !jsonNumbersEqual(at[i], bt[i]) {
				return false
			}
		}
		return true
	case json.Number:
		bn, ok := b.(json.Number)
		return ok && bn.String() == at.String()
	default:
		return a == b
	}
}

// FallbackGuard：kinNil 语料经完整 parseOpenAPISpec 链仍必须产出 InputSchema
// （旧路径兜底不丢函数）。
func TestInputSchemaParity_FallbackStillDerives(t *testing.T) {
	for _, tc := range parityCorpus() {
		if !tc.kinNil {
			continue
		}
		p := NewProvider()
		require.NoError(t, p.parseOpenAPISpec([]byte(tc.doc)), tc.name)
		methods := p.methodMap
		require.NotEmpty(t, methods, "%s: fallback must keep methods registered", tc.name)
		for name, m := range methods {
			require.NotEmpty(t, m.InputSchema, "%s/%s: fallback InputSchema must be non-empty", tc.name, name)
		}
	}
}
