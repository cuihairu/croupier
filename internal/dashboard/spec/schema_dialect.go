package spec

import (
	"fmt"
	"sort"
	"strings"
)

// SchemaDialect identifies the OpenAPI schema dialect a document declares.
// The two dialects differ in how nullability and union types are expressed:
//   - OpenAPI 3.0 Schema Objects are a JSON-Schema subset where nullability is
//     the extension keyword `nullable: true` and `type` is a single string;
//   - OpenAPI 3.1 Schema Objects follow JSON Schema 2020-12 where nullability
//     and unions are `type: [<T>, "null"]` / `type: [<T1>, <T2>, ...]`.
//
// SUI-009 normalizes both dialects to one canonical internal form (2020-12
// style: `type` array with `"null"` last when the value may be null) so the
// stored contract, the operator-facing form and the server-side validator all
// agree on the same semantics — see docs/architecture/openapi-request-contract.md
// (Schema dialect normalization (SUI-009)).
type SchemaDialect string

const (
	SchemaDialectOpenAPI30 SchemaDialect = "openapi30"
	SchemaDialectOpenAPI31 SchemaDialect = "openapi31"
	SchemaDialectUnknown   SchemaDialect = "unknown"
)

// DetectSchemaDialect returns the declared dialect from a document root map.
// Swagger 2.0 documents (no `openapi` field) and unknown versions return
// Unknown; the normalization treats Unknown like 3.0 (a no-op for schemas that
// do not use `nullable`).
func DetectSchemaDialect(document map[string]interface{}) SchemaDialect {
	raw, _ := document["openapi"].(string)
	v := strings.TrimSpace(raw)
	if strings.HasPrefix(v, "3.1") {
		return SchemaDialectOpenAPI31
	}
	if strings.HasPrefix(v, "3.0") {
		return SchemaDialectOpenAPI30
	}
	return SchemaDialectUnknown
}

// DialectCapability is the classification of a schema keyword's support level
// in the OpenAPI→UI chain (SUI-009 / SUI-011):
//   - Supported: the keyword renders a real form control directly;
//   - Degraded: the keyword is not directly formable — the operator must use
//     JSON editing or add a manual binding / discriminator;
//   - Unsupported: the keyword cannot be executed by the platform.
//
// The Go classifier and its web mirror (web/src/utils/schemaDialect.ts) must
// return the same level for the same sample schema; the shared sample set is
// the support matrix documented in
// docs/architecture/openapi-sdk-descriptor-v2.md#json-schema-支持子集 and
// locked by the parity tests on both sides.
type DialectCapability string

const (
	DialectCapabilitySupported   DialectCapability = "supported"
	DialectCapabilityDegraded    DialectCapability = "degraded"
	DialectCapabilityUnsupported DialectCapability = "unsupported"
)

// DialectNote records one schema keyword that carries a capability judgment
// (SUI-009). Path is a JSON pointer into the schema (relative to the schema
// root); Keyword names the keyword; Capability is the judgment.
type DialectNote struct {
	Path       string            `json:"path"`
	Keyword    string            `json:"keyword"`
	Capability DialectCapability `json:"capability"`
	Message    string            `json:"message"`
}

// DiagSchemaDialectKeyword records a dialect keyword the schema-driven form
// cannot render directly (SUI-009). Field is the schema JSON pointer; the
// message names the keyword and the required handling (JSON editing or a
// manual binding). Severity is warning: the schema is still usable, but the
// keyword is never silently mis-rendered — it enters review.
const DiagSchemaDialectKeyword = "openapi_schema_dialect_keyword"

// DialectKeywordDiagnostic builds the proposal-level diagnostic for a note.
func DialectKeywordDiagnostic(note DialectNote) Diagnostic {
	return Diagnostic{
		Code:     DiagSchemaDialectKeyword,
		Severity: SeverityWarning,
		Field:    note.Path,
		Message:  note.Message,
	}
}

// DialectNotesToDiagnostics converts notes to contract diagnostics, sorted by
// (path, keyword) so the diagnostic order is deterministic regardless of the
// schema map iteration order — a re-imported document always yields the same
// contract byte-for-byte (SUI-007 determinism extends to dialect notes).
func DialectNotesToDiagnostics(notes []DialectNote) []Diagnostic {
	sorted := make([]DialectNote, len(notes))
	copy(sorted, notes)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Keyword < sorted[j].Keyword
	})
	out := make([]Diagnostic, 0, len(sorted))
	for _, note := range sorted {
		out = append(out, DialectKeywordDiagnostic(note))
	}
	return out
}

// NormalizeSchemaDialect canonically normalizes a schema map from either the
// OpenAPI 3.0 or 3.1 dialect to the shared internal form (SUI-009):
//
//   - `nullable: true` folds into the `type` array (`type: [T, "null"]`) and
//     the `nullable` key is removed, so nullability is expressed the same way
//     regardless of the declaring dialect;
//   - a `type` array has `"null"` moved last, so the primary (non-null) type
//     is always the first element (the form derives its control from it);
//   - a multi-type union with no `"null"` is kept verbatim but recorded as a
//     degraded capability note (not directly formable);
//   - conditional schemas (`if`/`then`/`else`, `dependentSchemas`) and tuples
//     (`prefixItems`, `items` as an array) are recorded as degraded capability
//     notes instead of being silently mis-rendered;
//   - subschema positions (properties, items, allOf/oneOf/anyOf, …) are
//     normalized recursively; non-schema JSON values (default/example/enum/…)
//     are passed through untouched.
//
// notes collects the capability judgments; it may be nil for callers that do
// not surface the boundary. The result is a deep copy of the input with the
// canonical nullability; schemas that do not use any of these keywords are
// returned structurally unchanged (the JSON text is byte-identical).
func NormalizeSchemaDialect(schema map[string]interface{}, dialect SchemaDialect, notes *[]DialectNote) map[string]interface{} {
	if schema == nil {
		return nil
	}
	return normalizeSchemaNode(schema, dialect, "", notes)
}

// ClassifySchemaSupport returns the worst-case capability of a whole schema
// tree (SUI-009). It is the sample-level judgment used by the front end and
// the server: supported when every keyword in the tree is directly formable,
// degraded when any conditional schema / tuple / multi-type union appears.
// The web mirror web/src/utils/schemaDialect.ts must classify the same samples
// identically (locked by the parity tests).
func ClassifySchemaSupport(schema map[string]interface{}) DialectCapability {
	if schema == nil {
		return DialectCapabilitySupported
	}
	if worst := classifyNode(schema); worst != DialectCapabilitySupported {
		return worst
	}
	return DialectCapabilitySupported
}

func classifyNode(node map[string]interface{}) DialectCapability {
	if hasConditionalSchemaKeyword(node) || hasTupleKeyword(node) || hasMultiTypeUnion(node["type"]) {
		return DialectCapabilityDegraded
	}
	// Recurse into every subschema position to find a deeper degraded keyword.
	for key, value := range node {
		for _, sub := range subschemasOf(key, value) {
			if sub != nil {
				if worst := classifyNode(sub); worst != DialectCapabilitySupported {
					return worst
				}
			}
		}
	}
	return DialectCapabilitySupported
}

func normalizeSchemaNode(node map[string]interface{}, dialect SchemaDialect, path string, notes *[]DialectNote) map[string]interface{} {
	out := make(map[string]interface{}, len(node))
	for key, value := range node {
		switch key {
		case "nullable", "type":
			continue // folded into the canonical type below
		default:
			out[key] = normalizeSchemaValue(key, value, dialect, path, notes)
		}
	}
	nullable, _ := node["nullable"].(bool)
	if typ := normalizeType(node["type"], nullable); typ != nil {
		out["type"] = typ
	}
	recordDialectNotes(node, path, notes)
	return out
}

// normalizeSchemaValue recurses into a subschema position only. Non-schema
// JSON values (default/example/enum/const/required, …) are passed through
// untouched so an enum/default object that happens to carry a "nullable" or
// "type" key is never mistaken for a schema.
func normalizeSchemaValue(key string, value interface{}, dialect SchemaDialect, path string, notes *[]DialectNote) interface{} {
	switch {
	case key == "items" || key == "additionalProperties" || key == "contains" ||
		key == "propertyNames" || key == "if" || key == "then" || key == "else" ||
		key == "not":
		if m, ok := value.(map[string]interface{}); ok {
			return normalizeSchemaNode(m, dialect, path+"/"+key, notes)
		}
		if key == "items" {
			// draft-07 tuple: items as an array of schemas.
			if arr, ok := value.([]interface{}); ok {
				return normalizeSubschemaArray(arr, dialect, path+"/"+key, notes)
			}
		}
		return value
	case key == "properties" || key == "patternProperties" || key == "$defs" ||
		key == "definitions" || key == "dependentSchemas":
		if m, ok := value.(map[string]interface{}); ok {
			nm := make(map[string]interface{}, len(m))
			for name, sub := range m {
				nm[name] = normalizeSubschemaValue(sub, dialect, path+"/"+key+"/"+escapeJSONPointer(name), notes)
			}
			return nm
		}
		return value
	case key == "allOf" || key == "anyOf" || key == "oneOf" || key == "prefixItems":
		if arr, ok := value.([]interface{}); ok {
			return normalizeSubschemaArray(arr, dialect, path+"/"+key, notes)
		}
		return value
	case key == "dependencies":
		// draft-07 property dependencies: value is a schema (map) or a
		// property-name array (not a schema).
		if m, ok := value.(map[string]interface{}); ok {
			nm := make(map[string]interface{}, len(m))
			for name, sub := range m {
				nm[name] = normalizeSubschemaValue(sub, dialect, path+"/"+key+"/"+escapeJSONPointer(name), notes)
			}
			return nm
		}
		return value
	default:
		return value
	}
}

func normalizeSubschemaValue(value interface{}, dialect SchemaDialect, path string, notes *[]DialectNote) interface{} {
	if m, ok := value.(map[string]interface{}); ok {
		return normalizeSchemaNode(m, dialect, path, notes)
	}
	return value
}

func normalizeSubschemaArray(arr []interface{}, dialect SchemaDialect, path string, notes *[]DialectNote) []interface{} {
	out := make([]interface{}, len(arr))
	for i, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			out[i] = normalizeSchemaNode(m, dialect, fmt.Sprintf("%s/%d", path, i), notes)
		} else {
			out[i] = item
		}
	}
	return out
}

// normalizeType returns the canonical type value for a schema node, folding
// `nullable` into the type array and moving "null" last. It returns nil when
// the node declares no type (any value is allowed; `nullable` alone adds
// nothing). Multi-type unions (more than one non-null type) keep their array
// form so the judgment stays faithful.
func normalizeType(raw interface{}, nullable bool) interface{} {
	switch typed := raw.(type) {
	case string:
		if nullable {
			return []interface{}{typed, "null"}
		}
		return typed
	case []interface{}:
		return normalizeTypeArray(typed, nullable)
	default:
		if nullable {
			// No declared type: `nullable` is already permissive (any value or
			// null) and is equivalent to no constraint — drop it.
			return nil
		}
		return raw
	}
}

func normalizeTypeArray(types []interface{}, nullable bool) []interface{} {
	hasNull := nullable
	nonNull := make([]interface{}, 0, len(types))
	for _, t := range types {
		if s, ok := t.(string); ok && s == "null" {
			hasNull = true
			continue
		}
		nonNull = append(nonNull, t)
	}
	if !hasNull {
		return types // no null: keep the declared order/values verbatim
	}
	out := make([]interface{}, 0, len(nonNull)+1)
	out = append(out, nonNull...)
	out = append(out, "null")
	return out
}

// recordDialectNotes classifies the current node's dialect keywords and
// appends a note for every keyword that is not directly formable (SUI-009).
func recordDialectNotes(node map[string]interface{}, path string, notes *[]DialectNote) {
	if notes == nil {
		return
	}
	for _, keyword := range conditionalSchemaKeywords(node) {
		*notes = append(*notes, DialectNote{
			Path:       path,
			Keyword:    keyword,
			Capability: DialectCapabilityDegraded,
			Message:    fmt.Sprintf("schema keyword %q (conditional schema) is not directly formable; the field requires JSON editing or a manual binding", keyword),
		})
	}
	for _, keyword := range tupleKeywords(node) {
		*notes = append(*notes, DialectNote{
			Path:       path,
			Keyword:    keyword,
			Capability: DialectCapabilityDegraded,
			Message:    fmt.Sprintf("schema keyword %q (tuple) is not directly formable; the field requires JSON editing or a manual binding", keyword),
		})
	}
	if types, ok := node["type"].([]interface{}); ok && multiNonNullTypes(types) {
		*notes = append(*notes, DialectNote{
			Path:       path,
			Keyword:    "type",
			Capability: DialectCapabilityDegraded,
			Message:    fmt.Sprintf("schema type union %s is not directly formable; the form uses the first type and JSON editing is required for the others", stringifyTypes(types)),
		})
	}
}

func conditionalSchemaKeywords(node map[string]interface{}) []string {
	var out []string
	for _, k := range []string{"if", "then", "else", "dependentSchemas"} {
		if _, ok := node[k]; ok {
			out = append(out, k)
		}
	}
	// draft-07 `dependencies` is conditional only when a value is a schema (a
	// plain property-name array is a plain dependency, not a schema keyword).
	if deps, ok := node["dependencies"].(map[string]interface{}); ok {
		for _, value := range deps {
			if _, isSchema := value.(map[string]interface{}); isSchema {
				out = append(out, "dependencies")
				break
			}
		}
	}
	return out
}

func tupleKeywords(node map[string]interface{}) []string {
	var out []string
	if _, ok := node["prefixItems"]; ok {
		out = append(out, "prefixItems")
	}
	if _, ok := node["items"].([]interface{}); ok {
		out = append(out, "items")
	}
	return out
}

func hasConditionalSchemaKeyword(node map[string]interface{}) bool {
	return len(conditionalSchemaKeywords(node)) > 0
}

func hasTupleKeyword(node map[string]interface{}) bool {
	return len(tupleKeywords(node)) > 0
}

func hasMultiTypeUnion(raw interface{}) bool {
	types, ok := raw.([]interface{})
	if !ok {
		return false
	}
	return multiNonNullTypes(types)
}

func multiNonNullTypes(types []interface{}) bool {
	count := 0
	for _, t := range types {
		if s, ok := t.(string); ok && s == "null" {
			continue
		}
		count++
		if count > 1 {
			return true
		}
	}
	return false
}

func stringifyTypes(types []interface{}) string {
	parts := make([]string, 0, len(types))
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%v", t))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// subschemasOf returns the subschema nodes nested under one schema key, used
// by ClassifySchemaSupport to walk the tree. Non-schema values (default,
// example, enum, required, …) are not visited.
func subschemasOf(key string, value interface{}) []map[string]interface{} {
	switch {
	case key == "properties" || key == "patternProperties" || key == "$defs" ||
		key == "definitions" || key == "dependentSchemas" || key == "dependencies":
		m, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		var out []map[string]interface{}
		for _, sub := range m {
			if s, ok := sub.(map[string]interface{}); ok {
				out = append(out, s)
			}
		}
		return out
	case key == "allOf" || key == "anyOf" || key == "oneOf" || key == "prefixItems" ||
		key == "items":
		// items may be a single schema (map) or a tuple (array of maps).
		if m, ok := value.(map[string]interface{}); ok {
			return []map[string]interface{}{m}
		}
		if arr, ok := value.([]interface{}); ok {
			var out []map[string]interface{}
			for _, sub := range arr {
				if s, ok := sub.(map[string]interface{}); ok {
					out = append(out, s)
				}
			}
			return out
		}
		return nil
	case key == "additionalProperties" || key == "contains" || key == "propertyNames" ||
		key == "if" || key == "then" || key == "else" || key == "not":
		if m, ok := value.(map[string]interface{}); ok {
			return []map[string]interface{}{m}
		}
		return nil
	default:
		return nil
	}
}

func escapeJSONPointer(segment string) string {
	segment = strings.ReplaceAll(segment, "~", "~0")
	segment = strings.ReplaceAll(segment, "/", "~1")
	return segment
}
