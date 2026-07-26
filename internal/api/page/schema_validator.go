package page

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

type pageComponentContract struct {
	AllowedProps map[string]struct{}
	Required     []string
	BindingUsage []spec.PageBindingUsage
	Validate     func(path string, props map[string]any, bindings map[string]spec.PageFunctionBinding) []spec.Diagnostic
}

var pageComponentContracts = map[string]pageComponentContract{
	"ConsolePage": {
		AllowedProps: propSet("schemaVersion", "pageKey", "resourceKey"),
		Required:     []string{"schemaVersion"},
		Validate:     validateConsolePageProps,
	},
	"QueryForm": {
		AllowedProps: propSet("bindingId", "inputMapping", "resultStateKey"),
		Required:     []string{"bindingId"},
		BindingUsage: []spec.PageBindingUsage{spec.BindingUsageQuery, spec.BindingUsageAction, spec.BindingUsageTask, spec.BindingUsageReport},
	},
	"DataTable": {
		AllowedProps: propSet("bindingId", "itemsPath", "totalPath", "pageField", "pageSizeField", "columns", "columnsPath", "rowActions"),
		Required:     []string{"bindingId", "itemsPath", "totalPath", "pageField", "pageSizeField"},
		BindingUsage: []spec.PageBindingUsage{spec.BindingUsageQuery, spec.BindingUsageReport},
		Validate:     validateDataTableProps,
	},
	"DetailPanel": {
		AllowedProps: propSet("bindingId", "stateKey", "dataPath"),
		Validate:     validateSourceProps("DetailPanel"),
	},
	"ActionButton": {
		AllowedProps: propSet("bindingId", "label", "risk", "inputMapping"),
		Required:     []string{"bindingId"},
		BindingUsage: []spec.PageBindingUsage{spec.BindingUsageAction, spec.BindingUsageTask},
		Validate:     validateActionButtonProps,
	},
	"ActionGroup": {
		AllowedProps: propSet("actions"),
		Validate:     validateActionGroupProps,
	},
	"ResultPanel": {
		AllowedProps: propSet("bindingId", "stateKey", "dataPath"),
		Validate:     validateSourceProps("ResultPanel"),
	},
	"TaskTimeline": {
		AllowedProps: propSet("bindingId", "stateKey"),
		Validate:     validateSourceProps("TaskTimeline"),
	},
	"ChartPanel": {
		AllowedProps: propSet("bindingId", "stateKey", "dataPath", "chartType", "categoryPath", "seriesPath", "valuePath"),
		Required:     []string{"dataPath", "chartType"},
		BindingUsage: []spec.PageBindingUsage{spec.BindingUsageReport},
		Validate:     validateSourceProps("ChartPanel"),
	},
}

var formilyFieldComponents = propSet(
	"Input",
	"Input.TextArea",
	"Password",
	"NumberPicker",
	"Select",
	"Switch",
	"DatePicker",
	"DatePicker.RangePicker",
	"TimePicker",
	"TimePicker.RangePicker",
	"ArrayTable",
	"ArrayItems",
	"ArrayCards",
	"ArrayCollapse",
	"ArrayTabs",
	"FormGrid",
	"FormCollapse",
	"FormTab",
	"FormStep",
	"Space",
	"Card",
	"Checkbox",
	"Checkbox.Group",
	"Radio",
	"Radio.Group",
	"Cascader",
	"TreeSelect",
	"Transfer",
	"Upload",
	"Upload.Dragger",
)

func propSet(keys ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
}

func validatePageSchema(schema spec.FormilySchema, bindings map[string]spec.PageFunctionBinding, publish bool) []spec.Diagnostic {
	if len(schema) == 0 {
		return []spec.Diagnostic{diagnostic("page_schema_missing", spec.SeverityError, "schema is required", "schema")}
	}
	var root map[string]any
	if err := json.Unmarshal(schema, &root); err != nil {
		return []spec.Diagnostic{diagnostic("page_schema_invalid_json", spec.SeverityError, "schema must be JSON object", "schema")}
	}
	var diags []spec.Diagnostic
	if root["type"] != "void" {
		diags = append(diags, diagnostic("page_root_type_invalid", spec.SeverityError, "schema root type must be void", "schema.type"))
	}
	component, _ := root["x-component"].(string)
	if component != "ConsolePage" {
		diags = append(diags, diagnostic("page_root_component_invalid", spec.SeverityError, "schema root x-component must be ConsolePage", "schema.x-component"))
	}
	walkPageSchema(root, "schema", bindings, publish, true, &diags)
	return diags
}

func walkPageSchema(
	node map[string]any,
	path string,
	bindings map[string]spec.PageFunctionBinding,
	publish bool,
	pageComponentContext bool,
	diags *[]spec.Diagnostic,
) {
	component, hasComponent := node["x-component"].(string)
	component = strings.TrimSpace(component)
	if hasComponent && component == "" {
		*diags = append(*diags, diagnostic("page_component_invalid", spec.SeverityError, "x-component must be a non-empty string", path+".x-component"))
	}

	nextPageComponentContext := pageComponentContext
	if component != "" {
		if contract, ok := pageComponentContracts[component]; ok {
			nextPageComponentContext = component != "QueryForm"
			if node["type"] != "void" {
				*diags = append(*diags, diagnostic("page_component_type_invalid", spec.SeverityError, component+" type must be void", path+".type"))
			}
			validatePageComponentProps(component, contract, path, node, bindings, publish, diags)
		} else if pageComponentContext {
			if _, ok := formilyFieldComponents[component]; ok {
				nextPageComponentContext = false
			} else {
				*diags = append(*diags, diagnostic("page_component_unknown", spec.SeverityError, "unknown page component: "+component, path+".x-component"))
			}
		} else if _, ok := formilyFieldComponents[component]; !ok {
			*diags = append(*diags, diagnostic("formily_component_unknown", spec.SeverityError, "unknown Formily field component: "+component, path+".x-component"))
		}
	}

	for key, child := range objectChildren(node["properties"]) {
		walkPageSchema(child, path+".properties."+key, bindings, publish, nextPageComponentContext, diags)
	}
	for index, child := range arrayChildren(node["items"]) {
		walkPageSchema(child, fmt.Sprintf("%s.items.%d", path, index), bindings, publish, nextPageComponentContext, diags)
	}
	if child, ok := objectChild(node["items"]); ok {
		walkPageSchema(child, path+".items", bindings, publish, nextPageComponentContext, diags)
	}
}

func validatePageComponentProps(
	component string,
	contract pageComponentContract,
	path string,
	node map[string]any,
	bindings map[string]spec.PageFunctionBinding,
	publish bool,
	diags *[]spec.Diagnostic,
) {
	props, ok := componentProps(node, path, diags)
	if !ok {
		return
	}
	for key := range props {
		if _, allowed := contract.AllowedProps[key]; !allowed {
			*diags = append(*diags, diagnostic("page_component_prop_unknown", spec.SeverityError, component+" does not support prop "+key, path+".x-component-props."+key))
		}
	}
	for _, key := range contract.Required {
		requireStringProp(props, key, path, diags)
	}
	if _, hasFunctionID := props["functionId"]; hasFunctionID {
		*diags = append(*diags, diagnostic("page_schema_function_id_forbidden", spec.SeverityError, "page schema must reference bindingId, not functionId", path+".x-component-props.functionId"))
	}
	if bindingID := validateBindingProp(props, path, bindings, diags); bindingID != "" && len(contract.BindingUsage) > 0 {
		validateBindingUsage(bindingID, contract.BindingUsage, path, bindings, diags)
	}
	if publish && component == "ConsolePage" {
		version, _ := props["schemaVersion"].(string)
		if version != rendererSchemaVersion {
			*diags = append(*diags, diagnostic("page_schema_version_invalid", spec.SeverityError, "ConsolePage schemaVersion must be "+rendererSchemaVersion, path+".x-component-props.schemaVersion"))
		}
	}
	if contract.Validate != nil {
		*diags = append(*diags, contract.Validate(path, props, bindings)...)
	}
}

func componentProps(node map[string]any, path string, diags *[]spec.Diagnostic) (map[string]any, bool) {
	raw, exists := node["x-component-props"]
	if !exists {
		return map[string]any{}, true
	}
	props, ok := raw.(map[string]any)
	if !ok {
		*diags = append(*diags, diagnostic("page_component_props_invalid", spec.SeverityError, "x-component-props must be an object", path+".x-component-props"))
		return nil, false
	}
	return props, true
}

func validateConsolePageProps(path string, props map[string]any, _ map[string]spec.PageFunctionBinding) []spec.Diagnostic {
	var diags []spec.Diagnostic
	for _, key := range []string{"pageKey", "resourceKey"} {
		validateOptionalStringProp(props, key, path, &diags)
	}
	return diags
}

func validateDataTableProps(path string, props map[string]any, bindings map[string]spec.PageFunctionBinding) []spec.Diagnostic {
	var diags []spec.Diagnostic
	for _, key := range []string{"itemsPath", "totalPath", "pageField", "pageSizeField", "columnsPath"} {
		validateOptionalStringProp(props, key, path, &diags)
	}
	hasColumns := false
	if rawColumns, exists := props["columns"]; exists {
		hasColumns = true
		validateColumns(rawColumns, path+".x-component-props.columns", &diags)
	}
	if !hasColumns {
		requireStringProp(props, "columnsPath", path, &diags)
	}
	if rawActions, exists := props["rowActions"]; exists {
		validateActions(rawActions, path+".x-component-props.rowActions", bindings, &diags)
	}
	return diags
}

func validateSourceProps(component string) func(string, map[string]any, map[string]spec.PageFunctionBinding) []spec.Diagnostic {
	return func(path string, props map[string]any, bindings map[string]spec.PageFunctionBinding) []spec.Diagnostic {
		var diags []spec.Diagnostic
		_, hasBinding := props["bindingId"]
		_, hasState := props["stateKey"]
		if !hasBinding && !hasState {
			diags = append(diags, diagnostic("page_component_source_missing", spec.SeverityError, component+" requires bindingId or stateKey", path+".x-component-props"))
		}
		for _, key := range []string{"stateKey", "dataPath", "chartType", "categoryPath", "seriesPath", "valuePath"} {
			validateOptionalStringProp(props, key, path, &diags)
		}
		if rawActions, exists := props["actions"]; exists {
			validateActions(rawActions, path+".x-component-props.actions", bindings, &diags)
		}
		return diags
	}
}

func validateActionButtonProps(path string, props map[string]any, _ map[string]spec.PageFunctionBinding) []spec.Diagnostic {
	var diags []spec.Diagnostic
	validateOptionalStringProp(props, "label", path, &diags)
	validateOptionalStringProp(props, "risk", path, &diags)
	if _, exists := props["inputMapping"]; !exists {
		diags = append(diags, diagnostic("page_component_prop_missing", spec.SeverityError, "inputMapping is required", path+".x-component-props.inputMapping"))
	} else {
		validateJSONMapping(props["inputMapping"], path+".x-component-props.inputMapping", &diags)
	}
	return diags
}

func validateActionGroupProps(path string, props map[string]any, bindings map[string]spec.PageFunctionBinding) []spec.Diagnostic {
	var diags []spec.Diagnostic
	rawActions, exists := props["actions"]
	if !exists {
		diags = append(diags, diagnostic("page_component_prop_missing", spec.SeverityError, "actions is required", path+".x-component-props.actions"))
		return diags
	}
	validateActions(rawActions, path+".x-component-props.actions", bindings, &diags)
	return diags
}

func validateBindingProp(props map[string]any, path string, bindings map[string]spec.PageFunctionBinding, diags *[]spec.Diagnostic) string {
	raw, exists := props["bindingId"]
	if !exists {
		return ""
	}
	bindingID, ok := raw.(string)
	if !ok || strings.TrimSpace(bindingID) == "" {
		*diags = append(*diags, diagnostic("page_schema_binding_id_invalid", spec.SeverityError, "bindingId must be a non-empty string", path+".x-component-props.bindingId"))
		return ""
	}
	bindingID = strings.TrimSpace(bindingID)
	if _, exists := bindings[bindingID]; !exists {
		*diags = append(*diags, diagnostic("page_schema_binding_unknown", spec.SeverityError, "bindingId is not defined: "+bindingID, path+".x-component-props.bindingId"))
	}
	return bindingID
}

func validateBindingUsage(bindingID string, allowed []spec.PageBindingUsage, path string, bindings map[string]spec.PageFunctionBinding, diags *[]spec.Diagnostic) {
	binding, exists := bindings[bindingID]
	if !exists {
		return
	}
	for _, usage := range allowed {
		if binding.Usage == usage {
			return
		}
	}
	*diags = append(*diags, diagnostic("page_binding_usage_mismatch", spec.SeverityError, "binding usage "+string(binding.Usage)+" is not valid for this component", path+".x-component-props.bindingId"))
}

func validateColumns(raw any, path string, diags *[]spec.Diagnostic) {
	columns, ok := raw.([]any)
	if !ok || len(columns) == 0 {
		*diags = append(*diags, diagnostic("page_component_prop_invalid", spec.SeverityError, "columns must be a non-empty array", path))
		return
	}
	for index, rawColumn := range columns {
		column, ok := rawColumn.(map[string]any)
		columnPath := fmt.Sprintf("%s.%d", path, index)
		if !ok {
			*diags = append(*diags, diagnostic("page_component_prop_invalid", spec.SeverityError, "column must be an object", columnPath))
			continue
		}
		requireStringValue(column, "title", columnPath, diags)
		requireStringValue(column, "dataIndex", columnPath, diags)
		validateOptionalStringValue(column, "key", columnPath, diags)
		for key := range column {
			if key != "title" && key != "dataIndex" && key != "key" {
				*diags = append(*diags, diagnostic("page_component_prop_unknown", spec.SeverityError, "DataTable column does not support prop "+key, columnPath+"."+key))
			}
		}
	}
}

func validateActions(raw any, path string, bindings map[string]spec.PageFunctionBinding, diags *[]spec.Diagnostic) {
	actions, ok := raw.([]any)
	if !ok || len(actions) == 0 {
		*diags = append(*diags, diagnostic("page_component_prop_invalid", spec.SeverityError, "actions must be a non-empty array", path))
		return
	}
	for index, rawAction := range actions {
		action, ok := rawAction.(map[string]any)
		actionPath := fmt.Sprintf("%s.%d", path, index)
		if !ok {
			*diags = append(*diags, diagnostic("page_component_prop_invalid", spec.SeverityError, "action must be an object", actionPath))
			continue
		}
		requireStringValue(action, "bindingId", actionPath, diags)
		validateOptionalStringValue(action, "label", actionPath, diags)
		validateOptionalStringValue(action, "risk", actionPath, diags)
		if _, exists := action["inputMapping"]; !exists {
			*diags = append(*diags, diagnostic("page_component_prop_missing", spec.SeverityError, "inputMapping is required", actionPath+".inputMapping"))
		} else {
			validateJSONMapping(action["inputMapping"], actionPath+".inputMapping", diags)
		}
		for key := range action {
			if key != "bindingId" && key != "label" && key != "risk" && key != "inputMapping" {
				*diags = append(*diags, diagnostic("page_component_prop_unknown", spec.SeverityError, "action does not support prop "+key, actionPath+"."+key))
			}
		}
		if bindingID, _ := action["bindingId"].(string); strings.TrimSpace(bindingID) != "" {
			bindingID = strings.TrimSpace(bindingID)
			if _, exists := bindings[bindingID]; !exists {
				*diags = append(*diags, diagnostic("page_schema_binding_unknown", spec.SeverityError, "bindingId is not defined: "+bindingID, actionPath+".bindingId"))
			} else {
				validateBindingUsage(bindingID, []spec.PageBindingUsage{spec.BindingUsageAction, spec.BindingUsageTask}, actionPath, bindings, diags)
			}
		}
	}
}

func requireStringProp(props map[string]any, key, path string, diags *[]spec.Diagnostic) {
	requireStringValue(props, key, path+".x-component-props", diags)
}

func validateJSONMapping(raw any, path string, diags *[]spec.Diagnostic) {
	if _, ok := raw.(map[string]any); !ok {
		*diags = append(*diags, diagnostic("page_component_prop_invalid", spec.SeverityError, "inputMapping must be an object", path))
	}
}

func requireStringValue(values map[string]any, key, path string, diags *[]spec.Diagnostic) {
	value, ok := values[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		*diags = append(*diags, diagnostic("page_component_prop_missing", spec.SeverityError, key+" is required", path+"."+key))
	}
}

func validateOptionalStringProp(props map[string]any, key, path string, diags *[]spec.Diagnostic) {
	validateOptionalStringValue(props, key, path+".x-component-props", diags)
}

func validateOptionalStringValue(values map[string]any, key, path string, diags *[]spec.Diagnostic) {
	if value, exists := values[key]; exists {
		if text, ok := value.(string); !ok || strings.TrimSpace(text) == "" {
			*diags = append(*diags, diagnostic("page_component_prop_invalid", spec.SeverityError, key+" must be a non-empty string", path+"."+key))
		}
	}
}

func objectChildren(value any) map[string]map[string]any {
	props, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]map[string]any)
	for key, raw := range props {
		child, ok := raw.(map[string]any)
		if ok {
			out[key] = child
		}
	}
	return out
}

func objectChild(value any) (map[string]any, bool) {
	child, ok := value.(map[string]any)
	return child, ok
}

func arrayChildren(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		child, ok := item.(map[string]any)
		if ok {
			out = append(out, child)
		}
	}
	return out
}
