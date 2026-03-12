// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数列表
func NewFunctionsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsListLogic {
	return &FunctionsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsListLogic) FunctionsList(req *types.FunctionsListRequest) (resp *types.FunctionsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
