// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实体关联的函数列表
func NewEntityFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityFunctionsLogic {
	return &EntityFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityFunctionsLogic) EntityFunctions(req *types.EntityFunctionsRequest) (resp *types.EntityFunctionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
