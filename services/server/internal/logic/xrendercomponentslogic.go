package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderComponentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

var defaultXRenderComponents = map[string][]map[string]interface{}{
	"form": {
		{
			"id":          "input",
			"name":        "Input",
			"widget":      "input",
			"icon":        "input",
			"category":    "form",
			"description": "Single-line text input",
			"properties": map[string]interface{}{
				"title":       map[string]interface{}{"type": "string", "title": "Label"},
				"placeholder": map[string]interface{}{"type": "string", "title": "Placeholder"},
				"maxLength":   map[string]interface{}{"type": "number", "title": "Max Length"},
				"required":    map[string]interface{}{"type": "boolean", "title": "Required", "default": false},
			},
			"schema_template": map[string]interface{}{
				"type":      "string",
				"title":     "",
				"maxLength": 100,
			},
		},
		{
			"id":          "textarea",
			"name":        "Textarea",
			"widget":      "textarea",
			"icon":        "textarea",
			"category":    "form",
			"description": "Multi-line text input",
			"properties": map[string]interface{}{
				"title":       map[string]interface{}{"type": "string", "title": "Label"},
				"placeholder": map[string]interface{}{"type": "string", "title": "Placeholder"},
				"rows":        map[string]interface{}{"type": "number", "title": "Rows", "default": 4},
				"required":    map[string]interface{}{"type": "boolean", "title": "Required", "default": false},
			},
			"schema_template": map[string]interface{}{
				"type":      "string",
				"title":     "",
				"maxLength": 1000,
			},
		},
		{
			"id":          "number",
			"name":        "Number",
			"widget":      "number",
			"icon":        "number",
			"category":    "form",
			"description": "Numeric input",
			"properties": map[string]interface{}{
				"title":    map[string]interface{}{"type": "string", "title": "Label"},
				"minimum":  map[string]interface{}{"type": "number", "title": "Minimum"},
				"maximum":  map[string]interface{}{"type": "number", "title": "Maximum"},
				"required": map[string]interface{}{"type": "boolean", "title": "Required", "default": false},
			},
			"schema_template": map[string]interface{}{
				"type":  "number",
				"title": "",
			},
		},
		{
			"id":          "select",
			"name":        "Select",
			"widget":      "select",
			"icon":        "select",
			"category":    "form",
			"description": "Option selector",
			"properties": map[string]interface{}{
				"title":    map[string]interface{}{"type": "string", "title": "Label"},
				"options":  map[string]interface{}{"type": "array", "title": "Options", "items": map[string]interface{}{"type": "string"}},
				"required": map[string]interface{}{"type": "boolean", "title": "Required", "default": false},
			},
			"schema_template": map[string]interface{}{
				"type":  "string",
				"title": "",
				"enum":  []string{},
			},
		},
		{
			"id":          "switch",
			"name":        "Switch",
			"widget":      "switch",
			"icon":        "switch",
			"category":    "form",
			"description": "Boolean toggle",
			"properties": map[string]interface{}{
				"title":   map[string]interface{}{"type": "string", "title": "Label"},
				"default": map[string]interface{}{"type": "boolean", "title": "Default", "default": false},
			},
			"schema_template": map[string]interface{}{
				"type":    "boolean",
				"title":   "",
				"default": false,
			},
		},
	},
	"layout": {
		{
			"id":          "object",
			"name":        "Object",
			"widget":      "object",
			"icon":        "appstore",
			"category":    "layout",
			"description": "Group fields together",
			"properties": map[string]interface{}{
				"title":       map[string]interface{}{"type": "string", "title": "Label"},
				"displayType": map[string]interface{}{"type": "string", "title": "Layout", "enum": []string{"row", "column"}},
			},
			"schema_template": map[string]interface{}{
				"type":       "object",
				"title":      "",
				"properties": map[string]interface{}{},
			},
		},
		{
			"id":          "array",
			"name":        "Array",
			"widget":      "list",
			"icon":        "unordered-list",
			"category":    "layout",
			"description": "Repeatable list of items",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{"type": "string", "title": "Label"},
			},
			"schema_template": map[string]interface{}{
				"type":  "array",
				"title": "",
				"items": map[string]interface{}{},
			},
		},
	},
	"display": {
		{
			"id":          "text",
			"name":        "Text",
			"widget":      "text",
			"icon":        "font-size",
			"category":    "display",
			"description": "Static text block",
			"properties": map[string]interface{}{
				"title":   map[string]interface{}{"type": "string", "title": "Title"},
				"content": map[string]interface{}{"type": "string", "title": "Content"},
			},
			"schema_template": map[string]interface{}{
				"type":  "string",
				"title": "",
			},
		},
	},
}

func NewXRenderComponentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderComponentsLogic {
	return &XRenderComponentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderComponentsLogic) XRenderComponents(req *types.XRenderComponentsRequest) (*types.XRenderComponentsResponse, error) {
	resp := map[string][]map[string]interface{}{}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		for k, v := range defaultXRenderComponents {
			resp[k] = v
		}
	} else if comps, ok := defaultXRenderComponents[category]; ok {
		resp[category] = comps
	}
	return &types.XRenderComponentsResponse{Components: resp}, nil
}
