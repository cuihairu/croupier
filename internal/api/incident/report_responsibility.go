package incident

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
)

// ReportResponsibility 责任人报告（GET /incident-reports/responsibility，仅周/月档）。
func (s *Service) ReportResponsibility(ctx context.Context, periodType, periodKey string) (*ResponsibilityReportResponse, error) {
	if periodType != PeriodWeek && periodType != PeriodMonth {
		return nil, errorx.NewBadRequest("责任人报告仅支持 week/month 档位")
	}
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
	overview := ResponsibilityOverview{
		Total:      buildCompare(cur, prev, yoy, pickTotal, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
		Duration:   buildCompare(cur, prev, yoy, pickDuration, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
		MTTR:       buildCompare(cur, prev, yoy, pickMTTR, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
		Recurrence: buildCompare(cur, prev, yoy, pickRecurrence, epochOK, epoch, p.PrevEnd, p.YoyEnd, p.YoyMissing),
	}

	// 类别矩阵：类别 → 计数/leader/按责任人分布（bugs 计入类别计数）
	names, err := s.categoryIndex(ctx)
	if err != nil {
		return nil, err
	}
	leaders, err := s.categoryLeaders(ctx)
	if err != nil {
		return nil, err
	}
	curGroups, err := s.groupStats(ctx, ViewCategory, p.Start, p.End, now)
	if err != nil {
		return nil, err
	}
	prevGroups, err := s.groupStats(ctx, ViewCategory, p.PrevStart, p.PrevEnd, now)
	if err != nil {
		return nil, err
	}
	var yoyGroups map[string]*groupAgg
	if !p.YoyMissing {
		if yoyGroups, err = s.groupStats(ctx, ViewCategory, p.YoyStart, p.YoyEnd, now); err != nil {
			return nil, err
		}
	}
	cats := make([]ResponsibilityCategoryRow, 0, len(curGroups))
	for key, g := range curGroups {
		row := ResponsibilityCategoryRow{
			CategoryID: g.CategoryID, Count: g.Incidents + g.Bugs,
			ByResponsible: map[string]int64{},
		}
		if cat, ok := names[g.CategoryID]; ok {
			row.Slug, row.Name = cat.slug, cat.name
		}
		row.Leader = leaders[g.CategoryID]
		row.Prev = groupDelta(g, prevGroups[key], BoardIncidents, p.PrevEnd, epochOK, epoch)
		if p.YoyMissing {
			row.Yoy = &CompareDelta{Missing: true}
		} else {
			row.Yoy = groupDelta(g, yoyGroups[key], BoardIncidents, p.YoyEnd, epochOK, epoch)
		}
		for _, r := range cur.rows {
			if r.CategoryID != g.CategoryID {
				continue
			}
			row.ByResponsible[responsibleKeyOf(r.ResponsibleID)]++
		}
		cats = append(cats, row)
	}
	sort.Slice(cats, func(i, j int) bool {
		if cats[i].Count != cats[j].Count {
			return cats[i].Count > cats[j].Count
		}
		return cats[i].CategoryID < cats[j].CategoryID
	})

	// 责任人明细：计数/类别分布/时长/MTTR/复发（复发链按链首归因）
	respGroups, err := s.groupStats(ctx, ViewResponsible, p.Start, p.End, now)
	if err != nil {
		return nil, err
	}
	respPrev, err := s.groupStats(ctx, ViewResponsible, p.PrevStart, p.PrevEnd, now)
	if err != nil {
		return nil, err
	}
	var respYoy map[string]*groupAgg
	if !p.YoyMissing {
		if respYoy, err = s.groupStats(ctx, ViewResponsible, p.YoyStart, p.YoyEnd, now); err != nil {
			return nil, err
		}
	}
	heads := recurrenceHeads(append(cur.rows, s.preWindowRows(ctx, p.Start)...), p.Start, p.End)
	recurByResp := map[string]int64{}
	for _, h := range heads {
		recurByResp[responsibleKeyOf(h.ResponsibleID)]++
	}
	responsibles := make([]ResponsibilityRow, 0, len(respGroups))
	for key, g := range respGroups {
		row := ResponsibilityRow{
			Key: key, Label: g.Label,
			Total: g.Incidents + g.Bugs, ByCategory: map[string]int64{},
			DurationMS: g.DurationMS, Resolved: g.ResolvedSample,
			Recurrences: recurByResp[key],
		}
		if g.ResolvedSample > 0 {
			m := g.MTTRMS
			row.MTTRMS = &m
		}
		row.Prev = groupDelta(g, respPrev[key], BoardIncidents, p.PrevEnd, epochOK, epoch)
		if p.YoyMissing {
			row.Yoy = &CompareDelta{Missing: true}
		} else {
			row.Yoy = groupDelta(g, respYoy[key], BoardIncidents, p.YoyEnd, epochOK, epoch)
		}
		for _, r := range cur.rows {
			if responsibleKeyOf(r.ResponsibleID) != key {
				continue
			}
			row.ByCategory[strconv.FormatUint(uint64(r.CategoryID), 10)]++
		}
		responsibles = append(responsibles, row)
	}
	sort.Slice(responsibles, func(i, j int) bool {
		if responsibles[i].Total != responsibles[j].Total {
			return responsibles[i].Total > responsibles[j].Total
		}
		return responsibles[i].Key < responsibles[j].Key
	})

	unattributed, err := s.unattributedList(ctx, p.Start, p.End)
	if err != nil {
		return nil, err
	}
	return &ResponsibilityReportResponse{
		PeriodType: p.Type, PeriodKey: p.Key, Start: p.Start, End: p.End, GeneratedAt: now,
		Overview: overview, Categories: cats, Responsibles: responsibles, Unattributed: unattributed,
	}, nil
}

// responsibleKeyOf 责任人分组键（空=未归因桶）。
func responsibleKeyOf(id string) string {
	if id == "" {
		return "unknown"
	}
	return id
}

// preWindowRows 取本期前复发窗口内的行（复发链需要期前 resolve 记录）。
func (s *Service) preWindowRows(ctx context.Context, start time.Time) []incidentAggRow {
	rows, err := s.scanIncidents(ctx, start.Add(-recurrenceWindow), start)
	if err != nil {
		return nil
	}
	return rows
}

// categoryLeaders 类别 → leader 映射（矩阵行展示）。
func (s *Service) categoryLeaders(ctx context.Context) (map[uint]string, error) {
	rows, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]string, len(rows))
	for i := range rows {
		out[rows[i].ID] = rows[i].Leader
	}
	return out, nil
}

// unattributedList 未归因清单（responsible_type=unknown 或 responsible_id 空）。
func (s *Service) unattributedList(ctx context.Context, start, end time.Time) ([]UnattributedIncident, error) {
	out := []UnattributedIncident{}
	err := s.svcCtx.DB.WithContext(ctx).Model(&model.Incident{}).
		Select("id, title, category_id, detected_at, severity").
		Where("detected_at >= ? AND detected_at < ?", start, end).
		Where("responsible_type = ? OR responsible_id = ?", model.IncidentRespUnknown, "").
		Order("detected_at DESC").Limit(reportScanLimit).Scan(&out).Error
	return out, err
}
