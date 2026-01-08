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

func (l *ListPlatformMethodsLogic) ListPlatformMethods(platformName string) (resp *types.ListPlatformMethodsResponse, err error) {
	// 检查平台加载器是否初始化
	if l.svcCtx.PlatformLoader == nil {
		return &types.ListPlatformMethodsResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
		}, nil
	}

	// 验证平台参数
	if platformName == "" {
		return &types.ListPlatformMethodsResponse{
			Code:    400,
			Message: "platform is required",
		}, nil
	}

	// 获取指定的平台提供者
	p, exists := l.svcCtx.PlatformLoader.GetProvider(platformName)
	if !exists {
		return &types.ListPlatformMethodsResponse{
			Code:    404,
			Message: "platform not found",
		}, nil
	}

	return &types.ListPlatformMethodsResponse{
		Code:    200,
		Message: "success",
		Methods: p.SupportedMethods(),
	}, nil
}
