package logic

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorFunnelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsBehaviorFunnelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorFunnelLogic {
	return &AnalyticsBehaviorFunnelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorFunnelLogic) AnalyticsBehaviorFunnel(req *types.AnalyticsBehaviorFunnelQuery) (*types.AnalyticsBehaviorFunnelResponse, error) {
	resp := &types.AnalyticsBehaviorFunnelResponse{Steps: []types.AnalyticsBehaviorFunnelStep{}}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	stepStr := strings.TrimSpace(req.Steps)
	if stepStr == "" {
		return resp, nil
	}
	steps := make([]string, 0)
	for _, part := range strings.Split(stepStr, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			steps = append(steps, part)
		}
	}
	if len(steps) == 0 {
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
	game := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	where := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
	args := []any{start, end}
	if game != "" {
		where += " AND game_id=?"
		args = append(args, game)
	}
	if env != "" {
		where += " AND env=?"
		args = append(args, env)
	}
	sequential := strings.TrimSpace(req.Sequential) == "1"
	if !sequential {
		return l.simpleFunnel(ch, where, args, steps)
	}
	sameSession := strings.TrimSpace(req.SameSession) == "1"
	gapSec := req.GapSec
	if gapSec < 0 {
		gapSec = 0
	}
	maxUsers := req.MaxUsers
	if maxUsers <= 0 || maxUsers > 200000 {
		maxUsers = 50000
	}
	return l.sequentialFunnel(ch, where, args, steps, sameSession, gapSec, maxUsers)
}

func (l *AnalyticsBehaviorFunnelLogic) simpleFunnel(ch clickhouse.Conn, where string, args []any, steps []string) (*types.AnalyticsBehaviorFunnelResponse, error) {
	resp := &types.AnalyticsBehaviorFunnelResponse{Steps: []types.AnalyticsBehaviorFunnelStep{}}
	firstArgs := append([]any{}, args...)
	firstArgs = append(firstArgs, steps[0])
	var first uint64
	_ = ch.QueryRow(l.ctx, "SELECT uniqExact(user_id) FROM analytics.events "+where+" AND event=?", firstArgs...).Scan(&first)
	firstSetSQL := "SELECT user_id FROM analytics.events" + where + " AND event=?"
	for i, step := range steps {
		count := uint64(0)
		if i == 0 {
			count = first
		} else {
			qry := "SELECT uniqExact(user_id) FROM analytics.events" + where + " AND event=? AND user_id IN (" + firstSetSQL + ")"
			stepArgs := append([]any{}, args...)
			stepArgs = append(stepArgs, step)
			stepArgs = append(stepArgs, firstArgs...)
			_ = ch.QueryRow(l.ctx, qry, stepArgs...).Scan(&count)
		}
		resp.Steps = append(resp.Steps, types.AnalyticsBehaviorFunnelStep{
			Step:  step,
			Users: count,
			Rate:  calcFunnelRate(first, count),
		})
	}
	return resp, nil
}

func (l *AnalyticsBehaviorFunnelLogic) sequentialFunnel(ch clickhouse.Conn, where string, args []any, steps []string, sameSession bool, gapSec int, maxUsers int) (*types.AnalyticsBehaviorFunnelResponse, error) {
	resp := &types.AnalyticsBehaviorFunnelResponse{Steps: []types.AnalyticsBehaviorFunnelStep{}}
	placeholders := make([]string, len(steps))
	for i := range steps {
		placeholders[i] = "?"
	}
	qry := "SELECT user_id, groupArray(toUnixTimestamp(event_time)) AS ts, groupArray(event) AS ev, groupArray(session_id) AS sid FROM analytics.events" +
		where + " AND event IN (" + strings.Join(placeholders, ",") + ") GROUP BY user_id LIMIT " + strconv.Itoa(maxUsers)
	stepArgs := append([]any{}, args...)
	for _, s := range steps {
		stepArgs = append(stepArgs, s)
	}
	rows, err := ch.Query(l.ctx, qry, stepArgs...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()
	counts := make([]uint64, len(steps))
	for rows.Next() {
		var uid string
		var ts []uint64
		var ev []string
		var sid []string
		if err := rows.Scan(&uid, &ts, &ev, &sid); err != nil {
			continue
		}
		n := len(ts)
		if n == 0 {
			continue
		}
		idx := make([]int, n)
		for i := 0; i < n; i++ {
			idx[i] = i
		}
		sort.Slice(idx, func(i, j int) bool {
			return ts[idx[i]] < ts[idx[j]]
		})
		matched := -1
		var lastTs uint64
		sess := ""
		for _, k := range idx {
			if matched+1 >= len(steps) {
				break
			}
			e := ev[k]
			next := steps[matched+1]
			if e != next {
				continue
			}
			currentTs := ts[k]
			sessionID := ""
			if k < len(sid) {
				sessionID = sid[k]
			}
			if matched >= 0 {
				if gapSec > 0 && currentTs > lastTs+uint64(gapSec) {
					continue
				}
				if sameSession {
					if sess == "" || sessionID == "" || sess != sessionID {
						continue
					}
				}
			}
			matched++
			lastTs = currentTs
			if matched == 0 && sameSession {
				sess = sessionID
			}
			if matched+1 == len(steps) {
				break
			}
		}
		if matched >= 0 {
			for i := 0; i <= matched && i < len(counts); i++ {
				counts[i]++
			}
		}
	}
	first := counts[0]
	for i, step := range steps {
		resp.Steps = append(resp.Steps, types.AnalyticsBehaviorFunnelStep{
			Step:  step,
			Users: counts[i],
			Rate:  calcFunnelRate(first, counts[i]),
		})
	}
	return resp, nil
}

func calcFunnelRate(base, val uint64) float64 {
	if base == 0 {
		return 0
	}
	return math.Round((float64(val) * 10000.0 / float64(base))) / 100.0
}
