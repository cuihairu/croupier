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

type FunctionUILogicV2 struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数UI配置（增强版：支持优先级合并）
func NewFunctionUILogicV2(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUILogicV2 {
	return &FunctionUILogicV2{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUILogicV2) FunctionUI(req *types.FunctionUIRequest) (*types.FunctionUIResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}
	resolved := resolveFunctionUI(l.svcCtx.Config, fn)

	return &types.FunctionUIResponse{
		Schema:         resolved.Schema,
		Layout:         resolved.Layout,
		Components:     resolved.Components,
		Custom:         resolved.Custom,
		HasDefault:     resolved.HasDefault,
		UISource:       resolved.UISource,
		UISourceDetail: resolved.UISourceDetail,
	}, nil
}
