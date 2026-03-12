// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionsPendingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取待处理函数
func NewFunctionsPendingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsPendingLogic {
	return &FunctionsPendingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsPendingLogic) FunctionsPending(req *types.FunctionsPendingRequest) (resp *types.FunctionsPendingResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
