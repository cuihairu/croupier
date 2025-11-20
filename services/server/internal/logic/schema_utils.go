package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func requireSchemaDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("schema dir not configured")
	}
	return dir, nil
}

func sanitizeSchemaID(id string) string {
	builder := strings.Builder{}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return builder.String()
}

func ensureSchemaDirectory(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func schemaFilePath(dir, id string) string {
	return filepath.Join(dir, id+".schema.json")
}

func uiSchemaFilePath(dir, id string) string {
	return filepath.Join(dir, id+".uischema.json")
}

func readJSONFile(path string, fallback interface{}) (interface{}, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback, nil, err
	}
	if fallback == nil {
		var out interface{}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, data, err
		}
		return out, data, nil
	}
	if err := json.Unmarshal(data, &fallback); err != nil {
		return fallback, data, err
	}
	return fallback, data, nil
}

func defaultUISchema(schema map[string]interface{}) map[string]interface{} {
	ui := map[string]interface{}{
		"type":        "object",
		"displayType": "row",
		"properties":  map[string]interface{}{},
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return ui
	}
	uiProps := ui["properties"].(map[string]interface{})
	for name, prop := range props {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}
		entry := map[string]interface{}{
			"title": propMap["title"],
		}
		switch propMap["type"] {
		case "boolean":
			entry["widget"] = "switch"
		case "number", "integer":
			entry["widget"] = "number"
		case "array":
			entry["widget"] = "list"
		case "object":
			entry["widget"] = "object"
		default:
			entry["widget"] = "input"
		}
		uiProps[name] = entry
	}
	return ui
}

func validateSchemaPayload(schema map[string]interface{}) error {
	if len(schema) == 0 {
		return errors.New("schema payload required")
	}
	if _, ok := schema["type"]; ok {
		return nil
	}
	if _, ok := schema["$schema"]; ok {
		return nil
	}
	return errors.New("invalid schema payload: missing 'type'")
}

func coerceSchemaID(requested string, schema map[string]interface{}) (string, error) {
	id := sanitizeSchemaID(strings.TrimSpace(requested))
	if id == "" && schema != nil {
		id = fmt.Sprintf("schema-%d", time.Now().UnixNano())
	}
	if id == "" {
		return "", errors.New("invalid schema id")
	}
	if requested != "" && id != requested {
		return "", errors.New("invalid schema id")
	}
	return id, nil
}

func generateSchemaFromComponents(schema, uiSchema map[string]interface{}, componentsConfig map[string]interface{}) {
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		props = map[string]interface{}{}
		schema["properties"] = props
	}
	uiProps, ok := uiSchema["properties"].(map[string]interface{})
	if !ok {
		uiProps = map[string]interface{}{}
		uiSchema["properties"] = uiProps
	}
	processComponentsRecursive(props, uiProps, componentsConfig)
}

func processComponentsRecursive(schemaProps, uiProps map[string]interface{}, config map[string]interface{}) {
	if configType, ok := config["type"].(string); !ok || configType != "object" {
		return
	}
	configProps, ok := config["properties"].(map[string]interface{})
	if !ok {
		return
	}
	for propName, rawConfig := range configProps {
		propConfigMap, ok := rawConfig.(map[string]interface{})
		if !ok {
			continue
		}
		component, _ := propConfigMap["component"].(string)
		title, _ := propConfigMap["title"].(string)
		schemaProp := map[string]interface{}{}
		uiProp := map[string]interface{}{}
		switch component {
		case "input":
			schemaProp["type"] = "string"
			schemaProp["title"] = title
			if maxLen, ok := propConfigMap["maxLength"].(float64); ok {
				schemaProp["maxLength"] = int(maxLen)
			}
			uiProp["widget"] = "input"
			if placeholder, ok := propConfigMap["placeholder"].(string); ok {
				uiProp["placeholder"] = placeholder
			}
		case "textarea":
			schemaProp["type"] = "string"
			schemaProp["title"] = title
			if maxLen, ok := propConfigMap["maxLength"].(float64); ok {
				schemaProp["maxLength"] = int(maxLen)
			}
			uiProp["widget"] = "textarea"
			if rows, ok := propConfigMap["rows"].(float64); ok {
				uiProp["rows"] = int(rows)
			}
		case "number":
			schemaProp["type"] = "number"
			schemaProp["title"] = title
			if min, ok := propConfigMap["minimum"].(float64); ok {
				schemaProp["minimum"] = min
			}
			if max, ok := propConfigMap["maximum"].(float64); ok {
				schemaProp["maximum"] = max
			}
			uiProp["widget"] = "number"
		case "select":
			schemaProp["type"] = "string"
			schemaProp["title"] = title
			if options, ok := propConfigMap["options"].([]interface{}); ok {
				enumValues := make([]string, 0, len(options))
				for _, opt := range options {
					if v, ok := opt.(string); ok {
						enumValues = append(enumValues, v)
					}
				}
				schemaProp["enum"] = enumValues
			}
			uiProp["widget"] = "select"
		case "switch":
			schemaProp["type"] = "boolean"
			schemaProp["title"] = title
			if def, ok := propConfigMap["default"].(bool); ok {
				schemaProp["default"] = def
			}
			uiProp["widget"] = "switch"
		case "object":
			schemaProp["type"] = "object"
			schemaProp["title"] = title
			nestedSchema := map[string]interface{}{}
			schemaProp["properties"] = nestedSchema
			uiProp["widget"] = "object"
			if displayType, ok := propConfigMap["displayType"].(string); ok {
				uiProp["displayType"] = displayType
			}
			nestedUI := map[string]interface{}{}
			if nestedProps, ok := propConfigMap["properties"].(map[string]interface{}); ok {
				processComponentsRecursive(nestedSchema, nestedUI, map[string]interface{}{
					"type":       "object",
					"properties": nestedProps,
				})
			}
			uiProp["properties"] = nestedUI
		case "array":
			schemaProp["type"] = "array"
			schemaProp["title"] = title
			if minItems, ok := propConfigMap["minItems"].(float64); ok {
				schemaProp["minItems"] = int(minItems)
			}
			if maxItems, ok := propConfigMap["maxItems"].(float64); ok {
				schemaProp["maxItems"] = int(maxItems)
			}
			if itemsConfig, ok := propConfigMap["items"].(map[string]interface{}); ok {
				itemSchema := map[string]interface{}{}
				itemUI := map[string]interface{}{}
				processComponentsRecursive(map[string]interface{}{"item": itemSchema}, map[string]interface{}{"item": itemUI}, map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"item": itemsConfig,
					},
				})
				if v, ok := itemSchema["item"]; ok {
					schemaProp["items"] = v
				}
				if v, ok := itemUI["item"]; ok {
					uiProp["items"] = v
				}
			}
			uiProp["widget"] = "list"
		default:
			schemaProp["type"] = "string"
			schemaProp["title"] = title
			uiProp["widget"] = "input"
		}
		schemaProps[propName] = schemaProp
		uiProps[propName] = uiProp
	}
}
