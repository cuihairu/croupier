// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksPluginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 pack web 插件
func NewPacksPluginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksPluginLogic {
	return &PacksPluginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksPluginLogic) PacksPlugin(req *types.PacksPluginRequest) (resp *types.PacksPluginResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
