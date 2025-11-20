package logic

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorPathsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsBehaviorPathsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorPathsLogic {
	return &AnalyticsBehaviorPathsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorPathsLogic) AnalyticsBehaviorPaths(req *types.AnalyticsBehaviorPathsQuery) (*types.AnalyticsBehaviorPathsResponse, error) {
	resp := &types.AnalyticsBehaviorPathsResponse{Paths: []types.AnalyticsBehaviorPath{}}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	per := strings.ToLower(strings.TrimSpace(req.Per))
	if per != "user" {
		per = "session"
	}
	steps := req.Steps
	if steps <= 0 || steps > 10 {
		steps = 5
	}
	limit := req.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	maxGroups := req.MaxGroups
	if maxGroups <= 0 || maxGroups > 200000 {
		maxGroups = 50000
	}
	gapSec := req.GapSec
	if gapSec < 0 {
		gapSec = 0
	}
	sameSession := strings.TrimSpace(req.SameSession) == "1"
	start, end := strings.TrimSpace(req.Start), strings.TrimSpace(req.End)
	if start == "" || end == "" {
		t2 := time.Now()
		t1 := t2.Add(-7 * 24 * time.Hour)
		start = t1.Format(time.RFC3339)
		end = t2.Format(time.RFC3339)
	}
	where := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
	args := []any{start, end}
	if game := strings.TrimSpace(req.GameId); game != "" {
		where += " AND game_id=?"
		args = append(args, game)
	}
	if env := strings.TrimSpace(req.Env); env != "" {
		where += " AND env=?"
		args = append(args, env)
	}
	if inc := parseCSV(req.Include); len(inc) > 0 {
		placeholders := make([]string, len(inc))
		for i := range inc {
			placeholders[i] = "?"
		}
		where += " AND event IN (" + strings.Join(placeholders, ",") + ")"
		for _, v := range inc {
			args = append(args, v)
		}
	}
	if exc := parseCSV(req.Exclude); len(exc) > 0 {
		placeholders := make([]string, len(exc))
		for i := range exc {
			placeholders[i] = "?"
		}
		where += " AND event NOT IN (" + strings.Join(placeholders, ",") + ")"
		for _, v := range exc {
			args = append(args, v)
		}
	}
	groupExpr := "user_id"
	if per == "session" {
		groupExpr = "concat(user_id, '\\u007c', session_id)"
	}
	query := fmt.Sprintf("SELECT %s AS grp, groupArray(toUnixTimestamp(event_time) ORDER BY event_time) AS ts, groupArray(event ORDER BY event_time) AS ev, groupArray(session_id ORDER BY event_time) AS sid FROM analytics.events%s GROUP BY grp LIMIT %d",
		groupExpr, where, maxGroups)
	rows, err := ch.Query(l.ctx, query, args...)
	if err != nil {
		return resp, nil
	}
	defer rows.Close()
	var includeRe, excludeRe *regexp.Regexp
	if pattern := strings.TrimSpace(req.PathRe); pattern != "" {
		if re, err := regexp.Compile(pattern); err == nil {
			includeRe = re
		}
	}
	if pattern := strings.TrimSpace(req.PathNotRe); pattern != "" {
		if re, err := regexp.Compile(pattern); err == nil {
			excludeRe = re
		}
	}
	counts := map[string]uint64{}
	for rows.Next() {
		var grp string
		var ts []uint64
		var events []string
		var sessions []string
		if err := rows.Scan(&grp, &ts, &events, &sessions); err != nil {
			continue
		}
		path := buildPath(events, ts, sessions, steps, sameSession, gapSec)
		if path == "" {
			continue
		}
		if includeRe != nil && !includeRe.MatchString(path) {
			continue
		}
		if excludeRe != nil && excludeRe.MatchString(path) {
			continue
		}
		counts[path]++
	}
	paths := make([]types.AnalyticsBehaviorPath, 0, len(counts))
	for p, c := range counts {
		paths = append(paths, types.AnalyticsBehaviorPath{Path: p, Groups: c})
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Groups == paths[j].Groups {
			return paths[i].Path < paths[j].Path
		}
		return paths[i].Groups > paths[j].Groups
	})
	if len(paths) > limit {
		paths = paths[:limit]
	}
	resp.Paths = paths
	return resp, nil
}

func buildPath(events []string, ts []uint64, sessions []string, steps int, sameSession bool, gapSec int) string {
	if len(events) == 0 {
		return ""
	}
	pathEvents := make([]string, 0, steps)
	var baseSession string
	var lastTs uint64
	for i := 0; i < len(events) && len(pathEvents) < steps; i++ {
		ev := strings.TrimSpace(events[i])
		if ev == "" {
			continue
		}
		if sameSession {
			s := safeSession(sessions, i)
			if len(pathEvents) == 0 {
				baseSession = s
			} else if baseSession == "" || s == "" || s != baseSession {
				continue
			}
		}
		if gapSec > 0 && len(pathEvents) > 0 && ts[i] > lastTs+uint64(gapSec) {
			break
		}
		pathEvents = append(pathEvents, ev)
		lastTs = ts[i]
	}
	if len(pathEvents) == 0 {
		return ""
	}
	return strings.Join(pathEvents, ">")
}

func safeSession(sessions []string, idx int) string {
	if idx >= 0 && idx < len(sessions) {
		return sessions[idx]
	}
	return ""
}
