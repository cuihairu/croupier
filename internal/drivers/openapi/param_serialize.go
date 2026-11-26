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

// OpenAPI parameter serialization (SUI-006): encode a caller-provided value
// into its wire form per position and style/explode. Support matrix (public
// contract in docs/architecture/openapi-request-contract.md):
//
//   - path: simple / label / matrix, explode both ways;
//   - query: form / spaceDelimited / pipeDelimited (explode both ways),
//     deepObject (object values, explode must be true);
//   - header: simple (explode both ways);
//   - cookie: form, primitive values only.
//
// Unexecutable combos return an error (same text as the extraction-side
// diagnostic) — never a silently mis-serialized request. Rules follow the
// OpenAPI 3.x appendix (Parameter Serialization / RFC 6570): plain scalars,
// comma-joined arrays, objects as alternating k,v or k=v pairs per explode.
//
// Re-land note: the original SUI-006 delivery (0109f42bf) placed the helpers
// in internal/dashboard/spec over a spec-owned contract type; the refactored
// driver owns RequestParameter now, so the helpers live here against it.

// queryPair is one URL-encoded name=value pair (both sides already escaped).
type queryPair struct {
	name, value string
}

// nameValue is one header/cookie pair (header values verbatim, cookie values
// URL-escaped).
type nameValue struct {
	name, value string
}

// paramWire is the wire product of one serialized parameter: path replacement
// text, query pairs, header/cookie value. present=false means the caller
// supplied no value and there is no default — no wire output at all.
type paramWire struct {
	pathSegment string
	queryPairs  []queryPair
	headerValue string
	cookieValue string
	present     bool
}

// paramWireSet is the wire product of all parameters of one call (SUI-006):
// path segment replacements, encoded query pairs, header and cookie values.
type paramWireSet struct {
	pathSegments map[string]string // "{" + name + "}" -> encoded segment
	query        []queryPair
	headers      []nameValue
	cookies      []nameValue
}

func (w *paramWireSet) merge(in string, name string, wire paramWire) {
	switch in {
	case "path":
		w.pathSegments["{"+name+"}"] = wire.pathSegment
	case "query":
		w.query = append(w.query, wire.queryPairs...)
	case "header":
		if wire.present {
			w.headers = append(w.headers, nameValue{name: name, value: wire.headerValue})
		}
	case "cookie":
		if wire.present {
			w.cookies = append(w.cookies, nameValue{name: name, value: wire.cookieValue})
		}
	}
}

// scalarString converts a decoded JSON scalar to string; non-scalars fail.
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

// objectEntries returns object key/value pairs (keys sorted for fixture
// stability). Non-scalar values error — a map's or slice's %v form never
// reaches the wire.
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

// delimFor returns the list separator for non-explode form /
// spaceDelimited / pipeDelimited. Comma and pipe stay literal per the OpenAPI
// appendix; space joins use "+" (form-encoded space, decodes back to space).
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

// escapeParts escapes each item and joins with sep (the separator itself is
// literal per the OpenAPI appendix — only values are percent-encoded).
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

// serializeSimple implements the simple style (path/header): plain value,
// escaped comma-joined arrays, objects as alternating k,v (explode=false) or
// k=v pairs (explode=true).
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

// serializeLabel implements the path label style: "." prefix. Arrays join by
// "." when explode=true, else ","; objects as k=v (explode=true) or
// alternating k,v (false), joined by the same separator.
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

// serializeMatrix implements the path matrix style: ";name=value". Arrays
// repeat ";name=" per item when explode=true, else join comma-separated;
// objects as ";k=v" items (explode=true) or ";name=k,v,…" (false).
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

// serializeQueryPairs implements query form / spaceDelimited / pipeDelimited
// / deepObject, returning URL-encoded name=value pairs.
func serializeQueryPairs(name string, value any, style string, explode bool) ([]queryPair, error) {
	escName := url.QueryEscape(name)
	if style == "deepObject" {
		obj, ok := value.(map[string]any)
		if !ok {
			if _, isArr := value.([]any); isArr {
				return nil, fmt.Errorf("deepObject is only executable for object values")
			}
			// Primitives degrade to name=value (lenient, like most clients).
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

// defaultStyleForPosition returns the OpenAPI default style for a position
// (path/header → simple, query/cookie → form). An empty declared style falls
// back to this.
func defaultStyleForPosition(in string) string {
	if in == "query" || in == "cookie" {
		return "form"
	}
	return "simple"
}

// defaultExplodeForStyle reports the OpenAPI default explode for a style
// (form defaults to true, everything else to false).
func defaultExplodeForStyle(style string) bool {
	return style == "form"
}

// isStyleValidForPosition reports whether the style is one the OpenAPI 3.x
// spec allows at the position. Wrong-position styles (form in path, simple in
// query, …) and unknown styles are invalid documents, never executable.
func isStyleValidForPosition(style, in string) bool {
	switch in {
	case "path":
		return style == "matrix" || style == "label" || style == "simple"
	case "query":
		return style == "form" || style == "spaceDelimited" || style == "pipeDelimited" || style == "deepObject"
	case "header":
		return style == "simple"
	case "cookie":
		return style == "form"
	}
	return false
}

// valueShape is the coarse JSON value shape of a parameter schema.
type valueShape string

const (
	shapePrimitive valueShape = "primitive"
	shapeArray     valueShape = "array"
	shapeObject    valueShape = "object"
	shapeUnknown   valueShape = "unknown"
)

// schemaValueShape reports the coarse shape of a JSON Schema: array/object
// from the root type; a type-less schema with properties is an object; a
// type-less schema without properties is unknown (the executor inspects the
// actual runtime value then). A non-JSON or empty schema is unknown.
func schemaValueShape(schema json.RawMessage) valueShape {
	if len(schema) == 0 {
		return shapeUnknown
	}
	var probe struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &probe); err != nil {
		return shapeUnknown
	}
	switch probe.Type {
	case "array":
		return shapeArray
	case "object":
		return shapeObject
	}
	if probe.Type != "" {
		return shapePrimitive
	}
	if len(probe.Properties) > 0 {
		return shapeObject
	}
	return shapeUnknown
}

// serializationUnsupportedReason explains why a parameter's serialization
// combo is not executable, or "" when it is. Single truth shared by the
// extraction diagnostic and the executor's runtime refusal. docShape carries
// the schema-derived shape; pass shapeUnknown to recompute from the schema.
func serializationUnsupportedReason(param RequestParameter, docShape valueShape) string {
	in := param.In
	style := param.Style
	if style == "" {
		style = defaultStyleForPosition(in)
	}
	if !isStyleValidForPosition(style, in) {
		return fmt.Sprintf("style %q is not valid for %s parameters", style, in)
	}
	shape := docShape
	if shape == shapeUnknown {
		shape = schemaValueShape(param.Schema)
	}
	if style == "deepObject" {
		explode := param.Explode != nil && *param.Explode
		if !explode {
			return "deepObject requires explode=true"
		}
		if shape == shapeArray {
			return "deepObject is only executable for object values"
		}
	}
	if in == "cookie" {
		if shape == shapeArray || shape == shapeObject {
			return "cookie parameters only support primitive values"
		}
	}
	return ""
}

// diagParameterSerializationUnsupported marks a parameter whose declared
// style/explode/schema combination the executor cannot serialize (SUI-006).
// Severity error: the proposal quality gate forces needs_review, and the
// executor refuses the request with the same reason instead of silently
// mis-serializing.
const diagParameterSerializationUnsupported = "openapi_parameter_serialization_unsupported"

// validateSerialization returns the diagnostics for a parameter whose
// serialization the executor cannot build. Empty slice means executable.
func validateSerialization(param RequestParameter) []spec.Diagnostic {
	reason := serializationUnsupportedReason(param, schemaValueShape(param.Schema))
	if reason == "" {
		return nil
	}
	explode := param.Explode != nil && *param.Explode
	return []spec.Diagnostic{{
		Code:     diagParameterSerializationUnsupported,
		Severity: spec.SeverityError,
		Field:    param.Name,
		Message: fmt.Sprintf(
			"parameter %s (%s, style=%s, explode=%v) cannot be serialized: %s",
			param.Name, param.In, param.Style, explode, reason),
	}}
}

// schemaDefault returns the "default" keyword of a parameter schema, if any.
// Belt-and-suspenders for direct executor calls; the client-side form already
// applies spec defaults.
func schemaDefault(schema json.RawMessage) (any, bool) {
	if len(schema) == 0 {
		return nil, false
	}
	var probe struct {
		Default json.RawMessage `json:"default"`
	}
	if err := json.Unmarshal(schema, &probe); err != nil || probe.Default == nil {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(probe.Default, &v); err != nil {
		return nil, false
	}
	return v, true
}

// serializeParamValue encodes one parameter per position and style/explode.
// Unexecutable combos error with the same text as the extraction diagnostic.
func serializeParamValue(in, name, style string, explode bool, value any) (paramWire, error) {
	if style == "" {
		style = defaultStyleForPosition(in)
	}
	switch in {
	case "path":
		var seg string
		var err error
		switch style {
		case "simple":
			seg, err = serializeSimple(value, explode, url.PathEscape)
		case "label":
			seg, err = serializeLabel(value, explode)
		case "matrix":
			seg, err = serializeMatrix(name, value, explode)
		default:
			err = fmt.Errorf("style %q is not valid for path parameters", style)
		}
		if err != nil {
			return paramWire{}, err
		}
		return paramWire{pathSegment: seg, present: true}, nil
	case "query":
		if !isStyleValidForPosition(style, in) {
			return paramWire{}, fmt.Errorf("style %q is not valid for query parameters", style)
		}
		if style == "deepObject" && !explode {
			return paramWire{}, fmt.Errorf("deepObject requires explode=true")
		}
		pairs, err := serializeQueryPairs(name, value, style, explode)
		if err != nil {
			return paramWire{}, err
		}
		return paramWire{queryPairs: pairs, present: true}, nil
	case "header":
		if style != "simple" {
			return paramWire{}, fmt.Errorf("style %q is not valid for header parameters", style)
		}
		v, err := serializeSimple(value, explode, func(s string) string { return s })
		if err != nil {
			return paramWire{}, err
		}
		return paramWire{headerValue: v, present: true}, nil
	case "cookie":
		if style != "form" {
			return paramWire{}, fmt.Errorf("style %q is not valid for cookie parameters", style)
		}
		s, ok := scalarString(value)
		if !ok {
			return paramWire{}, fmt.Errorf("cookie parameters only support primitive values")
		}
		return paramWire{cookieValue: url.QueryEscape(s), present: true}, nil
	default:
		return paramWire{}, fmt.Errorf("parameter position %q is not serializable", in)
	}
}
