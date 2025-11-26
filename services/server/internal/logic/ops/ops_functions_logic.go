// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数列表
func NewOpsFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsFunctionsLogic {
	return &OpsFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsFunctionsLogic) OpsFunctions(req *types.OpsFunctionsRequest) (resp *types.OpsFunctionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
