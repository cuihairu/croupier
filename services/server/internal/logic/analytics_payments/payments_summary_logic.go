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

type PaymentsSummaryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付摘要
func NewPaymentsSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsSummaryLogic {
	return &PaymentsSummaryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsSummaryLogic) PaymentsSummary(req *types.PaymentsSummaryRequest) (*types.PaymentsSummaryResponse, error) {
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

	stats, err := l.svcCtx.PaymentsModel.DailyRevenue(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	items := summarizePayments(stats, req.GroupBy)
	return &types.PaymentsSummaryResponse{
		Items: items,
	}, nil
}
