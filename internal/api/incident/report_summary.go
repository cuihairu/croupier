package incident

import (
	"context"
	"sort"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// ReportSummary 统计卡聚合（GET /incident-reports/summary）。
func (s *Service) ReportSummary(ctx context.Context, periodType, periodKey string) (*ReportSummaryResponse, error) {
	now := time.Now()
	p, err := resolvePeriod(periodType, periodKey, now)
	if err != nil {
		return nil, err
	}
	cur, err := s.periodStats(ctx, p.Start, p.End, now)
	if err != nil {
		return nil, err
	}
	prev, err := s.periodStats(ctx, p.PrevStart, p.PrevEnd, now)
	if err != nil {
		return nil, err
	}
	var yoy *periodStats
	if !p.YoyMissing {
		if yoy, err = s.periodStats(ctx, p.YoyStart, p.YoyEnd, now); err != nil {
			return nil, err
		}
	}
	epoch, epochOK := s.dataEpoch(ctx)
	breakdown, err := s.categoryShare(ctx, p.Start, p.End, cur.Total)
	if err != nil {
		return nil, err
	}
	return &ReportSummaryResponse{
		Period: p.Type, PeriodKey: p.Key, Start: p.Start, End: p.End, GeneratedAt: now,
		Metrics: ReportMetrics{
			Total:      buildCompare(cur, prev, yoy, pickTotal, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
			Duration:   buildCompare(cur, prev, yoy, pickDuration, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
			MTTR:       buildCompare(cur, prev, yoy, pickMTTR, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
			Recurrence: buildCompare(cur, prev, yoy, pickRecurrence, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
		},
		Incidents: cur.Incidents, Bugs: cur.Bugs,
		Breakdown:        breakdown,
		ResolvedSample:   cur.ResolvedSample,
		RecurrenceChains: cur.RecurrenceChains,
	}, nil
}

// categoryShare 分类占比（incidents + bugs，§5.2）。
func (s *Service) categoryShare(ctx context.Context, start, end time.Time, total int64) ([]CategoryShare, error) {
	counts := map[uint]int64{}
	var incRows []struct {
		CategoryID uint
	}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Incident{}).
		Select("category_id").Where("detected_at >= ? AND detected_at < ?", start, end).
		Limit(reportScanLimit).Scan(&incRows).Error; err != nil {
		return nil, err
	}
	for _, r := range incRows {
		counts[r.CategoryID]++
	}
	var bugRows []struct {
		CategoryID uint
	}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Bug{}).
		Select("category_id").Where("created_at >= ? AND created_at < ?", start, end).
		Limit(reportScanLimit).Scan(&bugRows).Error; err != nil {
		return nil, err
	}
	for _, r := range bugRows {
		counts[r.CategoryID]++
	}
	names, err := s.categoryIndex(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryShare, 0, len(counts))
	for id, c := range counts {
		share := CategoryShare{CategoryID: id, Count: c}
		if cat, ok := names[id]; ok {
			share.Slug, share.Name = cat.slug, cat.name
		}
		if total > 0 {
			pct := float64(c) / float64(total) * 100
			share.SharePct = &pct
		}
		out = append(out, share)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out, nil
}

func (s *Service) categoryIndex(ctx context.Context) (map[uint]categoryName, error) {
	rows, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]categoryName, len(rows))
	for i := range rows {
		out[rows[i].ID] = categoryName{slug: rows[i].Slug, name: rows[i].Name}
	}
	return out, nil
}

// ---- Service：趋势 ----
