// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformMethodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定平台支持的方法列表
func NewListPlatformMethodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformMethodsLogic {
	return &ListPlatformMethodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformMethodsLogic) ListPlatformMethods() (resp *types.ListPlatformMethodsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
