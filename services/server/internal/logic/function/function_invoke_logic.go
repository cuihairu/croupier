// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionInvokeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调用函数
func NewFunctionInvokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInvokeLogic {
	return &FunctionInvokeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInvokeLogic) FunctionInvoke(req *types.FunctionInvokeRequest) (*types.FunctionInvokeResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	if _, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Params)
	if err != nil {
		return nil, err
	}

	jobID, err := l.svcCtx.Dispatcher.StartJob(l.ctx, functionID, payload)
	if err != nil {
		return nil, err
	}

	return &types.FunctionInvokeResponse{
		JobId: jobID,
	}, nil
}
