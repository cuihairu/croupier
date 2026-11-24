package openapi

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-010 验收（驱动层）：readOnly/writeOnly 字段方向。请求侧剔除 readOnly
// 字段、响应侧剔除 writeOnly 字段，两侧 required 集同步收缩；普通字段、
// 嵌套字段与 $ref 内联字段（SUI-008 后为独立 schema）同规则。请求经 kin 与
// map 两个前端对拍（同一文档产出相同 schema），响应经 extractOutputSchema
// 验证输出 schema 已过滤——与上传管线 openAPIResponseSchema 同口径。

// directionRequestDoc：readOnly 字段分布于普通（internalId）、嵌套
// （profile.ssn）与 $ref 内联（body 整体为 UserUpdate 引用）位置。
const directionRequestDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "direction", "version": "1.0.0"},
  "paths": {
    "/users/{id}": {"post": {"operationId": "user.update",
      "parameters": [
        {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
      ],
      "requestBody": {"required": true, "content": {"application/json": {"schema": {
        "$ref": "#/components/schemas/UserUpdate"
      }}}},
      "responses": {"200": {"description": "ok"}}
    }}
  },
  "components": {"schemas": {"UserUpdate": {
    "type": "object",
    "required": ["name", "internalId", "profile"],
    "properties": {
      "name": {"type": "string"},
      "internalId": {"type": "string", "readOnly": true},
      "profile": {
        "type": "object",
        "required": ["displayName", "ssn"],
        "properties": {
          "displayName": {"type": "string"},
          "ssn": {"type": "string", "readOnly": true}
        }
      }
    }
  }}}
}`

func directionProjection(t *testing.T, doc string, apiPath, method, operationID string) InputProjection {
	t.Helper()
	var root map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	requests := kinRequestContracts([]byte(doc))
	require.NotNil(t, requests, "the kin front-end must accept this document")
	request, ok := requests[requestOperation{path: apiPath, method: method}]
	require.True(t, ok, "the operation must carry an extracted contract")
	return ProjectInput(operationID, request, root)
}

func directionFlatProps(t *testing.T, projection InputProjection) (map[string]any, map[string]any) {
	t.Helper()
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(projection.Schema), &schema))
	props := schema["properties"].(map[string]any)
	required, _ := schema["required"].([]any)
	names := make(map[string]any, len(required))
	for _, name := range required {
		names[name.(string)] = true
	}
	return props, names
}

// TestDirection_RequestDropsReadOnlyAndShrinksRequired：readOnly 字段在
// 普通/嵌套/引用内联位置全部剔除，required 集逐层收缩，剔除进契约诊断。
func TestDirection_RequestDropsReadOnlyAndShrinksRequired(t *testing.T) {
	projection := directionProjection(t, directionRequestDoc, "/users/{id}", "POST", "user.update")
	props, required := directionFlatProps(t, projection)
	// 普通字段：internalId（readOnly）剔除，name 保留；required 收缩。
	assert.NotContains(t, props, "internalId")
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "profile")
	assert.NotContains(t, required, "internalId")
	assert.Contains(t, required, "name")
	assert.Contains(t, required, "profile")
	// 嵌套字段：profile.ssn（readOnly）剔除，profile 自身保留并独立收缩。
	profile := props["profile"].(map[string]any)
	profileProps := profile["properties"].(map[string]any)
	assert.NotContains(t, profileProps, "ssn")
	assert.Contains(t, profileProps, "displayName")
	assert.Equal(t, `["displayName"]`, mustJSON(t, profile["required"]))

	fields := map[string]bool{}
	for _, diagnostic := range projection.Diagnostics {
		if diagnostic.Code == spec.DiagSchemaReadonlyDropped {
			fields[diagnostic.Field] = true
			assert.Equal(t, spec.SeverityWarning, diagnostic.Severity)
			assert.Equal(t, "user.update", diagnostic.FunctionID)
		}
	}
	assert.True(t, fields["/properties/internalId"], "plain readOnly drop is diagnosed")
	assert.True(t, fields["/properties/profile/properties/ssn"], "nested readOnly drop is diagnosed")
}

// TestDirection_KinAndMapFrontendsDropTheSame：kin 前端（ProjectInput）与 map
// 回落（extractInputSchemaWithIssues）对同一文档产出相同的请求 schema。
func TestDirection_KinAndMapFrontendsDropTheSame(t *testing.T) {
	projection := directionProjection(t, directionRequestDoc, "/users/{id}", "POST", "user.update")
	root := dialectRoot(t, directionRequestDoc)
	var methodObj map[string]any
	paths := root["paths"].(map[string]any)
	pathItem := paths["/users/{id}"].(map[string]any)
	methodObj = pathItem["post"].(map[string]any)
	fallback, fallbackDiagnostics := extractInputSchemaWithIssues(methodObj, root)
	require.JSONEq(t, fallback, projection.Schema, "kin and map front-ends must drop the same readOnly fields")
	// 回落侧同样携带方向诊断。
	found := false
	for _, diagnostic := range fallbackDiagnostics {
		if diagnostic.Code == spec.DiagSchemaReadonlyDropped && diagnostic.Field == "/properties/internalId" {
			found = true
		}
	}
	assert.True(t, found, "the map fallback must record the readOnly drop")
}

// TestDirection_ResponseDropsWriteOnly：extractOutputSchema 对响应侧 schema
// 剔除 writeOnly 字段（普通 + 嵌套 + $ref 内联），required 集收缩。
func TestDirection_ResponseDropsWriteOnly(t *testing.T) {
	root := map[string]interface{}{
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"UserView": map[string]interface{}{
					"type":     "object",
					"required": []interface{}{"id", "secret", "profile"},
					"properties": map[string]interface{}{
						"id":     map[string]interface{}{"type": "integer"},
						"secret": map[string]interface{}{"type": "string", "writeOnly": true},
						"profile": map[string]interface{}{
							"type":     "object",
							"required": []interface{}{"displayName", "token"},
							"properties": map[string]interface{}{
								"displayName": map[string]interface{}{"type": "string"},
								"token":       map[string]interface{}{"type": "string", "writeOnly": true},
							},
						},
					},
				},
			},
		},
	}
	methodObj := map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"$ref": "#/components/schemas/UserView"},
					},
				},
			},
		},
	}
	out := extractOutputSchema(methodObj, root)
	require.JSONEq(t,
		`{"type":"object","required":["id","profile"],"properties":{"id":{"type":"integer"},"profile":{"type":"object","required":["displayName"],"properties":{"displayName":{"type":"string"}}}}}`,
		out)
}

// TestDirection_ResponseWholeSchemaWriteOnlyDropped：整个响应 schema 属
// 请求侧（根 writeOnly）→ 无输出 schema 可校验，提取落到下一 2xx 候选。
func TestDirection_ResponseWholeSchemaWriteOnlyDropped(t *testing.T) {
	root := map[string]interface{}{
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"SecretOnly": map[string]interface{}{
					"type":       "object",
					"writeOnly":  true,
					"properties": map[string]interface{}{"secret": map[string]interface{}{"type": "string"}},
				},
			},
		},
	}
	methodObj := map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"$ref": "#/components/schemas/SecretOnly"},
					},
				},
			},
			"204": map[string]interface{}{"description": "no content"},
		},
	}
	// 200 的整个 schema 被剔除 → 尝试 204（无 schema）→ 空串（跳过校验）。
	assert.Empty(t, extractOutputSchema(methodObj, root))
}

// TestDirection_RequestNoReadOnlyNoDiagnostics：无 readOnly 字段的请求
// 契约不产生方向诊断（静默保真，方向过滤仅在真实剔除时生效）。
func TestDirection_RequestNoReadOnlyNoDiagnostics(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"t","version":"1.0.0"},"paths":{"/x":{"post":{"operationId":"x.create","requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}},"required":["a"]}}}},"responses":{"200":{"description":"ok"}}}}}}`
	projection := directionProjection(t, doc, "/x", "POST", "x.create")
	for _, diagnostic := range projection.Diagnostics {
		assert.NotEqual(t, spec.DiagSchemaReadonlyDropped, diagnostic.Code, "no readOnly field → no drop diagnostic")
		assert.NotEqual(t, spec.DiagSchemaWriteonlyDropped, diagnostic.Code)
	}
	props, required := directionFlatProps(t, projection)
	assert.Contains(t, props, "a")
	assert.Contains(t, props, "b")
	assert.Contains(t, required, "a")
}

// TestDirection_DroppedPropertyNoLongerCollides：body 属性与参数同名时
// 原本触发冲突诊断；该属性是 readOnly（被方向过滤剔除）时不再冲突——
// 平铺诊断须在过滤后计算，否则输出假阳性冲突。
func TestDirection_DroppedPropertyNoLongerCollides(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"t","version":"1.0.0"},"paths":{"/u":{"post":{"operationId":"u.update","parameters":[{"name":"id","in":"query","required":true,"schema":{"type":"string"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string","readOnly":true},"a":{"type":"string"}}}}}},"responses":{"200":{"description":"ok"}}}}}}`
	projection := directionProjection(t, doc, "/u", "POST", "u.update")
	for _, diagnostic := range projection.Diagnostics {
		assert.NotEqual(t, FlatInputCollision, diagnostic.Code,
			"the colliding readOnly property is dropped, so no collision diagnostic")
	}
	assert.False(t, projection.Grouped, "filtered body stays flat")
	props, _ := directionFlatProps(t, projection)
	assert.Contains(t, props, "a")
	// 平铺后 id 槽位由参数占据（body 侧的 readOnly 副本已被剔除）。
	assert.JSONEq(t, `{"type":"string"}`, mustJSON(t, props["id"]), "the flat id slot is the parameter schema")
}

// TestDirection_FlaggedRootRequestBodyDropped：body 根级 readOnly → 整个
// body 槽位消失（操作无 body 可发），不产生「无 JSON body」约束诊断。
func TestDirection_FlaggedRootRequestBodyDropped(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"t","version":"1.0.0"},"paths":{"/x":{"post":{"operationId":"x.create","requestBody":{"content":{"application/json":{"schema":{"type":"object","readOnly":true,"properties":{"a":{"type":"string"}}}}}},"responses":{"200":{"description":"ok"}}}}}}`
	projection := directionProjection(t, doc, "/x", "POST", "x.create")
	require.NotEmpty(t, projection.Diagnostics)
	assert.Equal(t, spec.DiagSchemaReadonlyDropped, projection.Diagnostics[0].Code)
	assert.Equal(t, "", projection.Diagnostics[0].Field, "the flagged root is recorded at the schema root")
	require.JSONEq(t, `{"type":"object","properties":{}}`, projection.Schema)
}

// TestDirection_ParameterRootReadOnlyKept：参数 schema 根级 readOnly →
// 参数保留（地址永不破坏），投影以缺省 string 类型呈现，剔除进诊断。
func TestDirection_ParameterRootReadOnlyKept(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"t","version":"1.0.0"},"paths":{"/u/{id}":{"post":{"operationId":"u.update","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string","readOnly":true}}],"responses":{"200":{"description":"ok"}}}}}}`
	projection := directionProjection(t, doc, "/u/{id}", "POST", "u.update")
	found := false
	for _, diagnostic := range projection.Diagnostics {
		if diagnostic.Code == spec.DiagSchemaReadonlyDropped {
			assert.Equal(t, "", diagnostic.Field)
			found = true
		}
	}
	require.True(t, found, "the flagged parameter root is diagnosed")
	props, required := directionFlatProps(t, projection)
	assert.Contains(t, props, "id", "the parameter survives with a usable type")
	assert.Contains(t, required, "id", "addressing stays required")
}
