package openapi

import (
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type requestOperation struct {
	path   string
	method string
}

// Swagger 2.0 and documents kin-openapi cannot load retain the map fallback.
func kinRequestContracts(spec []byte) map[requestOperation]*RequestContract {
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil || !strings.HasPrefix(doc.OpenAPI, "3.") || doc.Paths == nil {
		return nil
	}
	out := make(map[requestOperation]*RequestContract)
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			key := requestOperation{path: path, method: strings.ToUpper(method)}
			out[key] = ExtractRequest(path, method, item, op)
		}
	}
	return out
}

// legacyInputSchema 从统一请求契约 推导调用方输入 schema，
// 输出形状与 extractInputSchema 完全一致：
//   - 参数全部平铺为顶层属性（$ref 内联、参数级 description/deprecated 随行、
//     无 schema 兜底 string、required 并入顶层）；
//   - application/json 请求体 object 形态合并属性与 required，其余形态挂
//     body 属性；
//   - 根恒为 {"type":"object","properties":{...}}，required 非空才带。
func legacyInputSchema(request *RequestContract, root map[string]interface{}) string {
	properties := make(map[string]interface{})
	var required []string

	for _, param := range request.Parameters {
		if param.Name == "" {
			continue
		}
		prop := map[string]interface{}{}
		if param.Schema != nil {
			if resolved, ok := resolvedRequestSchema(param.Schema, root); ok {
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

	if request.Body != nil {
		if media, ok := request.Body.Content["application/json"]; ok {
			if resolved, ok := resolvedRequestSchema(media.Schema, root); ok {
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

// resolvedRequestSchema retains the existing local reference behavior of input projections.
func resolvedRequestSchema(raw json.RawMessage, root map[string]interface{}) (map[string]interface{}, bool) {
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	resolved, ok := resolveSchemaRefs(value, root, 0).(map[string]interface{})
	return resolved, ok
}
