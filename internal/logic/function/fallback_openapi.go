package function

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type fallbackField struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

func BuildFallbackOpenAPIOperation(functionID string) *openapi3.Operation {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return nil
	}

	resource, action := inferFallbackResourceAction(functionID)
	fields := fallbackFields()
	requestSchema := buildFallbackRequestSchema(fields)
	responseDesc := "Auto-generated success response"

	op := &openapi3.Operation{
		OperationID: functionID,
		Summary:     functionID,
		Description: fmt.Sprintf("Auto-generated fallback operation for %s", functionID),
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Required: true,
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{Value: requestSchema},
					},
				},
			},
		},
		Responses: openapi3.NewResponses(
			openapi3.WithStatus(200, &openapi3.ResponseRef{
				Value: &openapi3.Response{
					Description: &responseDesc,
				},
			}),
		),
		Extensions: map[string]interface{}{
			"x-resource":  resource,
			"x-operation": action,
		},
	}
	return op
}

func BuildFallbackUISchema(functionID string) map[string]interface{} {
	fields := fallbackFields()

	properties := map[string]interface{}{}
	required := make([]string, 0, len(fields))
	for _, field := range fields {
		component, decorator := fallbackFormilyComponent(field.Type)
		prop := map[string]interface{}{
			"type":        field.Type,
			"title":       field.Name,
			"description": field.Description,
			"x-component": component,
		}
		if decorator != "" {
			prop["x-decorator"] = decorator
		}
		if ph := fallbackPlaceholder(field); ph != "" {
			prop["x-component-props"] = map[string]interface{}{
				"placeholder": ph,
			}
		}
		properties[field.Name] = prop
		if field.Required {
			required = append(required, field.Name)
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

// fallbackFormilyComponent maps a fallback field type to Formily component.
func fallbackFormilyComponent(typ string) (component, decorator string) {
	switch typ {
	case "boolean":
		return "Switch", "FormItem"
	case "integer", "number":
		return "NumberPicker", "FormItem"
	case "object":
		return "Card", "FormItem"
	default:
		return "Input", "FormItem"
	}
}

// fallbackPlaceholder generates a placeholder for a fallback field.
func fallbackPlaceholder(field fallbackField) string {
	if field.Description != "" {
		return "请输入" + field.Description
	}
	return "请输入" + field.Name
}

func buildFallbackRequestSchema(fields []fallbackField) *openapi3.Schema {
	objectType := openapi3.Types{"object"}
	properties := map[string]*openapi3.SchemaRef{}
	required := make([]string, 0, len(fields))
	for _, field := range fields {
		schemaType := openapi3.Types{field.Type}
		properties[field.Name] = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type:        &schemaType,
				Description: field.Description,
			},
		}
		if field.Required {
			required = append(required, field.Name)
		}
	}

	return &openapi3.Schema{
		Type:       &objectType,
		Properties: properties,
		Required:   required,
	}
}

func inferFallbackResourceAction(functionID string) (string, string) {
	parts := strings.FieldsFunc(strings.TrimSpace(strings.ToLower(functionID)), func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '/'
	})
	if len(parts) == 0 {
		return "", "invoke"
	}
	if len(parts) >= 3 {
		return sanitizeFallbackToken(parts[len(parts)-2]), sanitizeFallbackToken(parts[len(parts)-1])
	}
	if len(parts) == 2 {
		return sanitizeFallbackToken(parts[0]), sanitizeFallbackToken(parts[1])
	}
	if len(parts) == 1 {
		return sanitizeFallbackToken(parts[0]), "invoke"
	}
	return "function", "invoke"
}

func sanitizeFallbackToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "_-")
}

func fallbackFields() []fallbackField {
	return []fallbackField{
		{Name: "payload", Type: "object", Description: "Invocation payload", Required: false},
	}
}
