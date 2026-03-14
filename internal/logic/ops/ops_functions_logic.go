package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type OpsFunctionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数列表
func NewOpsFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsFunctionsLogic {
	return &OpsFunctionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsFunctionsLogic) OpsFunctions(req *OpsFunctionsRequest) (resp *OpsFunctionsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsFunctions not implemented")
}
