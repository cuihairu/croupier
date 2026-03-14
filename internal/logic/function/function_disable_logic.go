package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionDisableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 禁用函数
func NewFunctionDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDisableLogic {
	return &FunctionDisableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDisableLogic) FunctionDisable(req *FunctionActionRequest) error {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return err
	}

	return l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{
		"status": model.StatusDisabled,
	})
}
