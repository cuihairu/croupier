package openapi

import (
	"encoding/json"
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
	properties := make(map[string]interface{})
	var required []string

	for _, param := range extractParameterSchemas(methodObj, root) {
		if param.name == "" {
			continue
		}
		if param.required {
			required = appendUniqueString(required, param.name)
		}
		properties[param.name] = param.schema
	}

	if resolved, ok := extractJSONBodySchema(methodObj, root); ok {
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

	schema := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	// Input originates from json.Unmarshal output, so Marshal cannot fail
	// (same shape as the spec merge in parseOpenAPISpec).
	encoded, _ := json.Marshal(schema)
	return string(encoded)
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
func extractParameterSchemas(methodObj, root map[string]interface{}) []derivedParam {
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
			if resolved, ok := resolveSchemaRefs(schema, root, 0).(map[string]interface{}); ok {
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
func extractJSONBodySchema(methodObj, root map[string]interface{}) (map[string]interface{}, bool) {
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
	resolved, ok := resolveSchemaRefs(schema, root, 0).(map[string]interface{})
	if !ok {
		return nil, false
	}
	return resolved, true
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
