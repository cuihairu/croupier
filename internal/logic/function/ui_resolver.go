package function

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"gopkg.in/yaml.v3"
)

type uiResolveResult struct {
	Schema         interface{}
	Layout         interface{}
	Components     interface{}
	Custom         bool
	HasDefault     bool
	UISource       string
	UISourceDetail string
}

func resolveFunctionUI(c config.Config, fn *model.Function) uiResolveResult {
	var customUI, fileUI, defaultUI interface{}
	if fn.Metadata != nil {
		customUI = fn.Metadata["ui"]
	}
	fileUI = loadUIConfigFromFiles(c, fn.FunctionID)
	if fn.OpenAPISpec != nil {
		defaultUI = fn.OpenAPISpec["x-ui"]
	}
	if defaultUI == nil {
		// Try to derive UI from input_schema (JSON Schema) stored in OpenAPISpec.
		// This produces a much better UI than the hardcoded fallback fields.
		inputSchema := extractInputSchema(fn)
		if inputSchema != nil {
			defaultUI = deriveUISchemaFromJSONSchema(inputSchema)
		}
	}
	if defaultUI == nil {
		defaultUI = BuildFallbackUISchema(fn.FunctionID)
	}

	resultUI := customUI
	if resultUI == nil {
		resultUI = fileUI
	}
	if resultUI == nil {
		resultUI = defaultUI
	}

	uiSource := "none"
	uiSourceDetail := "no ui schema configured"
	switch {
	case customUI != nil:
		uiSource = "custom_metadata"
		uiSourceDetail = "metadata.ui (custom override)"
	case fileUI != nil:
		uiSource = "config_file_override"
		uiSourceDetail = "configs/ui/functions(.override) file"
	case defaultUI != nil:
		uiSource = "generated_default"
		uiSourceDetail = "generated default ui schema"
		if fn.OpenAPISpec != nil {
			if _, hasXUI := fn.OpenAPISpec["x-ui"]; hasXUI {
				uiSource = "openapi_x_ui"
				uiSourceDetail = "openapi_spec.x-ui (provider default)"
			}
		}
	}

	var layout interface{}
	var components interface{}
	if fn.Metadata != nil {
		layout = fn.Metadata["layout"]
		components = fn.Metadata["components"]
	}
	if layout == nil {
		layout = map[string]interface{}{
			"type": "grid",
			"cols": 2,
		}
	}
	if components == nil {
		components = map[string]interface{}{}
	}

	return uiResolveResult{
		Schema:         resultUI,
		Layout:         layout,
		Components:     components,
		Custom:         customUI != nil,
		HasDefault:     fileUI != nil || defaultUI != nil,
		UISource:       uiSource,
		UISourceDetail: uiSourceDetail,
	}
}

func loadUIConfigFromFiles(c config.Config, functionID string) interface{} {
	if strings.TrimSpace(functionID) == "" {
		return nil
	}
	for _, baseDir := range uiConfigBaseDirs(c) {
		base := readUIConfigFile(filepath.Join(baseDir, "functions"), functionID)
		override := readUIConfigFile(filepath.Join(baseDir, "functions.override"), functionID)
		switch {
		case base == nil && override == nil:
			continue
		case base == nil:
			return override
		case override == nil:
			return base
		default:
			return mergeAny(base, override)
		}
	}
	return nil
}

func uiConfigBaseDirs(c config.Config) []string {
	dirs := make([]string, 0, 4)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return
		}
		for _, existing := range dirs {
			if existing == abs {
				return
			}
		}
		dirs = append(dirs, abs)
	}

	if env := strings.TrimSpace(os.Getenv("CROUPIER_UI_CONFIG_DIR")); env != "" {
		add(env)
	}
	if c.BootstrapData.BaseDir != "" {
		add(filepath.Join(c.BootstrapData.BaseDir, "ui"))
	}
	add(filepath.Join("configs", "ui"))
	add(filepath.Join("..", "..", "configs", "ui"))
	return dirs
}

func readUIConfigFile(dir, functionID string) interface{} {
	for _, ext := range []string{"yaml", "yml", "json"} {
		path := filepath.Join(dir, functionID+"."+ext)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parsed, err := parseConfigContent(ext, data)
		if err != nil {
			continue
		}
		if picked := pickFunctionUIConfig(parsed, functionID); picked != nil {
			return picked
		}
	}
	return nil
}

func parseConfigContent(ext string, data []byte) (interface{}, error) {
	var out interface{}
	if ext == "json" {
		err := json.Unmarshal(data, &out)
		return out, err
	}
	err := yaml.Unmarshal(data, &out)
	return out, err
}

func pickFunctionUIConfig(raw interface{}, functionID string) interface{} {
	root, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	if byID, ok := root[functionID]; ok {
		return unwrapUIConfig(byID)
	}
	return unwrapUIConfig(root)
}

func unwrapUIConfig(v interface{}) interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return v
	}
	if ui, ok := m["x-ui"]; ok {
		return ui
	}
	return m
}

func mergeAny(base, override interface{}) interface{} {
	baseMap, baseOK := base.(map[string]interface{})
	overrideMap, overrideOK := override.(map[string]interface{})
	if !baseOK || !overrideOK {
		return override
	}
	merged := make(map[string]interface{}, len(baseMap))
	for k, v := range baseMap {
		merged[k] = v
	}
	for k, v := range overrideMap {
		if existing, ok := merged[k]; ok {
			merged[k] = mergeAny(existing, v)
		} else {
			merged[k] = v
		}
	}
	return merged
}

// extractInputSchema extracts the input JSON Schema from a function's stored data.
// It checks multiple locations in OpenAPISpec and Metadata.
func extractInputSchema(fn *model.Function) map[string]interface{} {
	if fn == nil {
		return nil
	}

	// Check OpenAPISpec for embedded request body schema
	if fn.OpenAPISpec != nil {
		if schema := extractSchemaFromOpenAPISpec(fn.OpenAPISpec); schema != nil {
			return schema
		}
	}

	// Check Metadata for input_schema (stored by SDK registration)
	if fn.Metadata != nil {
		if raw, ok := fn.Metadata["input_schema"]; ok {
			if schema, ok := raw.(map[string]interface{}); ok {
				return schema
			}
			if str, ok := raw.(string); ok && str != "" {
				var schema map[string]interface{}
				if json.Unmarshal([]byte(str), &schema) == nil {
					return schema
				}
			}
		}
		if raw, ok := fn.Metadata["inputSchema"]; ok {
			if schema, ok := raw.(map[string]interface{}); ok {
				return schema
			}
			if str, ok := raw.(string); ok && str != "" {
				var schema map[string]interface{}
				if json.Unmarshal([]byte(str), &schema) == nil {
					return schema
				}
			}
		}
	}

	return nil
}

// extractSchemaFromOpenAPISpec extracts the request body JSON Schema from an
// OpenAPI 3.0.3 Operation object stored as a map.
func extractSchemaFromOpenAPISpec(spec map[string]interface{}) map[string]interface{} {
	// Try requestBody.content.application/json.schema
	rb, ok := spec["requestBody"].(map[string]interface{})
	if !ok {
		return nil
	}
	content, ok := rb["content"].(map[string]interface{})
	if !ok {
		return nil
	}
	jsonMedia, ok := content["application/json"].(map[string]interface{})
	if !ok {
		return nil
	}
	schema, ok := jsonMedia["schema"].(map[string]interface{})
	if !ok {
		return nil
	}
	return schema
}

// deriveUISchemaFromJSONSchema converts a JSON Schema into a UI schema suitable
// for the Dashboard form renderer. It maps JSON Schema types and constraints
// to widget types and validation rules.
func deriveUISchemaFromJSONSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}

	schemaType, _ := schema["type"].(string)
	if schemaType != "object" {
		return nil
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok || len(props) == 0 {
		return nil
	}

	requiredSet := map[string]bool{}
	if reqArr, ok := schema["required"].([]interface{}); ok {
		for _, r := range reqArr {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	uiProperties := map[string]interface{}{}
	required := make([]string, 0)

	for name, raw := range props {
		prop, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		uiProp := map[string]interface{}{
			"type":  prop["type"],
			"title": firstNonEmptyString(prop["title"], prop["description"], name),
		}
		if desc, ok := prop["description"].(string); ok && desc != "" {
			uiProp["description"] = desc
		}
		if def := prop["default"]; def != nil {
			uiProp["default"] = def
		}
		if enum := prop["enum"]; enum != nil {
			uiProp["enum"] = enum
		}
		if f := prop["format"]; f != nil {
			uiProp["format"] = f
		}
		if min := prop["minimum"]; min != nil {
			uiProp["minimum"] = min
		}
		if max := prop["maximum"]; max != nil {
			uiProp["maximum"] = max
		}
		if minLen := prop["minLength"]; minLen != nil {
			uiProp["minLength"] = minLen
		}
		if maxLen := prop["maxLength"]; maxLen != nil {
			uiProp["maxLength"] = maxLen
		}
		if pat := prop["pattern"]; pat != nil {
			uiProp["pattern"] = pat
		}

		uiProperties[name] = uiProp
		if requiredSet[name] {
			required = append(required, name)
		}
	}

	result := map[string]interface{}{
		"type":       "object",
		"properties": uiProperties,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func firstNonEmptyString(values ...interface{}) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
