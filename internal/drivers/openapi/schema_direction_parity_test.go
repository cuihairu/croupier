package openapi

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-010 验收（驱动层）：readOnly/writeOnly 字段方向。请求侧剔除 readOnly
// 字段、响应侧剔除 writeOnly 字段，两侧 required 集同步收缩；普通字段、
// 嵌套字段与 $ref 内联字段（SUI-008 后为独立 schema）同规则。请求经 kin 与
// map 两个前端对拍（同一文档产出相同契约），响应经 extractOutputSchema
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

// TestDirectionFilter_RequestKinMapParityAndDrops：kin 与 map 前端对同一文档
// 提取的请求契约逐 JSON 同形；readOnly 字段在普通/嵌套/引用内联位置全部
// 剔除，required 集逐层收缩，剔除进契约诊断。
func TestDirectionFilter_RequestKinMapParityAndDrops(t *testing.T) {
	kinContracts := kinOperationContracts([]byte(directionRequestDoc), mustMapRoot(t, directionRequestDoc))
	require.NotNil(t, kinContracts, "3.0 doc must be kin-loadable")
	kin := kinContracts["user.update"]

	root := mustMapRoot(t, directionRequestDoc)
	paths := root["paths"].(map[string]interface{})
	pathItem := paths["/users/{id}"].(map[string]interface{})
	methodObj := pathItem["post"].(map[string]interface{})
	m := ExtractOperationRequestContractMap(methodObj, root, nil)

	assert.Equal(t, jsonText(t, kin), jsonText(t, m), "kin and map front-ends must drop the same readOnly fields")

	require.NotNil(t, kin.Body)
	body := parseObject(kin.Body.Schema)
	props := body["properties"].(map[string]interface{})
	// 普通字段：internalId（readOnly）剔除，name 保留。
	assert.NotContains(t, props, "internalId")
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "profile")
	assert.Equal(t, `["name","profile"]`, jsonText(t, body["required"]), "top-level required shrinks past the dropped field")
	// 嵌套字段：profile.ssn（readOnly）剔除，profile 自身保留。
	profile := props["profile"].(map[string]interface{})
	profileProps := profile["properties"].(map[string]interface{})
	assert.NotContains(t, profileProps, "ssn")
	assert.Contains(t, profileProps, "displayName")
	assert.Equal(t, `["displayName"]`, jsonText(t, profile["required"]), "nested required shrinks independently")

	// 剔除进契约诊断（输入侧有诊断通道）。
	fields := map[string]bool{}
	for _, diag := range kin.Diagnostics {
		if diag.Code == spec.DiagSchemaReadonlyDropped {
			fields[diag.Field] = true
			assert.Equal(t, spec.SeverityWarning, diag.Severity)
		}
	}
	assert.True(t, fields["/properties/internalId"], "plain readOnly drop is diagnosed")
	assert.True(t, fields["/properties/profile/properties/ssn"], "nested readOnly drop is diagnosed")
}

// TestDirectionFilter_ResponseDropsWriteOnly：extractOutputSchema 对响应侧
// schema 剔除 writeOnly 字段（普通 + 嵌套 + $ref 内联），required 集收缩。
func TestDirectionFilter_ResponseDropsWriteOnly(t *testing.T) {
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

// TestDirectionFilter_ResponseWholeSchemaWriteOnlyDropped：整个响应 schema 属
// 请求侧（根 writeOnly）→ 无输出 schema 可校验，提取落到下一 2xx 候选。
func TestDirectionFilter_ResponseWholeSchemaWriteOnlyDropped(t *testing.T) {
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

// TestDirectionFilter_RequestNoReadOnlyNoDiagnostics：无 readOnly 字段的请求
// 契约不产生方向诊断（静默保真，方向过滤仅在真实剔除时生效）。
func TestDirectionFilter_RequestNoReadOnlyNoDiagnostics(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"t","version":"1.0.0"},"paths":{"/x":{"post":{"operationId":"x.create","requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}},"required":["a"]}}}},"responses":{"200":{"description":"ok"}}}}}}`
	kinContracts := kinOperationContracts([]byte(doc), mustMapRoot(t, doc))
	require.NotNil(t, kinContracts)
	kin := kinContracts["x.create"]
	for _, diag := range kin.Diagnostics {
		assert.NotEqual(t, spec.DiagSchemaReadonlyDropped, diag.Code, "no readOnly field → no drop diagnostic")
		assert.NotEqual(t, spec.DiagSchemaWriteonlyDropped, diag.Code)
	}
	require.NotNil(t, kin.Body)
	body := parseObject(kin.Body.Schema)
	props := body["properties"].(map[string]interface{})
	assert.Contains(t, props, "a")
	assert.Contains(t, props, "b")
	assert.Equal(t, `["a"]`, jsonText(t, body["required"]))
}

// TestDirectionFilter_DroppedPropertyNoLongerCollides：body 属性与参数同名时
// 原本触发冲突诊断（SUI-004）；该属性是 readOnly（被方向过滤剔除）时不再
// 冲突——平铺/序列化诊断须在过滤后计算，否则输出假阳性冲突。
func TestDirectionFilter_DroppedPropertyNoLongerCollides(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"t","version":"1.0.0"},"paths":{"/u":{"post":{"operationId":"u.update","parameters":[{"name":"id","in":"query","required":true,"schema":{"type":"string"}}],"requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string","readOnly":true},"a":{"type":"string"}}}}}},"responses":{"200":{"description":"ok"}}}}}}`
	kinContracts := kinOperationContracts([]byte(doc), mustMapRoot(t, doc))
	require.NotNil(t, kinContracts)
	kin := kinContracts["u.update"]
	for _, diag := range kin.Diagnostics {
		assert.NotEqual(t, spec.DiagRequestParameterCollision, diag.Code,
			"the colliding readOnly property is dropped, so no collision diagnostic")
	}
	require.NotNil(t, kin.Body)
	body := parseObject(kin.Body.Schema)
	props := body["properties"].(map[string]interface{})
	assert.NotContains(t, props, "id")
	assert.Contains(t, props, "a")
	// 无 body/body-only 形状诊断：过滤后 body 是普通 object。
	schema, mode := ProjectOperationInputSchema(kin)
	assert.Equal(t, "flat", string(mode))
	_ = schema
}
