// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReloadPlatformConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重新加载平台配置
func NewReloadPlatformConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReloadPlatformConfigLogic {
	return &ReloadPlatformConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReloadPlatformConfigLogic) ReloadPlatformConfig() (resp *types.ReloadPlatformConfigResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
