// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionPermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数权限
func NewFunctionPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionPermissionsLogic {
	return &FunctionPermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionPermissionsLogic) FunctionPermissions(req *types.FunctionPermissionsRequest) (resp *types.FunctionPermissionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
