package function

import (
	"fmt"
	"strings"
)

var supportedFormilyComponents = map[string]struct{}{
	"Input":                  {},
	"Input.TextArea":         {},
	"Password":               {},
	"NumberPicker":           {},
	"Select":                 {},
	"Switch":                 {},
	"DatePicker":             {},
	"DatePicker.RangePicker": {},
	"TimePicker":             {},
	"TimePicker.RangePicker": {},
	"ArrayTable":             {},
	"ArrayItems":             {},
	"ArrayCards":             {},
	"ArrayCollapse":          {},
	"ArrayTabs":              {},
	"FormGrid":               {},
	"FormCollapse":           {},
	"FormTab":                {},
	"FormStep":               {},
	"Space":                  {},
	"Card":                   {},
	"Checkbox":               {},
	"Checkbox.Group":         {},
	"Radio":                  {},
	"Radio.Group":            {},
	"Cascader":               {},
	"TreeSelect":             {},
	"Transfer":               {},
	"Upload":                 {},
	"Upload.Dragger":         {},
}

var forbiddenUISchemaKeys = []string{
	"fields",
	"ui:layout",
	"ui:groups",
	"ui:order",
	"widget",
	"ui:widget",
}

func validateFormilySchema(schema interface{}) error {
	root, ok := schema.(map[string]interface{})
	if !ok || root == nil {
		return fmt.Errorf("formily schema must be an object")
	}
	for _, key := range forbiddenUISchemaKeys {
		if _, exists := root[key]; exists {
			return fmt.Errorf("legacy ui key %q is not allowed in function ui schema", key)
		}
	}
	if typ, _ := root["type"].(string); typ != "object" {
		return fmt.Errorf("formily schema top-level type must be object")
	}
	props, ok := root["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("formily schema top-level properties must be an object")
	}
	if len(props) == 0 {
		return fmt.Errorf("formily schema must contain at least one field")
	}
	if rawRequired, exists := root["required"]; exists {
		if err := validateStringArray(rawRequired, "$.required"); err != nil {
			return err
		}
	}
	if !hasFormilyMarker(root) {
		return fmt.Errorf("formily schema fields must declare x-component or x-decorator")
	}
	return validateFormilyNode(root, "$")
}

func validateFormilyNode(node interface{}, path string) error {
	m, ok := node.(map[string]interface{})
	if !ok || m == nil {
		return fmt.Errorf("%s must be an object", path)
	}
	for _, key := range forbiddenUISchemaKeys {
		if _, exists := m[key]; exists {
			return fmt.Errorf("%s uses legacy ui key %q", path, key)
		}
	}
	if rawComponent, exists := m["x-component"]; exists {
		component, ok := rawComponent.(string)
		if !ok || strings.TrimSpace(component) == "" {
			return fmt.Errorf("%s.x-component must be a non-empty string", path)
		}
		if _, ok := supportedFormilyComponents[component]; !ok {
			return fmt.Errorf("%s.x-component is not supported: %s", path, component)
		}
	}
	if rawDecorator, exists := m["x-decorator"]; exists {
		if _, ok := rawDecorator.(string); !ok {
			return fmt.Errorf("%s.x-decorator must be a string", path)
		}
	}
	if rawProps, exists := m["x-component-props"]; exists {
		if _, ok := rawProps.(map[string]interface{}); !ok {
			return fmt.Errorf("%s.x-component-props must be an object", path)
		}
	}
	if rawProperties, exists := m["properties"]; exists {
		props, ok := rawProperties.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s.properties must be an object", path)
		}
		for key, child := range props {
			if err := validateFormilyNode(child, path+".properties."+key); err != nil {
				return err
			}
		}
	}
	if rawItems, exists := m["items"]; exists {
		switch items := rawItems.(type) {
		case map[string]interface{}:
			if err := validateFormilyNode(items, path+".items"); err != nil {
				return err
			}
		case []interface{}:
			for idx, child := range items {
				if err := validateFormilyNode(child, fmt.Sprintf("%s.items.%d", path, idx)); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("%s.items must be an object or object array", path)
		}
	}
	return nil
}

func hasFormilyMarker(node interface{}) bool {
	m, ok := node.(map[string]interface{})
	if !ok || m == nil {
		return false
	}
	if component, ok := m["x-component"].(string); ok && strings.TrimSpace(component) != "" {
		return true
	}
	if decorator, ok := m["x-decorator"].(string); ok && strings.TrimSpace(decorator) != "" {
		return true
	}
	if props, ok := m["properties"].(map[string]interface{}); ok {
		for _, child := range props {
			if hasFormilyMarker(child) {
				return true
			}
		}
	}
	switch items := m["items"].(type) {
	case map[string]interface{}:
		return hasFormilyMarker(items)
	case []interface{}:
		for _, child := range items {
			if hasFormilyMarker(child) {
				return true
			}
		}
	}
	return false
}

func validateStringArray(value interface{}, path string) error {
	items, ok := value.([]interface{})
	if !ok {
		if _, ok := value.([]string); ok {
			return nil
		}
		return fmt.Errorf("%s must be a string array", path)
	}
	for idx, item := range items {
		if _, ok := item.(string); !ok {
			return fmt.Errorf("%s.%d must be a string", path, idx)
		}
	}
	return nil
}
