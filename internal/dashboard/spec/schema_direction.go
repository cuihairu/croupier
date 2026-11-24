package spec

import (
	"sort"
)

// Schema field direction (SUI-010): OpenAPI `readOnly`/`writeOnly` decide
// which side of the contract a property belongs to. The request schema must
// not carry `readOnly` fields (the client must not send them) and the
// response schema must not carry `writeOnly` fields (the server must not
// return them); the `required` set of each side drops the fields that were
// removed. Nested fields and reference-inlined fields follow the same rule —
// the filter runs after $ref inlining (SUI-008) and dialect normalization
// (SUI-009), so it sees the standalone canonical schema.

// SchemaDirection names the side of the contract a schema is being filtered
// for. Request schemas drop `readOnly` fields; response schemas drop
// `writeOnly` fields.
type SchemaDirection string

const (
	SchemaDirectionRequest  SchemaDirection = "request"
	SchemaDirectionResponse SchemaDirection = "response"
)

// droppedKeyword returns the keyword that names the opposite-side fields for
// the direction.
func droppedKeyword(direction SchemaDirection) string {
	if direction == SchemaDirectionRequest {
		return "readOnly"
	}
	return "writeOnly"
}

// DiagSchemaReadonlyDropped records a `readOnly` field removed from a request
// schema (SUI-010). Field is the schema JSON pointer; the operator sees why
// the field is absent from the input form instead of mistaking the drop for a
// descriptor bug.
const DiagSchemaReadonlyDropped = "openapi_schema_readonly_dropped"

// DiagSchemaWriteonlyDropped records a `writeOnly` field removed from a
// response schema (SUI-010).
const DiagSchemaWriteonlyDropped = "openapi_schema_writeonly_dropped"

// DirectionNote records one subschema dropped by direction filtering. Path is
// a JSON pointer into the schema root; Keyword is `readOnly` or `writeOnly`.
type DirectionNote struct {
	Path    string `json:"path"`
	Keyword string `json:"keyword"`
	Message string `json:"message"`
}

// DirectionNoteDiagnosticCode maps a note's keyword to its diagnostic code.
func DirectionNoteDiagnosticCode(keyword string) string {
	if keyword == "readOnly" {
		return DiagSchemaReadonlyDropped
	}
	return DiagSchemaWriteonlyDropped
}

// DirectionNotesToDiagnostics converts notes to contract diagnostics, sorted
// by (path, keyword) so the order is deterministic regardless of the schema
// map iteration order — the same document always yields the same diagnostics
// (SUI-007 determinism, extended to direction notes like SUI-009's).
func DirectionNotesToDiagnostics(notes []DirectionNote) []Diagnostic {
	sorted := make([]DirectionNote, len(notes))
	copy(sorted, notes)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Keyword < sorted[j].Keyword
	})
	out := make([]Diagnostic, 0, len(sorted))
	for _, note := range sorted {
		out = append(out, Diagnostic{
			Code:     DirectionNoteDiagnosticCode(note.Keyword),
			Severity: SeverityWarning,
			Field:    note.Path,
			Message:  note.Message,
		})
	}
	return out
}

// FilterSchemaDirection returns a deep copy of the schema with the fields the
// given side must not carry removed (SUI-010): request schemas lose
// `readOnly` fields, response schemas lose `writeOnly` fields. Every removal
// also removes the field from the enclosing `required` list, so a required
// read-only field never makes the request unsendable.
//
// Recursion covers the subschema positions that carry named fields or element
// schemas — properties, items, additionalProperties, allOf/anyOf/oneOf
// members and dependentSchemas entries. Tuple positions (`prefixItems`,
// `items` as an array) and conditional schemas (if/then/else) are recursed
// but their members are never dropped: repositioning a tuple member or
// removing a conditional branch changes tuple/branch semantics, and those
// keywords are already classified degraded (SUI-009). `$defs`/`definitions`
// entries are left untouched — after $ref inlining (SUI-008) they are unused
// material, not rendered schema.
//
// The flags that match the direction survive (`writeOnly` stays on a request
// field, `readOnly` stays on a response field) — both validators and the form
// ignore unknown keywords, and the flag keeps the direction information
// readable. Schemas without any such keyword are returned structurally
// unchanged (the JSON text is byte-identical). notes collects the removals;
// it may be nil for callers that do not surface them.
func FilterSchemaDirection(schema map[string]interface{}, direction SchemaDirection, notes *[]DirectionNote) map[string]interface{} {
	if schema == nil {
		return nil
	}
	keyword := droppedKeyword(direction)
	if flagTrue(schema[keyword]) {
		// A flagged root means the whole payload belongs to the other
		// side: the request body slot / output schema is dropped entirely.
		recordDirectionNote(notes, "", keyword, direction)
		return nil
	}
	// The input is not mutated: the filter works on a deep copy.
	node := deepCopyMap(schema)
	dropped := false
	filterDirectionNode(node, keyword, "", notes, &dropped)
	return node
}

func recordDirectionNote(notes *[]DirectionNote, path, keyword string, direction SchemaDirection) {
	if notes == nil {
		return
	}
	side := "response"
	if direction == SchemaDirectionRequest {
		side = "request"
	}
	*notes = append(*notes, DirectionNote{
		Path:    path,
		Keyword: keyword,
		Message: keyword + " field dropped from the " + side + " schema (required set adjusted)",
	})
}

// flagTrue reports whether the raw value is JSON `true`.
func flagTrue(value interface{}) bool {
	flag, ok := value.(bool)
	return ok && flag
}

func filterDirectionNode(node map[string]interface{}, keyword, path string, notes *[]DirectionNote, dropped *bool) {
	*dropped = dropFlaggedProperties(node, keyword, path, notes, *dropped) || *dropped
	*dropped = filterFlaggedSubschema(node, "items", keyword, path, notes, *dropped) || *dropped
	*dropped = filterFlaggedSubschema(node, "additionalProperties", keyword, path, notes, *dropped) || *dropped
	for _, combinator := range []string{"allOf", "anyOf", "oneOf"} {
		*dropped = filterFlaggedCombinator(node, combinator, keyword, path, notes, *dropped) || *dropped
	}
	*dropped = filterDirectionDependentSchemas(node, keyword, path, notes, *dropped) || *dropped
	// Conditional / tuple members are recursed but never repositioned.
	for _, single := range []string{"if", "then", "else", "not", "contains", "propertyNames"} {
		if sub, ok := node[single].(map[string]interface{}); ok {
			filterDirectionNode(sub, keyword, joinPointer(path, single), notes, dropped)
		}
	}
	if props, ok := node["patternProperties"].(map[string]interface{}); ok {
		for key, sub := range props {
			if subMap, ok := sub.(map[string]interface{}); ok {
				filterDirectionNode(subMap, keyword, joinPointer(path, "patternProperties/"+key), notes, dropped)
			}
		}
	}
	if items, ok := node["items"].([]interface{}); ok {
		for i, member := range items {
			if memberMap, ok := member.(map[string]interface{}); ok {
				filterDirectionNode(memberMap, keyword, indexedPointer(path, "items", i), notes, dropped)
			}
		}
	}
	if members, ok := node["prefixItems"].([]interface{}); ok {
		for i, member := range members {
			if memberMap, ok := member.(map[string]interface{}); ok {
				filterDirectionNode(memberMap, keyword, indexedPointer(path, "prefixItems", i), notes, dropped)
			}
		}
	}
	adjustRequired(node, *dropped)
}

// dropFlaggedProperties removes properties flagged with the opposite-side
// keyword and recurses into the survivors. Reports whether anything dropped.
func dropFlaggedProperties(node map[string]interface{}, keyword, path string, notes *[]DirectionNote, dropped bool) bool {
	properties, ok := node["properties"].(map[string]interface{})
	if !ok {
		return dropped
	}
	for key, value := range properties {
		subMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		if flagTrue(subMap[keyword]) {
			delete(properties, key)
			recordDirectionNote(notes, joinPointer(path, "properties/"+key), keyword, directionForKeyword(keyword))
			dropped = true
			continue
		}
		filterDirectionNode(subMap, keyword, joinPointer(path, "properties/"+key), notes, &dropped)
	}
	return dropped
}

// filterFlaggedSubschema removes a single subschema position (items /
// additionalProperties) when it is flagged with the opposite-side keyword;
// otherwise it recurses into it.
func filterFlaggedSubschema(node map[string]interface{}, key, keyword, path string, notes *[]DirectionNote, dropped bool) bool {
	subMap, ok := node[key].(map[string]interface{})
	if !ok {
		return dropped
	}
	if flagTrue(subMap[keyword]) {
		delete(node, key)
		recordDirectionNote(notes, joinPointer(path, key), keyword, directionForKeyword(keyword))
		return true
	}
	filterDirectionNode(subMap, keyword, joinPointer(path, key), notes, &dropped)
	return dropped
}

// filterFlaggedCombinator removes flagged members from an allOf/anyOf/oneOf
// array and recurses into the survivors.
func filterFlaggedCombinator(node map[string]interface{}, key, keyword, path string, notes *[]DirectionNote, dropped bool) bool {
	members, ok := node[key].([]interface{})
	if !ok {
		return dropped
	}
	kept := make([]interface{}, 0, len(members))
	changed := false
	for i, member := range members {
		memberMap, ok := member.(map[string]interface{})
		if !ok {
			kept = append(kept, member)
			continue
		}
		if flagTrue(memberMap[keyword]) {
			recordDirectionNote(notes, indexedPointer(path, key, i), keyword, directionForKeyword(keyword))
			changed = true
			dropped = true
			continue
		}
		sub := memberMap
		subDropped := false
		filterDirectionNode(sub, keyword, indexedPointer(path, key, i), notes, &subDropped)
		kept = append(kept, sub)
	}
	if changed {
		node[key] = kept
	}
	return dropped
}

// filterDirectionDependentSchemas removes flagged dependentSchemas entries
// (keyed by property name, like properties) and recurses into the survivors.
func filterDirectionDependentSchemas(node map[string]interface{}, keyword, path string, notes *[]DirectionNote, dropped bool) bool {
	dependent, ok := node["dependentSchemas"].(map[string]interface{})
	if !ok {
		return dropped
	}
	for key, value := range dependent {
		subMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		if flagTrue(subMap[keyword]) {
			delete(dependent, key)
			recordDirectionNote(notes, joinPointer(path, "dependentSchemas/"+key), keyword, directionForKeyword(keyword))
			dropped = true
			continue
		}
		filterDirectionNode(subMap, keyword, joinPointer(path, "dependentSchemas/"+key), notes, &dropped)
	}
	return dropped
}

// adjustRequired drops removed fields from the required list. Only schemas
// that actually lost a field are adjusted — untouched schemas stay
// byte-identical even if their required list already names absent keys.
func adjustRequired(node map[string]interface{}, dropped bool) {
	if !dropped {
		return
	}
	properties, ok := node["properties"].(map[string]interface{})
	if !ok {
		return
	}
	required, ok := node["required"].([]interface{})
	if !ok {
		return
	}
	kept := make([]interface{}, 0, len(required))
	for _, name := range required {
		if key, ok := name.(string); ok {
			if _, present := properties[key]; !present {
				continue
			}
		}
		kept = append(kept, name)
	}
	if len(kept) == 0 {
		delete(node, "required")
		return
	}
	node["required"] = kept
}

// directionForKeyword maps a keyword back to the direction whose side the
// keyword names (readOnly → request drops it; writeOnly → response drops it).
func directionForKeyword(keyword string) SchemaDirection {
	if keyword == "readOnly" {
		return SchemaDirectionRequest
	}
	return SchemaDirectionResponse
}

func joinPointer(path, token string) string {
	if path == "" {
		return "/" + token
	}
	return path + "/" + token
}

func indexedPointer(path, key string, index int) string {
	return joinPointer(path, key) + "/" + itoa(index)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

// deepCopyMap copies the map tree the filter mutates, so the input schema is
// never aliased.
func deepCopyMap(input map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = deepCopyValue(value)
	}
	return out
}

func deepCopyValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return deepCopyMap(typed)
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = deepCopyValue(item)
		}
		return out
	default:
		return value
	}
}
