package openapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// 本文件实现 OpenAPI 参数序列化（SUI-006）：把调用方提供的参数值按位置与
// style/explode 组合编码为实际 HTTP 线上的形态。支持矩阵（公开声明见
// docs/architecture/openapi-request-contract.md#参数序列化）：
//
//   - path：simple / label / matrix，explode 均支持；
//   - query：form / spaceDelimited / pipeDelimited（explode 均支持）、
//     deepObject（对象值，explode 必须为 true）；
//   - header：simple（explode 均支持）；
//   - cookie：form，仅原始值（数组/对象值不可执行）。
//
// 无法执行的组合返回 error（与提取侧诊断同文），绝不静默错序列化。
// 序列化规则与 OpenAPI 3.x 附录（Parameter Serialization / RFC 6570）一致：
// 简单值、逗号连接数组、对象按 explode 区分 k,v 交替或 k=v 对。

// paramWire 是单个参数序列化后的线上产物：path 段替换文本、query 键值对、
// header/cookie 值。present 为 false 表示调用方未提供值且无默认——不产生
// 任何线上输出。
type paramWire struct {
	pathSegment string
	queryPairs  []queryPair
	headerValue string
	cookieValue string
	present     bool
}

// queryPair 是已按 URL 编码的 name=value 键值对（name 与 value 均已转义）。
type queryPair struct {
	name, value string
}

// scalarString 把 JSON 解码后的标量值转成字符串；非标量返回 false。
func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32), true
	case int:
		return strconv.Itoa(t), true
	case int32:
		return strconv.FormatInt(int64(t), 10), true
	case int64:
		return strconv.FormatInt(t, 10), true
	case json.Number:
		return t.String(), true
	case nil:
		return "", false
	}
	return "", false
}

// objectEntries 返回对象键值对（键按字典序，fixture 稳定）。值非标量时报错，
// 绝不把 map/slice 的 %v 形态送上线上。
func objectEntries(o map[string]any) ([]queryPair, error) {
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]queryPair, 0, len(keys))
	for _, k := range keys {
		s, ok := scalarString(o[k])
		if !ok {
			return nil, fmt.Errorf("object value of key %q is not scalar", k)
		}
		out = append(out, queryPair{name: k, value: s})
	}
	return out, nil
}

// delimFor 返回非 explode 的 form / spaceDelimited / pipeDelimited 列表分隔符。
// 逗号与竖线按 OpenAPI 附录保持字面；空格分隔用 "+"（表单编码的空格，线上
// 合法，解码后即空格连接的值）。
func delimFor(style string) string {
	switch style {
	case "spaceDelimited":
		return "+"
	case "pipeDelimited":
		return "|"
	default:
		return ","
	}
}

// escapeParts 逐项转义并以 sep 连接（sep 为字面分隔符，不再被转义——分隔符
// 本身按 OpenAPI 附录保持原样，只有值被百分号编码）。出现非标量项返回错误。
func escapeParts(arr []any, sep string, escape func(string) string) (string, error) {
	parts := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := scalarString(e)
		if !ok {
			return "", fmt.Errorf("array item of type %T is not scalar", e)
		}
		parts = append(parts, escape(s))
	}
	return strings.Join(parts, sep), nil
}

// serializeSimple 实现 path/header 的 simple 样式：原始值、逐项转义的逗号连
// 接数组、对象按 explode 区分 k,v 交替（false）或 k=v 对（true）。escape 按
// 位置选择（path 用 PathEscape，header 原样）。
func serializeSimple(value any, explode bool, escape func(string) string) (string, error) {
	if s, ok := scalarString(value); ok {
		return escape(s), nil
	}
	if arr, ok := value.([]any); ok {
		return escapeParts(arr, ",", escape)
	}
	if obj, ok := value.(map[string]any); ok {
		entries, err := objectEntries(obj)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, len(entries)*2)
		for _, e := range entries {
			if explode {
				parts = append(parts, escape(e.name)+"="+escape(e.value))
			} else {
				parts = append(parts, escape(e.name), escape(e.value))
			}
		}
		return strings.Join(parts, ","), nil
	}
	return "", fmt.Errorf("value of type %T is not serializable with simple style", value)
}

// serializeLabel 实现 path 的 label 样式：以 "." 为前缀。数组 explode=true 逐
// 项以 "." 连接，否则逗号连接；对象 explode=true 为 k=v 以 "." 连接，否则
// k,v 交替以 "." 连接。
func serializeLabel(value any, explode bool) (string, error) {
	if s, ok := scalarString(value); ok {
		return "." + url.PathEscape(s), nil
	}
	if arr, ok := value.([]any); ok {
		sep := ","
		if explode {
			sep = "."
		}
		joined, err := escapeParts(arr, sep, url.PathEscape)
		if err != nil {
			return "", err
		}
		return "." + joined, nil
	}
	if obj, ok := value.(map[string]any); ok {
		entries, err := objectEntries(obj)
		if err != nil {
			return "", err
		}
		sep := ","
		if explode {
			sep = "."
		}
		parts := make([]string, 0, len(entries)*2)
		for _, e := range entries {
			if explode {
				parts = append(parts, url.PathEscape(e.name)+"="+url.PathEscape(e.value))
			} else {
				parts = append(parts, url.PathEscape(e.name), url.PathEscape(e.value))
			}
		}
		return "." + strings.Join(parts, sep), nil
	}
	return "", fmt.Errorf("value of type %T is not serializable with label style", value)
}

// serializeMatrix 实现 path 的 matrix 样式：";name=value"。数组 explode=true
// 逐项重复 ";name="，否则逗号连接；对象 explode=true 为 ";k=v" 逐项，否则
// ";name=k,v,…"。
func serializeMatrix(name string, value any, explode bool) (string, error) {
	prefix := ";" + name + "="
	if s, ok := scalarString(value); ok {
		return prefix + url.PathEscape(s), nil
	}
	if arr, ok := value.([]any); ok {
		if explode {
			parts := make([]string, 0, len(arr))
			for _, e := range arr {
				s, ok := scalarString(e)
				if !ok {
					return "", fmt.Errorf("array item of type %T is not scalar", e)
				}
				parts = append(parts, prefix+url.PathEscape(s))
			}
			return strings.Join(parts, ""), nil
		}
		joined, err := escapeParts(arr, ",", url.PathEscape)
		if err != nil {
			return "", err
		}
		return prefix + joined, nil
	}
	if obj, ok := value.(map[string]any); ok {
		entries, err := objectEntries(obj)
		if err != nil {
			return "", err
		}
		if explode {
			parts := make([]string, 0, len(entries))
			for _, e := range entries {
				parts = append(parts, ";"+url.PathEscape(e.name)+"="+url.PathEscape(e.value))
			}
			return strings.Join(parts, ""), nil
		}
		parts := make([]string, 0, len(entries)*2)
		for _, e := range entries {
			parts = append(parts, url.PathEscape(e.name), url.PathEscape(e.value))
		}
		return prefix + strings.Join(parts, ","), nil
	}
	return "", fmt.Errorf("value of type %T is not serializable with matrix style", value)
}

// serializeQueryPairs 实现 query 的 form / spaceDelimited / pipeDelimited /
// deepObject 样式，返回已 URL 编码的 name=value 对。
func serializeQueryPairs(name string, value any, style string, explode bool) ([]queryPair, error) {
	escName := url.QueryEscape(name)
	if style == "deepObject" {
		obj, ok := value.(map[string]any)
		if !ok {
			if _, isArr := value.([]any); isArr {
				return nil, fmt.Errorf("deepObject is only executable for object values")
			}
			// 原始值退化为 name=value（宽容，多数客户端如此）。
			if s, ok := scalarString(value); ok {
				return []queryPair{{name: escName, value: url.QueryEscape(s)}}, nil
			}
			return nil, fmt.Errorf("value of type %T is not serializable with deepObject style", value)
		}
		entries, err := objectEntries(obj)
		if err != nil {
			return nil, err
		}
		out := make([]queryPair, 0, len(entries))
		for _, e := range entries {
			out = append(out, queryPair{
				name:  url.QueryEscape(name + "[" + e.name + "]"),
				value: url.QueryEscape(e.value),
			})
		}
		return out, nil
	}

	// form / spaceDelimited / pipeDelimited
	if arr, ok := value.([]any); ok {
		if explode {
			out := make([]queryPair, 0, len(arr))
			for _, e := range arr {
				s, ok := scalarString(e)
				if !ok {
					return nil, fmt.Errorf("array item of type %T is not scalar", e)
				}
				out = append(out, queryPair{name: escName, value: url.QueryEscape(s)})
			}
			return out, nil
		}
		joined, err := escapeParts(arr, delimFor(style), url.QueryEscape)
		if err != nil {
			return nil, err
		}
		return []queryPair{{name: escName, value: joined}}, nil
	}
	if obj, ok := value.(map[string]any); ok {
		entries, err := objectEntries(obj)
		if err != nil {
			return nil, err
		}
		if explode {
			out := make([]queryPair, 0, len(entries))
			for _, e := range entries {
				out = append(out, queryPair{
					name:  url.QueryEscape(e.name),
					value: url.QueryEscape(e.value),
				})
			}
			return out, nil
		}
		sep := delimFor(style)
		vals := make([]string, 0, len(entries)*2)
		for _, e := range entries {
			vals = append(vals, url.QueryEscape(e.name), url.QueryEscape(e.value))
		}
		return []queryPair{{name: escName, value: strings.Join(vals, sep)}}, nil
	}
	if s, ok := scalarString(value); ok {
		return []queryPair{{name: escName, value: url.QueryEscape(s)}}, nil
	}
	return nil, fmt.Errorf("value of type %T is not serializable with %s style", value, style)
}

// serializeParamValue 把一个参数按位置与 style/explode 编码为线上形态。
// 无法执行的组合返回 error（与提取侧诊断同文）。
func serializeParamValue(p spec.RequestParameterSpec, value any) (paramWire, error) {
	style := p.Serialization.Style
	if style == "" {
		style = spec.DefaultStyleForPosition(p.In)
	}
	explode := p.Serialization.Explode
	switch p.In {
	case spec.ParameterPositionPath:
		var seg string
		var err error
		switch style {
		case "simple":
			seg, err = serializeSimple(value, explode, url.PathEscape)
		case "label":
			seg, err = serializeLabel(value, explode)
		case "matrix":
			seg, err = serializeMatrix(p.Name, value, explode)
		default:
			err = fmt.Errorf("style %q is not valid for path parameters", style)
		}
		if err != nil {
			return paramWire{}, err
		}
		return paramWire{pathSegment: seg, present: true}, nil
	case spec.ParameterPositionQuery:
		if !spec.IsStyleValidForPosition(style, spec.ParameterPositionQuery) {
			return paramWire{}, fmt.Errorf("style %q is not valid for query parameters", style)
		}
		if style == "deepObject" && !explode {
			return paramWire{}, fmt.Errorf("deepObject requires explode=true")
		}
		pairs, err := serializeQueryPairs(p.Name, value, style, explode)
		if err != nil {
			return paramWire{}, err
		}
		return paramWire{queryPairs: pairs, present: true}, nil
	case spec.ParameterPositionHeader:
		if style != "simple" {
			return paramWire{}, fmt.Errorf("style %q is not valid for header parameters", style)
		}
		v, err := serializeSimple(value, explode, func(s string) string { return s })
		if err != nil {
			return paramWire{}, err
		}
		return paramWire{headerValue: v, present: true}, nil
	case spec.ParameterPositionCookie:
		if style != "form" {
			return paramWire{}, fmt.Errorf("style %q is not valid for cookie parameters", style)
		}
		s, ok := scalarString(value)
		if !ok {
			return paramWire{}, fmt.Errorf("cookie parameters only support primitive values")
		}
		return paramWire{cookieValue: url.QueryEscape(s), present: true}, nil
	default:
		return paramWire{}, fmt.Errorf("parameter position %q is not serializable", p.In)
	}
}
