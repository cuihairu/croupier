// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionRouteUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数路由配置
func NewFunctionRouteUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRouteUpdateLogic {
	return &FunctionRouteUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRouteUpdateLogic) FunctionRouteUpdate(req *types.FunctionRouteUpdateRequest) (resp *types.FunctionRouteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
