// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionWarningsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询函数注册告警
func NewFunctionWarningsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionWarningsLogic {
	return &FunctionWarningsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionWarningsLogic) FunctionWarnings(req *types.FunctionWarningsRequest) (resp *types.FunctionWarningsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
