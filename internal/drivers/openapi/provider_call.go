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
	var reqData map[string]interface{}
	if len(request) > 0 {
		if err := json.Unmarshal(request, &reqData); err != nil {
			return nil, fmt.Errorf("invalid request JSON: %w", err)
		}
	}

	// Build HTTP request
	httpReq, err := p.buildRequest(ctx, apiMethod, reqData)
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

// buildRequest builds an HTTP request from the API method and request data.
func (p *Provider) buildRequest(ctx context.Context, apiMethod *APIMethod, reqData map[string]interface{}) (*http.Request, error) {
	// Build URL with path and query parameters
	url := p.buildURL(apiMethod, reqData)

	var body io.Reader
	var contentType string

	// Build request body if needed
	if apiMethod.Method == "POST" || apiMethod.Method == "PUT" || apiMethod.Method == "PATCH" {
		if apiMethod.RequestBody != nil {
			bodyBytes, ct := p.buildBody(apiMethod.RequestBody, reqData)
			body = bytes.NewReader(bodyBytes)
			contentType = ct
		} else if len(reqData) > 0 {
			// OpenAPI semantics: parameters consumed by path/query/header
			// addressing must not leak into the request body.
			data, _ := json.Marshal(filterConsumedParams(apiMethod.Parameters, reqData))
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

	// Add parameters as headers if configured
	for _, param := range apiMethod.Parameters {
		if param.In == "header" {
			val := p.getParamValue(reqData, param)
			if val != "" {
				req.Header.Set(param.Name, val)
			}
		}
	}

	return req, nil
}

// buildURL builds the full URL with path parameters and query string.
func (p *Provider) buildURL(apiMethod *APIMethod, reqData map[string]interface{}) string {
	baseURL := strings.TrimSuffix(p.openapiConfig.BaseURL, "/")
	apiPath := apiMethod.Path

	// Replace path parameters
	for _, param := range apiMethod.Parameters {
		if param.In == "path" {
			val := p.getParamValue(reqData, param)
			placeholder := "{" + param.Name + "}"
			apiPath = strings.ReplaceAll(apiPath, placeholder, val)
		}
	}

	url := baseURL + apiPath

	// Add query parameters
	queryParts := []string{}
	for _, param := range apiMethod.Parameters {
		if param.In == "query" {
			val := p.getParamValue(reqData, param)
			if val != "" {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", param.Name, val))
			}
		}
	}

	if len(queryParts) > 0 {
		url += "?" + strings.Join(queryParts, "&")
	}

	return url
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
	if len(reqData) == 0 {
		return reqData
	}
	consumed := make(map[string]struct{}, len(parameters))
	for _, param := range parameters {
		from := param.From
		if from == "" {
			from = param.Name
		}
		if from != "" {
			consumed[from] = struct{}{}
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
