package validation

import (
	"fmt"
	"strings"
)

// ValidateEntityDefinition validates an entity definition structure
func ValidateEntityDefinition(entity map[string]any) []string {
	var errors []string

	// Check required fields
	if _, ok := entity["id"]; !ok {
		errors = append(errors, "missing required field: id")
	} else if id, ok := entity["id"].(string); !ok || strings.TrimSpace(id) == "" {
		errors = append(errors, "id must be a non-empty string")
	}

	if entityType, ok := entity["type"].(string); !ok || entityType != "entity" {
		errors = append(errors, "type must be 'entity'")
	}

	if _, ok := entity["schema"]; !ok {
		errors = append(errors, "missing required field: schema")
	} else {
		if schema, ok := entity["schema"].(map[string]any); ok {
			errors = append(errors, validateJSONSchema(schema)...)
		} else {
			errors = append(errors, "schema must be a JSON object")
		}
	}

	// Validate operations if present
	if operations, ok := entity["operations"]; ok {
		if operationsMap, ok := operations.(map[string]any); ok {
			errors = append(errors, validateOperations(operationsMap)...)
		} else {
			errors = append(errors, "operations must be a JSON object")
		}
	}

	// Validate UI configuration if present
	if ui, ok := entity["ui"]; ok {
		if uiMap, ok := ui.(map[string]any); ok {
			errors = append(errors, validateUIConfig(uiMap)...)
		} else {
			errors = append(errors, "ui must be a JSON object")
		}
	}

	return errors
}

// validateJSONSchema validates the JSON Schema structure
func validateJSONSchema(schema map[string]any) []string {
	var errors []string

	// Check required schema fields
	if schemaType, ok := schema["type"].(string); !ok || schemaType != "object" {
		errors = append(errors, "schema.type must be 'object'")
	}

	if _, ok := schema["properties"]; !ok {
		errors = append(errors, "schema must have properties")
	} else if properties, ok := schema["properties"].(map[string]any); ok {
		errors = append(errors, validateSchemaProperties(properties)...)
	} else {
		errors = append(errors, "schema.properties must be a JSON object")
	}

	// Validate required array if present
	if required, ok := schema["required"]; ok {
		if requiredArray, ok := required.([]any); ok {
			for i, req := range requiredArray {
				if _, ok := req.(string); !ok {
					errors = append(errors, fmt.Sprintf("schema.required[%d] must be a string", i))
				}
			}
		} else {
			errors = append(errors, "schema.required must be an array")
		}
	}

	return errors
}

// validateSchemaProperties validates individual properties in the schema
func validateSchemaProperties(properties map[string]any) []string {
	var errors []string

	for propName, propDef := range properties {
		if propDefMap, ok := propDef.(map[string]any); ok {
			// Check if property has a type
			if propType, ok := propDefMap["type"]; ok {
				if propTypeStr, ok := propType.(string); ok {
					switch propTypeStr {
					case "string", "integer", "number", "boolean", "array", "object":
						// Valid types
					default:
						errors = append(errors, fmt.Sprintf("property '%s' has invalid type '%s'", propName, propTypeStr))
					}
				} else {
					errors = append(errors, fmt.Sprintf("property '%s' type must be a string", propName))
				}
			} else {
				errors = append(errors, fmt.Sprintf("property '%s' must have a type", propName))
			}

			// Validate enum if present
			if enum, ok := propDefMap["enum"]; ok {
				if enumArray, ok := enum.([]any); ok {
					if len(enumArray) == 0 {
						errors = append(errors, fmt.Sprintf("property '%s' enum must not be empty", propName))
					}
				} else {
					errors = append(errors, fmt.Sprintf("property '%s' enum must be an array", propName))
				}
			}

			// Validate format if present
			if format, ok := propDefMap["format"]; ok {
				if formatStr, ok := format.(string); ok {
					validFormats := []string{"date", "date-time", "email", "uri", "hostname", "ipv4", "ipv6"}
					isValid := false
					for _, vf := range validFormats {
						if formatStr == vf {
							isValid = true
							break
						}
					}
					if !isValid {
						errors = append(errors, fmt.Sprintf("property '%s' has invalid format '%s'", propName, formatStr))
					}
				} else {
					errors = append(errors, fmt.Sprintf("property '%s' format must be a string", propName))
				}
			}

			// Validate string constraints
			if propType, ok := propDefMap["type"].(string); ok && propType == "string" {
				if minLength, ok := propDefMap["minLength"]; ok {
					if minLengthNum, ok := minLength.(float64); !ok || minLengthNum < 0 {
						errors = append(errors, fmt.Sprintf("property '%s' minLength must be a non-negative number", propName))
					}
				}
				if maxLength, ok := propDefMap["maxLength"]; ok {
					if maxLengthNum, ok := maxLength.(float64); !ok || maxLengthNum < 0 {
						errors = append(errors, fmt.Sprintf("property '%s' maxLength must be a non-negative number", propName))
					}
				}
			}

			// Validate number constraints
			if propType, ok := propDefMap["type"].(string); ok && (propType == "number" || propType == "integer") {
				if minimum, ok := propDefMap["minimum"]; ok {
					if _, ok := minimum.(float64); !ok {
						errors = append(errors, fmt.Sprintf("property '%s' minimum must be a number", propName))
					}
				}
				if maximum, ok := propDefMap["maximum"]; ok {
					if _, ok := maximum.(float64); !ok {
						errors = append(errors, fmt.Sprintf("property '%s' maximum must be a number", propName))
					}
				}
			}
		} else {
			errors = append(errors, fmt.Sprintf("property '%s' definition must be a JSON object", propName))
		}
	}

	return errors
}

// validateOperations validates the operations mapping
func validateOperations(operations map[string]any) []string {
	var errors []string

	validOperations := []string{"create", "read", "update", "delete", "list"}

	for opName, opValue := range operations {
		// Check if operation name is valid
		isValidOp := false
		for _, validOp := range validOperations {
			if opName == validOp {
				isValidOp = true
				break
			}
		}
		if !isValidOp {
			errors = append(errors, fmt.Sprintf("invalid operation '%s'", opName))
		}

		// Check if operation value is a function ID or array of function IDs
		if opArray, ok := opValue.([]any); ok {
			for i, funcID := range opArray {
				if _, ok := funcID.(string); !ok {
					errors = append(errors, fmt.Sprintf("operations.%s[%d] must be a string", opName, i))
				}
			}
		} else if _, ok := opValue.(string); !ok {
			errors = append(errors, fmt.Sprintf("operations.%s must be a string or array of strings", opName))
		}
	}

	return errors
}

// validateUIConfig validates the UI configuration
func validateUIConfig(ui map[string]any) []string {
	var errors []string

	// Validate display_field if present
	if displayField, ok := ui["display_field"]; ok {
		if _, ok := displayField.(string); !ok {
			errors = append(errors, "ui.display_field must be a string")
		}
	}

	// Validate title_template if present
	if titleTemplate, ok := ui["title_template"]; ok {
		if _, ok := titleTemplate.(string); !ok {
			errors = append(errors, "ui.title_template must be a string")
		}
	}

	// Validate icon_field if present
	if iconField, ok := ui["icon_field"]; ok {
		if _, ok := iconField.(string); !ok {
			errors = append(errors, "ui.icon_field must be a string")
		}
	}

	// Validate status_field if present
	if statusField, ok := ui["status_field"]; ok {
		if _, ok := statusField.(string); !ok {
			errors = append(errors, "ui.status_field must be a string")
		}
	}

	return errors
}
