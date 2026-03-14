// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type PacksPluginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 pack web 插件
func NewPacksPluginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksPluginLogic {
	return &PacksPluginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksPluginLogic) PacksPlugin(req *types.PacksPluginRequest) (resp *types.PacksPluginResponse, err error) {
	return nil, errorx.NewNotImplemented("PacksPlugin not implemented")
}
