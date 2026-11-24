package openapi

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// canonicalizeRequest returns a copy of the contract whose schemas are
// standalone and canonical (SUI-008/SUI-009/SUI-010): local references are
// inlined against the document, the OpenAPI 3.0/3.1 dialect is normalized,
// and the request side is stripped of the fields it must not carry
// (`readOnly` is response-side). The returned diagnostics are the contract
// level ones — reference, dialect and direction — so the projection path
// never re-collects them; projection-level diagnostics stay the only ones
// ProjectInput adds itself.
//
// The contract is copied because the DTO keeps the raw snapshot: the
// canonical form is what gets projected and rendered, and it must not
// rewrite what is persisted.
func canonicalizeRequest(operationID string, request *RequestContract, root map[string]any) (*RequestContract, []spec.Diagnostic) {
	if request == nil {
		return nil, nil
	}
	dialect := spec.DetectSchemaDialect(root)
	var issues []refIssue
	var notes []spec.DialectNote
	var directionNotes []spec.DirectionNote
	out := &RequestContract{}
	if len(request.Parameters) > 0 {
		out.Parameters = make([]RequestParameter, len(request.Parameters))
		copy(out.Parameters, request.Parameters)
		for i := range out.Parameters {
			param := &out.Parameters[i]
			canonical, kept := canonicalSchemaJSON(param.Schema, root, dialect, &issues, &notes, &directionNotes)
			if !kept {
				// The parameter is kept with an empty schema: addressing
				// (a required path parameter) must never break, and the
				// form still shows the field instead of losing it silently.
				canonical = json.RawMessage(`{}`)
			}
			param.Schema = canonical
			param.Content = canonicalContent(param.Content, root, dialect, &issues, &notes, &directionNotes)
		}
	}
	if request.Body != nil {
		body := *request.Body
		body.Content = canonicalContent(request.Body.Content, root, dialect, &issues, &notes, &directionNotes)
		if len(body.Content) > 0 {
			out.Body = &body
		}
	}
	diagnostics := refIssuesToDiagnostics(issues)
	diagnostics = append(diagnostics, spec.DialectNotesToDiagnostics(notes)...)
	diagnostics = append(diagnostics, spec.DirectionNotesToDiagnostics(directionNotes)...)
	for i := range diagnostics {
		diagnostics[i].FunctionID = operationID
	}
	return out, diagnostics
}

// canonicalContent canonicalizes every declared media type, dropping the
// entries whose schema belongs to the response side entirely (SUI-010).
func canonicalContent(content map[string]RequestMedia, root map[string]any, dialect spec.SchemaDialect,
	issues *[]refIssue, notes *[]spec.DialectNote, directionNotes *[]spec.DirectionNote) map[string]RequestMedia {
	if len(content) == 0 {
		return nil
	}
	out := make(map[string]RequestMedia, len(content))
	for mediaType, media := range content {
		canonical, kept := canonicalSchemaJSON(media.Schema, root, dialect, issues, notes, directionNotes)
		if !kept {
			continue
		}
		media.Schema = canonical
		out[mediaType] = media
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// canonicalSchemaJSON returns the canonical standalone form of one raw schema:
// references inlined, dialect normalized, request-side direction filtering
// applied (SUI-010). The second result is false when the schema belongs to
// the response side entirely (its root is flagged readOnly) and must not
// appear in the request contract at all.
func canonicalSchemaJSON(raw json.RawMessage, root map[string]any, dialect spec.SchemaDialect,
	issues *[]refIssue, notes *[]spec.DialectNote, directionNotes *[]spec.DirectionNote) (json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw, true
	}
	resolved, ok := resolvedRequestSchema(raw, root, issues)
	if !ok {
		return raw, true
	}
	schema := spec.NormalizeSchemaDialect(resolved, dialect, notes)
	filtered := spec.FilterSchemaDirection(schema, spec.SchemaDirectionRequest, directionNotes)
	if filtered == nil {
		return nil, false
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return raw, true
	}
	return encoded, true
}
