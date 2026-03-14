package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *FunctionUILogic) FunctionUI(req *FunctionUIRequest) (resp *FunctionUIResponse, err error) {
	return nil, errorx.NewNotImplemented("FunctionUI not implemented")
}
