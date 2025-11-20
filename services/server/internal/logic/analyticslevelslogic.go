package logic

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsLevelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsLevelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsLevelsLogic {
	return &AnalyticsLevelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsLevelsLogic) AnalyticsLevels(req *types.AnalyticsLevelsQuery) (*types.AnalyticsLevelsResponse, error) {
	resp := &types.AnalyticsLevelsResponse{
		Funnel:           []types.AnalyticsLevelsFunnelStep{},
		PerLevel:         []types.AnalyticsLevelsPerLevel{},
		PerLevelSegments: types.AnalyticsLevelsSegments{New: []types.AnalyticsLevelsSegmentEntry{}, Payer: []types.AnalyticsLevelsSegmentEntry{}, Returning: []types.AnalyticsLevelsSegmentEntry{}},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	start, end := defaultRange(req.Start, req.End, 7)
	filters := map[string]string{
		"game_id": strings.TrimSpace(req.GameId),
		"env":     strings.TrimSpace(req.Env),
	}
	where, args := buildEventsWhere(start, end, filters)
	if ep := strings.TrimSpace(req.Episode); ep != "" {
		where += " AND JSON_VALUE(props_json,'$.episode') = ?"
		args = append(args, ep)
	}
	startEvents := []string{"level_start", "level_enter", "level_begin"}
	clearEvents := []string{"level_clear", "level_pass", "level_win"}
	failEvents := []string{"level_fail", "level_lose", "level_dead"}
	resp.Funnel = l.buildFunnel(where, args, startEvents, clearEvents, failEvents)
	perLevel, attempts, clears := l.buildPerLevel(where, args, startEvents, clearEvents)
	resp.PerLevel = perLevel
	resp.PerLevelSegments = l.buildSegments(where, args, startEvents, clearEvents, attempts, clears, start, end)
	return resp, nil
}

func (l *AnalyticsLevelsLogic) buildFunnel(where string, args []any, startEvents, clearEvents, failEvents []string) []types.AnalyticsLevelsFunnelStep {
	ch := l.svcCtx.ClickHouse()
	startWhere := where + " AND event IN (" + quoteEvents(startEvents) + ")"
	clearWhere := where + " AND event IN (" + quoteEvents(clearEvents) + ")"
	failWhere := where + " AND event IN (" + quoteEvents(failEvents) + ")"
	var s1, s2, s3 uint64
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events"+startWhere, args...).Scan(&s1)
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events"+clearWhere, args...).Scan(&s2)
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events"+failWhere, args...).Scan(&s3)
	rate := func(n uint64) float64 {
		if s1 == 0 {
			return 0
		}
		return math.Round((float64(n) * 10000.0 / float64(s1))) / 100.0
	}
	return []types.AnalyticsLevelsFunnelStep{
		{Step: "开始关卡", Users: s1, Rate: 100.0},
		{Step: "完成关卡", Users: s2, Rate: rate(s2)},
		{Step: "失败过", Users: s3, Rate: rate(s3)},
	}
}

func (l *AnalyticsLevelsLogic) buildPerLevel(where string, args []any, startEvents, clearEvents []string) ([]types.AnalyticsLevelsPerLevel, map[string]uint64, map[string]uint64) {
	attempts := l.levelCounts(where, args, startEvents)
	clears := l.levelCounts(where, args, clearEvents)
	detail := l.levelDetail(where, args, clearEvents)
	per := make([]types.AnalyticsLevelsPerLevel, 0, len(attempts))
	for lvl, players := range attempts {
		clr := clears[lvl]
		winRate := 0.0
		if players > 0 {
			winRate = math.Round((float64(clr) * 10000.0 / float64(players))) / 100.0
		}
		d := detail[lvl]
		per = append(per, types.AnalyticsLevelsPerLevel{
			Level:          lvl,
			Players:        players,
			WinRate:        winRate,
			AvgDurationSec: d.avgDuration,
			AvgRetries:     d.avgRetries,
			Difficulty:     difficulty(winRate),
		})
	}
	sort.Slice(per, func(i, j int) bool {
		return strings.Compare(per[i].Level, per[j].Level) < 0
	})
	return per, attempts, clears
}

func (l *AnalyticsLevelsLogic) buildSegments(where string, args []any, startEvents, clearEvents []string, attempts, clears map[string]uint64, start, end string) types.AnalyticsLevelsSegments {
	startWhere := where + " AND event IN (" + quoteEvents(startEvents) + ")"
	clearWhere := where + " AND event IN (" + quoteEvents(clearEvents) + ")"
	newAttempts := l.segmentCounts("SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+startWhere+" AND user_id IN (SELECT user_id FROM analytics.events"+where+" AND event IN ('register','first_active')) GROUP BY lvl", append(cloneArgs(args), args...))
	newClears := l.segmentCounts("SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+clearWhere+" AND user_id IN (SELECT user_id FROM analytics.events"+where+" AND event IN ('register','first_active')) GROUP BY lvl", append(cloneArgs(args), args...))
	payerSub := "SELECT user_id FROM analytics.payments WHERE time>=toDateTime(?) AND time<=toDateTime(?) AND status='success'" + strings.ReplaceAll(strings.ReplaceAll(where, " WHERE", " AND"), "event_time", "time")
	payerArgs := append([]any{start, end}, cloneArgs(args)...)
	payerAttempts := l.segmentCounts("SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+startWhere+" AND user_id IN ("+payerSub+") GROUP BY lvl", append(cloneArgs(args), payerArgs...))
	payerClears := l.segmentCounts("SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+clearWhere+" AND user_id IN ("+payerSub+") GROUP BY lvl", append(cloneArgs(args), payerArgs...))
	return types.AnalyticsLevelsSegments{
		New:       buildSegmentEntries(newAttempts, newClears),
		Payer:     buildSegmentEntries(payerAttempts, payerClears),
		Returning: buildSegmentEntries(diffCounts(attempts, newAttempts), diffCounts(clears, newClears)),
	}
}

func (l *AnalyticsLevelsLogic) levelCounts(where string, args []any, events []string) map[string]uint64 {
	ch := l.svcCtx.ClickHouse()
	query := "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events" + where + " AND event IN (" + quoteEvents(events) + ") GROUP BY lvl"
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return map[string]uint64{}
	}
	defer rows.Close()
	out := map[string]uint64{}
	for rows.Next() {
		var lvl string
		var n uint64
		if err := rows.Scan(&lvl, &n); err != nil {
			continue
		}
		if strings.TrimSpace(lvl) != "" {
			out[lvl] = n
		}
	}
	return out
}

type levelDetailStat struct {
	avgDuration float64
	avgRetries  float64
}

func (l *AnalyticsLevelsLogic) levelDetail(where string, args []any, events []string) map[string]levelDetailStat {
	ch := l.svcCtx.ClickHouse()
	query := "SELECT JSON_VALUE(props_json,'$.level') as lvl, avgOrNull(toFloat64OrNull(JSON_VALUE(props_json,'$.duration_sec'))), avgOrNull(toFloat64OrNull(JSON_VALUE(props_json,'$.retries'))) FROM analytics.events" + where + " AND event IN (" + quoteEvents(events) + ") GROUP BY lvl"
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return map[string]levelDetailStat{}
	}
	defer rows.Close()
	out := map[string]levelDetailStat{}
	for rows.Next() {
		var lvl string
		var dur, retry *float64
		if err := rows.Scan(&lvl, &dur, &retry); err != nil {
			continue
		}
		stat := levelDetailStat{}
		if dur != nil {
			stat.avgDuration = math.Round((*dur)*100.0) / 100.0
		}
		if retry != nil {
			stat.avgRetries = math.Round((*retry)*100.0) / 100.0
		}
		out[lvl] = stat
	}
	return out
}

func (l *AnalyticsLevelsLogic) segmentCounts(query string, args []any) map[string]uint64 {
	ch := l.svcCtx.ClickHouse()
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return map[string]uint64{}
	}
	defer rows.Close()
	out := map[string]uint64{}
	for rows.Next() {
		var lvl string
		var n uint64
		if err := rows.Scan(&lvl, &n); err != nil {
			continue
		}
		out[lvl] = n
	}
	return out
}

func buildSegmentEntries(attempts, clears map[string]uint64) []types.AnalyticsLevelsSegmentEntry {
	out := make([]types.AnalyticsLevelsSegmentEntry, 0, len(attempts))
	for lvl, a := range attempts {
		c := clears[lvl]
		wr := 0.0
		if a > 0 {
			wr = math.Round((float64(c) * 10000.0 / float64(a))) / 100.0
		}
		out = append(out, types.AnalyticsLevelsSegmentEntry{
			Level:   lvl,
			Players: a,
			WinRate: wr,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Compare(out[i].Level, out[j].Level) < 0
	})
	return out
}

func diffCounts(all, subset map[string]uint64) map[string]uint64 {
	out := map[string]uint64{}
	for lvl, total := range all {
		sub := subset[lvl]
		if total > sub {
			out[lvl] = total - sub
		} else {
			out[lvl] = 0
		}
	}
	return out
}

func difficulty(winRate float64) string {
	switch {
	case winRate > 70:
		return "低"
	case winRate >= 40:
		return "中"
	default:
		return "高"
	}
}

func quoteEvents(events []string) string {
	arr := make([]string, len(events))
	for i, e := range events {
		arr[i] = "'" + e + "'"
	}
	return strings.Join(arr, ",")
}
