package openapi

// 覆盖目标（组 E）：input_schema.go 的形态分支——参数投影（无名/非对象/
// 无 schema/弃用标注）、requestBody 形态缺失链、$ref 指针中途穿入非对象
// 节点、required 去重。纯函数表驱动：输入全部来自 json.Unmarshal 的
// map[string]interface{} 形态（与 parseOpenAPISpec 的真实数据形态一致）。

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 参数投影四形态一并过 extractInputSchema（内部调用 extractParameterSchemas）：
// 无 name 参数整段跳过（不产生空键属性）；非对象参数跳过；无 schema 参数
// 默认 string；deprecated 标注透传进属性。
func TestCoverageE_ExtractInputSchemaParameterShapes(t *testing.T) {
	methodObj := map[string]interface{}{
		"parameters": []interface{}{
			// 无 name：有 schema 也整段跳过（调用侧 getParamValue 读不到无名参数）
			map[string]interface{}{"schema": map[string]interface{}{"type": "integer"}},
			// 非对象条目：跳过，不让整条提取链失败
			"not-a-map",
			// 无 schema：属性默认 string
			map[string]interface{}{"name": "limit"},
			// 弃用标注透传
			map[string]interface{}{
				"name":       "legacyOp",
				"deprecated": true,
				"schema":     map[string]interface{}{"type": "string"},
			},
		},
	}
	out := extractInputSchema(methodObj, map[string]interface{}{})
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"limit": {"type": "string"},
			"legacyOp": {"type": "string", "deprecated": true}
		}
	}`, out)
}

// extractParameterSchemas 直接断言非对象参数分支（required 透传一并核对）。
func TestCoverageE_ExtractParameterSchemasSkipsNonObjectEntries(t *testing.T) {
	methodObj := map[string]interface{}{
		"parameters": []interface{}{
			"bogus",
			map[string]interface{}{"name": "page", "required": true},
		},
	}
	params := extractParameterSchemas(methodObj, map[string]interface{}{})
	assert.Len(t, params, 1)
	assert.Equal(t, "page", params[0].name)
	assert.True(t, params[0].required)
	assert.Equal(t, map[string]interface{}{"type": "string"}, params[0].schema)
}

// requestBody 形态缺失链：requestBody 非对象 / content 非对象 /
// application/json 非对象 / schema 非对象 / 仅有其他 content-type——
// 全部返回 (nil, false)，操作退化为纯参数输入。
func TestCoverageE_ExtractJSONBodySchemaNegativeShapes(t *testing.T) {
	root := map[string]interface{}{}
	cases := []map[string]interface{}{
		{"requestBody": "not-a-map"},
		{"requestBody": map[string]interface{}{"content": "not-a-map"}},
		{"requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": "not-a-map"}}},
		{"requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": map[string]interface{}{"schema": "not-a-map"}}}},
		{"requestBody": map[string]interface{}{"content": map[string]interface{}{"text/plain": map[string]interface{}{}}}},
	}
	for i, methodObj := range cases {
		res, ok := extractJSONBodySchema(methodObj, root)
		assert.False(t, ok, "case %d", i)
		assert.Nil(t, res, "case %d", i)
	}
}

// resolveRefPointer：指针中途穿入非对象节点（#/a/b/c 而 b 是字符串标量）
// 坍缩为空对象 schema——悬挂 ref 不让提取链炸掉。
func TestCoverageE_ResolveRefPointerThroughNonMapNode(t *testing.T) {
	root := map[string]interface{}{
		"a": map[string]interface{}{"b": "scalar"},
	}
	res := resolveRefPointer("#/a/b/c", root, 0)
	assert.Equal(t, map[string]interface{}{"type": "object"}, res)
}

// extractJSONBodySchema 的解析后非对象分支：schema 本身是对象，但 $ref
// 指向文档中的标量节点——resolveRefPointer 尾部对解析结果继续深解析时原样
// 返回标量，类型断言失败 → (nil, false)。
func TestCoverageE_ExtractJSONBodySchemaRefToScalarNode(t *testing.T) {
	root := map[string]interface{}{
		"info": map[string]interface{}{"title": "scalar"},
	}
	methodObj := map[string]interface{}{
		"requestBody": map[string]interface{}{
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{"$ref": "#/info/title"},
				},
			},
		},
	}
	res, ok := extractJSONBodySchema(methodObj, root)
	assert.False(t, ok)
	assert.Nil(t, res)
}

// appendUniqueString：重复值不追加——参数与 body required 合并时的去重语义。
func TestCoverageE_AppendUniqueStringDeduplicates(t *testing.T) {
	assert.Equal(t, []string{"id"}, appendUniqueString([]string{"id"}, "id"))
	assert.Equal(t, []string{"id", "name"}, appendUniqueString([]string{"id"}, "name"))
	assert.Equal(t, []string{"id"}, appendUniqueString([]string{}, "id"))
}
