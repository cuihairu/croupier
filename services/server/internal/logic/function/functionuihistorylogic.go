// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionUIHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数UI配置历史
func NewFunctionUIHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUIHistoryLogic {
	return &FunctionUIHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUIHistoryLogic) FunctionUIHistory(req *types.FunctionUIHistoryRequest) (resp *types.FunctionUIHistoryResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
