// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FunctionUILogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数UI配置
func NewFunctionUILogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUILogic {
	return &FunctionUILogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUILogic) FunctionUI(req *types.FunctionUIRequest) (resp *types.FunctionUIResponse, err error) {
	return nil, errorx.NewNotImplemented("FunctionUI not implemented")
}
