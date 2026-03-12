// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除函数
func NewFunctionDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDeleteLogic {
	return &FunctionDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDeleteLogic) FunctionDelete(req *types.FunctionActionRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
