package incident

import (
	"context"
	"sort"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// ReportLeaderboard 排行榜（GET /incident-reports/leaderboard）。
func (s *Service) ReportLeaderboard(ctx context.Context, periodType, periodKey, view, board string) (*ReportLeaderboardResponse, error) {
	if view == "" {
		view = ViewResponsible
	}
	if view != ViewResponsible && view != ViewCategory {
		return nil, errorx.NewBadRequest("无效的榜单视图: " + view + "（支持 responsible/category）")
	}
	if board == "" {
		board = BoardIncidents
	}
	if board != BoardIncidents && board != BoardDuration && board != BoardMTTR {
		return nil, errorx.NewBadRequest("无效的榜单类型: " + board + "（支持 incidents/duration/mttr）")
	}
	now := time.Now()
	p, err := resolvePeriod(periodType, periodKey, now)
	if err != nil {
		return nil, err
	}
	cur, err := s.groupStats(ctx, view, p.Start, p.End, now)
	if err != nil {
		return nil, err
	}
	prev, err := s.groupStats(ctx, view, p.PrevStart, p.PrevEnd, now)
	if err != nil {
		return nil, err
	}
	var yoy map[string]*groupAgg
	if !p.YoyMissing {
		if yoy, err = s.groupStats(ctx, view, p.YoyStart, p.YoyEnd, now); err != nil {
			return nil, err
		}
	}
	epoch, epochOK := s.dataEpoch(ctx)

	rows := make([]LeaderboardRow, 0, len(cur))
	for key, g := range cur {
		row := LeaderboardRow{
			Key: key, Label: g.Label,
			CategoryID: g.CategoryID, CategoryName: g.CategoryName, CategorySlug: g.CategorySlug,
			Incidents: g.Incidents, Bugs: g.Bugs,
			DurationMS: g.DurationMS, Resolved: g.ResolvedSample,
		}
		if g.ResolvedSample > 0 {
			m := g.MTTRMS
			row.MTTRMS = &m
		}
		switch board {
		case BoardDuration:
			row.Value = float64(g.DurationMS)
		case BoardMTTR:
			row.Value = g.MTTRMS
		default:
			row.Value = float64(g.Incidents + g.Bugs)
		}
		row.Prev = groupDelta(g, prev[key], board, p.PrevEnd, epochOK, epoch)
		if p.YoyMissing {
			row.Yoy = &CompareDelta{Missing: true}
		} else {
			row.Yoy = groupDelta(g, yoy[key], board, p.YoyEnd, epochOK, epoch)
		}
		rows = append(rows, row)
	}
	rows = filterAndSortLeaderboard(rows, board)
	return &ReportLeaderboardResponse{
		Period: p.Type, PeriodKey: p.Key, View: view, Board: board, Rows: rows,
	}, nil
}

// groupDelta 榜单行的对照期差值（与 buildDelta 同语义，按 groupAgg 取值）。
func groupDelta(cur, base *groupAgg, board string, baseEnd time.Time, epochOK bool, epoch time.Time) *CompareDelta {
	if base == nil || !epochOK || !baseEnd.After(epoch) {
		return &CompareDelta{Missing: true}
	}
	var curVal, baseVal float64
	switch board {
	case BoardDuration:
		curVal, baseVal = float64(cur.DurationMS), float64(base.DurationMS)
	case BoardMTTR:
		if cur.ResolvedSample == 0 || base.ResolvedSample == 0 {
			return &CompareDelta{Missing: true}
		}
		curVal, baseVal = cur.MTTRMS, base.MTTRMS
	default:
		curVal = float64(cur.Incidents + cur.Bugs)
		baseVal = float64(base.Incidents + base.Bugs)
	}
	bv := baseVal
	d := &CompareDelta{Value: &bv}
	delta := curVal - baseVal
	d.Delta = &delta
	if baseVal != 0 {
		pct := delta / baseVal * 100
		d.Pct = &pct
	}
	return d
}

// filterAndSortLeaderboard 按榜单口径过滤（空样本不进榜）并排序。
func filterAndSortLeaderboard(rows []LeaderboardRow, board string) []LeaderboardRow {
	out := rows[:0]
	for _, r := range rows {
		switch board {
		case BoardDuration:
			if r.DurationMS <= 0 {
				continue
			}
		case BoardMTTR:
			if r.Resolved == 0 {
				continue // 空样本不进榜（§5.5）
			}
		default:
			if r.Incidents+r.Bugs == 0 {
				continue
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			if board == BoardMTTR {
				return out[i].Value < out[j].Value // MTTR 升序=修复快者在前
			}
			return out[i].Value > out[j].Value
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// ---- Service：责任人报告 ----
