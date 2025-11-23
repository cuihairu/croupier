// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorFunnelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为漏斗
func NewAnalyticsBehaviorFunnelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorFunnelLogic {
	return &AnalyticsBehaviorFunnelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorFunnelLogic) AnalyticsBehaviorFunnel(req *types.AnalyticsBehaviorFunnelRequest) (resp *types.AnalyticsBehaviorFunnelResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
