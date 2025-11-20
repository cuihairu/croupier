package logic

import (
	"context"
	"math"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsPaymentsSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsSummaryLogic {
	return &AnalyticsPaymentsSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsSummaryLogic) AnalyticsPaymentsSummary(req *types.AnalyticsPaymentsSummaryQuery) (*types.AnalyticsPaymentsSummaryResponse, error) {
	resp := &types.AnalyticsPaymentsSummaryResponse{
		Totals:     types.AnalyticsPaymentsSummaryTotals{},
		ByChannel:  []types.AnalyticsPaymentsSummaryItem{},
		ByPlatform: []types.AnalyticsPaymentsSummaryItem{},
		ByCountry:  []types.AnalyticsPaymentsSummaryItem{},
		ByRegion:   []types.AnalyticsPaymentsSummaryItem{},
		ByCity:     []types.AnalyticsPaymentsSummaryItem{},
		ByProduct:  []types.AnalyticsPaymentsSummaryItem{},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	start, end := defaultRange(req.Start, req.End, 7)
	filters := map[string]string{
		"game_id":  strings.TrimSpace(req.GameId),
		"env":      strings.TrimSpace(req.Env),
		"channel":  strings.TrimSpace(req.Channel),
		"platform": strings.TrimSpace(req.Platform),
		"country":  strings.TrimSpace(req.Country),
		"region":   strings.TrimSpace(req.Region),
		"city":     strings.TrimSpace(req.City),
	}
	where, args := buildPaymentsWhere(start, end, filters)
	var revSucc, revRefund, succCnt, totalCnt uint64
	_ = ch.QueryRow(l.ctx, "SELECT sumIf(amount_cents,status='success'), sumIf(amount_cents,status='refund'), countIf(status='success'), count() FROM analytics.payments"+where, args...).Scan(&revSucc, &revRefund, &succCnt, &totalCnt)
	resp.Totals.RevenueCents = revSucc
	resp.Totals.RefundsCents = revRefund
	resp.Totals.Failed = totalCnt - succCnt
	if totalCnt > 0 {
		resp.Totals.SuccessRate = roundPercent(float64(succCnt) * 100.0 / float64(totalCnt))
	}
	resp.ByChannel = l.queryPaymentsDim(l.ctx, ch, where, args, "channel", func(item *types.AnalyticsPaymentsSummaryItem, dim string) {
		item.Channel = dim
	})
	resp.ByPlatform = l.queryPaymentsDim(l.ctx, ch, where, args, "platform", func(item *types.AnalyticsPaymentsSummaryItem, dim string) {
		item.Platform = dim
	})
	resp.ByCountry = l.queryPaymentsDim(l.ctx, ch, where, args, "country", func(item *types.AnalyticsPaymentsSummaryItem, dim string) {
		item.Country = dim
	})
	resp.ByRegion = l.queryPaymentsDim(l.ctx, ch, where, args, "region", func(item *types.AnalyticsPaymentsSummaryItem, dim string) {
		item.Region = dim
	})
	resp.ByCity = l.queryPaymentsDim(l.ctx, ch, where, args, "city", func(item *types.AnalyticsPaymentsSummaryItem, dim string) {
		item.City = dim
	})
	resp.ByProduct = l.queryPaymentsDim(l.ctx, ch, where, args, "product_id", func(item *types.AnalyticsPaymentsSummaryItem, dim string) {
		item.ProductId = dim
	})
	return resp, nil
}

func (l *AnalyticsPaymentsSummaryLogic) queryPaymentsDim(ctx context.Context, ch clickhouse.Conn, where string, args []any, column string, setter func(*types.AnalyticsPaymentsSummaryItem, string)) []types.AnalyticsPaymentsSummaryItem {
	rows, err := ch.Query(ctx, "SELECT "+column+", sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+where+" GROUP BY "+column+" ORDER BY revenue DESC", args...)
	if err != nil {
		return []types.AnalyticsPaymentsSummaryItem{}
	}
	defer rows.Close()
	out := []types.AnalyticsPaymentsSummaryItem{}
	for rows.Next() {
		var dim string
		var revenue, succ, total uint64
		if err := rows.Scan(&dim, &revenue, &succ, &total); err != nil {
			continue
		}
		item := types.AnalyticsPaymentsSummaryItem{
			RevenueCents: revenue,
			Success:      succ,
			Total:        total,
			SuccessRate:  calcSummaryRate(total, succ),
		}
		setter(&item, dim)
		out = append(out, item)
	}
	return out
}

func calcSummaryRate(total, succ uint64) float64 {
	if total == 0 {
		return 0
	}
	return roundPercent(float64(succ) * 100.0 / float64(total))
}

func roundPercent(val float64) float64 {
	return math.Round(val*100) / 100
}
