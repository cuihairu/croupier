// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderComponentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取XRender组件
func NewXRenderComponentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderComponentsLogic {
	return &XRenderComponentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderComponentsLogic) XRenderComponents(req *types.XRenderComponentsRequest) (resp *types.XRenderComponentsResponse, err error) {
	components, err := loadXRenderComponents(l.svcCtx.Config)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(components))
	packCounts := make(map[string]int)
	for _, comp := range components {
		items = append(items, comp.toMap())
		packCounts[comp.Pack]++
	}

	return &types.XRenderComponentsResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": len(items),
			"packs": packCounts,
		},
	}, nil
}
