package openapi

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/getkin/kin-openapi/openapi3"
)

// Kin 前端与 map 前端共享的请求契约提取与投影。
// 契约类型：internal/dashboard/spec/request_contract.go；
// 规范：docs/architecture/openapi-request-contract.md。

// ExtractOperationRequestContractKin 从 kin-openapi 解析出的 operation 提取
// 统一请求契约（SUI-002）。pathItem 级参数并入（operation 级按 (in,name) 覆盖）。
// root 是整份文档的 map 表示，用于 $ref 内联（与 map 前端同机器，保证对拍同形）。
func ExtractOperationRequestContractKin(
	op *openapi3.Operation,
	pathItemParams []*openapi3.ParameterRef,
	root map[string]interface{},
) spec.OperationRequestContract {
	contract := spec.OperationRequestContract{}
	if op == nil {
		return contract
	}
	seenParam := map[string]int{} // "in:name" -> index into Parameters
	for _, paramRef := range op.Parameters {
		appendKinParameter(&contract, paramRef, root, seenParam)
	}
	for _, paramRef := range pathItemParams {
		appendKinParameter(&contract, paramRef, root, seenParam)
	}
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		if mediaType, schemaText, ok := firstKinMediaSchema(op.RequestBody.Value.Content, root); ok {
			contract.Body = &spec.RequestBodySpec{
				MediaType: mediaType,
				Required:  op.RequestBody.Value.Required,
				Schema:    spec.JSONSchema(schemaText),
			}
		}
	}
	contract.Diagnostics = append(contract.Diagnostics, requestContractDiagnostics(contract)...)
	return contract
}

func appendKinParameter(
	contract *spec.OperationRequestContract,
	paramRef *openapi3.ParameterRef,
	root map[string]interface{},
	seenParam map[string]int,
) {
	if paramRef == nil || paramRef.Value == nil {
		return
	}
	param := paramRef.Value
	name := strings.TrimSpace(param.Name)
	if name == "" {
		return
	}
	position := spec.ParameterPosition(strings.TrimSpace(param.In))
	if !spec.IsValidParameterPosition(position) {
		return // unknown position is surfaced by the caller's scan layer
	}
	key := string(position) + ":" + name
	if _, ok := seenParam[key]; ok {
		// operation 级参数先于 path-item 级处理，已占用槽位；同名 path-item
		// 参数被 operation 级覆盖（不再替换/追加，保持声明顺序稳定）。
		return
	}
	seenParam[key] = len(contract.Parameters)
	contract.Parameters = append(contract.Parameters, kinParameterSpec(param, root))
}

func kinParameterSpec(param *openapi3.Parameter, root map[string]interface{}) spec.RequestParameterSpec {
	schemaText := "{\"type\":\"string\"}"
	if param.Schema != nil {
		schemaText = kinSchemaText(param.Schema, root)
	}
	style := strings.TrimSpace(param.Style)
	explode := false
	if param.Explode != nil {
		explode = *param.Explode
	} else if style == "" {
		// OpenAPI 默认：query/cookie → form（explode=true），其余 simple
		// （explode=false）——与 map 前端 mapParameterSpec 逐位一致。
		explode = param.In == string(spec.ParameterPositionQuery) || param.In == string(spec.ParameterPositionCookie)
	}
	return spec.RequestParameterSpec{
		Name:        strings.TrimSpace(param.Name),
		In:          spec.ParameterPosition(strings.TrimSpace(param.In)),
		Required:    param.Required,
		Deprecated:  param.Deprecated,
		Description: strings.TrimSpace(param.Description),
		Serialization: defaultRequestSerialization(
			spec.ParameterPosition(strings.TrimSpace(param.In)),
			strings.TrimSpace(param.Style),
			explode,
		),
		Schema: spec.JSONSchema(schemaText),
	}
}

func firstKinMediaSchema(content openapi3.Content, root map[string]interface{}) (string, string, bool) {
	if len(content) == 0 {
		return "", "", false
	}
	ordered := make([]string, 0, len(content))
	for key := range content {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	// 优先 application/json；否则取排序后第一个 content 条目（确定性）。
	preferred := "application/json"
	if media, ok := content[preferred]; ok {
		if text, ok := kinMediaSchemaText(media, root); ok {
			return preferred, text, true
		}
	}
	for _, key := range ordered {
		if key == preferred {
			continue
		}
		if media, ok := content[key]; ok {
			if text, ok := kinMediaSchemaText(media, root); ok {
				return key, text, true
			}
		}
	}
	return "", "", false
}

func kinMediaSchemaText(media *openapi3.MediaType, root map[string]interface{}) (string, bool) {
	if media == nil || media.Schema == nil {
		return "", false
	}
	if isDegenerateSchemaRef(media.Schema) {
		return "", false
	}
	return kinSchemaText(media.Schema, root), true
}

// isDegenerateSchemaRef 报告既无指针又无值的 SchemaRef——kin 的 MarshalJSON
// 对该形态会 nil 解引用，契约侧按「未声明 schema」处理。
func isDegenerateSchemaRef(ref *openapi3.SchemaRef) bool {
	return ref != nil && strings.TrimSpace(ref.Ref) == "" && ref.Value == nil
}

// kinSchemaText 把 kin SchemaRef 序列化为 schema JSON 文本：优先复用
// schemaRefToMap（$ref 经根文档内联，与 map 前端逐键同形）；root 为 nil 时
// （直调构造 op 的测试路径）用 kin 自身解析出的 Value——两种序列化均为
// 键序归一的等价 JSON。
func kinSchemaText(ref *openapi3.SchemaRef, root map[string]interface{}) string {
	if ref == nil || isDegenerateSchemaRef(ref) {
		return "{\"type\":\"string\"}"
	}
	if resolved, ok := schemaRefToMap(ref, root); ok {
		if encoded, err := json.Marshal(resolved); err == nil {
			return string(encoded)
		}
	}
	if ref.Value != nil {
		if encoded, err := json.Marshal(ref.Value); err == nil {
			return string(encoded)
		}
	}
	return "{\"type\":\"string\"}"
}

// defaultRequestSerialization 按位置回退 OpenAPI 默认 style/explode。
func defaultRequestSerialization(position spec.ParameterPosition, style string, explode bool) spec.RequestSerialization {
	if style == "" {
		switch position {
		case spec.ParameterPositionQuery, spec.ParameterPositionCookie:
			style = "form"
		default:
			style = "simple"
		}
	}
	return spec.RequestSerialization{Style: style, Explode: explode}
}

// crossPositionParamCollision 报告是否存在同名参数跨位置出现（path id +
// query id 等）——平铺投影只能表达一个 id，必须结构化以无损保留两者。
func crossPositionParamCollision(params []spec.RequestParameterSpec) bool {
	firstPos := map[string]string{}
	for _, p := range params {
		if pos, ok := firstPos[p.Name]; ok && pos != string(p.In) {
			return true
		}
		if _, ok := firstPos[p.Name]; !ok {
			firstPos[p.Name] = string(p.In)
		}
	}
	return false
}

// requestContractDiagnostics 在契约提取后计算不可平铺/冲突诊断（SUI-004）。
// 仅在平铺投影会丢信息时输出：无参数时 body_only 模式无损，不发诊断。
func requestContractDiagnostics(contract spec.OperationRequestContract) []spec.Diagnostic {
	var diags []spec.Diagnostic
	paramByName := map[string]string{} // name -> position of first occurrence
	seen := map[string]bool{}
	for _, p := range contract.Parameters {
		if firstPos, ok := paramByName[p.Name]; ok && firstPos != string(p.In) {
			diags = append(diags, spec.Diagnostic{
				Code:     spec.DiagRequestParameterCollision,
				Severity: spec.SeverityWarning,
				Field:    p.Name,
				Message:  "parameter name " + p.Name + " appears in positions " + firstPos + " and " + string(p.In),
			})
		}
		if !seen[p.Name] {
			paramByName[p.Name] = string(p.In)
			seen[p.Name] = true
		}
	}
	if contract.Body == nil || len(contract.Parameters) == 0 {
		return diags
	}
	// 参数名与 body 属性名冲突：这是平铺不可行的独立原因，输出冲突诊断。
	bodyProps := objectPropertiesOf(contract.Body.Schema)
	for _, p := range contract.Parameters {
		if _, ok := bodyProps[p.Name]; ok {
			diags = append(diags, spec.Diagnostic{
				Code:     spec.DiagRequestParameterCollision,
				Severity: spec.SeverityWarning,
				Field:    p.Name,
				Message:  "parameter " + p.Name + " collides with request-body property of the same name",
			})
		}
	}
	// body 根形状不可平铺（根级约束/非 object/无 properties）时单独输出形状诊断。
	if !bodyRootFlatPreservable(contract.Body) {
		diags = append(diags, spec.Diagnostic{
			Code:     spec.DiagRequestBodyShapeNonFlat,
			Severity: spec.SeverityWarning,
			Field:    "body",
			Message:  "request body root cannot be lifted into the flat input shape; structured projection is used",
		})
	}
	return diags
}

// objectPropertiesOf returns the property-name set of an object schema (JSON text).
func objectPropertiesOf(schemaJSON spec.JSONSchema) map[string]struct{} {
	out := map[string]struct{}{}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &root); err != nil {
		return out
	}
	props, _ := root["properties"].(map[string]interface{})
	for name := range props {
		out[name] = struct{}{}
	}
	return out
}

// ProjectOperationInputSchema 把统一请求契约投影为操作者侧输入 schema
// （SUI-001 投影规则）。返回 (schemaJSON, mode)；空输入返回 ("", flat)。
func ProjectOperationInputSchema(contract spec.OperationRequestContract) (string, spec.InputProjectionMode) {
	params := contract.Parameters
	body := contract.Body
	if len(params) == 0 {
		if body == nil {
			return "", spec.InputProjectionFlat
		}
		// 无寻址参数且有 body：body 原样（所有根级约束保留）。
		return string(body.Schema), spec.InputProjectionBodyOnly
	}
	if body == nil {
		// 同名参数跨位置时平铺投影只能保留一个，走结构化以无损保留。
		if crossPositionParamCollision(params) {
			return structuredSchema(params, nil), spec.InputProjectionStructured
		}
		return flatObjectSchema(paramProperties(params), paramRequired(params)), spec.InputProjectionFlat
	}
	if !bodyFlatMergeable(body, params) || crossPositionParamCollision(params) {
		return structuredSchema(params, body), spec.InputProjectionStructured
	}
	props := paramProperties(params)
	required := paramRequired(params)
	mergeObjectBodyProperties(body.Schema, props, &required)
	return flatObjectSchema(props, required), spec.InputProjectionFlat
}

// bodyRootFlatPreservable 判断 body 根是否可安全提升进平铺 object（不含参数名
// 冲突检查）：根必须是无额外约束的 object（根键 ⊆ 平铺可保留集合）。
func bodyRootFlatPreservable(body *spec.RequestBodySpec) bool {
	root := parseObject(body.Schema)
	typ, _ := root["type"].(string)
	_, hasProps := root["properties"].(map[string]interface{})
	if typ != "" && typ != "object" {
		return false
	}
	if !hasProps && typ == "" {
		return false // 无 type 且无 properties：非 object 根（如 allOf/标量）
	}
	for key := range root {
		if key == "properties" || key == "required" || key == "type" ||
			key == "description" || key == "title" || key == "default" || key == "example" {
			continue
		}
		return false
	}
	return true
}

// bodyFlatMergeable 判断平铺投影能否无损表达该 body 与参数组合：
//   - body 根必须是无额外约束的 object（根键 ⊆ 平铺可保留集合）；
//   - 参数名不得与 body 属性名同名。
func bodyFlatMergeable(body *spec.RequestBodySpec, params []spec.RequestParameterSpec) bool {
	if !bodyRootFlatPreservable(body) {
		return false
	}
	root := parseObject(body.Schema)
	props, hasProps := root["properties"].(map[string]interface{})
	if !hasProps {
		return true // object 无 properties（空对象），平铺无损
	}
	propNames := make(map[string]struct{}, len(props))
	for name := range props {
		propNames[name] = struct{}{}
	}
	for _, p := range params {
		if _, ok := propNames[p.Name]; ok {
			return false
		}
	}
	return true
}

// mergeObjectBodyProperties 把 object body 的属性与 required 提升进平铺
// properties/required（旧 `openAPIRequestSchema` 的合并语义，顺序保持一致）。
func mergeObjectBodyProperties(schemaJSON spec.JSONSchema, props map[string]interface{}, required *[]string) {
	root := parseObject(schemaJSON)
	bodyProps, _ := root["properties"].(map[string]interface{})
	for name, value := range bodyProps {
		props[name] = value
	}
	if list, ok := root["required"].([]interface{}); ok {
		for _, item := range list {
			if name, ok := item.(string); ok {
				*required = appendUniqueString(*required, name)
			}
		}
	}
}

// flatObjectSchema 生成旧平铺形态的输入 schema（键序与旧实现一致：
// properties, required, type——json.Marshal 对 map 按键名排序）。
// required 去重后排序，与旧 openAPIRequestSchema 的最终形态一致。
func flatObjectSchema(props map[string]interface{}, required []string) string {
	sort.Strings(required)
	schema := map[string]interface{}{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	encoded, _ := json.Marshal(schema)
	return string(encoded)
}

// structuredSchema 生成命名空间化输入 schema（SUI-001）：params + body 槽位。
func structuredSchema(params []spec.RequestParameterSpec, body *spec.RequestBodySpec) string {
	paramProps := paramProperties(params)
	paramRequired := paramRequired(params)
	paramsObj := map[string]interface{}{"type": "object", "properties": paramProps}
	if len(paramRequired) > 0 {
		paramsObj["required"] = paramRequired
	}
	properties := map[string]interface{}{
		"params": paramsObj,
	}
	var required []string
	if len(params) > 0 {
		required = append(required, "params")
	}
	if body != nil {
		properties["body"] = parseObject(body.Schema)
		if body.Required {
			required = append(required, "body")
		}
	}
	root := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		root["required"] = required
	}
	encoded, _ := json.Marshal(root)
	return string(encoded)
}

// paramProperties 按声明顺序生成参数属性（同名参数 first-wins，冲突由诊断标注）。
func paramProperties(params []spec.RequestParameterSpec) map[string]interface{} {
	props := make(map[string]interface{}, len(params))
	for _, p := range params {
		if _, ok := props[p.Name]; ok {
			continue
		}
		var value interface{}
		if err := json.Unmarshal([]byte(p.Schema), &value); err != nil || value == nil {
			value = map[string]interface{}{"type": "string"}
		}
		props[p.Name] = value
	}
	return props
}

func paramRequired(params []spec.RequestParameterSpec) []string {
	required := make([]string, 0, len(params))
	for _, p := range params {
		if p.Required {
			required = appendUniqueString(required, p.Name)
		}
	}
	return required
}

func parseObject(schemaJSON spec.JSONSchema) map[string]interface{} {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &root); err != nil {
		return map[string]interface{}{}
	}
	if root == nil {
		return map[string]interface{}{}
	}
	return root
}
