// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package routes

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRoutesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取动态路由配置
func NewGetRoutesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoutesLogic {
	return &GetRoutesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRoutesLogic) GetRoutes() (resp *types.GetRoutesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
