package openapi

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// RequestContract describes the HTTP input independently of its invocation shape.
type RequestContract struct {
	Parameters []RequestParameter `json:"parameters,omitempty"`
	Body       *RequestBody       `json:"body,omitempty"`
}

type RequestOrigin struct {
	Pointer string `json:"pointer"`
	Ref     string `json:"ref,omitempty"`
}

type RequestMedia struct {
	Schema json.RawMessage `json:"schema,omitempty"`
}

type RequestParameter struct {
	In              string                  `json:"in"`
	Name            string                  `json:"name"`
	Required        bool                    `json:"required"`
	Style           string                  `json:"style,omitempty"`
	Explode         *bool                   `json:"explode,omitempty"`
	AllowReserved   bool                    `json:"allowReserved,omitempty"`
	AllowEmptyValue bool                    `json:"allowEmptyValue,omitempty"`
	Schema          json.RawMessage         `json:"schema,omitempty"`
	Content         map[string]RequestMedia `json:"content,omitempty"`
	Description     string                  `json:"description,omitempty"`
	Deprecated      bool                    `json:"deprecated,omitempty"`
	Origin          RequestOrigin           `json:"origin"`
}

type RequestBody struct {
	Required bool                    `json:"required"`
	Content  map[string]RequestMedia `json:"content"`
	Origin   RequestOrigin           `json:"origin"`
}

// ExtractRequest merges parameter declarations without mutating the document.
func ExtractRequest(path, method string, item *openapi3.PathItem, op *openapi3.Operation) *RequestContract {
	pathPointer := "#/paths/" + pointerToken(path)
	opPointer := pathPointer + "/" + strings.ToLower(method)
	result := &RequestContract{}
	index := make(map[[2]string]int)
	add := func(params openapi3.Parameters, pointer string) {
		for i, ref := range params {
			if ref == nil || ref.Value == nil {
				continue
			}
			p := ref.Value
			origin := RequestOrigin{Pointer: pointer + "/parameters/" + strconv.Itoa(i), Ref: ref.Ref}
			param := RequestParameter{
				In: p.In, Name: p.Name, Required: p.Required,
				AllowReserved: p.AllowReserved, AllowEmptyValue: p.AllowEmptyValue,
				Schema: requestSchema(p.Schema), Content: requestContent(p.Content),
				Description: p.Description, Deprecated: p.Deprecated, Origin: origin,
			}
			if len(param.Content) == 0 {
				param.Style = p.Style
				if param.Style == "" {
					param.Style = "simple"
					if p.In == "query" || p.In == "cookie" {
						param.Style = "form"
					}
				}
				explode := param.Style == "form"
				if p.Explode != nil {
					explode = *p.Explode
				}
				param.Explode = &explode
			}
			name := p.Name
			if p.In == "header" {
				name = strings.ToLower(name)
			}
			key := [2]string{p.In, name}
			if pos, exists := index[key]; exists {
				result.Parameters[pos] = param
				continue
			}
			index[key] = len(result.Parameters)
			result.Parameters = append(result.Parameters, param)
		}
	}
	if item != nil {
		add(item.Parameters, pathPointer)
	}
	add(op.Parameters, opPointer)
	if ref := op.RequestBody; ref != nil && ref.Value != nil {
		result.Body = &RequestBody{
			Required: ref.Value.Required,
			Content:  requestContent(ref.Value.Content),
			Origin:   RequestOrigin{Pointer: opPointer + "/requestBody", Ref: ref.Ref},
		}
	}
	return result
}

func pointerToken(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
}

func requestContent(content openapi3.Content) map[string]RequestMedia {
	if len(content) == 0 {
		return nil
	}
	result := make(map[string]RequestMedia, len(content))
	for name, media := range content {
		var schema json.RawMessage
		if media != nil {
			schema = requestSchema(media.Schema)
		}
		result[name] = RequestMedia{Schema: schema}
	}
	return result
}

func requestSchema(ref *openapi3.SchemaRef) json.RawMessage {
	if ref == nil {
		return nil
	}
	raw, _ := json.Marshal(ref)
	return raw
}
