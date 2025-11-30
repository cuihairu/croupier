// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionDisableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 禁用函数
func NewFunctionDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDisableLogic {
	return &FunctionDisableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDisableLogic) FunctionDisable(req *types.FunctionActionRequest) error {
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
