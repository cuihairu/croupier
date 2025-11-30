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

type FunctionInstancesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数实例
func NewFunctionInstancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInstancesLogic {
	return &FunctionInstancesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInstancesLogic) FunctionInstances(req *types.FunctionInstancesRequest) (*types.FunctionInstancesResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	instances, err := l.svcCtx.FunctionModel.ListInstances(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	return &types.FunctionInstancesResponse{
		Items: utils.BuildFunctionInstances(instances),
	}, nil
}
