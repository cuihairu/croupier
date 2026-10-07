package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// extractOutputSchema derives the JSON Schema (as JSON text) of one
// operation's success (2xx) application/json response, so the provider can
// validate what it is about to hand to the platform before it leaves the
// agent (能力矩阵 #4，批次 C：InvokeResponse 此前无 schema 级校验、直接透传).
//
//   - candidate order: 200 first, then remaining 2xx codes ascending; the
//     first candidate carrying a JSON schema wins;
//   - OpenAPI 3.x shape (responses.<code>.content.application/json.schema)
//     and Swagger 2.0 shape (responses.<code>.schema) are both accepted;
//   - local $ref pointers are resolved against the full document via the
//     same machinery as the input schema; non-local refs are preserved;
//   - empty string when the operation declares no JSON success response —
//     callers skip validation for such methods (兼容既有 provider).
func extractOutputSchema(methodObj, root map[string]interface{}) string {
	responses, ok := methodObj["responses"].(map[string]interface{})
	if !ok {
		return ""
	}
	codes := make([]string, 0, len(responses))
	for code := range responses {
		if len(code) == 3 && code[0] == '2' {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return ""
	}
	sort.Strings(codes)
	for i, code := range codes { // 200 提到其他 2xx 之前
		if code == "200" && i != 0 {
			codes[0], codes[i] = codes[i], codes[0]
			break
		}
	}

	for _, code := range codes {
		resp, ok := responses[code].(map[string]interface{})
		if !ok {
			continue
		}
		var schema interface{}
		if content, ok := resp["content"].(map[string]interface{}); ok {
			if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
				schema = jsonContent["schema"]
			}
		}
		if schema == nil {
			// Swagger 2.0: response carries the schema directly.
			schema = resp["schema"]
		}
		schemaMap, ok := schema.(map[string]interface{})
		if !ok {
			continue
		}
		resolved, ok := resolveSchemaRefs(schemaMap, root, 0).(map[string]interface{})
		if !ok {
			continue
		}
		// Input originates from json.Unmarshal output, so Marshal cannot
		// fail (same shape as the spec merge in parseOpenAPISpec).
		encoded, _ := json.Marshal(resolved)
		return string(encoded)
	}
	return ""
}

// validateOutput 校验 provider 返回（已按 Transform 配置变换）的响应数据。
// schema 编译失败属 spec 缺陷，按批次 C「不破坏既有调用」口径降级为告警跳过；
// 数据与已编译 schema 不符则硬失败——错误沿 Provider.Call 返回链进入既有
// InvokeResponse 错误上报通道，不静默吞。
func (p *Provider) validateOutput(method, schemaText string, data []byte) error {
	compiled, err := p.outputValidator(method, schemaText)
	if err != nil {
		slog.Warn("output schema compile failed, skipping response validation", "method", method, "error", err)
		return nil
	}
	payload, err := decodeResponsePayload(data)
	if err != nil {
		return fmt.Errorf("response validation failed for %s: %w", method, err)
	}
	if err := compiled.Validate(payload); err != nil {
		return fmt.Errorf("response validation failed for %s: %w", method, err)
	}
	return nil
}

// outputValidator 返回 method 的已编译响应 schema，编译结果（含失败）按方法
// 缓存——编译一次性成本不落在热路径上。
func (p *Provider) outputValidator(method, schemaText string) (*jsonschema.Schema, error) {
	if cached, ok := p.outputValidators.Load(method); ok {
		if entry, ok := cached.(cachedOutputSchema); ok {
			return entry.schema, entry.err
		}
	}
	compiled, err := compileOutputSchema(schemaText)
	p.outputValidators.Store(method, cachedOutputSchema{compiled, err})
	return compiled, err
}

type cachedOutputSchema struct {
	schema *jsonschema.Schema
	err    error
}

func compileOutputSchema(schemaText string) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaText))
	if err != nil {
		return nil, fmt.Errorf("invalid output schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	// 固定合法资源名 + 新 compiler 无重复资源，AddResource 不解析内容，
	// schema 合法性错误全部由 Compile 报告（同 internal/validation 既有口径）。
	_ = compiler.AddResource("output.json", doc)
	return compiler.Compile("output.json")
}

// decodeResponsePayload 与 internal/validation 的 decodeJSONPayload 同口径：
// UseNumber 保数值精度（jsonschema/v6 对 json.Number 有专门分支），多余
// JSON 值拒收。
func decodeResponsePayload(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("response is not valid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("response contains multiple JSON values")
	}
	return payload, nil
}
