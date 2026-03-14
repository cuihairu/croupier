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

type FunctionPublishLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 发布函数
func NewFunctionPublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionPublishLogic {
	return &FunctionPublishLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionPublishLogic) FunctionPublish(req *types.FunctionPublishRequest) (*types.FunctionPublishResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{
		"status": model.StatusEnabled,
	}); err != nil {
		return nil, err
	}

	// 如果存在待审批记录，发布成功后移除
	_ = l.svcCtx.FunctionModel.DeletePending(l.ctx, functionID)

	return &types.FunctionPublishResponse{
		Published: true,
	}, nil
}
