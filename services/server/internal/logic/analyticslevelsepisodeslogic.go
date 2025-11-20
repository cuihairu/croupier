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

type AnalyticsLevelsEpisodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsLevelsEpisodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsLevelsEpisodesLogic {
	return &AnalyticsLevelsEpisodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsLevelsEpisodesLogic) AnalyticsLevelsEpisodes(req *types.AnalyticsLevelsEpisodesQuery) (*types.AnalyticsLevelsEpisodesResponse, error) {
	resp := &types.AnalyticsLevelsEpisodesResponse{Episodes: []types.AnalyticsLevelsEpisode{}}
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
	type key struct {
		Ep  string
		Lvl string
	}
	attempts := l.groupEpisodeLevel("SELECT JSON_VALUE(props_json,'$.episode') as ep, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+where+" AND event IN ('level_start','level_enter','level_begin') GROUP BY ep, lvl", args)
	clears := l.groupEpisodeLevel("SELECT JSON_VALUE(props_json,'$.episode') as ep, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+where+" AND event IN ('level_clear','level_pass','level_win') GROUP BY ep, lvl", args)
	episodes := map[string][]types.AnalyticsLevelsPerLevelSummary{}
	for k, players := range attempts {
		if strings.TrimSpace(k.Ep) == "" || strings.TrimSpace(k.Lvl) == "" {
			continue
		}
		clr := clears[k]
		winRate := 0.0
		if players > 0 {
			winRate = math.Round((float64(clr) * 10000.0 / float64(players))) / 100.0
		}
		episodes[k.Ep] = append(episodes[k.Ep], types.AnalyticsLevelsPerLevelSummary{
			Level:   k.Lvl,
			Players: players,
			WinRate: winRate,
		})
	}
	names := make([]string, 0, len(episodes))
	for ep := range episodes {
		names = append(names, ep)
	}
	sort.Strings(names)
	for _, ep := range names {
		per := episodes[ep]
		sort.Slice(per, func(i, j int) bool {
			return strings.Compare(per[i].Level, per[j].Level) < 0
		})
		resp.Episodes = append(resp.Episodes, types.AnalyticsLevelsEpisode{
			Episode:  ep,
			PerLevel: per,
		})
	}
	return resp, nil
}

type AnalyticsLevelsMapsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsLevelsMapsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsLevelsMapsLogic {
	return &AnalyticsLevelsMapsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsLevelsMapsLogic) AnalyticsLevelsMaps(req *types.AnalyticsLevelsEpisodesQuery) (*types.AnalyticsLevelsMapsResponse, error) {
	resp := &types.AnalyticsLevelsMapsResponse{Maps: []types.AnalyticsLevelsMap{}}
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
	type key struct {
		Map string
		Lvl string
	}
	attempts := l.groupMapLevel("SELECT JSON_VALUE(props_json,'$.map') as mp, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+where+" AND event IN ('level_start','level_enter','level_begin') GROUP BY mp, lvl", args)
	clears := l.groupMapLevel("SELECT JSON_VALUE(props_json,'$.map') as mp, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+where+" AND event IN ('level_clear','level_pass','level_win') GROUP BY mp, lvl", args)
	maps := map[string][]types.AnalyticsLevelsPerLevelSummary{}
	for k, players := range attempts {
		if strings.TrimSpace(k.Map) == "" || strings.TrimSpace(k.Lvl) == "" {
			continue
		}
		clr := clears[k]
		winRate := 0.0
		if players > 0 {
			winRate = math.Round((float64(clr) * 10000.0 / float64(players))) / 100.0
		}
		maps[k.Map] = append(maps[k.Map], types.AnalyticsLevelsPerLevelSummary{
			Level:   k.Lvl,
			Players: players,
			WinRate: winRate,
		})
	}
	names := make([]string, 0, len(maps))
	for m := range maps {
		names = append(names, m)
	}
	sort.Strings(names)
	for _, m := range names {
		per := maps[m]
		sort.Slice(per, func(i, j int) bool {
			return strings.Compare(per[i].Level, per[j].Level) < 0
		})
		resp.Maps = append(resp.Maps, types.AnalyticsLevelsMap{
			Map:      m,
			PerLevel: per,
		})
	}
	return resp, nil
}

func (l *AnalyticsLevelsEpisodesLogic) groupEpisodeLevel(query string, args []any) map[struct{ Ep, Lvl string }]uint64 {
	ch := l.svcCtx.ClickHouse()
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return map[struct{ Ep, Lvl string }]uint64{}
	}
	defer rows.Close()
	out := map[struct{ Ep, Lvl string }]uint64{}
	for rows.Next() {
		var ep, lvl string
		var n uint64
		if err := rows.Scan(&ep, &lvl, &n); err != nil {
			continue
		}
		out[struct{ Ep, Lvl string }{Ep: ep, Lvl: lvl}] = n
	}
	return out
}

func (l *AnalyticsLevelsMapsLogic) groupMapLevel(query string, args []any) map[struct{ Map, Lvl string }]uint64 {
	ch := l.svcCtx.ClickHouse()
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return map[struct{ Map, Lvl string }]uint64{}
	}
	defer rows.Close()
	out := map[struct{ Map, Lvl string }]uint64{}
	for rows.Next() {
		var mp, lvl string
		var n uint64
		if err := rows.Scan(&mp, &lvl, &n); err != nil {
			continue
		}
		out[struct{ Map, Lvl string }{Map: mp, Lvl: lvl}] = n
	}
	return out
}
