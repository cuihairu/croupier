// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionUIRollbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 回滚函数UI配置
func NewFunctionUIRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUIRollbackLogic {
	return &FunctionUIRollbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUIRollbackLogic) FunctionUIRollback(req *types.FunctionUIRollbackRequest) (resp *types.FunctionUIRollbackResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
