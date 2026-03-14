// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

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

func (l *FunctionEnableLogic) FunctionEnable(req *types.FunctionActionRequest) error {
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
