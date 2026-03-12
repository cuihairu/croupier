// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取所有可用的第三方平台列表
func NewListPlatformsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformsLogic {
	return &ListPlatformsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformsLogic) ListPlatforms() (resp *types.ListPlatformsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
