package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
)

// Call invokes an API method.
func (p *Provider) Call(ctx context.Context, method string, request []byte) ([]byte, error) {
	// Check rate limit
	if p.rateLimiter != nil {
		if err := p.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	p.mu.RLock()
	apiMethod, exists := p.methodMap[method]
	if !exists {
		// 运行时 function ID 注册与调用侧统一折叠为小写（control_handler 折叠），
		// 而 methodMap 以 OpenAPI operationId 原样（可能含驼峰）为键。先精确命中，
		// 未命中再按大小写不敏感解析，保证 camelCase operationId 经折叠调用后
		// 仍可路由。
		for name, candidate := range p.methodMap {
			if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(method)) {
				apiMethod, exists = candidate, true
				break
			}
		}
	}
	p.mu.RUnlock()

	if !exists {
		return nil, &provider.MethodNotSupportedError{Provider: p.Name(), Method: method}
	}

	if !p.IsEnabled() {
		return nil, &provider.ProviderDisabledError{Name: p.Name()}
	}

	// Parse request
	var payload interface{}
	if len(request) > 0 {
		if err := json.Unmarshal(request, &payload); err != nil {
			return nil, fmt.Errorf("invalid request JSON: %w", err)
		}
	}
	in := p.resolveCallInput(apiMethod, payload)

	// Build HTTP request
	httpReq, err := p.buildRequest(ctx, apiMethod, in)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Add authentication
	p.addAuth(httpReq)

	// Add default headers
	for k, v := range p.openapiConfig.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute request with retry。退避与 agent 侧统一走 cenkalti/backoff/v5：
	// 原线性 (i+1)s 恒定 1s——RetryCount 通常个位数，线性增长的尾次等待对失败
	// 恢复帮助有限；指数化留待后续单独评估（能力矩阵 #14）。
	var resp *http.Response
	retryWait := backoff.NewConstantBackOff(time.Second)
	for i := 0; i <= p.openapiConfig.RetryCount; i++ {
		resp, err = p.httpClient.Do(httpReq)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		// C-025：仅在实际重试时提前关 body（此时响应体作废）。末次（i ==
		// RetryCount）不关——留给下方 defer + io.ReadAll，保留真实 HTTP
		// 状态与错误体；否则读已关 body 会以 read-on-closed 掩盖真实错误。
		if resp != nil && i < p.openapiConfig.RetryCount {
			_ = resp.Body.Close()
		}
		if i < p.openapiConfig.RetryCount {
			timer := time.NewTimer(retryWait.NextBackOff())
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s: %s", resp.Status, string(body))
	}

	// Transform response if configured
	data, err := p.transformResponse(body)
	if err != nil {
		return nil, err
	}
	// 校验已变换的返回（能力矩阵 #4，批次 C）：有 response schema 才校验，
	// 无 schema 原样放行（兼容既有 provider）；数据不符沿错误通道上报。
	if apiMethod.OutputSchema != "" {
		if err := p.validateOutput(method, apiMethod.OutputSchema, data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

// resolvedCallInput 是调用方 payload 按请求契约投影模式解析后的结果：
//   - params：寻址参数（path/query/header 取值来源）；
//   - body/hasBody：body 槽位的原始 JSON 值（根数组/标量/对象原样发送）；
//   - bodyFromSlot：body 由契约槽位显式给出——structured 无 body 槽时抑制
//     旧的 filterConsumedParams 兜底（参数已分离，不能再把 params 当 body）。
type resolvedCallInput struct {
	params       map[string]interface{}
	body         interface{}
	hasBody      bool
	bodyFromSlot bool
}

// resolveCallInput 依据请求契约投影模式解析调用方 payload（SUI-005）：
//   - body_only：payload 本身即 body（数组/标量原样），无寻址参数；
//   - structured：{params, body} 命名空间拆分，body 原样发送；
//   - flat / 无契约：沿用旧 map 语义（reqData 即参数面，body 走
//     RequestBodyMapping 或过滤兜底）。
func (p *Provider) resolveCallInput(apiMethod *APIMethod, payload interface{}) resolvedCallInput {
	reqData, _ := payload.(map[string]interface{})
	out := resolvedCallInput{params: reqData}
	if apiMethod == nil || apiMethod.RequestContract == nil {
		return out
	}
	_, mode := ProjectOperationInputSchema(*apiMethod.RequestContract)
	switch mode {
	case spec.InputProjectionBodyOnly:
		return resolvedCallInput{params: map[string]interface{}{}, body: payload, hasBody: true, bodyFromSlot: true}
	case spec.InputProjectionStructured:
		params, _ := reqData["params"].(map[string]interface{})
		if params == nil {
			params = map[string]interface{}{}
		}
		body, hasBody := reqData["body"]
		return resolvedCallInput{params: params, body: body, hasBody: hasBody, bodyFromSlot: true}
	default:
		return out
	}
}

// paramWireSet 是一次方法调用所有参数的线上产物（SUI-006）：path 段替换、
// query 编码对、header 与 cookie 值。
type paramWireSet struct {
	pathSegments map[string]string // "{" + name + "}" -> 已编码的段内容
	query        []queryPair
	headers      []nameValue
	cookies      []nameValue
}

// nameValue 是一个 header / cookie 名称与值的对（header 值原样，cookie 值
// 已 URL 编码）。
type nameValue struct {
	name, value string
}

func (w *paramWireSet) merge(in spec.ParameterPosition, name string, wire paramWire) {
	switch in {
	case spec.ParameterPositionPath:
		w.pathSegments["{"+name+"}"] = wire.pathSegment
	case spec.ParameterPositionQuery:
		w.query = append(w.query, wire.queryPairs...)
	case spec.ParameterPositionHeader:
		if wire.present {
			w.headers = append(w.headers, nameValue{name: name, value: wire.headerValue})
		}
	case spec.ParameterPositionCookie:
		if wire.present {
			w.cookies = append(w.cookies, nameValue{name: name, value: wire.cookieValue})
		}
	}
}

// serializeParams 把调用方入参按位置的 style/explode 序列化为线上形态
// （SUI-006）。契约参数优先于 legacy ParameterMapping；无法执行的组合返回
// error，由 buildRequest 传播。
func (p *Provider) serializeParams(apiMethod *APIMethod, reqData map[string]interface{}) (paramWireSet, error) {
	set := paramWireSet{pathSegments: map[string]string{}}
	if apiMethod == nil {
		return set, nil
	}
	if apiMethod.RequestContract != nil {
		for _, cparam := range apiMethod.RequestContract.Parameters {
			value, present := p.resolveContractParamValue(cparam, reqData)
			placeholder := "{" + cparam.Name + "}"
			if !present {
				// 与 legacy 路径一致：缺失的 path 参数替换为空串，不泄漏占位符。
				if cparam.In == spec.ParameterPositionPath {
					set.pathSegments[placeholder] = ""
				}
				continue
			}
			wire, err := serializeParamValue(cparam, value)
			if err != nil {
				return set, err
			}
			if cparam.In == spec.ParameterPositionPath {
				set.pathSegments[placeholder] = wire.pathSegment
			} else {
				set.merge(cparam.In, cparam.Name, wire)
			}
		}
		return set, nil
	}
	for _, param := range apiMethod.Parameters {
		if param.In == "" {
			continue
		}
		in := spec.ParameterPosition(strings.TrimSpace(param.In))
		value, present := p.resolveLegacyParamValue(reqData, param)
		if !present {
			if in == spec.ParameterPositionPath {
				set.pathSegments["{"+param.Name+"}"] = ""
			}
			continue
		}
		cparam := spec.RequestParameterSpec{
			Name: param.Name,
			In:   in,
			Serialization: spec.RequestSerialization{
				Style:   spec.DefaultStyleForPosition(in),
				Explode: in == spec.ParameterPositionQuery || in == spec.ParameterPositionCookie,
			},
		}
		wire, err := serializeParamValue(cparam, value)
		if err != nil {
			return set, err
		}
		if in == spec.ParameterPositionPath {
			set.pathSegments["{"+param.Name+"}"] = wire.pathSegment
		} else {
			set.merge(in, param.Name, wire)
		}
	}
	return set, nil
}

// resolveContractParamValue 解析契约参数的值：调用方键名即 OpenAPI 参数名，
// 缺失时回退 schema 的 default。空串值按缺失处理（与 legacy 行为一致）。
func (p *Provider) resolveContractParamValue(cparam spec.RequestParameterSpec, reqData map[string]interface{}) (any, bool) {
	if reqData != nil {
		if v, ok := reqData[cparam.Name]; ok && v != nil {
			if s, isStr := v.(string); isStr && s == "" {
				return nil, false
			}
			return v, true
		}
	}
	if v, ok := spec.SchemaDefault(cparam.Schema); ok {
		return v, true
	}
	return nil, false
}

// resolveLegacyParamValue 解析 config 声明参数的值（From 重映射 + Default）。
func (p *Provider) resolveLegacyParamValue(reqData map[string]interface{}, param ParameterMapping) (any, bool) {
	from := param.From
	if from == "" {
		from = param.Name
	}
	if v, ok := reqData[from]; ok && v != nil {
		if s, isStr := v.(string); isStr && s == "" {
			return nil, false
		}
		return v, true
	}
	if param.Default != nil {
		return param.Default, true
	}
	return nil, false
}

// consumedKeysFromMethod 返回方法寻址参数占用的调用方键名（contract 与 legacy
// 两种来源），供 body 过滤排除寻址参数。
func consumedKeysFromMethod(apiMethod *APIMethod) []string {
	if apiMethod == nil {
		return nil
	}
	if apiMethod.RequestContract != nil {
		keys := make([]string, 0, len(apiMethod.RequestContract.Parameters))
		for _, p := range apiMethod.RequestContract.Parameters {
			if p.Name != "" {
				keys = append(keys, p.Name)
			}
		}
		return keys
	}
	keys := make([]string, 0, len(apiMethod.Parameters))
	for _, p := range apiMethod.Parameters {
		from := p.From
		if from == "" {
			from = p.Name
		}
		if from != "" {
			keys = append(keys, from)
		}
	}
	return keys
}

// buildRequest builds an HTTP request from the API method and resolved input.
func (p *Provider) buildRequest(ctx context.Context, apiMethod *APIMethod, in resolvedCallInput) (*http.Request, error) {
	reqData := in.params

	// 参数序列化（SUI-006）：path/query/header/cookie 统一经 style/explode 规则。
	wire, err := p.serializeParams(apiMethod, reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters: %w", err)
	}
	url := p.urlFromWire(apiMethod, wire)

	var body io.Reader
	var contentType string

	// Contract-driven body slot wins: root arrays/scalars/namespaced bodies
	// are sent verbatim (SUI-005), regardless of HTTP method.
	if in.hasBody {
		data, err := json.Marshal(in.body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	} else if apiMethod.Method == "POST" || apiMethod.Method == "PUT" || apiMethod.Method == "PATCH" {
		if apiMethod.RequestBody != nil {
			bodyBytes, ct := p.buildBody(apiMethod.RequestBody, reqData)
			body = bytes.NewReader(bodyBytes)
			contentType = ct
		} else if len(reqData) > 0 && !in.bodyFromSlot {
			// OpenAPI semantics: parameters consumed by path/query/header
			// addressing must not leak into the request body.
			data, _ := json.Marshal(filterConsumedParamsByKeys(consumedKeysFromMethod(apiMethod), reqData))
			body = bytes.NewReader(data)
			contentType = "application/json"
		}
	}

	req, err := http.NewRequestWithContext(ctx, apiMethod.Method, url, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// 序列化后的 header 参数（simple 样式）。
	for _, h := range wire.headers {
		req.Header.Set(h.name, h.value)
	}

	// cookie 位置参数（form 样式，仅原始值）——旧实现完全忽略该位置。
	if len(wire.cookies) > 0 {
		parts := make([]string, 0, len(wire.cookies))
		for _, c := range wire.cookies {
			parts = append(parts, c.name+"="+c.value)
		}
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}

	return req, nil
}

// buildURL builds the full URL with path parameters and query string.
func (p *Provider) buildURL(apiMethod *APIMethod, reqData map[string]interface{}) string {
	wire, err := p.serializeParams(apiMethod, reqData)
	if err != nil {
		// 序列化失败由 buildRequest 传播；此处保持可调用（直调测试面）。
		wire = paramWireSet{pathSegments: map[string]string{}}
	}
	return p.urlFromWire(apiMethod, wire)
}

// urlFromWire 把线上产物拼成完整 URL：path 段替换（已百分号编码）+ query
// 键值对（已 URL 编码，按声明顺序）。
func (p *Provider) urlFromWire(apiMethod *APIMethod, wire paramWireSet) string {
	baseURL := strings.TrimSuffix(p.openapiConfig.BaseURL, "/")
	apiPath := apiMethod.Path
	for placeholder, seg := range wire.pathSegments {
		apiPath = strings.ReplaceAll(apiPath, placeholder, seg)
	}

	u := baseURL + apiPath
	if len(wire.query) > 0 {
		parts := make([]string, 0, len(wire.query))
		for _, q := range wire.query {
			parts = append(parts, q.name+"="+q.value)
		}
		u += "?" + strings.Join(parts, "&")
	}

	return u
}

// buildBody builds the request body.
func (p *Provider) buildBody(bodyConfig *RequestBodyMapping, reqData map[string]interface{}) ([]byte, string) {
	if bodyConfig.Type == "json" || bodyConfig.Type == "" {
		if bodyConfig.Template != "" {
			// Use template
			result := p.applyTemplate(bodyConfig.Template, reqData)
			return []byte(result), "application/json"
		}
		// Use field mapping
		result := make(map[string]interface{})
		for dstField, srcField := range bodyConfig.Fields {
			if val, ok := reqData[srcField]; ok {
				result[dstField] = val
			}
		}
		data, _ := json.Marshal(result)
		return data, "application/json"
	}
	return nil, ""
}

// applyTemplate applies a simple template with {{ .field }} placeholders.
func (p *Provider) applyTemplate(template string, data map[string]interface{}) string {
	result := template
	for key, val := range data {
		placeholder := fmt.Sprintf("{{ .%s }}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", val))
	}
	return result
}

// getParamValue gets a parameter value from request data.
func (p *Provider) getParamValue(reqData map[string]interface{}, param ParameterMapping) string {
	from := param.From
	if from == "" {
		from = param.Name
	}

	if val, ok := reqData[from]; ok && val != nil {
		return fmt.Sprintf("%v", val)
	}

	if param.Default != nil {
		return fmt.Sprintf("%v", param.Default)
	}

	return ""
}

// filterConsumedParams returns a copy of reqData without fields already
// consumed by path/query/header parameter mappings, so auto-discovered
// methods without an explicit request body mapping do not leak addressing
// parameters into the JSON body.
func filterConsumedParams(parameters []ParameterMapping, reqData map[string]interface{}) map[string]interface{} {
	keys := make([]string, 0, len(parameters))
	for _, param := range parameters {
		from := param.From
		if from == "" {
			from = param.Name
		}
		if from != "" {
			keys = append(keys, from)
		}
	}
	return filterConsumedParamsByKeys(keys, reqData)
}

// filterConsumedParamsByKeys is the key-generic form used by the executor
// (contract parameters use the OpenAPI name as the caller key, legacy
// mappings use From).
func filterConsumedParamsByKeys(keys []string, reqData map[string]interface{}) map[string]interface{} {
	if len(reqData) == 0 {
		return reqData
	}
	consumed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key != "" {
			consumed[key] = struct{}{}
		}
	}
	if len(consumed) == 0 {
		return reqData
	}
	filtered := make(map[string]interface{}, len(reqData))
	for key, value := range reqData {
		if _, ok := consumed[key]; ok {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

// addAuth adds authentication to the request.
func (p *Provider) addAuth(req *http.Request) {
	if p.openapiConfig.Auth == nil {
		return
	}

	switch p.openapiConfig.Auth.Type {
	case "bearer":
		if p.openapiConfig.Auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+p.openapiConfig.Auth.Token)
		}
	case "basic":
		if p.openapiConfig.Auth.Username != "" || p.openapiConfig.Auth.Password != "" {
			req.SetBasicAuth(p.openapiConfig.Auth.Username, p.openapiConfig.Auth.Password)
		}
	case "api_key":
		if p.openapiConfig.Auth.APIKey != nil {
			if p.openapiConfig.Auth.APIKey.In == "header" {
				req.Header.Set(p.openapiConfig.Auth.APIKey.Name, p.openapiConfig.Auth.APIKey.Value)
			}
		}
	case "custom":
		for k, v := range p.openapiConfig.Auth.CustomHeaders {
			req.Header.Set(k, v)
		}
	}
}

// transformResponse transforms the response based on configuration.
func (p *Provider) transformResponse(body []byte) ([]byte, error) {
	if p.openapiConfig.Transform == nil {
		return body, nil
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body, nil // Return as-is if not JSON
	}

	transformed := make(map[string]interface{})

	// Check success field
	if p.openapiConfig.Transform.SuccessField != "" {
		if successVal, ok := resp[p.openapiConfig.Transform.SuccessField]; ok {
			transformed["success"] = successVal == p.openapiConfig.Transform.SuccessValue
		}
	}

	// Extract data field
	if p.openapiConfig.Transform.DataField != "" {
		if dataVal, ok := resp[p.openapiConfig.Transform.DataField]; ok {
			transformed["data"] = dataVal
		}
	} else {
		transformed["data"] = resp
	}

	// Extract error field
	if p.openapiConfig.Transform.ErrorField != "" {
		if errVal, ok := resp[p.openapiConfig.Transform.ErrorField]; ok {
			transformed["error"] = errVal
		}
	}

	return json.Marshal(transformed)
}

// Close closes the provider.
func (p *Provider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
