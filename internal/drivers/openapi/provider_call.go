package openapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
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

	if apiMethod.projection.Grouped {
		return nil, fmt.Errorf("operation %q requires location-grouped input", apiMethod.Name)
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

// serializeParams serializes the caller input into wire form per position
// and style/explode (SUI-006). Contract parameters (apiMethod.Request) take
// precedence; config-declared methods fall back to ParameterMapping with the
// OpenAPI default styles. Unexecutable combos return an error — buildRequest
// propagates it, the request is never sent mis-serialized.
func (p *Provider) serializeParams(apiMethod *APIMethod, reqData map[string]interface{}) (paramWireSet, error) {
	set := paramWireSet{pathSegments: map[string]string{}}
	if apiMethod == nil {
		return set, nil
	}
	if apiMethod.Request != nil && len(apiMethod.Request.Parameters) > 0 {
		for _, cparam := range apiMethod.Request.Parameters {
			value, present := p.resolveContractParamValue(cparam, reqData)
			placeholder := "{" + cparam.Name + "}"
			if !present {
				// 与 legacy 路径一致：缺失的 path 参数替换为空串，不泄漏占位符。
				if cparam.In == "path" {
					set.pathSegments[placeholder] = ""
				}
				continue
			}
			style := cparam.Style
			if style == "" {
				style = defaultStyleForPosition(cparam.In)
			}
			explode := cparam.Explode != nil && *cparam.Explode
			wire, err := serializeParamValue(cparam.In, cparam.Name, style, explode, value)
			if err != nil {
				return set, err
			}
			set.merge(cparam.In, cparam.Name, wire)
		}
		return set, nil
	}
	for _, param := range apiMethod.Parameters {
		if param.In == "" {
			continue
		}
		in := strings.TrimSpace(param.In)
		value, present := p.resolveLegacyParamValue(reqData, param)
		if !present {
			if in == "path" {
				set.pathSegments["{"+param.Name+"}"] = ""
			}
			continue
		}
		style := defaultStyleForPosition(in)
		wire, err := serializeParamValue(in, param.Name, style, defaultExplodeForStyle(style), value)
		if err != nil {
			return set, err
		}
		set.merge(in, param.Name, wire)
	}
	return set, nil
}

// resolveContractParamValue resolves a contract parameter's value: the caller
// key is the OpenAPI parameter name, falling back to the schema default. An
// empty-string value counts as absent (legacy behavior).
func (p *Provider) resolveContractParamValue(cparam RequestParameter, reqData map[string]interface{}) (any, bool) {
	if reqData != nil {
		if v, ok := reqData[cparam.Name]; ok && v != nil {
			if s, isStr := v.(string); isStr && s == "" {
				return nil, false
			}
			return v, true
		}
	}
	return schemaDefault(cparam.Schema)
}

// resolveLegacyParamValue resolves a config-declared parameter's value
// (From remapping + Default). An empty-string value counts as absent.
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

// consumedKeysFromMethod returns the caller key names occupied by addressing
// parameters (contract and legacy sources) so the body filter can exclude
// them.
func consumedKeysFromMethod(apiMethod *APIMethod) []string {
	if apiMethod == nil {
		return nil
	}
	if apiMethod.Request != nil && len(apiMethod.Request.Parameters) > 0 {
		keys := make([]string, 0, len(apiMethod.Request.Parameters))
		for _, p := range apiMethod.Request.Parameters {
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

// buildRequest builds an HTTP request from the API method and request data.
func (p *Provider) buildRequest(ctx context.Context, apiMethod *APIMethod, reqData map[string]interface{}) (*http.Request, error) {
	// 参数序列化（SUI-006）：path/query/header/cookie 统一经 style/explode 规则。
	wire, err := p.serializeParams(apiMethod, reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters: %w", err)
	}
	callURL := p.urlFromWire(apiMethod, wire)

	var body io.Reader
	var contentType string

	// Build request body if needed
	if apiMethod.Method == "POST" || apiMethod.Method == "PUT" || apiMethod.Method == "PATCH" {
		if apiMethod.RequestBody != nil {
			// SUI-013: a multipart body is streamed from a temp buffer with the
			// boundary the writer generated, so it takes the reader branch; the
			// other serializers are buffered bytes.
			if apiMethod.RequestBody.Type == "multipart" {
				payload, ct, err := buildMultipartBody(apiMethod.RequestBody, reqData)
				if err != nil {
					return nil, err
				}
				body, contentType = payload, ct
			} else {
				bodyBytes, ct := p.buildBody(apiMethod.RequestBody, reqData)
				body = bytes.NewReader(bodyBytes)
				contentType = ct
			}
		} else if len(reqData) > 0 {
			// OpenAPI semantics: parameters consumed by path/query/header
			// addressing must not leak into the request body.
			data, _ := json.Marshal(filterConsumedParamsByKeys(consumedKeysFromMethod(apiMethod), reqData))
			body = bytes.NewReader(data)
			contentType, err = contractBodyContentType(apiMethod)
			if err != nil {
				return nil, err
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, apiMethod.Method, callURL, body)
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

	// cookie 位置参数（form 样式，仅原始值）。
	if len(wire.cookies) > 0 {
		parts := make([]string, 0, len(wire.cookies))
		for _, c := range wire.cookies {
			parts = append(parts, c.name+"="+c.value)
		}
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}

	return req, nil
}

// contractBodyContentType resolves the Content-Type the executor sends for a
// contract-driven JSON body (SUI-007). The media type selected at import time
// is sent verbatim, so a vendor JSON tree (application/vnd.api+json) keeps its
// declared type instead of being rewritten to application/json. A contract
// whose body media types are all non-executable is refused with the same
// reason the import recorded — a stale persisted contract is never silently
// mislabeled as JSON. Methods without a request contract keep the historical
// default.
func contractBodyContentType(apiMethod *APIMethod) (string, error) {
	if apiMethod.Request == nil || apiMethod.Request.Body == nil || len(apiMethod.Request.Body.Content) == 0 {
		// A body slot with no declared media at all carries no selection
		// guidance; the historical JSON default applies.
		return "application/json", nil
	}
	mediaType, reason := SelectRequestBodyMediaType(apiMethod.Request.Body.Content)
	if mediaType == "" {
		return "", fmt.Errorf("%s", reason)
	}
	return mediaType, nil
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

// urlFromWire assembles the full URL: path segment replacements
// (percent-encoded) plus query pairs (URL-encoded, declaration order).
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
	// SUI-012: a form body is serialized as application/x-www-form-urlencoded,
	// where every value is escaped and repeated fields keep one key with
	// several entries.
	if bodyConfig.Type == "form" {
		return buildFormBody(bodyConfig, reqData)
	}
	return nil, ""
}

// buildFormBody serializes one form-urlencoded request body. An explicit field
// mapping names the wire keys; without one the remaining request data is sent
// as-is. url.Values.Encode escapes every value and sorts the keys, so the
// encoded body is deterministic across calls.
func buildFormBody(bodyConfig *RequestBodyMapping, reqData map[string]interface{}) ([]byte, string) {
	values := url.Values{}
	if len(bodyConfig.Fields) > 0 {
		for dstField, srcField := range bodyConfig.Fields {
			if value, ok := reqData[srcField]; ok {
				addFormValue(values, dstField, value)
			}
		}
	} else {
		for key, value := range reqData {
			addFormValue(values, key, value)
		}
	}
	return []byte(values.Encode()), "application/x-www-form-urlencoded"
}

// addFormValue records one form field. A list value becomes several values
// under the same key, which is how a form declares a repeated field; a nested
// object takes the RFC 7578 exploded shape key[subkey]=value, and an empty
// object keeps its field present as {}.
func addFormValue(values url.Values, key string, value interface{}) {
	switch typed := value.(type) {
	case nil:
		return
	case []interface{}:
		for _, item := range typed {
			addFormValue(values, key, item)
		}
	case []string:
		for _, item := range typed {
			values.Add(key, item)
		}
	case map[string]interface{}:
		if len(typed) == 0 {
			values.Add(key, "{}")
			return
		}
		for _, subKey := range sortedMapKeys(typed) {
			addFormValue(values, key+"["+subKey+"]", typed[subKey])
		}
	default:
		values.Add(key, formValueString(typed))
	}
}

// sortedMapKeys returns the keys of a JSON object in sorted order so a nested
// form field always encodes in the same order.
func sortedMapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// formValueString renders a scalar field the way a caller would type it: JSON
// style booleans and numbers instead of Go's %v formatting of them.
func formValueString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// multipartFileValue is one file field: the request data carries an object
// with a filename and the bytes the operator uploaded.
type multipartFileValue struct {
	Filename string
	Content  []byte
}

// multipartFileValues decodes one request-data field into file parts. The
// upload contract (SUI-013) is the web Upload widget's value: a data URL or a
// URL string plus the original filename — single value for a file field,
// an array for a multi-file field. A bare string is treated as a data URL
// whose filename is derived from the media type.
func multipartFileValues(value interface{}) ([]multipartFileValue, error) {
	switch typed := value.(type) {
	case []interface{}:
		parts := make([]multipartFileValue, 0, len(typed))
		for _, item := range typed {
			part, err := multipartFileValueFromField(item)
			if err != nil {
				return nil, err
			}
			if part != nil {
				parts = append(parts, *part)
			}
		}
		return parts, nil
	default:
		part, err := multipartFileValueFromField(value)
		if err != nil {
			return nil, err
		}
		if part == nil {
			return nil, nil
		}
		return []multipartFileValue{*part}, nil
	}
}

func multipartFileValueFromField(value interface{}) (*multipartFileValue, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case map[string]interface{}:
		// Object form: {filename, content}; content is either a data URL or
		// raw base64.
		filename, _ := typed["filename"].(string)
		contentRaw, hasContent := typed["content"]
		if !hasContent {
			return nil, nil
		}
		data, mediaType, err := decodeUploadPayload(contentRaw)
		if err != nil {
			return nil, err
		}
		if filename == "" {
			filename = uploadFilename(mediaType)
		}
		return &multipartFileValue{Filename: filename, Content: data}, nil
	case string:
		// A bare string is only unambiguously a file when it is a data URL;
		// anything else is companion text (a name, an id, a note) and stays a
		// text part. Raw base64 is only a file payload inside the upload
		// object form, where the field is explicitly the content slot.
		if !strings.HasPrefix(typed, "data:") {
			return nil, nil
		}
		data, mediaType, err := decodeUploadPayload(typed)
		if err != nil {
			return nil, err
		}
		return &multipartFileValue{Filename: uploadFilename(mediaType), Content: data}, nil
	default:
		return nil, fmt.Errorf("file field value must be an upload object or a data URL string, got %T", value)
	}
}

// decodeUploadPayload accepts the two payload shapes the upload widget
// contract can reach the executor with: a data URL (data:<type>;base64,<data>)
// or raw base64. The media type of a data URL is returned for filename
// derivation.
func decodeUploadPayload(value interface{}) ([]byte, string, error) {
	text, ok := value.(string)
	if !ok {
		return nil, "", fmt.Errorf("file content must be a data URL or base64 text, got %T", value)
	}
	if strings.HasPrefix(text, "data:") {
		separator := strings.Index(text, ";base64,")
		if separator < 0 {
			return nil, "", fmt.Errorf("file data URL must be base64 encoded")
		}
		mediaType := strings.TrimPrefix(text[:separator], "data:")
		data, err := base64.StdEncoding.DecodeString(text[separator+len(";base64,"):])
		if err != nil {
			return nil, "", fmt.Errorf("file data URL payload is not valid base64: %w", err)
		}
		return data, mediaType, nil
	}
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, "", fmt.Errorf("file content must be a data URL or base64 text: %w", err)
	}
	return data, "", nil
}

// uploadFilename derives a filename from the declared media type so the
// upstream sees a recognizable part even when the caller sent a bare URL.
func uploadFilename(mediaType string) string {
	extension := ""
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		extension = "png"
	case "image/jpeg", "image/jpg":
		extension = "jpg"
	case "image/gif":
		extension = "gif"
	case "image/webp":
		extension = "webp"
	case "application/pdf":
		extension = "pdf"
	case "text/plain":
		extension = "txt"
	case "application/json":
		extension = "json"
	case "application/octet-stream":
		extension = "bin"
	}
	if extension == "" {
		return "upload"
	}
	return "upload." + extension
}

// buildMultipartBody serializes one multipart/form-data request body. An
// explicit field mapping names the wire keys; without one the remaining
// request data is sent under its own names. A field whose value carries
// upload payloads becomes file parts (one per value); every other field
// becomes a text part. The body is buffered, so the generated boundary is
// part of the returned Content-Type header.
func buildMultipartBody(bodyConfig *RequestBodyMapping, reqData map[string]interface{}) (io.Reader, string, error) {
	// (source field -> wire name) pairs; without an explicit mapping every
	// request-data field is sent under its own name. Source fields are
	// processed in sorted order so the wire form is deterministic.
	type wireField struct {
		source string
		name   string
	}
	var fields []wireField
	if len(bodyConfig.Fields) > 0 {
		for dstField, srcField := range bodyConfig.Fields {
			if _, ok := reqData[srcField]; ok {
				fields = append(fields, wireField{source: srcField, name: dstField})
			}
		}
	} else {
		for key := range reqData {
			fields = append(fields, wireField{source: key, name: key})
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].source < fields[j].source })

	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	for _, field := range fields {
		if err := addMultipartField(writer, field.name, reqData[field.source]); err != nil {
			return nil, "", err
		}
	}
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return bytes.NewReader(buffer.Bytes()), contentType, nil
}

// addMultipartField writes one field: upload payloads as file parts, any other
// value as a text part with the form encoding used by formValueString.
func addMultipartField(writer *multipart.Writer, name string, value interface{}) error {
	if files, err := multipartFileValues(value); err != nil {
		return err
	} else if len(files) > 0 {
		for _, file := range files {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(name), escapeQuotes(file.Filename)))
			header.Set("Content-Type", "application/octet-stream")
			part, err := writer.CreatePart(header)
			if err != nil {
				return err
			}
			if _, err := part.Write(file.Content); err != nil {
				return err
			}
		}
		return nil
	}
	if value == nil {
		return nil
	}
	part, err := writer.CreateFormField(name)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(formValueString(value)))
	return err
}

// escapeQuotes quotes a name or filename for a Content-Disposition header.
func escapeQuotes(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
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

// filterConsumedParamsByKeys is the key-set core of filterConsumedParams,
// shared by the contract-parameter path (SUI-006) whose addressing keys come
// from the request contract.
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
