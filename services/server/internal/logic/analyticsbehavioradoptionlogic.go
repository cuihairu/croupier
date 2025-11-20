package logic

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorAdoptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsBehaviorAdoptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorAdoptionLogic {
	return &AnalyticsBehaviorAdoptionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorAdoptionLogic) AnalyticsBehaviorAdoption(req *types.AnalyticsBehaviorAdoptionQuery) (*types.AnalyticsBehaviorAdoptionResponse, error) {
	resp := &types.AnalyticsBehaviorAdoptionResponse{
		Features: []types.AnalyticsBehaviorAdoptionFeature{},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	features := parseCSV(req.Features)
	if len(features) == 0 {
		return resp, nil
	}
	per := strings.ToLower(strings.TrimSpace(req.Per))
	grp := "user_id"
	if per == "session" {
		grp = "concat(user_id, '\\u007c', session_id)"
	}
	start, end := defaultRange(req.Start, req.End, 7)
	where, args := buildEventsWhere(start, end, map[string]string{
		"game_id": strings.TrimSpace(req.GameId),
		"env":     strings.TrimSpace(req.Env),
	})
	var baseline uint64
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact("+grp+") FROM analytics.events"+where, args...).Scan(&baseline)
	resp.Baseline = baseline
	placeholders := make([]string, len(features))
	featArgs := append([]any{}, args...)
	for i := range features {
		placeholders[i] = "?"
		featArgs = append(featArgs, features[i])
	}
	query := "SELECT event, uniqExact(" + grp + ") FROM analytics.events" + where + " AND event IN (" + strings.Join(placeholders, ",") + ") GROUP BY event"
	rows, err := ch.Query(l.ctx, query, featArgs...)
	if err != nil {
		return resp, nil
	}
	defer rows.Close()
	counts := map[string]uint64{}
	for rows.Next() {
		var event string
		var n uint64
		if err := rows.Scan(&event, &n); err != nil {
			continue
		}
		counts[event] = n
	}
	for _, feature := range features {
		n := counts[feature]
		rate := 0.0
		if baseline > 0 {
			rate = math.Round((float64(n) * 10000.0 / float64(baseline))) / 100.0
		}
		resp.Features = append(resp.Features, types.AnalyticsBehaviorAdoptionFeature{
			Feature: feature,
			Groups:  n,
			Rate:    rate,
		})
	}
	return resp, nil
}

type AnalyticsBehaviorAdoptionBreakdownLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsBehaviorAdoptionBreakdownLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorAdoptionBreakdownLogic {
	return &AnalyticsBehaviorAdoptionBreakdownLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorAdoptionBreakdownLogic) AnalyticsBehaviorAdoptionBreakdown(req *types.AnalyticsBehaviorAdoptionBreakdownQuery) (*types.AnalyticsBehaviorAdoptionBreakdownResponse, error) {
	resp := &types.AnalyticsBehaviorAdoptionBreakdownResponse{
		By:   normalizedDim(req.By),
		Rows: []types.AnalyticsBehaviorAdoptionBreakdownRow{},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	features := parseCSV(req.Features)
	if len(features) == 0 {
		return resp, nil
	}
	per := strings.ToLower(strings.TrimSpace(req.Per))
	grp := "user_id"
	if per == "session" {
		grp = "concat(user_id, '\\u007c', session_id)"
	}
	start, end := defaultRange(req.Start, req.End, 7)
	where, args := buildEventsWhere(start, end, map[string]string{
		"game_id": strings.TrimSpace(req.GameId),
		"env":     strings.TrimSpace(req.Env),
	})
	baseSQL := "SELECT " + resp.By + ", uniqExact(" + grp + ") FROM analytics.events" + where + " GROUP BY " + resp.By
	baseRows, err := ch.Query(l.ctx, baseSQL, args...)
	if err != nil {
		return resp, nil
	}
	defer baseRows.Close()
	baseline := map[string]uint64{}
	for baseRows.Next() {
		var dim string
		var n uint64
		if err := baseRows.Scan(&dim, &n); err != nil {
			continue
		}
		baseline[dim] = n
	}
	placeholders := make([]string, len(features))
	featArgs := append([]any{}, args...)
	for i := range features {
		placeholders[i] = "?"
		featArgs = append(featArgs, features[i])
	}
	featSQL := "SELECT " + resp.By + ", uniqExact(" + grp + ") FROM analytics.events" + where + " AND event IN (" + strings.Join(placeholders, ",") + ") GROUP BY " + resp.By
	featRows, err := ch.Query(l.ctx, featSQL, featArgs...)
	if err != nil {
		return resp, nil
	}
	defer featRows.Close()
	counts := map[string]uint64{}
	for featRows.Next() {
		var dim string
		var n uint64
		if err := featRows.Scan(&dim, &n); err != nil {
			continue
		}
		counts[dim] = n
	}
	for dim, base := range baseline {
		num := counts[dim]
		rate := 0.0
		if base > 0 {
			rate = math.Round((float64(num) * 10000.0 / float64(base))) / 100.0
		}
		resp.Rows = append(resp.Rows, types.AnalyticsBehaviorAdoptionBreakdownRow{
			Dim:      dim,
			Baseline: base,
			Groups:   num,
			Rate:     rate,
		})
	}
	sort.Slice(resp.Rows, func(i, j int) bool {
		return resp.Rows[i].Dim < resp.Rows[j].Dim
	})
	return resp, nil
}

func normalizedDim(by string) string {
	switch strings.ToLower(strings.TrimSpace(by)) {
	case "platform":
		return "platform"
	case "country":
		return "country"
	default:
		return "channel"
	}
}
