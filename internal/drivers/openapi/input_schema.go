package openapi

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// maxSchemaRefDepth bounds $ref resolution depth to guard against cyclic
// component references (tree-shaped schemas commonly self-reference). Past
// the limit the subtree collapses to an empty object instead of recursing
// forever.
const maxSchemaRefDepth = 16

// Resolution boundary reasons (SUI-008): the distinct ways a $ref can fail to
// inline into the standalone contract. They are recorded verbatim in the
// diagnostic message, so the proposal shows the reason instead of silently
// degrading the schema to an anonymous object.
const (
	refReasonMissing  = "missing target in the document"
	refReasonCyclic   = "cyclic reference past the inlining depth cap"
	refReasonExternal = "reference outside the document"
)

// refIssue records one schema $ref that resolveSchemaRefs could not fully
// inline into the standalone request contract (SUI-008). Ref is the reference
// pointer as declared; reason names the boundary that stopped the inlining.
type refIssue struct {
	ref    string
	reason string
}

// refIssuesToDiagnostics converts unresolved references into proposal-level
// diagnostics, deduplicated by (reference, reason): one document commonly
// references the same missing or external schema from several operations.
func refIssuesToDiagnostics(issues []refIssue) []spec.Diagnostic {
	out := make([]spec.Diagnostic, 0, len(issues))
	seen := map[string]bool{}
	for _, issue := range issues {
		key := issue.ref + "\x00" + issue.reason
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, spec.SchemaReferenceUnresolvedDiagnostic(issue.ref, issue.reason))
	}
	return out
}

// extractInputSchema derives a JSON Schema (as JSON text) describing the
// caller-facing input of one operation, so the platform can generate a real
// form instead of falling back to an empty one:
//
//   - every parameter (path/query/header/cookie) becomes a top-level
//     property — at call time all of them are read from the caller payload
//     (getParamValue), so they are all caller-facing;
//   - an application/json request body typed as object merges its properties
//     and required list into the top level; any other body shape is kept
//     under the "body" property;
//   - local $ref pointers (#/components/...) are resolved against the full
//     document; dangling refs collapse to an empty schema and non-local refs
//     are preserved as-is. Each boundary that stops the inlining is recorded
//     as a contract diagnostic (SUI-008) so the standalone schema enters
//     review instead of degrading silently.
//
// The returned text is always a valid JSON object (possibly an empty-object
// schema when the operation declares no input at all).
func extractInputSchema(methodObj, root map[string]interface{}) string {
	schema, _ := extractInputSchemaWithIssues(methodObj, root)
	return schema
}

// extractInputSchemaWithIssues is extractInputSchema plus the contract-level
// diagnostics of the map fallback path (SUI-008/SUI-009/SUI-010): unresolved
// schema references, degraded dialect keywords and dropped readOnly fields.
// The provider surfaces them as method diagnostics.
func extractInputSchemaWithIssues(methodObj, root map[string]interface{}) (string, []spec.Diagnostic) {
	var issues []refIssue
	var notes []spec.DialectNote
	var directionNotes []spec.DirectionNote
	properties := make(map[string]interface{})
	var required []string

	for _, param := range extractParameterSchemas(methodObj, root, &issues) {
		if param.name == "" {
			continue
		}
		if param.required {
			required = appendUniqueString(required, param.name)
		}
		filtered := spec.FilterSchemaDirection(param.schema, spec.SchemaDirectionRequest, &directionNotes)
		if filtered == nil {
			// A flagged parameter root keeps the parameter with an empty
			// schema (SUI-010): addressing must never break, so the field
			// stays visible in the flat schema instead of vanishing.
			filtered = map[string]interface{}{}
		}
		properties[param.name] = filtered
	}

	if resolved, ok := extractJSONBodySchema(methodObj, root, &issues); ok {
		// SUI-010: the request side loses its readOnly fields; a body whose
		// root is flagged belongs to the response side entirely and the
		// operation is left with no body to send.
		resolved = spec.FilterSchemaDirection(resolved, spec.SchemaDirectionRequest, &directionNotes)
		if resolved != nil {
			if typ, _ := resolved["type"].(string); typ == "object" {
				if bodyProps, ok := resolved["properties"].(map[string]interface{}); ok {
					for key, value := range bodyProps {
						properties[key] = value
					}
				}
				if bodyRequired, ok := resolved["required"].([]interface{}); ok {
					for _, item := range bodyRequired {
						if name, ok := item.(string); ok {
							required = appendUniqueString(required, name)
						}
					}
				}
			} else {
				properties["body"] = resolved
			}
		}
	}

	schema := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	// SUI-009: normalize the assembled schema to the canonical dialect form
	// so the fallback path matches the contract path (nullable folding,
	// degraded-keyword notes).
	dialect := spec.DetectSchemaDialect(root)
	schema = spec.NormalizeSchemaDialect(schema, dialect, &notes)
	// Input originates from json.Unmarshal output, so Marshal cannot fail
	// (same shape as the spec merge in parseOpenAPISpec).
	encoded, _ := json.Marshal(schema)
	diagnostics := refIssuesToDiagnostics(issues)
	diagnostics = append(diagnostics, spec.DialectNotesToDiagnostics(notes)...)
	diagnostics = append(diagnostics, spec.DirectionNotesToDiagnostics(directionNotes)...)
	return string(encoded), diagnostics
}

// derivedParam is one caller-facing parameter projected into the input schema.
type derivedParam struct {
	name     string
	required bool
	schema   map[string]interface{}
}

// extractParameterSchemas projects operation parameters into schema
// properties. Parameter-level description/deprecated ride along; the schema
// object itself is $ref-resolved and defaults to a string when absent.
func extractParameterSchemas(methodObj, root map[string]interface{}, issues *[]refIssue) []derivedParam {
	paramList, ok := methodObj["parameters"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]derivedParam, 0, len(paramList))
	for _, param := range paramList {
		pObj, ok := param.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := pObj["name"].(string)
		prop := map[string]interface{}{}
		if schema, ok := pObj["schema"].(map[string]interface{}); ok {
			if resolved, ok := resolveSchemaRefs(schema, root, 0, issues).(map[string]interface{}); ok {
				for key, value := range resolved {
					prop[key] = value
				}
			}
		}
		if len(prop) == 0 {
			prop["type"] = "string"
		}
		if desc, ok := pObj["description"].(string); ok && desc != "" {
			prop["description"] = desc
		}
		if deprecated, _ := pObj["deprecated"].(bool); deprecated {
			prop["deprecated"] = true
		}
		out = append(out, derivedParam{
			name:     name,
			required: pObj["required"] == true,
			schema:   prop,
		})
	}
	return out
}

// extractJSONBodySchema returns the $ref-resolved schema of the operation's
// application/json request body, if any.
func extractJSONBodySchema(methodObj, root map[string]interface{}, issues *[]refIssue) (map[string]interface{}, bool) {
	requestBody, ok := methodObj["requestBody"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	content, ok := requestBody["content"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	jsonContent, ok := content["application/json"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	schema, ok := jsonContent["schema"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	resolved, ok := resolveSchemaRefs(schema, root, 0, issues).(map[string]interface{})
	if !ok {
		return nil, false
	}
	return resolved, true
}

// resolveSchemaRefs deep-copies node while resolving local $ref pointers
// against root. Scalars pass through untouched. issues collects the references
// that could not be inlined (SUI-008); it may be nil for callers that do not
// surface the boundary (output-schema derivation, legacy input projection).
func resolveSchemaRefs(node interface{}, root map[string]interface{}, depth int, issues *[]refIssue) interface{} {
	switch typed := node.(type) {
	case map[string]interface{}:
		if ref, ok := typed["$ref"].(string); ok {
			return resolveRefPointer(ref, root, depth, issues)
		}
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = resolveSchemaRefs(value, root, depth, issues)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = resolveSchemaRefs(value, root, depth, issues)
		}
		return out
	default:
		return node
	}
}

// resolveRefPointer follows one local JSON pointer ("#/a/b"), then resumes
// deep resolution with an incremented depth so cyclic refs terminate at
// maxSchemaRefDepth. Every boundary that stops the inlining is recorded on
// issues (SUI-008) instead of degrading the schema silently.
func resolveRefPointer(ref string, root map[string]interface{}, depth int, issues *[]refIssue) interface{} {
	record := func(reason string) {
		if issues != nil {
			*issues = append(*issues, refIssue{ref: ref, reason: reason})
		}
	}
	if depth >= maxSchemaRefDepth {
		record(refReasonCyclic)
		return map[string]interface{}{"type": "object"}
	}
	if !strings.HasPrefix(ref, "#/") {
		// Non-local references (other documents / URLs) cannot be resolved
		// here; keep them for downstream tooling instead of dropping input.
		record(refReasonExternal)
		return map[string]interface{}{"$ref": ref}
	}
	var current interface{} = root
	for _, segment := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		obj, ok := current.(map[string]interface{})
		if !ok {
			record(refReasonMissing)
			return map[string]interface{}{"type": "object"}
		}
		next, ok := obj[segment]
		if !ok {
			record(refReasonMissing)
			return map[string]interface{}{"type": "object"}
		}
		current = next
	}
	return resolveSchemaRefs(current, root, depth+1, issues)
}

func appendUniqueString(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
