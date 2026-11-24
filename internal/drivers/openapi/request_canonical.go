package openapi

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// canonicalizeRequest makes the request contract standalone and canonical
// before it is projected (SUI-008/SUI-009): local $ref pointers are inlined
// against the document root (unresolved references recorded), then every
// schema is normalized to the dialect-independent canonical form (3.0
// `nullable` / 3.1 type unions folded into one shape, degraded keywords
// recorded). It returns a copy — the input contract is not mutated — plus
// the contract-level diagnostics in stable order (references first, then
// dialect keywords).
func canonicalizeRequest(operationID string, request *RequestContract, root map[string]any) (*RequestContract, []spec.Diagnostic) {
	if request == nil {
		return nil, nil
	}
	dialect := spec.DetectSchemaDialect(root)
	var issues []refIssue
	var notes []spec.DialectNote
	out := &RequestContract{}
	if len(request.Parameters) > 0 {
		out.Parameters = make([]RequestParameter, len(request.Parameters))
		copy(out.Parameters, request.Parameters)
		for i := range out.Parameters {
			param := &out.Parameters[i]
			param.Schema = canonicalSchemaJSON(param.Schema, root, dialect, &issues, &notes)
			param.Content = canonicalContent(param.Content, root, dialect, &issues, &notes)
		}
	}
	if request.Body != nil {
		body := *request.Body
		body.Content = canonicalContent(request.Body.Content, root, dialect, &issues, &notes)
		out.Body = &body
	}
	diagnostics := refIssuesToDiagnostics(issues)
	diagnostics = append(diagnostics, spec.DialectNotesToDiagnostics(notes)...)
	for i := range diagnostics {
		diagnostics[i].FunctionID = operationID
	}
	return out, diagnostics
}

// canonicalContent canonicalizes every media-type schema of a content map.
// The input map is not mutated.
func canonicalContent(content map[string]RequestMedia, root map[string]any, dialect spec.SchemaDialect, issues *[]refIssue, notes *[]spec.DialectNote) map[string]RequestMedia {
	if len(content) == 0 {
		return content
	}
	out := make(map[string]RequestMedia, len(content))
	for mediaType, media := range content {
		media.Schema = canonicalSchemaJSON(media.Schema, root, dialect, issues, notes)
		out[mediaType] = media
	}
	return out
}

// canonicalSchemaJSON resolves one schema against the document root and
// normalizes its dialect. A schema that is not a JSON object (or empty) is
// kept verbatim.
func canonicalSchemaJSON(raw json.RawMessage, root map[string]any, dialect spec.SchemaDialect, issues *[]refIssue, notes *[]spec.DialectNote) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	resolved, ok := resolvedRequestSchema(raw, root, issues)
	if !ok {
		return raw
	}
	encoded, err := json.Marshal(spec.NormalizeSchemaDialect(resolved, dialect, notes))
	if err != nil {
		return raw
	}
	return encoded
}
