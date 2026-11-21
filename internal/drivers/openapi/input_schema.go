package openapi

import (
	"strings"
)

// maxSchemaRefDepth bounds $ref resolution depth to guard against cyclic
// component references (tree-shaped schemas commonly self-reference). Past
// the limit the subtree collapses to an empty object instead of recursing
// forever.
const maxSchemaRefDepth = 16

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
//     are preserved as-is.
//
// The returned text is always a valid JSON object (possibly an empty-object
// schema when the operation declares no input at all).
func extractInputSchema(methodObj, root map[string]interface{}) string {
	return extractContractInputSchema(ExtractOperationRequestContractMap(methodObj, root, nil))
}

// resolveSchemaRefs deep-copies node while resolving local $ref pointers
// against root. Scalars pass through untouched.
func resolveSchemaRefs(node interface{}, root map[string]interface{}, depth int) interface{} {
	switch typed := node.(type) {
	case map[string]interface{}:
		if ref, ok := typed["$ref"].(string); ok {
			return resolveRefPointer(ref, root, depth)
		}
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = resolveSchemaRefs(value, root, depth)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = resolveSchemaRefs(value, root, depth)
		}
		return out
	default:
		return node
	}
}

// resolveRefPointer follows one local JSON pointer ("#/a/b"), then resumes
// deep resolution with an incremented depth so cyclic refs terminate at
// maxSchemaRefDepth.
func resolveRefPointer(ref string, root map[string]interface{}, depth int) interface{} {
	if depth >= maxSchemaRefDepth {
		return map[string]interface{}{"type": "object"}
	}
	if !strings.HasPrefix(ref, "#/") {
		// Non-local references (other documents / URLs) cannot be resolved
		// here; keep them for downstream tooling instead of dropping input.
		return map[string]interface{}{"$ref": ref}
	}
	var current interface{} = root
	for _, segment := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		obj, ok := current.(map[string]interface{})
		if !ok {
			return map[string]interface{}{"type": "object"}
		}
		next, ok := obj[segment]
		if !ok {
			return map[string]interface{}{"type": "object"}
		}
		current = next
	}
	return resolveSchemaRefs(current, root, depth+1)
}

func appendUniqueString(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
