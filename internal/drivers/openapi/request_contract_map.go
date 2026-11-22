package openapi

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// map 前端：从整份文档的 map 表示提取统一请求契约。服务两类调用方：
//   - 驱动的兼容回落路径（kin 拒载：Swagger 2.0、悬空 $ref 等）；
//   - 对拍测试的旧实现侧。
// 3.x 文档与 kin 前端产出相同契约（对拍锁定）；Swagger 2.0 仅走本前端：
// `in: body` 参数成为 body 槽位，`in: formData` 参数被提升进隐式 object
// body 属性（规范见 docs/architecture/openapi-request-contract.md）。

// ExtractOperationRequestContractMap 从 map 表示提取一个 operation 的统一
// 请求契约。pathItemParams 是 path-item 级 parameters 原始列表（operation
// 级参数按 (in, name) 覆盖同名 path-item 级参数）。
func ExtractOperationRequestContractMap(
	methodObj map[string]interface{},
	root map[string]interface{},
	pathItemParams []interface{},
) spec.OperationRequestContract {
	contract := spec.OperationRequestContract{}
	if methodObj == nil {
		return contract
	}
	seenParam := map[string]int{}
	issues := []refIssue{}
	appendMapParameterList(&contract, methodObj["parameters"], root, seenParam, &issues)
	appendMapParameterList(&contract, pathItemParams, root, seenParam, &issues)

	if body, reason := extractMapRequestBody(methodObj, root, &issues); body != nil {
		contract.Body = body
	} else if reason != "" {
		// 声明了 body schema 但无一媒体类型可执行（SUI-007）：记录原因，
		// 不落入 2.0 回落（3.x requestBody 已存在）。
		contract.Diagnostics = append(contract.Diagnostics, spec.RequestBodyMediaDiagnostic(reason))
	} else if body, reason := extractSwagger20Body(methodObj, root, &issues); body != nil {
		contract.Body = body
	} else if reason != "" {
		// Swagger 2.0：consumes 声明了条目但无一可执行（SUI-007）——记录原因。
		contract.Diagnostics = append(contract.Diagnostics, spec.RequestBodyMediaDiagnostic(reason))
	}
	contract.Diagnostics = append(contract.Diagnostics, refIssuesToDiagnostics(issues)...)
	contract.Diagnostics = append(contract.Diagnostics, requestContractDiagnostics(contract)...)
	// SUI-009：与 kin 前端同机的方言归一（见 request_contract.go 注释）。
	return spec.NormalizeRequestContractDialect(contract, spec.DetectSchemaDialect(root))
}

// appendMapParameterList 把一份 parameters 列表并入契约（仅固定位置参数；
// Swagger 2.0 的 body/formData 参数由 extractSwagger20Body 处理）。
func appendMapParameterList(
	contract *spec.OperationRequestContract,
	list interface{},
	root map[string]interface{},
	seenParam map[string]int,
	issues *[]refIssue,
) {
	paramList, ok := list.([]interface{})
	if !ok {
		return
	}
	for _, param := range paramList {
		pObj, ok := param.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := pObj["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		inRaw, _ := pObj["in"].(string)
		position := spec.ParameterPosition(strings.TrimSpace(inRaw))
		if !spec.IsValidParameterPosition(position) {
			continue // body/formData/未知位置由 body 提取或调用方处理
		}
		key := string(position) + ":" + name
		if _, ok := seenParam[key]; ok {
			// operation 级先处理并占用槽位；同名 path-item 参数被覆盖跳过
			// （与 kin 前端一致，保证对拍同形）。
			continue
		}
		seenParam[key] = len(contract.Parameters)
		contract.Parameters = append(contract.Parameters, mapParameterSpec(pObj, root, position, name, issues))
	}
}

func mapParameterSpec(
	pObj map[string]interface{},
	root map[string]interface{},
	position spec.ParameterPosition,
	name string,
	issues *[]refIssue,
) spec.RequestParameterSpec {
	required, _ := pObj["required"].(bool)
	deprecated, _ := pObj["deprecated"].(bool)
	description, _ := pObj["description"].(string)
	style, _ := pObj["style"].(string)
	explode := false
	if raw, ok := pObj["explode"].(bool); ok {
		explode = raw
	} else if style == "" {
		// OpenAPI 默认：query/cookie form explode=true，其余 simple explode=false
		explode = position == spec.ParameterPositionQuery || position == spec.ParameterPositionCookie
	}
	schemaText := "{\"type\":\"string\"}"
	if schema, ok := pObj["schema"].(map[string]interface{}); ok {
		if resolved, ok := resolveSchemaRefs(schema, root, 0, issues).(map[string]interface{}); ok && resolved != nil {
			if encoded, err := json.Marshal(resolved); err == nil {
				schemaText = string(encoded)
			}
		}
	}
	return spec.RequestParameterSpec{
		Name:          name,
		In:            position,
		Required:      required,
		Deprecated:    deprecated,
		Description:   strings.TrimSpace(description),
		Serialization: defaultRequestSerialization(position, style, explode),
		Schema:        spec.JSONSchema(schemaText),
	}
}

// extractMapRequestBody 提取 3.x requestBody 的 body 槽位。媒体类型按执行器
// 支持能力选择（SUI-007，与 kin 前端同源）：无可执行媒体类型时返回
// (nil, reason)，由调用方记入提案诊断。声明了 requestBody 但无可用
// schema（条目缺 schema）返回 (nil, "")，保持旧静默口径。无法内联的
// schema 引用记入 issues（SUI-008）。
func extractMapRequestBody(methodObj, root map[string]interface{}, issues *[]refIssue) (*spec.RequestBodySpec, string) {
	requestBody, ok := methodObj["requestBody"].(map[string]interface{})
	if !ok {
		return nil, ""
	}
	content, ok := requestBody["content"].(map[string]interface{})
	if !ok {
		return nil, ""
	}
	schemas := make(map[string]spec.JSONSchema, len(content))
	for mediaType, entry := range content {
		mediaObj, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		schema, ok := mediaObj["schema"].(map[string]interface{})
		if !ok {
			continue
		}
		resolved, ok := resolveSchemaRefs(schema, root, 0, issues).(map[string]interface{})
		if !ok || resolved == nil {
			continue
		}
		encoded, err := json.Marshal(resolved)
		if err != nil {
			continue
		}
		schemas[mediaType] = spec.JSONSchema(encoded)
	}
	mediaType, schemaText, reason := spec.SelectRequestBodyMediaType(schemas)
	if mediaType == "" {
		if len(schemas) > 0 {
			return nil, reason
		}
		return nil, ""
	}
	required, _ := requestBody["required"].(bool)
	return &spec.RequestBodySpec{
		MediaType: mediaType,
		Required:  required,
		Schema:    spec.JSONSchema(schemaText),
	}, ""
}

// extractSwagger20Body 合成 Swagger 2.0 的 body 槽位：`in: body` 参数给出
// schema，`in: formData` 参数提升为隐式 object body 属性；媒体类型取
// method 级或文档级 `consumes` 的第一个可执行条目（缺省 application/json，
// 执行器能力选择见 swagger20MediaType）。无可执行媒体类型时返回
// (nil, reason)；两者都缺时返回 (nil, "")。
func extractSwagger20Body(methodObj, root map[string]interface{}, issues *[]refIssue) (*spec.RequestBodySpec, string) {
	var bodyParam map[string]interface{}
	formProps := map[string]interface{}{}
	var formRequired []string
	paramList, _ := methodObj["parameters"].([]interface{})
	for _, param := range paramList {
		pObj, ok := param.(map[string]interface{})
		if !ok {
			continue
		}
		in, _ := pObj["in"].(string)
		switch strings.TrimSpace(in) {
		case "body":
			if bodyParam == nil {
				bodyParam = pObj
			}
		case "formData":
			name, _ := pObj["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			formProps[name] = swagger20Property(pObj)
			if required, _ := pObj["required"].(bool); required {
				formRequired = appendUniqueString(formRequired, name)
			}
		}
	}
	if bodyParam == nil && len(formProps) == 0 {
		return nil, ""
	}

	var schema map[string]interface{}
	required := false
	if bodyParam != nil {
		if s, ok := bodyParam["schema"].(map[string]interface{}); ok {
			if resolved, ok := resolveSchemaRefs(s, root, 0, issues).(map[string]interface{}); ok && resolved != nil {
				schema = resolved
			}
		}
		required, _ = bodyParam["required"].(bool)
	}
	if schema == nil {
		schema = map[string]interface{}{"type": "object"}
	}
	// formData 提升进 object 属性（与显式 body schema 的同名属性冲突时
	// 以显式 body schema 为准——2.0 规范二者互斥，此处仅为容错）。
	if len(formProps) > 0 {
		if schema["type"] == nil {
			schema["type"] = "object"
		}
		props, _ := schema["properties"].(map[string]interface{})
		if props == nil {
			props = map[string]interface{}{}
		}
		for name, prop := range formProps {
			if _, exists := props[name]; !exists {
				props[name] = prop
			}
		}
		schema["properties"] = props
		for _, name := range formRequired {
			list, _ := schema["required"].([]interface{})
			schema["required"] = appendInterfaceUnique(list, name)
		}
	}

	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, ""
	}
	mediaType, reason := swagger20MediaType(methodObj, root)
	if mediaType == "" {
		return nil, reason
	}
	return &spec.RequestBodySpec{
		MediaType: mediaType,
		Required:  required,
		Schema:    spec.JSONSchema(encoded),
	}, ""
}

// swagger20Property 把 2.0 参数级字段（type/format/enum/default/items 等）
// 组装为 body object 属性 schema。
func swagger20Property(pObj map[string]interface{}) map[string]interface{} {
	prop := map[string]interface{}{}
	for _, key := range []string{"type", "format", "description", "default", "items", "enum", "$ref", "minimum", "maximum", "maxLength", "minLength", "pattern"} {
		if value, ok := pObj[key]; ok {
			prop[key] = value
		}
	}
	if len(prop) == 0 {
		prop["type"] = "string"
	}
	return prop
}

// swagger20MediaType 选 2.0 可执行媒体类型（SUI-007）：method 级 consumes
// 优先、文档级兜底、缺省 application/json；候选内取第一个可执行条目（2.0
// consumes 语义=声明顺序）。声明了 consumes 但无一可执行时返回 ("", reason)。
func swagger20MediaType(methodObj, root map[string]interface{}) (string, string) {
	for _, source := range []map[string]interface{}{methodObj, root} {
		if source == nil {
			continue
		}
		list, _ := source["consumes"].([]interface{})
		declared := make([]string, 0, len(list))
		for _, entry := range list {
			if s, ok := entry.(string); ok && strings.TrimSpace(s) != "" {
				declared = append(declared, strings.TrimSpace(s))
			}
		}
		if len(declared) == 0 {
			continue // 该级未声明，落到文档级/缺省
		}
		mediaType, reason := spec.SelectExecutableMediaType(declared)
		if mediaType != "" {
			return mediaType, ""
		}
		return "", reason
	}
	return "application/json", ""
}

func appendInterfaceUnique(list []interface{}, value string) []interface{} {
	for _, existing := range list {
		if s, ok := existing.(string); ok && s == value {
			return list
		}
	}
	return append(list, value)
}
