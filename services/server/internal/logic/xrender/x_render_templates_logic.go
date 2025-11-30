// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取XRender模板
func NewXRenderTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderTemplatesLogic {
	return &XRenderTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderTemplatesLogic) XRenderTemplates(req *types.XRenderTemplatesRequest) (resp *types.XRenderTemplatesResponse, err error) {
	templates, err := loadXRenderTemplates(l.svcCtx.Config)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(templates))
	rendererCounts := make(map[string]int)
	for _, tpl := range templates {
		items = append(items, tpl.toMap())
		if tpl.Renderer != "" {
			rendererCounts[tpl.Renderer]++
		}
	}

	return &types.XRenderTemplatesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items":    items,
			"total":    len(items),
			"renderer": rendererCounts,
		},
	}, nil
}
