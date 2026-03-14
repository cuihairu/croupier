// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type ListPlatformsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取所有可用的第三方平台列表
func NewListPlatformsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformsLogic {
	return &ListPlatformsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformsLogic) ListPlatforms() (resp *types.ListPlatformsResponse, err error) {
	// 如果平台加载器未初始化，返回空列表
	if l.svcCtx.PlatformLoader == nil {
		return &types.ListPlatformsResponse{
			Code:      200,
			Message:   "success",
			Platforms: []types.PlatformInfo{},
		}, nil
	}

	// 获取所有已注册的平台提供者
	providers := l.svcCtx.PlatformLoader.ListProviders()
	platforms := make([]types.PlatformInfo, 0, len(providers))

	for _, p := range providers {
		platforms = append(platforms, types.PlatformInfo{
			Name:    p.Name(),
			Enabled: p.IsEnabled(),
			Methods: p.SupportedMethods(),
		})
	}

	return &types.ListPlatformsResponse{
		Code:      200,
		Message:   "success",
		Platforms: platforms,
	}, nil
}
