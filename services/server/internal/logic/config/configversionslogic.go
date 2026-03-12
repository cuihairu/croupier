// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取配置版本列表
func NewConfigVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigVersionsLogic {
	return &ConfigVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigVersionsLogic) ConfigVersions(req *types.ConfigVersionsRequest) (resp *types.ConfigVersionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
