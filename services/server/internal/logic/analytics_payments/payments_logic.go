// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type PaymentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付分析
func NewPaymentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsLogic {
	return &PaymentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsLogic) Payments(req *types.PaymentsRequest) (*types.PaymentsResponse, error) {
	if l.svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolvePaymentsRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)

	agg, err := l.svcCtx.PaymentsModel.AggregateRevenue(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	activeUsers := agg.Payers
	if l.svcCtx.BehaviorModel != nil {
		if count, countErr := l.svcCtx.BehaviorModel.CountDistinctUsers(l.ctx, gameID, env, start, end); countErr == nil && count > 0 {
			activeUsers = count
		}
	}

	arpuBase := float64(activeUsers)
	metrics := types.PaymentsMetrics{
		Revenue:      agg.Revenue,
		Transactions: int(agg.Transactions),
		PayingUsers:  int(agg.Payers),
		ARPU:         safeDivide(agg.Revenue, arpuBase),
		ARPPU:        safeDivide(agg.Revenue, float64(agg.Payers)),
	}
	if arpuBase > 0 {
		metrics.ConversionRate = safeDivide(float64(agg.Payers), arpuBase)
	}

	revenueStats, err := l.svcCtx.PaymentsModel.DailyRevenue(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	trends := buildPaymentsTrends(revenueStats)

	return &types.PaymentsResponse{
		Metrics: metrics,
		Trends:  trends,
	}, nil
}
