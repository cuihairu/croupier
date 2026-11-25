package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// RequestCompatibility carries the SUI-011 classification of one operation's
// request contract: which tier of page generation it can reach, and the
// field-level diagnostics that justify that tier.
type RequestCompatibility struct {
	Status      string            `json:"status"`
	Diagnostics []spec.Diagnostic `json:"diagnostics,omitempty"`
}

// AssessRequestCompatibility classifies how far one operation's request
// contract can be taken into a generated page (SUI-011).
//
// The contract is canonicalized first (SUI-008/009/010) so the assessment looks
// at the same standalone form the projection and the form renderer look at:
// local references inlined, dialect normalized, response-side media dropped.
// Only the constructs that survive canonicalization are classified here, so the
// result never disagrees with what the generated form actually renders.
//
// Tiers:
//
//	direct   – nothing outside the supported subset was declared.
//	degraded – the input is understandable but cannot be shown as a structured
//	           form (a composite schema, a variant branch, a root array, a bare
//	           value); the operator edits the JSON payload instead.
//	manual   – the executor cannot send what was declared: an unresolvable
//	           reference, a discriminator-picked variant, or a body whose media
//	           types it cannot serialize at all. Manual is the tier that keeps an
//	           operation out of the directly executable default proposals; the
//	           proposal quality gate reads the diagnostic codes.
func AssessRequestCompatibility(operationID string, request *RequestContract, root map[string]any) RequestCompatibility {
	canonical, canonicalIssues := canonicalizeRequest(operationID, request, root)
	if canonical == nil {
		return RequestCompatibility{Status: spec.RequestCompatibilityDirect}
	}
	var findings []spec.Diagnostic
	for _, issue := range canonicalIssues {
		if issue.Code != spec.DiagSchemaReferenceUnresolved {
			continue
		}
		findings = append(findings, finding(operationID, spec.RequestCompatibilityManual, issue.Field,
			"request schema reference "+issue.Field+" cannot be resolved; the input shape is unknown until it is fixed in the source"))
	}
	if canonical.Body != nil {
		findings = append(findings, assessBody(operationID, canonical.Body)...)
	}
	for i, param := range canonical.Parameters {
		pointer := fmt.Sprintf("$.parameters[%d]", i)
		findings = append(findings, assessSchemaNodes(operationID, pointer+"/schema", param.Schema)...)
		if hasNonJSONMedia(param.Content) {
			findings = append(findings, finding(operationID, spec.RequestCompatibilityDegraded, pointer+"/content",
				"parameter "+param.In+" "+param.Name+" is declared as "+joinMediaTypes(param.Content)+
					", not application/json; the executor serializes parameters from the JSON schema"))
		}
	}
	diagnostics := dedupeAndSortDiagnostics(findings)
	return RequestCompatibility{Status: compatibilityStatus(diagnostics), Diagnostics: diagnostics}
}

// assessBody classifies the request body: the media types first (an operation
// is unsendable when nothing it declared can be serialized) and then the
// declared shape.
func assessBody(operationID string, body *RequestBody) []spec.Diagnostic {
	var findings []spec.Diagnostic
	if body == nil || len(body.Content) == 0 {
		return nil
	}
	jsonMediaType, hasJSON := pickJSONMedia(body.Content)
	formOnly := true
	for _, media := range sortedMediaTypes(body.Content) {
		pointer := fmt.Sprintf("$.body.content.%s", pointerToken(media))
		if mediaIsJSON(media) {
			if !hasJSON || media != jsonMediaType {
				continue
			}
			mediaEntry := body.Content[media]
			findings = append(findings, assessBodyShape(operationID, pointer+"/schema", mediaEntry.Schema)...)
			findings = append(findings, assessSchemaNodes(operationID, pointer+"/schema", mediaEntry.Schema)...)
			continue
		}
		if mediaIsForm(media) {
			// SUI-012: the executor can serialize this media type, but it picks
			// the serializer from the provider request body config, not from the
			// media types declared here. The generated page cannot express that
			// choice, so the operation is not directly executable even though it
			// is sendable.
			findings = append(findings, finding(operationID, spec.RequestCompatibilityDegraded, pointer,
				"request body media "+media+" is sent only when the executor request body type is configured for forms (requestBody.type: form); the generated page cannot make that choice"))
			continue
		}
		formOnly = false
		findings = append(findings, finding(operationID, spec.RequestCompatibilityManual, pointer,
			"request body media "+media+" is not sent by the executor; declare application/json to make the operation callable"))
	}
	if !hasJSON {
		if formOnly {
			findings = append(findings, finding(operationID, spec.RequestCompatibilityDegraded, "$.body.content",
				"request body declares no application/json media type (declared: "+joinMediaTypes(body.Content)+
					"); the executor sends it with the form serializer when configured"))
			return findings
		}
		findings = append(findings, finding(operationID, spec.RequestCompatibilityManual, "$.body.content",
			"request body declares no application/json media type (declared: "+joinMediaTypes(body.Content)+
				"); the executor cannot send those media types"))
	}
	return findings
}

// assessBodyShape flags the shapes the form renderer cannot turn into fields.
// A root array or a bare value still travels through the body wrapper, so they
// stay in the degraded tier rather than blocking execution.
func assessBodyShape(operationID string, pointer string, schema json.RawMessage) []spec.Diagnostic {
	object, ok := schemaObject(schema)
	if !ok {
		return nil
	}
	var typ string
	if err := json.Unmarshal(object["type"], &typ); err != nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "array":
		return []spec.Diagnostic{finding(operationID, spec.RequestCompatibilityDegraded, pointer,
			"request body is a root array; the form renderer has no field list for it, the JSON payload is edited directly")}
	case "string", "number", "integer", "boolean":
		return []spec.Diagnostic{finding(operationID, spec.RequestCompatibilityDegraded, pointer,
			"request body is a bare "+typ+" value; the JSON payload is edited directly instead of a form")}
	}
	return nil
}

// assessSchemaNodes walks one schema reporting the constructs the generator
// cannot render. Paths are JSON pointers into the request contract so the
// operator can locate each construct in the source document.
func assessSchemaNodes(operationID string, pointer string, schema json.RawMessage) []spec.Diagnostic {
	object, ok := schemaObject(schema)
	if !ok {
		return nil
	}
	var findings []spec.Diagnostic
	if _, ok := object["allOf"]; ok {
		findings = append(findings, finding(operationID, spec.RequestCompatibilityDegraded, pointer+"/allOf",
			"combined schema (allOf) is flattened before rendering, so field order and inherited constraints cannot be shown"))
	}
	if _, ok := object["oneOf"]; ok {
		findings = append(findings, finding(operationID, spec.RequestCompatibilityDegraded, pointer+"/oneOf",
			"variant branch (oneOf) is not selectable in the form; the JSON payload is edited with the chosen branch"))
	}
	if _, ok := object["anyOf"]; ok {
		findings = append(findings, finding(operationID, spec.RequestCompatibilityDegraded, pointer+"/anyOf",
			"variant branch (anyOf) is not selectable in the form; the JSON payload is edited with the chosen branch"))
	}
	if _, ok := object["discriminator"]; ok {
		findings = append(findings, finding(operationID, spec.RequestCompatibilityManual, pointer+"/discriminator",
			"the discriminator decides which variant applies, which the executor cannot pick on its own; send the concrete variant as application/json"))
	}
	// References are classified by the canonicalization result instead: every
	// unresolvable reference (missing target, cyclic past the depth cap, or
	// outside the document) already reaches this function as a reference
	// diagnostic, so re-walking for $ref would report one problem twice.
	for _, key := range sortedKeys(object) {
		if key == "$ref" {
			continue
		}
		findings = append(findings, assessSchemaNodes(operationID, pointer+"/"+pointerToken(key), object[key])...)
	}
	return findings
}

// finding records one incompatible construct at its field path.
func finding(operationID string, status, pointer, detail string) spec.Diagnostic {
	diagnostic := spec.SchemaCompatibilityDiagnostic(status, pointer, detail)
	diagnostic.FunctionID = operationID
	return diagnostic
}

// schemaObject decodes a raw schema into its keyword map.
func schemaObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, false
	}
	return object, true
}

// mediaIsJSON reports whether the media type, after stripping its parameters,
// is the JSON type the executor sends.
func mediaIsJSON(media string) bool {
	return normalizedMediaType(media) == "application/json"
}

// mediaIsForm reports whether the media type is the form-urlencoded type the
// executor serializes since SUI-012.
func mediaIsForm(media string) bool {
	return normalizedMediaType(media) == "application/x-www-form-urlencoded"
}

// normalizedMediaType strips parameters and case so a declared media type is
// compared with the types the executor actually sends.
func normalizedMediaType(media string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(media, ";", 2)[0]))
}

func hasNonJSONMedia(content map[string]RequestMedia) bool {
	for media := range content {
		if !mediaIsJSON(media) {
			return true
		}
	}
	return false
}

func pickJSONMedia(content map[string]RequestMedia) (string, bool) {
	for _, media := range sortedMediaTypes(content) {
		if mediaIsJSON(media) {
			return media, true
		}
	}
	return "", false
}

func sortedMediaTypes(content map[string]RequestMedia) []string {
	types := make([]string, 0, len(content))
	for media := range content {
		types = append(types, media)
	}
	sort.Strings(types)
	return types
}

func joinMediaTypes(content map[string]RequestMedia) string {
	return strings.Join(sortedMediaTypes(content), ", ")
}

func sortedKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// dedupeAndSortDiagnostics makes the output deterministic: map iteration order
// must not reach the stored contract or the proposal.
func dedupeAndSortDiagnostics(findings []spec.Diagnostic) []spec.Diagnostic {
	if len(findings) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(findings))
	out := make([]spec.Diagnostic, 0, len(findings))
	for _, diagnostic := range findings {
		key := diagnostic.Code + "\x00" + diagnostic.Field + "\x00" + diagnostic.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, diagnostic)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// compatibilityStatus derives the tier from the recorded diagnostics: one
// manual finding takes the operation manual, any degraded finding degrades it.
func compatibilityStatus(diagnostics []spec.Diagnostic) string {
	status := spec.RequestCompatibilityDirect
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == spec.DiagSchemaCompatibilityManual {
			return spec.RequestCompatibilityManual
		}
		status = spec.RequestCompatibilityDegraded
	}
	return status
}
