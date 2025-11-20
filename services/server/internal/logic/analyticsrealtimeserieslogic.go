package logic

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsRealtimeSeriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsRealtimeSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsRealtimeSeriesLogic {
	return &AnalyticsRealtimeSeriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsRealtimeSeriesLogic) AnalyticsRealtimeSeries(req *types.AnalyticsRealtimeSeriesQuery) (*types.AnalyticsRealtimeSeriesResponse, error) {
	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)
	if start == "" || end == "" {
		return nil, ErrInvalidRequest
	}
	resp := &types.AnalyticsRealtimeSeriesResponse{
		Online:       [][]any{},
		Active5mSum:  [][]any{},
		Active15mSum: [][]any{},
		RevenueCents: [][]any{},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	wGame, wEnv, filterArgs := buildAnalyticsFilters(strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	args := append([]any{start, end}, filterArgs...)
	rows, err := ch.Query(l.ctx, "SELECT m, online, sum(online) OVER (ORDER BY m ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) AS a5, sum(online) OVER (ORDER BY m ROWS BETWEEN 14 PRECEDING AND CURRENT ROW) AS a15 FROM analytics.minute_online WHERE m>=toDateTime(?) AND m<=toDateTime(?)"+wGame+wEnv+" ORDER BY m", args...)
	if err == nil {
		for rows.Next() {
			var m time.Time
			var v, a5, a15 uint64
			_ = rows.Scan(&m, &v, &a5, &a15)
			ts := m.Format(time.RFC3339)
			resp.Online = append(resp.Online, []any{ts, v})
			resp.Active5mSum = append(resp.Active5mSum, []any{ts, a5})
			resp.Active15mSum = append(resp.Active15mSum, []any{ts, a15})
		}
		rows.Close()
	}
	rows, err = ch.Query(l.ctx, "SELECT toStartOfMinute(time) AS m, sumIf(amount_cents,status='success') AS rev FROM analytics.payments WHERE time>=toDateTime(?) AND time<=toDateTime(?)"+wGame+wEnv+" GROUP BY m ORDER BY m", args...)
	if err == nil {
		for rows.Next() {
			var m time.Time
			var v uint64
			_ = rows.Scan(&m, &v)
			resp.RevenueCents = append(resp.RevenueCents, []any{m.Format(time.RFC3339), v})
		}
		rows.Close()
	}
	return resp, nil
}
