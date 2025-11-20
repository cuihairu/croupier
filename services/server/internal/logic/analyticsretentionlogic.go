package logic

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsRetentionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsRetentionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsRetentionLogic {
	return &AnalyticsRetentionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsRetentionLogic) AnalyticsRetention(req *types.AnalyticsRetentionQuery) (*types.AnalyticsRetentionResponse, error) {
	resp := &types.AnalyticsRetentionResponse{Cohorts: []types.AnalyticsRetentionCohort{}}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)
	if start == "" || end == "" {
		t2 := time.Now()
		t1 := t2.Add(-14 * 24 * time.Hour)
		start = t1.Format("2006-01-02")
		end = t2.Format("2006-01-02")
	}
	t1, err := time.Parse("2006-01-02", start[:10])
	if err != nil {
		return nil, ErrInvalidRequest
	}
	t2, err := time.Parse("2006-01-02", end[:10])
	if err != nil {
		return nil, ErrInvalidRequest
	}
	if t2.Before(t1) {
		t1, t2 = t2, t1
	}
	baseEvent := "register"
	if strings.EqualFold(strings.TrimSpace(req.Cohort), "first_active") {
		baseEvent = "first_active"
	}
	filters := map[string]string{
		"game_id": strings.TrimSpace(req.GameId),
		"env":     strings.TrimSpace(req.Env),
	}
	wGame, wEnv, extra := buildRetentionFilters(filters)
	for dt := t1; !dt.After(t2); dt = dt.Add(24 * time.Hour) {
		day := dt.Format("2006-01-02")
		var total uint64
		_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event=?"+wGame+wEnv,
			append([]any{day, baseEvent}, extra...)...).Scan(&total)
		if total == 0 {
			resp.Cohorts = append(resp.Cohorts, types.AnalyticsRetentionCohort{Day: day})
			continue
		}
		calc := func(offset int) float64 {
			tgt := dt.Add(time.Duration(offset) * 24 * time.Hour).Format("2006-01-02")
			var kept uint64
			_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?)"+wGame+wEnv+
				" AND user_id IN (SELECT user_id FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event=?"+wGame+wEnv+")",
				append(append([]any{tgt, day, baseEvent}, extra...), extra...)...).Scan(&kept)
			return roundRetentionPercent(kept, total)
		}
		resp.Cohorts = append(resp.Cohorts, types.AnalyticsRetentionCohort{
			Day: day,
			D1:  calc(1),
			D7:  calc(7),
			D30: calc(30),
		})
	}
	return resp, nil
}

func buildRetentionFilters(filters map[string]string) (string, string, []any) {
	wGame, wEnv := "", ""
	args := []any{}
	if val := strings.TrimSpace(filters["game_id"]); val != "" {
		wGame = " AND game_id=?"
		args = append(args, val)
	}
	if val := strings.TrimSpace(filters["env"]); val != "" {
		wEnv = " AND env=?"
		args = append(args, val)
	}
	return wGame, wEnv, args
}

func roundRetentionPercent(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round((float64(part) * 10000.0 / float64(total))) / 100.0
}
