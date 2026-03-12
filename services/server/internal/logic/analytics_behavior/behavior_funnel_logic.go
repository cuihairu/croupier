// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BehaviorFunnelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为漏斗
func NewBehaviorFunnelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorFunnelLogic {
	return &BehaviorFunnelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorFunnelLogic) BehaviorFunnel(req *types.BehaviorFunnelRequest) (resp *types.BehaviorFunnelResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
