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

// BuildFallbackInputJSONSchema 返回兜底输入 JSON Schema。
// functionID 参数仅为保持既有 API 形状保留，schema 内容与具体函数无关。
func BuildFallbackInputJSONSchema(functionID string) map[string]interface{} {
	return buildFallbackInputSchema(fallbackFields())
}

// buildFallbackInputSchema 将字段清单投影为 JSON Schema map。抽出为独立
// helper 是为了让 required 追加分支可被单测直接注入合成字段驱动（生产
// 数据源 fallbackFields 当前全部字段 Required=false）。
func buildFallbackInputSchema(fields []fallbackField) map[string]interface{} {
	properties := map[string]interface{}{}
	required := make([]string, 0, len(fields))
	for _, field := range fields {
		prop := map[string]interface{}{
			"type":        field.Type,
			"title":       field.Name,
			"description": field.Description,
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
	switch len(parts) {
	case 0:
		return "", "invoke"
	case 1:
		return sanitizeFallbackToken(parts[0]), "invoke"
	case 2:
		return sanitizeFallbackToken(parts[0]), sanitizeFallbackToken(parts[1])
	default:
		return sanitizeFallbackToken(parts[len(parts)-2]), sanitizeFallbackToken(parts[len(parts)-1])
	}
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
