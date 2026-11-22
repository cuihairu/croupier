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
	// DiagSchemaReferenceUnresolved marks a schema $ref the extraction could
	// not fully inline into the standalone request contract (SUI-008): a
	// missing target, a cyclic reference past the inlining depth cap, or a
	// reference outside the document. Field is the $ref pointer; the message
	// names the boundary that stopped the inlining.
	DiagSchemaReferenceUnresolved = "openapi_schema_reference_unresolved"
)

// SchemaReferenceUnresolvedDiagnostic builds the proposal-level diagnostic
// recorded when a schema reference cannot be inlined into the standalone
// request contract (SUI-008). Severity is warning: the schema is still usable
// (partial inlining), but the unresolved reference is never silently dropped —
// the schema explicitly enters review instead of degrading to an anonymous
// object the operator cannot distinguish from a real one.
func SchemaReferenceUnresolvedDiagnostic(ref, reason string) Diagnostic {
	return Diagnostic{
		Code:     DiagSchemaReferenceUnresolved,
		Severity: SeverityWarning,
		Field:    ref,
		Message:  "schema reference " + ref + " could not be inlined (" + reason + "); the standalone schema enters pending review",
	}
}

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

// NormalizeRequestContractDialect normalizes every schema carried by a request
// contract (parameters and body) to the canonical dialect-independent form and
// appends the capability diagnostics for keywords that are not directly
// formable (SUI-009). It returns a new contract; the input is not mutated.
// Both extraction front-ends call this with the document dialect so the stored
// contract, the operator form and the server validator share one form.
func NormalizeRequestContractDialect(contract OperationRequestContract, dialect SchemaDialect) OperationRequestContract {
	notes := []DialectNote{}
	out := contract
	out.Parameters = make([]RequestParameterSpec, len(contract.Parameters))
	copy(out.Parameters, contract.Parameters)
	for i, param := range out.Parameters {
		out.Parameters[i].Schema = normalizeJSONSchemaDialect(param.Schema, dialect, &notes)
	}
	if out.Body != nil {
		body := *out.Body
		body.Schema = normalizeJSONSchemaDialect(body.Schema, dialect, &notes)
		out.Body = &body
	}
	if len(notes) > 0 {
		out.Diagnostics = append(out.Diagnostics, DialectNotesToDiagnostics(notes)...)
	}
	return out
}

func normalizeJSONSchemaDialect(schema JSONSchema, dialect SchemaDialect, notes *[]DialectNote) JSONSchema {
	if len(schema) == 0 {
		return schema
	}
	var node map[string]interface{}
	if err := json.Unmarshal(schema, &node); err != nil || node == nil {
		return schema
	}
	encoded, err := json.Marshal(NormalizeSchemaDialect(node, dialect, notes))
	if err != nil {
		return schema
	}
	return JSONSchema(encoded)
}

// CanonicalizeRequestContract applies both extraction canonicalizations in
// order: dialect normalization (SUI-009) then direction filtering (SUI-010).
// Both extraction front-ends call it, so the stored contract, the operator
// form and the server validator share one schema — `readOnly` fields never
// reach the request side.
func CanonicalizeRequestContract(contract OperationRequestContract, dialect SchemaDialect) OperationRequestContract {
	return FilterRequestContractDirection(NormalizeRequestContractDialect(contract, dialect))
}

// FilterRequestContractDirection removes `readOnly` fields from every schema
// a request contract carries (SUI-010). A flagged body root drops the body
// slot itself — a read-only payload has no request body to send. Returns a
// new contract; the input is not mutated.
func FilterRequestContractDirection(contract OperationRequestContract) OperationRequestContract {
	notes := []DirectionNote{}
	out := contract
	out.Parameters = make([]RequestParameterSpec, len(contract.Parameters))
	copy(out.Parameters, contract.Parameters)
	for i, param := range out.Parameters {
		out.Parameters[i].Schema = filterJSONSchemaDirection(param.Schema, SchemaDirectionRequest, &notes)
	}
	if out.Body != nil {
		body := *out.Body
		body.Schema = filterJSONSchemaDirection(body.Schema, SchemaDirectionRequest, &notes)
		out.Body = &body
		if len(body.Schema) == 0 {
			out.Body = nil
		}
	}
	if len(notes) > 0 {
		out.Diagnostics = append(out.Diagnostics, DirectionNotesToDiagnostics(notes)...)
	}
	return out
}

func filterJSONSchemaDirection(schema JSONSchema, direction SchemaDirection, notes *[]DirectionNote) JSONSchema {
	if len(schema) == 0 {
		return schema
	}
	var node map[string]interface{}
	if err := json.Unmarshal(schema, &node); err != nil || node == nil {
		return schema
	}
	filtered := FilterSchemaDirection(node, direction, notes)
	if filtered == nil {
		return JSONSchema("")
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return schema
	}
	return JSONSchema(encoded)
}

// FilterJSONSchemaDirection removes the fields a schema must not carry for the
// direction and reports the removals. The response-side output schema callers
// use it; a nil result means the whole schema belongs to the other side.
func FilterJSONSchemaDirection(schema JSONSchema, direction SchemaDirection, notes *[]DirectionNote) JSONSchema {
	if len(schema) == 0 {
		return schema
	}
	var node map[string]interface{}
	if err := json.Unmarshal(schema, &node); err != nil || node == nil {
		return schema
	}
	filtered := FilterSchemaDirection(node, direction, notes)
	if filtered == nil {
		return JSONSchema("")
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return schema
	}
	return JSONSchema(encoded)
}
