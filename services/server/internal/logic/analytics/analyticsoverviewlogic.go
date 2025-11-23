// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析概览
func NewAnalyticsOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsOverviewLogic {
	return &AnalyticsOverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsOverviewLogic) AnalyticsOverview(req *types.AnalyticsOverviewRequest) (resp *types.AnalyticsOverviewResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
