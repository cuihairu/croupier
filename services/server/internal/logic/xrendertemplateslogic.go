package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewXRenderTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderTemplatesLogic {
	return &XRenderTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderTemplatesLogic) XRenderTemplates() (*types.XRenderTemplatesResponse, error) {
	templates := map[string]interface{}{
		"user_form": map[string]interface{}{
			"name":        "User Profile",
			"description": "Basic user information template",
			"components": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"username":   map[string]interface{}{"component": "input", "title": "Username", "required": true},
					"email":      map[string]interface{}{"component": "input", "title": "Email", "required": true},
					"bio":        map[string]interface{}{"component": "textarea", "title": "Bio"},
					"newsletter": map[string]interface{}{"component": "switch", "title": "Subscribe"},
				},
			},
		},
		"game_config": map[string]interface{}{
			"name":        "Game Configuration",
			"description": "Example configuration for game features",
			"components": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server_name": map[string]interface{}{"component": "input", "title": "Server Name", "required": true},
					"max_players": map[string]interface{}{"component": "number", "title": "Max Players", "required": true},
					"features": map[string]interface{}{
						"component": "array",
						"title":     "Features",
						"items": map[string]interface{}{
							"component": "select",
							"title":     "Feature",
							"options":   []string{"pvp", "guild", "chat", "auction"},
						},
					},
				},
			},
		},
	}
	return &types.XRenderTemplatesResponse{Templates: templates}, nil
}
