package incident

import (
	"context"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// groupAgg 是一个分组（责任人/类别）在单周期的聚合。
type groupAgg struct {
	Key            string
	Label          string
	CategoryID     uint
	CategoryName   string
	CategorySlug   string
	Incidents      int64
	Bugs           int64
	DurationMS     int64
	ResolvedSample int64
	MTTRMS         float64
}

// groupStats 按视图分组聚合一个周期（incidents 影响时长/MTTR + bugs 计数）。
func (s *Service) groupStats(ctx context.Context, view string, start, end, now time.Time) (map[string]*groupAgg, error) {
	rows, err := s.scanIncidents(ctx, start, end)
	if err != nil {
		return nil, err
	}
	groups := map[string]*groupAgg{}
	get := func(key, label string) *groupAgg {
		g, ok := groups[key]
		if !ok {
			g = &groupAgg{Key: key, Label: label}
			groups[key] = g
		}
		return g
	}
	for _, r := range rows {
		key, label := groupKeyOf(view, r.ResponsibleID, r.CategoryID)
		g := get(key, label)
		g.Incidents++
		g.CategoryID = r.CategoryID
		stop := end
		if now.Before(stop) {
			stop = now
		}
		if r.ResolvedAt != nil {
			if d := r.ResolvedAt.Sub(r.DetectedAt); d > 0 {
				g.DurationMS += d.Milliseconds()
			}
			g.ResolvedSample++
			g.MTTRMS += float64(r.ResolvedAt.Sub(r.DetectedAt).Milliseconds())
		} else if tail := stop.Sub(r.DetectedAt); tail > 0 {
			g.DurationMS += tail.Milliseconds()
		}
	}
	for _, g := range groups {
		if g.ResolvedSample > 0 {
			g.MTTRMS /= float64(g.ResolvedSample)
		}
	}
	// bugs：责任人视图按 assignee，类别视图按 category_id
	type bugRow struct {
		CategoryID uint
		Assignee   string
	}
	bugs := []bugRow{}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Bug{}).
		Select("category_id, assignee").Where("created_at >= ? AND created_at < ?", start, end).
		Limit(reportScanLimit).Scan(&bugs).Error; err != nil {
		return nil, err
	}
	names, err := s.categoryIndex(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range bugs {
		var key, label string
		if view == ViewCategory {
			key, label = groupKeyOf(view, "", b.CategoryID)
		} else {
			key, label = groupKeyOf(view, b.Assignee, b.CategoryID)
		}
		g := get(key, label)
		g.Bugs++
		if view == ViewCategory {
			g.CategoryID = b.CategoryID
		}
	}
	for _, g := range groups {
		if g.CategoryID != 0 {
			if cat, ok := names[g.CategoryID]; ok {
				g.CategoryName, g.CategorySlug = cat.name, cat.slug
			}
		}
	}
	return groups, nil
}

// groupKeyOf 视图分组键：责任人视图按 responsibleID（空=未归因桶），
// 类别视图按 category_id。
func groupKeyOf(view, responsibleID string, categoryID uint) (string, string) {
	if view == ViewCategory {
		return "c" + strconv.FormatUint(uint64(categoryID), 10), ""
	}
	if responsibleID == "" {
		return "unknown", "未归因"
	}
	return responsibleID, responsibleID
}
