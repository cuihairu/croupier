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

func (l *FunctionPermissionsUpdateLogic) FunctionPermissionsUpdate(req *types.FunctionPermissionsUpdateRequest) (*types.FunctionPermissionsResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	records, err := utils.ConvertFunctionPermissions(functionID, req.Permissions)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.FunctionModel.ReplacePermissions(l.ctx, functionID, records); err != nil {
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
