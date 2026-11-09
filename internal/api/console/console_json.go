package console

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// specErrorDiagnostics 取出 error 级诊断（发布校验未通过项）。
func specErrorDiagnostics(diags []spec.Diagnostic) []spec.Diagnostic {
	out := make([]spec.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// specDiagnosticDetails 把诊断压平成 422 details（稳定机器码 + 可读消息）。
func specDiagnosticDetails(diags []spec.Diagnostic) map[string]string {
	out := make(map[string]string, len(diags))
	for _, d := range diags {
		key := strings.TrimSpace(d.Code)
		if key == "" {
			key = "diagnostic"
		}
		out[key] = strings.TrimSpace(d.Message)
	}
	return out
}

// isNullRawJSON reports whether the raw JSON is a literal null.
func isNullRawJSON(raw json.RawMessage) bool {
	return len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null"
}

func pickSelectionValues(raw json.RawMessage, path string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	// 设计债清理：raw 非空已前置保证，validRawJSON 要么报错要么成功，
	// 原 found==false 分支不可达（下同）。
	value, err := validRawJSON(raw, "selection")
	if err != nil {
		return nil, false, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(value, &rows); err != nil {
		return nil, false, errorx.NewBadRequest("selection context must be an array")
	}
	values := make([]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		selected, ok := getJSONPointerValue(row, path)
		if !ok {
			return nil, false, nil
		}
		values = append(values, selected)
	}
	// 设计债清理：values 元素均为合法 JSON 文档的子树（Unmarshal 切片产物），
	// Marshal 恒成功，原 err 分支不可达已删除。
	result, _ := json.Marshal(values)
	return result, true, nil
}

func valueFromRawContext(raw json.RawMessage, path string, sourceName string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	value, err := validRawJSON(raw, sourceName)
	if err != nil {
		return nil, false, err
	}
	if path == "" {
		return value, true, nil
	}
	selected, ok := getJSONPointerValue(value, path)
	return selected, ok, nil
}

// validRawJSON 校验原始 JSON 并返回防御性拷贝。
// 设计债清理：全部调用方都已前置拦截空输入，且非空输入下要么报错要么成功，
// 不存在 (nil,false,nil) 形态，bool 返回值与空值短路分支已随签名收紧删除。
func validRawJSON(raw json.RawMessage, sourceName string) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, errorx.NewBadRequest(sourceName + " context must be valid JSON")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func getJSONPointerValue(value json.RawMessage, path string) (json.RawMessage, bool) {
	if !isJSONPointer(path) {
		return nil, false
	}
	current := value
	for _, token := range jsonPointerTokens(path) {
		trimmed := bytes.TrimSpace(current)
		if len(trimmed) == 0 {
			return nil, false
		}
		switch trimmed[0] {
		case '{':
			var object map[string]json.RawMessage
			if err := json.Unmarshal(trimmed, &object); err != nil {
				return nil, false
			}
			next, ok := object[token]
			if !ok {
				return nil, false
			}
			current = next
		case '[':
			var array []json.RawMessage
			if err := json.Unmarshal(trimmed, &array); err != nil {
				return nil, false
			}
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(array) {
				return nil, false
			}
			current = array[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func setJSONPointerValue(payload map[string]json.RawMessage, path string, value json.RawMessage) error {
	if !isJSONPointer(path) || path == "" {
		return errorx.NewValidationError("target must be a non-empty JSON Pointer")
	}
	return setJSONObjectPointer(payload, jsonPointerTokens(path), value)
}

func setJSONObjectPointer(object map[string]json.RawMessage, tokens []string, value json.RawMessage) error {
	if len(tokens) == 0 || tokens[0] == "" {
		return errorx.NewValidationError("target contains an empty object key")
	}
	key := tokens[0]
	if len(tokens) == 1 {
		object[key] = value
		return nil
	}
	child := map[string]json.RawMessage{}
	if existing, ok := object[key]; ok {
		if err := json.Unmarshal(existing, &child); err != nil {
			return errorx.NewValidationError("target conflicts with a non-object parent")
		}
	}
	if err := setJSONObjectPointer(child, tokens[1:], value); err != nil {
		return err
	}
	raw, err := json.Marshal(child)
	if err != nil {
		return err
	}
	object[key] = raw
	return nil
}

func isJSONPointer(path string) bool {
	return path == "" || strings.HasPrefix(path, "/")
}

func jsonPointerTokens(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}
