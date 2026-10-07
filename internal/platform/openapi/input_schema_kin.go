package openapi

import (
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// kinInputSchemas 用 kin-openapi（getkin，官方生态事实标准）解析 spec 并按
// extractInputSchema 的既有输出契约推导每个 operation 的 inputSchema
// （能力矩阵 #3，批次 D：spec 解析与 schema 遍历层换官方库，输出形状由
// input_schema_parity_test 锁定）。返回 methodName → schema 文本。
//
// kin 无法解析（Swagger 2.0、悬空/外链 $ref 等旧实现可容忍的形态）时返回
// nil，调用方回落 map 遍历旧路径——旧推导保留为兼容回落而非主路径。
func kinInputSchemas(spec []byte, root map[string]interface{}) map[string]string {
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil {
		return nil
	}
	// kin 的 LoadFromData 不校验版本字段、对 Swagger 2.0 文档也能加载——但 2.0
	// 语义（definitions/produces/formData）kin 不保真，主路径只对 3.x 负责，
	// 其余回落旧 map 遍历（对拍裁定记录：非 3.x 一律走兼容路径）。
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil
	}
	out := make(map[string]string)
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			httpMethod := strings.ToUpper(method)
			name := op.OperationID
			if name == "" {
				name = pathToMethodName(path, httpMethod)
			}
			out[name] = extractInputSchemaKin(op, root)
		}
	}
	return out
}

// extractInputSchemaKin 从 kin 解析出的 operation 推导调用方输入 schema，
// 输出形状与 extractInputSchema 完全一致：
//   - 参数全部平铺为顶层属性（$ref 内联、参数级 description/deprecated 随行、
//     无 schema 兜底 string、required 并入顶层）；
//   - application/json 请求体 object 形态合并属性与 required，其余形态挂
//     body 属性；
//   - 根恒为 {"type":"object","properties":{...}}，required 非空才带。
func extractInputSchemaKin(op *openapi3.Operation, root map[string]interface{}) string {
	properties := make(map[string]interface{})
	var required []string

	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		if param == nil || param.Name == "" {
			continue
		}
		prop := map[string]interface{}{}
		if param.Schema != nil {
			if resolved, ok := schemaRefToMap(param.Schema, root); ok {
				for key, value := range resolved {
					prop[key] = value
				}
			}
		}
		if len(prop) == 0 {
			prop["type"] = "string"
		}
		if param.Description != "" {
			prop["description"] = param.Description
		}
		if param.Deprecated {
			prop["deprecated"] = true
		}
		if param.Required {
			required = appendUniqueString(required, param.Name)
		}
		properties[param.Name] = prop
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		if media, ok := op.RequestBody.Value.Content["application/json"]; ok && media != nil && media.Schema != nil {
			if resolved, ok := schemaRefToMap(media.Schema, root); ok {
				// 与旧实现同口径：仅 string 型 "object" 走合并分支（3.1 数组
				// 形态保持 body 属性语义，不扩展）。
				if typ, _ := resolved["type"].(string); typ == "object" {
					if bodyProps, ok := resolved["properties"].(map[string]interface{}); ok {
						for key, value := range bodyProps {
							properties[key] = value
						}
					}
					if bodyRequired, ok := resolved["required"].([]interface{}); ok {
						for _, item := range bodyRequired {
							if name, ok := item.(string); ok {
								required = appendUniqueString(required, name)
							}
						}
					}
				} else {
					properties["body"] = resolved
				}
			}
		}
	}

	schema := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	// Kin MarshalYAML 的产物再经 json 编解码，Marshal 恒成功。
	encoded, _ := json.Marshal(schema)
	return string(encoded)
}

// schemaRefToMap 把 kin SchemaRef 序列化回通用 map，再复用既有 $ref 内联
// 机器：kin 对内部引用回吐 {"$ref":"#/..."}（悬空/外链在 loader 层已失败、
// 走回落），resolveSchemaRefs 对根文档内联后与旧实现的输出逐键同形。
func schemaRefToMap(ref *openapi3.SchemaRef, root map[string]interface{}) (map[string]interface{}, bool) {
	encoded, err := json.Marshal(ref)
	if err != nil {
		return nil, false
	}
	var raw interface{}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return nil, false
	}
	resolved, ok := resolveSchemaRefs(raw, root, 0).(map[string]interface{})
	return resolved, ok
}
