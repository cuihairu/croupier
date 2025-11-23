// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付摘要
func NewAnalyticsPaymentsSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsSummaryLogic {
	return &AnalyticsPaymentsSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsSummaryLogic) AnalyticsPaymentsSummary(req *types.AnalyticsPaymentsSummaryRequest) (resp *types.AnalyticsPaymentsSummaryResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
