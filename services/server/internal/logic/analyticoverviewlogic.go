package logic

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsOverviewLogic {
	return &AnalyticsOverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsOverviewLogic) AnalyticsOverview(req *types.AnalyticsOverviewQuery) (*types.AnalyticsOverviewResponse, error) {
	resp := &types.AnalyticsOverviewResponse{
		Series: types.AnalyticsOverviewSeries{
			NewUsers:     [][]any{},
			PeakOnline:   [][]any{},
			RevenueCents: [][]any{},
		},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)
	if start == "" || end == "" {
		t2 := time.Now()
		t1 := t2.Add(-7 * 24 * time.Hour)
		start = t1.Format(time.RFC3339)
		end = t2.Format(time.RFC3339)
	}
	wGame, wEnv, filterArgs := buildAnalyticsFilters(strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	args := append([]any{start, end}, filterArgs...)
	rows, err := ch.Query(l.ctx, "SELECT toDate(d) AS d, sum(new_users) FROM analytics.daily_users WHERE d BETWEEN toDate(?) AND toDate(?)"+wGame+wEnv+" GROUP BY d ORDER BY d", args...)
	if err == nil {
		for rows.Next() {
			var d time.Time
			var v uint64
			_ = rows.Scan(&d, &v)
			resp.Series.NewUsers = append(resp.Series.NewUsers, []any{d.Format(time.RFC3339), v})
		}
		rows.Close()
	}
	rows, err = ch.Query(l.ctx, "SELECT d, maxMerge(peak_online) FROM analytics.daily_online_peak WHERE d BETWEEN toDate(?) AND toDate(?)"+wGame+wEnv+" GROUP BY d ORDER BY d", args...)
	if err == nil {
		for rows.Next() {
			var d time.Time
			var v uint64
			_ = rows.Scan(&d, &v)
			resp.Series.PeakOnline = append(resp.Series.PeakOnline, []any{d.Format(time.RFC3339), v})
		}
		rows.Close()
	}
	rows, err = ch.Query(l.ctx, "SELECT toDate(d) AS d, sum(revenue_cents) FROM analytics.daily_revenue WHERE d BETWEEN toDate(?) AND toDate(?)"+wGame+wEnv+" GROUP BY d ORDER BY d", args...)
	if err == nil {
		for rows.Next() {
			var d time.Time
			var v uint64
			_ = rows.Scan(&d, &v)
			resp.Series.RevenueCents = append(resp.Series.RevenueCents, []any{d.Format(time.RFC3339), v})
		}
		rows.Close()
	}
	filterOnly := append([]any{}, filterArgs...)
	today := time.Now().Format("2006-01-02")
	_ = ch.QueryRow(l.ctx, "SELECT sum(dau) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, filterOnly...)...).Scan(&resp.Dau)
	_ = ch.QueryRow(l.ctx, "SELECT sum(new_users) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, filterOnly...)...).Scan(&resp.NewUsers)
	_ = ch.QueryRow(l.ctx, "SELECT sum(revenue_cents) FROM analytics.daily_revenue WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, filterOnly...)...).Scan(&resp.RevenueCents)
	var payers uint64
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.payments WHERE status='success' AND time>=toDateTime(?) AND time<toDateTime(?)"+wGame+wEnv,
		append([]any{today + " 00:00:00", today + " 23:59:59"}, filterOnly...)...).Scan(&payers)
	if resp.Dau > 0 {
		resp.Arpu = float64(resp.RevenueCents) / float64(resp.Dau)
	}
	if payers > 0 {
		resp.Arppu = float64(resp.RevenueCents) / float64(payers)
	}
	if resp.Dau > 0 {
		resp.PayRate = float64(payers) * 100.0 / float64(resp.Dau)
	}
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE event_time>=now()-interval 7 day"+wGame+wEnv, filterOnly...).Scan(&resp.Wau)
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE event_time>=now()-interval 30 day"+wGame+wEnv, filterOnly...).Scan(&resp.Mau)
	y := time.Now().Add(-24 * time.Hour)
	ymd := y.Format("2006-01-02")
	var cohort uint64
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event IN ('register','first_active')"+wGame+wEnv,
		append([]any{ymd}, filterOnly...)...).Scan(&cohort)
	calcRet := func(offsetDays int) float64 {
		if cohort == 0 {
			return 0
		}
		tgt := y.Add(time.Duration(offsetDays) * 24 * time.Hour).Format("2006-01-02")
		params := append([]any{tgt, ymd}, filterOnly...)
		params = append(params, filterOnly...)
		var kept uint64
		_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?)"+wGame+wEnv+
			" AND user_id IN (SELECT user_id FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event IN ('register','first_active')"+wGame+wEnv+")",
			params...).Scan(&kept)
		return math.Round((float64(kept) * 10000.0 / float64(cohort))) / 100.0
	}
	resp.D1 = calcRet(1)
	resp.D7 = calcRet(7)
	resp.D30 = calcRet(30)
	return resp, nil
}
