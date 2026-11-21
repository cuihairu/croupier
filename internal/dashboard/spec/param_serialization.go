package spec

import (
	"encoding/json"
	"fmt"
)

// DiagParameterSerializationUnsupported marks a parameter whose declared
// style/explode/schema-type combination the executor cannot serialize onto the
// wire (SUI-006). Field is the parameter name; the message states the
// position, style, explode and the concrete reason. A parameter carrying this
// diagnostic is excluded from the directly-executable set: the executor fails
// the request with the same reason instead of silently mis-serializing.
const DiagParameterSerializationUnsupported = "openapi_parameter_serialization_unsupported"

// DefaultStyleForPosition returns the OpenAPI default serialization style for
// a parameter position (path/header → simple, query/cookie → form). Empty
// style in a derived contract falls back to this value.
func DefaultStyleForPosition(in ParameterPosition) string {
	switch in {
	case ParameterPositionQuery, ParameterPositionCookie:
		return "form"
	default:
		return "simple"
	}
}

// IsStyleValidForPosition reports whether style is one of the styles the
// OpenAPI 3.x specification allows for the position. A style assigned to the
// wrong position (form in path, simple in query, …) or an unknown style is an
// invalid document and is never executable.
func IsStyleValidForPosition(style string, in ParameterPosition) bool {
	switch in {
	case ParameterPositionPath:
		return style == "matrix" || style == "label" || style == "simple"
	case ParameterPositionQuery:
		return style == "form" || style == "spaceDelimited" || style == "pipeDelimited" || style == "deepObject"
	case ParameterPositionHeader:
		return style == "simple"
	case ParameterPositionCookie:
		return style == "form"
	}
	return false
}

// ValueShape is the coarse JSON value shape of a parameter schema. It decides
// whether a serialization combo is executable (deepObject needs an object;
// cookies only take primitives).
type ValueShape string

const (
	ShapePrimitive ValueShape = "primitive"
	ShapeArray     ValueShape = "array"
	ShapeObject    ValueShape = "object"
	ShapeUnknown   ValueShape = "unknown"
)

// SchemaValueShape reports the coarse shape of a JSON Schema. "array"/"object"
// come from the root type; a type-less schema with properties is an object; a
// type-less schema without properties is unknown (the executor inspects the
// actual value at runtime). A non-JSON or empty schema is also unknown.
func SchemaValueShape(schema JSONSchema) ValueShape {
	if len(schema) == 0 {
		return ShapeUnknown
	}
	var probe struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &probe); err != nil {
		return ShapeUnknown
	}
	switch probe.Type {
	case "array":
		return ShapeArray
	case "object":
		return ShapeObject
	}
	if probe.Type != "" {
		return ShapePrimitive
	}
	if len(probe.Properties) > 0 {
		return ShapeObject
	}
	return ShapeUnknown
}

// SerializationUnsupportedReason explains why a parameter's serialization
// combo is not executable, or "" when it is. It is the single truth used both
// by the extraction diagnostics and by the executor's runtime guard. docShape
// is the schema-derived value shape; pass ShapeUnknown to let the function
// recompute it from the parameter schema.
func SerializationUnsupportedReason(param RequestParameterSpec, docShape ValueShape) string {
	in := param.In
	style := param.Serialization.Style
	if style == "" {
		style = DefaultStyleForPosition(in)
	}
	if !IsStyleValidForPosition(style, in) {
		return fmt.Sprintf("style %q is not valid for %s parameters", style, in)
	}
	shape := docShape
	if shape == ShapeUnknown {
		shape = SchemaValueShape(param.Schema)
	}
	switch style {
	case "deepObject":
		if !param.Serialization.Explode {
			return "deepObject requires explode=true"
		}
		if shape == ShapeArray {
			return "deepObject is only executable for object values"
		}
	}
	if in == ParameterPositionCookie {
		if shape == ShapeArray || shape == ShapeObject {
			return "cookie parameters only support primitive values"
		}
	}
	return ""
}

// SchemaDefault returns the "default" keyword of a schema, if any. Used by the
// executor to fill an absent parameter value from the schema default (the
// client-side applySpecDefaults already does this for form inputs; this is the
// belt-and-suspenders for direct executor calls).
func SchemaDefault(schema JSONSchema) (any, bool) {
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

// ValidateSerialization returns the diagnostics for a parameter whose
// serialization the executor cannot build (SUI-006). An empty slice means the
// combo is executable. Both the kin and map extraction front-ends append these
// through requestContractDiagnostics, keeping the two paths in parity.
func ValidateSerialization(param RequestParameterSpec) []Diagnostic {
	reason := SerializationUnsupportedReason(param, SchemaValueShape(param.Schema))
	if reason == "" {
		return nil
	}
	return []Diagnostic{{
		Code:     DiagParameterSerializationUnsupported,
		Severity: SeverityError,
		Field:    param.Name,
		Message: fmt.Sprintf(
			"parameter %s (%s, style=%s, explode=%v) cannot be serialized: %s",
			param.Name, param.In, param.Serialization.Style, param.Serialization.Explode, reason),
	}}
}
