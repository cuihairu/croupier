// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionPermissionsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数权限
func NewFunctionPermissionsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionPermissionsUpdateLogic {
	return &FunctionPermissionsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionPermissionsUpdateLogic) FunctionPermissionsUpdate(req *types.FunctionPermissionsUpdateRequest) (resp *types.FunctionPermissionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
