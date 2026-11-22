package openapi

// 覆盖目标（组 E）：请求契约提取与投影的形态分支（SUI-002/003）——参数投影
// （无名/非对象/无 schema/弃用标注）、requestBody 形态缺失链、$ref 指针中途
// 穿入非对象节点、required 去重。纯函数表驱动：输入全部来自 json.Unmarshal
// 的 map[string]interface{} 形态（与 parseOpenAPISpec 的真实数据形态一致）。

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 参数投影形态一并过 ExtractOperationRequestContractMap + ProjectOperationInputSchema：
// 无 name 参数整段跳过（不产生空键属性）；非对象参数跳过；无 schema 参数默认
// string；弃用标注留在契约参数级、不进属性 schema（与旧上传路径一致）。
func TestCoverageE_ExtractInputSchemaParameterShapes(t *testing.T) {
	methodObj := map[string]interface{}{
		"parameters": []interface{}{
			// 无 name：有 schema 也整段跳过（调用侧 getParamValue 读不到无名参数）
			map[string]interface{}{"in": "query", "schema": map[string]interface{}{"type": "integer"}},
			// 非对象条目：跳过，不让整条提取链失败
			"not-a-map",
			// 无 schema：属性默认 string
			map[string]interface{}{"name": "limit", "in": "query"},
			// 弃用标注：留在契约参数级（Deprecated），不进属性 schema
			map[string]interface{}{
				"name":       "legacyOp",
				"in":         "query",
				"deprecated": true,
				"schema":     map[string]interface{}{"type": "string"},
			},
		},
	}
	contract := ExtractOperationRequestContractMap(methodObj, map[string]interface{}{}, nil)
	schema, mode := ProjectOperationInputSchema(contract)
	assert.Equal(t, "flat", string(mode))
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"limit": {"type": "string"},
			"legacyOp": {"type": "string"}
		}
	}`, schema)
	require.Len(t, contract.Parameters, 2)
	assert.True(t, contract.Parameters[1].Deprecated, "deprecated must stay on the contract parameter")
}

// 参数位置分类：合法位置全部收入契约；未知位置跳过（由扫描层负责诊断）。
func TestCoverageE_ExtractContractClassifiesPositions(t *testing.T) {
	methodObj := map[string]interface{}{
		"parameters": []interface{}{
			map[string]interface{}{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
			map[string]interface{}{"name": "q", "in": "query", "schema": map[string]interface{}{"type": "string"}},
			map[string]interface{}{"name": "X-Token", "in": "header", "schema": map[string]interface{}{"type": "string"}},
			map[string]interface{}{"name": "session", "in": "cookie", "required": true, "schema": map[string]interface{}{"type": "string"}},
			map[string]interface{}{"name": "bogus", "in": "matrix", "schema": map[string]interface{}{"type": "string"}},
		},
	}
	contract := ExtractOperationRequestContractMap(methodObj, map[string]interface{}{}, nil)
	require.Len(t, contract.Parameters, 4, "unknown position must be skipped, known positions kept")
	// 序列化默认：path→simple、query→form、header→simple、cookie→form。
	byName := map[string]string{}
	for _, p := range contract.Parameters {
		byName[p.Name] = p.Serialization.Style
	}
	assert.Equal(t, "simple", byName["id"])
	assert.Equal(t, "form", byName["q"])
	assert.Equal(t, "simple", byName["X-Token"])
	assert.Equal(t, "form", byName["session"])
}

// requestBody 形态缺失链：requestBody 非对象 / content 非对象 /
// application/json 非对象 / schema 非对象 / 仅有其他 content-type——
// 全部无 body 槽位，操作退化为纯参数输入。
func TestCoverageE_ExtractRequestBodyNegativeShapes(t *testing.T) {
	root := map[string]interface{}{}
	cases := []map[string]interface{}{
		{"requestBody": "not-a-map"},
		{"requestBody": map[string]interface{}{"content": "not-a-map"}},
		{"requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": "not-a-map"}}},
		{"requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": map[string]interface{}{"schema": "not-a-map"}}}},
		{"requestBody": map[string]interface{}{"content": map[string]interface{}{"text/plain": map[string]interface{}{}}}},
	}
	for i, methodObj := range cases {
		contract := ExtractOperationRequestContractMap(methodObj, root, nil)
		assert.Nil(t, contract.Body, "case %d", i)
	}
}

// resolveRefPointer：指针中途穿入非对象节点（#/a/b/c 而 b 是字符串标量）
// 坍缩为空对象 schema——悬挂 ref 不让提取链炸掉。
func TestCoverageE_ResolveRefPointerThroughNonMapNode(t *testing.T) {
	root := map[string]interface{}{
		"a": map[string]interface{}{"b": "scalar"},
	}
	res := resolveRefPointer("#/a/b/c", root, 0, nil)
	assert.Equal(t, map[string]interface{}{"type": "object"}, res)
}

// extractMapRequestBody 的解析后非对象分支：schema 本身是对象，但 $ref
// 指向文档中的标量节点——resolveRefPointer 尾部对解析结果继续深解析时原样
// 返回标量，类型断言失败 → 无 body 槽位。
func TestCoverageE_ExtractRequestBodyRefToScalarNode(t *testing.T) {
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
	contract := ExtractOperationRequestContractMap(methodObj, root, nil)
	assert.Nil(t, contract.Body)
}

// appendUniqueString：重复值不追加——参数与 body required 合并时的去重语义。
func TestCoverageE_AppendUniqueStringDeduplicates(t *testing.T) {
	assert.Equal(t, []string{"id"}, appendUniqueString([]string{"id"}, "id"))
	assert.Equal(t, []string{"id", "name"}, appendUniqueString([]string{"id"}, "name"))
	assert.Equal(t, []string{"id"}, appendUniqueString([]string{}, "id"))
}
