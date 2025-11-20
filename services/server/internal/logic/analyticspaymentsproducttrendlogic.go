package logic

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsProductTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsPaymentsProductTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsProductTrendLogic {
	return &AnalyticsPaymentsProductTrendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsProductTrendLogic) AnalyticsPaymentsProductTrend(req *types.AnalyticsPaymentsProductTrendQuery) (*types.AnalyticsPaymentsProductTrendResponse, error) {
	ch := l.svcCtx.ClickHouse()
	resp := &types.AnalyticsPaymentsProductTrendResponse{Products: []types.AnalyticsPaymentsProductTrend{}}
	if ch == nil {
		return resp, nil
	}
	ids := parseCSV(req.ProductId)
	if len(ids) == 0 {
		return nil, ErrInvalidRequest
	}
	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)
	if start == "" || end == "" {
		return nil, ErrInvalidRequest
	}
	gran := strings.ToLower(strings.TrimSpace(req.Granularity))
	if gran != "minute" {
		gran = "hour"
	}
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
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	bucket := "toStartOfHour(time)"
	if gran == "minute" {
		bucket = "toStartOfMinute(time)"
	}
	query := "SELECT " + bucket + " AS t, product_id, sumIf(amount_cents,status='success') AS rev, countIf(status='success') AS succ, count() AS total FROM analytics.payments" +
		where + " AND product_id IN (" + strings.Join(placeholders, ",") + ") GROUP BY t, product_id ORDER BY t"
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type point struct {
		T              time.Time
		Rev, Succ, Tot uint64
	}
	data := map[string][]point{}
	for rows.Next() {
		var ts time.Time
		var pid string
		var rev, succ, tot uint64
		if err := rows.Scan(&ts, &pid, &rev, &succ, &tot); err != nil {
			return nil, err
		}
		data[pid] = append(data[pid], point{T: ts, Rev: rev, Succ: succ, Tot: tot})
	}
	for _, id := range ids {
		points := make([]types.AnalyticsPaymentsProductTrendPoint, 0, len(data[id]))
		for _, p := range data[id] {
			points = append(points, types.AnalyticsPaymentsProductTrendPoint{
				Time:         p.T.Format(time.RFC3339),
				Success:      p.Succ,
				Total:        p.Tot,
				RevenueCents: p.Rev,
			})
		}
		resp.Products = append(resp.Products, types.AnalyticsPaymentsProductTrend{
			ProductId: id,
			Points:    points,
		})
	}
	return resp, nil
}

func parseCSV(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
