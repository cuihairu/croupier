package spec

import "encoding/json"

// ParameterPosition is the fixed position of one OpenAPI parameter. The four
// OpenAPI positions are the complete set; an unknown position is a contract
// extraction error and must surface as a diagnostic, never as a silent drop.
type ParameterPosition string

const (
	ParameterPositionPath   ParameterPosition = "path"
	ParameterPositionQuery  ParameterPosition = "query"
	ParameterPositionHeader ParameterPosition = "header"
	ParameterPositionCookie ParameterPosition = "cookie"
)

// IsValidParameterPosition reports whether the position is one of the four
// OpenAPI parameter locations.
func IsValidParameterPosition(position ParameterPosition) bool {
	switch position {
	case ParameterPositionPath,
		ParameterPositionQuery,
		ParameterPositionHeader,
		ParameterPositionCookie:
		return true
	default:
		return false
	}
}

// RequestSerialization records how one parameter is serialized onto the
// request (OpenAPI style/explode). Empty style falls back to the position's
// OpenAPI default (path→simple, query→form, header→simple, cookie→form); the
// extraction layer resolves defaults so this value is always concrete for a
// derived contract.
type RequestSerialization struct {
	Style   string `json:"style"`
	Explode bool   `json:"explode"`
}

// RequestParameterSpec is one fixed-position parameter of an operation in the
// unified request contract (docs/architecture/openapi-request-contract.md).
type RequestParameterSpec struct {
	Name          string               `json:"name"`
	In            ParameterPosition    `json:"in"`
	Required      bool                 `json:"required"`
	Deprecated    bool                 `json:"deprecated,omitempty"`
	Description   string               `json:"description,omitempty"`
	Serialization RequestSerialization `json:"serialization"`
	Schema        JSONSchema           `json:"schema,omitempty"`
}

// RequestBodySpec is the request-body slot of the unified request contract.
// Schema is kept verbatim — the root may be an object, an array, or a scalar,
// and every root-level constraint survives extraction.
type RequestBodySpec struct {
	MediaType string     `json:"mediaType"`
	Required  bool       `json:"required"`
	Schema    JSONSchema `json:"schema"`
}

// OperationRequestContract is the unified request representation of one
// OpenAPI operation: fixed-position parameters plus one media-typed body
// slot. It is the single derivation source for both the operator-facing input
// schema (flat/structured/body_only projection) and the upstream request
// built by the executor. Both the upload pipeline and the runtime provider
// discovery must derive identical values for the same document (SUI-003).
type OperationRequestContract struct {
	Parameters  []RequestParameterSpec `json:"parameters"`
	Body        *RequestBodySpec       `json:"body,omitempty"`
	Diagnostics []Diagnostic           `json:"diagnostics,omitempty"`
}

// InputProjectionMode classifies how an OperationRequestContract projects to
// the operator-facing input schema.
type InputProjectionMode string

const (
	// InputProjectionFlat lifts parameters and (for a flat-able object body)
	// body properties into one object — the legacy lossless shape.
	InputProjectionFlat InputProjectionMode = "flat"
	// InputProjectionStructured namespaces parameters under "params" and the
	// whole body under "body" — used whenever the flat shape would lose data
	// (root array/scalar body, non-preservable body constraints, or a
	// parameter/property name collision).
	InputProjectionStructured InputProjectionMode = "structured"
	// InputProjectionBodyOnly returns the body schema verbatim — used when
	// the operation declares no addressing parameters but does declare a
	// body, so every body constraint survives unchanged.
	InputProjectionBodyOnly InputProjectionMode = "body_only"
)

// RequestContractDiagnostics carries the structured diagnostic codes emitted
// when the flat projection cannot represent a contract losslessly. See the
// compatibility boundary in docs/architecture/openapi-request-contract.md.
const (
	DiagRequestParameterCollision = "openapi_request_parameter_collision"
	DiagRequestBodyShapeNonFlat   = "openapi_request_body_shape_nonflat"
)

// IsFlatPreservableSchemaKey reports whether a top-level key of an object body
// schema survives the flat projection. Everything else (minProperties,
// additionalProperties, patternProperties, allOf/oneOf/anyOf, …) forces the
// structured/body_only projection so the constraint is not silently dropped.
func IsFlatPreservableSchemaKey(key string) bool {
	switch key {
	case "type", "properties", "required", "description", "title", "default", "example":
		return true
	default:
		return false
	}
}

// ParseRequestBodySchemaJSON is a helper for consumers that need the body
// schema as raw JSON; it tolerates an absent body.
func ParseRequestBodySchemaJSON(body *RequestBodySpec) json.RawMessage {
	if body == nil {
		return nil
	}
	return json.RawMessage(body.Schema)
}
