// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数变更历史
func NewFunctionHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionHistoryLogic {
	return &FunctionHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionHistoryLogic) FunctionHistory(req *types.FunctionHistoryRequest) (resp []types.FunctionHistoryItem, err error) {
	// todo: add your logic here and delete this line

	return
}
