// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigVersionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取配置版本详情
func NewConfigVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigVersionDetailLogic {
	return &ConfigVersionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigVersionDetailLogic) ConfigVersionDetail(req *types.ConfigVersionDetailRequest) (resp *types.ConfigVersionDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
