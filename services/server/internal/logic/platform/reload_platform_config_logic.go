// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type ReloadPlatformConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重新加载平台配置
func NewReloadPlatformConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReloadPlatformConfigLogic {
	return &ReloadPlatformConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReloadPlatformConfigLogic) ReloadPlatformConfig() (resp *types.ReloadPlatformConfigResponse, err error) {
	// 检查平台加载器是否初始化
	if l.svcCtx.PlatformLoader == nil {
		return &types.ReloadPlatformConfigResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
			Success: false,
		}, nil
	}

	// 重新加载平台配置
	if err := l.svcCtx.PlatformLoader.Reload(l.ctx); err != nil {
		return &types.ReloadPlatformConfigResponse{
			Code:    500,
			Message: err.Error(),
			Success: false,
		}, nil
	}

	return &types.ReloadPlatformConfigResponse{
		Code:    200,
		Message: "Platform configuration reloaded successfully",
		Success: true,
	}, nil
}
