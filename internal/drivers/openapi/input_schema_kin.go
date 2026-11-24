package openapi

import (
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type requestOperation struct {
	path   string
	method string
}

// Swagger 2.0 and documents kin-openapi cannot load retain the map fallback.
func kinRequestContracts(spec []byte) map[requestOperation]*RequestContract {
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil || !strings.HasPrefix(doc.OpenAPI, "3.") || doc.Paths == nil {
		return nil
	}
	out := make(map[requestOperation]*RequestContract)
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			key := requestOperation{path: path, method: strings.ToUpper(method)}
			out[key] = ExtractRequest(path, method, item, op)
		}
	}
	return out
}

// legacyInputSchema joins a compatible set of parameters and object fields.
func legacyInputSchema(request *RequestContract, root map[string]any, body map[string]any) string {
	properties := make(map[string]interface{})
	var required []string

	for _, param := range request.Parameters {
		if param.Name == "" {
			continue
		}
		if param.Required {
			required = appendUniqueString(required, param.Name)
		}
		properties[param.Name] = parameterInputSchema(param, root)
	}

	bodyProps, _ := body["properties"].(map[string]any)
	for key, value := range bodyProps {
		properties[key] = value
	}
	bodyRequired, _ := body["required"].([]any)
	for _, item := range bodyRequired {
		if name, ok := item.(string); ok {
			required = appendUniqueString(required, name)
		}
	}

	schema := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	// Kin MarshalYAML 的产物再经 json 编解码，Marshal 恒成功。
	encoded, _ := json.Marshal(schema)
	return string(encoded)
}

func parameterInputSchema(param RequestParameter, root map[string]any) map[string]any {
	prop, _ := resolvedRequestSchema(param.Schema, root, nil)
	if len(prop) == 0 {
		prop = map[string]any{"type": "string"}
	}
	if param.Description != "" {
		prop["description"] = param.Description
	}
	if param.Deprecated {
		prop["deprecated"] = true
	}
	return prop
}

// resolvedRequestSchema retains the existing local reference behavior of
// input projections. issues collects the references that could not be
// inlined (SUI-008); nil keeps the legacy silent behavior.
func resolvedRequestSchema(raw json.RawMessage, root map[string]interface{}, issues *[]refIssue) (map[string]interface{}, bool) {
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	resolved, ok := resolveSchemaRefs(value, root, 0, issues).(map[string]interface{})
	return resolved, ok
}
