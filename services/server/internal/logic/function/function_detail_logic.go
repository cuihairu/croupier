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

type FunctionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数详情
func NewFunctionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDetailLogic {
	return &FunctionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDetailLogic) FunctionDetail(req *types.FunctionDetailRequest) (*types.FunctionDetailResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	desc := types.FunctionDescriptor{}
	descs, err := l.svcCtx.FunctionModel.ListDescriptors(l.ctx, functionID)
	if err == nil && len(descs) > 0 {
		desc = types.FunctionDescriptor{
			Input:  descs[0].Input,
			Output: descs[0].Output,
			Schema: descs[0].Schema,
		}
	}

	return &types.FunctionDetailResponse{
		Function:   utils.BuildFunctionDTO(fn),
		Descriptor: desc,
	}, nil
}
