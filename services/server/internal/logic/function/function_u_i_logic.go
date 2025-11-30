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

type FunctionUILogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数UI配置
func NewFunctionUILogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUILogic {
	return &FunctionUILogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUILogic) FunctionUI(req *types.FunctionUIRequest) (*types.FunctionUIResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	var layout interface{}
	var components interface{}
	if fn.Metadata != nil {
		layout = fn.Metadata["layout"]
		components = fn.Metadata["components"]
	}

	return &types.FunctionUIResponse{
		Schema:     fn.Schema,
		Layout:     layout,
		Components: components,
	}, nil
}
