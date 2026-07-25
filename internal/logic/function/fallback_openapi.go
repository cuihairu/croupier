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

	entity, action := inferFallbackEntityAction(functionID)
	fields := fallbackFields(entity, action)
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
			"x-entity":    entity,
			"x-operation": action,
			"x-action":    action,
		},
	}
	return op
}

func BuildFallbackUISchema(functionID string) map[string]interface{} {
	entity, action := inferFallbackEntityAction(functionID)
	fields := fallbackFields(entity, action)

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

func inferFallbackEntityAction(functionID string) (string, string) {
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

func fallbackFields(entity, action string) []fallbackField {
	switch entity {
	case "player":
		return fallbackPlayerFields(action)
	case "order":
		return fallbackOrderFields(action)
	case "inventory":
		return fallbackInventoryFields(action)
	case "leaderboard":
		return fallbackLeaderboardFields(action)
	case "mail":
		return fallbackMailFields(action)
	default:
		return []fallbackField{
			{Name: "id", Type: "string", Description: "Target identifier", Required: true},
			{Name: "payload", Type: "object", Description: "Invocation payload", Required: false},
		}
	}
}

func fallbackPlayerFields(action string) []fallbackField {
	switch action {
	case "list":
		return []fallbackField{
			{Name: "page", Type: "integer", Description: "Page number", Required: false},
			{Name: "pageSize", Type: "integer", Description: "Page size", Required: false},
			{Name: "keyword", Type: "string", Description: "Search keyword", Required: false},
		}
	case "get", "delete":
		return []fallbackField{{Name: "playerId", Type: "string", Description: "Player ID", Required: true}}
	case "create":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "nickname", Type: "string", Description: "Player nickname", Required: false},
			{Name: "level", Type: "integer", Description: "Initial level", Required: false},
		}
	case "update":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "nickname", Type: "string", Description: "Player nickname", Required: false},
			{Name: "level", Type: "integer", Description: "Player level", Required: false},
			{Name: "status", Type: "string", Description: "Player status", Required: false},
		}
	default:
		return []fallbackField{{Name: "playerId", Type: "string", Description: "Player ID", Required: true}}
	}
}

func fallbackOrderFields(action string) []fallbackField {
	switch action {
	case "list":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: false},
			{Name: "status", Type: "string", Description: "Order status", Required: false},
			{Name: "page", Type: "integer", Description: "Page number", Required: false},
			{Name: "pageSize", Type: "integer", Description: "Page size", Required: false},
		}
	case "get", "delete":
		return []fallbackField{{Name: "orderId", Type: "string", Description: "Order ID", Required: true}}
	case "create":
		return []fallbackField{
			{Name: "orderId", Type: "string", Description: "Order ID", Required: true},
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "productId", Type: "string", Description: "Product ID", Required: true},
			{Name: "amount", Type: "integer", Description: "Order amount", Required: false},
		}
	case "update":
		return []fallbackField{
			{Name: "orderId", Type: "string", Description: "Order ID", Required: true},
			{Name: "status", Type: "string", Description: "Order status", Required: false},
			{Name: "amount", Type: "integer", Description: "Order amount", Required: false},
		}
	default:
		return []fallbackField{{Name: "orderId", Type: "string", Description: "Order ID", Required: true}}
	}
}

func fallbackInventoryFields(action string) []fallbackField {
	switch action {
	case "list":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "page", Type: "integer", Description: "Page number", Required: false},
			{Name: "pageSize", Type: "integer", Description: "Page size", Required: false},
		}
	case "grant", "consume":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "itemId", Type: "string", Description: "Item ID", Required: true},
			{Name: "amount", Type: "integer", Description: "Item amount", Required: true},
		}
	default:
		return []fallbackField{{Name: "playerId", Type: "string", Description: "Player ID", Required: true}}
	}
}

func fallbackLeaderboardFields(action string) []fallbackField {
	switch action {
	case "list":
		return []fallbackField{
			{Name: "leaderboardId", Type: "string", Description: "Leaderboard ID", Required: true},
			{Name: "page", Type: "integer", Description: "Page number", Required: false},
			{Name: "pageSize", Type: "integer", Description: "Page size", Required: false},
		}
	case "upsert":
		return []fallbackField{
			{Name: "leaderboardId", Type: "string", Description: "Leaderboard ID", Required: true},
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "score", Type: "integer", Description: "Player score", Required: true},
		}
	case "reset":
		return []fallbackField{{Name: "leaderboardId", Type: "string", Description: "Leaderboard ID", Required: true}}
	default:
		return []fallbackField{{Name: "leaderboardId", Type: "string", Description: "Leaderboard ID", Required: true}}
	}
}

func fallbackMailFields(action string) []fallbackField {
	switch action {
	case "list":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "page", Type: "integer", Description: "Page number", Required: false},
			{Name: "pageSize", Type: "integer", Description: "Page size", Required: false},
		}
	case "claim":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "mailId", Type: "string", Description: "Mail ID", Required: true},
		}
	case "send":
		return []fallbackField{
			{Name: "playerId", Type: "string", Description: "Player ID", Required: true},
			{Name: "title", Type: "string", Description: "Mail title", Required: true},
			{Name: "content", Type: "string", Description: "Mail content", Required: true},
		}
	default:
		return []fallbackField{{Name: "playerId", Type: "string", Description: "Player ID", Required: true}}
	}
}
