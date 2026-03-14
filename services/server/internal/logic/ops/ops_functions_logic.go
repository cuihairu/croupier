// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsFunctionsLogic) OpsFunctions(req *types.OpsFunctionsRequest) (resp *types.OpsFunctionsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsFunctions not implemented")
}
