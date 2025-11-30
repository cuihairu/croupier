// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
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

func (l *FunctionPermissionsLogic) FunctionPermissions(req *types.FunctionPermissionsRequest) (*types.FunctionPermissionsResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	perms, err := l.svcCtx.FunctionModel.ListPermissions(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	return &types.FunctionPermissionsResponse{
		Items: utils.BuildFunctionPermissions(perms),
	}, nil
}
