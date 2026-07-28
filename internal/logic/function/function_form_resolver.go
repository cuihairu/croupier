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

type formResolveResult struct {
	Schema           interface{}
	Custom           bool
	HasDefault       bool
	FormSource       string
	FormSourceDetail string
}

func resolveFunctionForm(c config.Config, fn *model.Function) formResolveResult {
	var customForm, fileForm, defaultForm interface{}
	if fn.Metadata != nil {
		customForm = fn.Metadata["form"]
	}
	fileForm = loadFormConfigFromFiles(c, fn.FunctionID)
	// Function registration never supplies form UI. The default function form is
	// derived from the executable input schema after registration.
	inputSchema := extractInputSchema(fn)
	if inputSchema != nil {
		defaultForm = deriveFormSchemaFromJSONSchema(inputSchema)
	}
	if defaultForm == nil {
		defaultForm = BuildFallbackFormSchema(fn.FunctionID)
	}

	resultForm := customForm
	if resultForm == nil {
		resultForm = fileForm
	}
	if resultForm == nil {
		resultForm = defaultForm
	}

	formSource := "none"
	formSourceDetail := "no function form schema configured"
	switch {
	case customForm != nil:
		formSource = "custom_metadata"
		formSourceDetail = "metadata.form (custom override)"
	case fileForm != nil:
		formSource = "config_file_override"
		formSourceDetail = "configs/form/functions(.override) file"
	case defaultForm != nil:
		formSource = "generated_default"
		formSourceDetail = "generated default function form schema"
	}

	return formResolveResult{
		Schema:           resultForm,
		Custom:           customForm != nil,
		HasDefault:       fileForm != nil || defaultForm != nil,
		FormSource:       formSource,
		FormSourceDetail: formSourceDetail,
	}
}

func loadFormConfigFromFiles(c config.Config, functionID string) interface{} {
	if strings.TrimSpace(functionID) == "" {
		return nil
	}
	for _, baseDir := range formConfigBaseDirs(c) {
		base := readFormConfigFile(filepath.Join(baseDir, "functions"), functionID)
		override := readFormConfigFile(filepath.Join(baseDir, "functions.override"), functionID)
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

func formConfigBaseDirs(c config.Config) []string {
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

	if env := strings.TrimSpace(os.Getenv("CROUPIER_FORM_CONFIG_DIR")); env != "" {
		add(env)
	}
	if c.BootstrapData.BaseDir != "" {
		add(filepath.Join(c.BootstrapData.BaseDir, "form"))
	}
	add(filepath.Join("configs", "form"))
	add(filepath.Join("..", "..", "configs", "form"))
	return dirs
}

func readFormConfigFile(dir, functionID string) interface{} {
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
		if picked := pickFunctionFormConfig(parsed, functionID); picked != nil {
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

func pickFunctionFormConfig(raw interface{}, functionID string) interface{} {
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
	return v
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

// deriveFormSchemaFromJSONSchema converts a JSON Schema into a Formily Schema
// for the Dashboard form renderer. It maps JSON Schema types and constraints
// to Formily x-component and x-component-props.
func deriveFormSchemaFromJSONSchema(schema map[string]interface{}) map[string]interface{} {
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
		uiProperties[name] = buildFormilyProperty(name, prop)
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

func buildFormilyProperty(name string, prop map[string]interface{}) map[string]interface{} {
	title := firstNonEmptyString(prop["title"], prop["description"], name)
	component, decorator := formilyComponent(prop)
	uiProp := map[string]interface{}{
		"type":        firstNonEmptyString(prop["type"], "string"),
		"title":       title,
		"x-component": component,
	}
	if decorator != "" {
		uiProp["x-decorator"] = decorator
	}

	if desc, ok := prop["description"].(string); ok && desc != "" && desc != title {
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

	componentProps := buildFormilyComponentProps(prop)
	if len(componentProps) > 0 {
		uiProp["x-component-props"] = componentProps
	}

	if nested := buildNestedFormilyProperties(prop); len(nested) > 0 {
		uiProp["properties"] = nested
	}
	if items, ok := prop["items"].(map[string]interface{}); ok {
		uiProp["items"] = buildFormilyProperty(name+"Item", items)
	}
	return uiProp
}

func buildFormilyComponentProps(prop map[string]interface{}) map[string]interface{} {
	componentProps := map[string]interface{}{}
	if ph := buildFormilyPlaceholder(prop); ph != "" {
		componentProps["placeholder"] = ph
	}
	if min := prop["minimum"]; min != nil {
		componentProps["min"] = min
	}
	if max := prop["maximum"]; max != nil {
		componentProps["max"] = max
	}
	if minLen := prop["minLength"]; minLen != nil {
		componentProps["minLength"] = minLen
	}
	if maxLen := prop["maxLength"]; maxLen != nil {
		componentProps["maxLength"] = maxLen
	}
	if pat := prop["pattern"]; pat != nil {
		componentProps["pattern"] = pat
	}
	if mode := formilySelectMode(prop); mode != "" {
		componentProps["mode"] = mode
	}
	return componentProps
}

func buildNestedFormilyProperties(prop map[string]interface{}) map[string]interface{} {
	props, ok := prop["properties"].(map[string]interface{})
	if !ok || len(props) == 0 {
		return nil
	}
	nested := map[string]interface{}{}
	for name, raw := range props {
		child, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		nested[name] = buildFormilyProperty(name, child)
	}
	return nested
}

// formilyComponent maps a JSON Schema property to a Formily component name
// and decorator name.
func formilyComponent(prop map[string]interface{}) (component, decorator string) {
	typ, _ := prop["type"].(string)
	format, _ := prop["format"].(string)
	_, hasEnum := prop["enum"]

	switch {
	case typ == "boolean":
		return "Switch", "FormItem"
	case typ == "integer" || typ == "number":
		return "NumberPicker", "FormItem"
	case hasEnum:
		return "Select", "FormItem"
	case format == "date":
		return "DatePicker", "FormItem"
	case format == "date-time":
		return "DatePicker", "FormItem"
	case format == "time":
		return "TimePicker", "FormItem"
	case format == "textarea":
		return "Input.TextArea", "FormItem"
	case typ == "array":
		if formilySelectMode(prop) != "" {
			return "Select", "FormItem"
		}
		return "ArrayItems", "FormItem"
	case typ == "object":
		return "Card", "FormItem"
	default:
		return "Input", "FormItem"
	}
}

func formilySelectMode(prop map[string]interface{}) string {
	if typ, _ := prop["type"].(string); typ != "array" {
		return ""
	}
	items, _ := prop["items"].(map[string]interface{})
	if items == nil {
		return ""
	}
	if _, ok := items["enum"]; ok {
		return "multiple"
	}
	return ""
}

// buildFormilyPlaceholder generates a placeholder string from JSON Schema metadata.
func buildFormilyPlaceholder(prop map[string]interface{}) string {
	if ph, ok := prop["placeholder"].(string); ok && ph != "" {
		return ph
	}
	if desc, ok := prop["description"].(string); ok && desc != "" {
		return desc
	}
	format, _ := prop["format"].(string)
	switch format {
	case "date":
		return "请选择日期"
	case "date-time":
		return "请选择日期时间"
	case "email":
		return "请输入邮箱地址"
	}
	return ""
}

func firstNonEmptyString(values ...interface{}) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
