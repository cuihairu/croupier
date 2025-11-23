// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsProductTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取产品趋势
func NewAnalyticsPaymentsProductTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsProductTrendLogic {
	return &AnalyticsPaymentsProductTrendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsProductTrendLogic) AnalyticsPaymentsProductTrend(req *types.AnalyticsPaymentsProductTrendRequest) (resp *types.AnalyticsPaymentsProductTrendResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
