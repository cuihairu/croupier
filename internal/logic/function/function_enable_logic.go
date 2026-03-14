package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionEnableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启用函数
func NewFunctionEnableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionEnableLogic {
	return &FunctionEnableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionEnableLogic) FunctionEnable(req *FunctionActionRequest) error {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return err
	}

	return l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{
		"status": model.StatusEnabled,
	})
}
