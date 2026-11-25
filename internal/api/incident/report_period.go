package incident

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// periodStats aggregates the metrics over [start,end). now is used to clip the
// tail of unresolved incidents (§5.2: those still open at the period end are
// counted up to min(now, period end)).
func (s *Service) periodStats(ctx context.Context, start, end, now time.Time) (*periodStats, error) {
	rows, err := s.scanIncidents(ctx, start.Add(-recurrenceWindow), end)
	if err != nil {
		return nil, err
	}
	st := &periodStats{}
	for _, r := range rows {
		if r.DetectedAt.Before(start) || !r.DetectedAt.Before(end) {
			continue // pre-period rows needed only for the recurrence window, not this period's metrics
		}
		st.rows = append(st.rows, r)
		st.Incidents++
		stop := end
		if now.Before(stop) {
			stop = now
		}
		if r.ResolvedAt != nil {
			if d := r.ResolvedAt.Sub(r.DetectedAt); d > 0 {
				st.DurationMS += d.Milliseconds()
			}
			st.ResolvedSample++
			st.MTTRMS += float64(r.ResolvedAt.Sub(r.DetectedAt).Milliseconds())
		} else if tail := stop.Sub(r.DetectedAt); tail > 0 {
			st.DurationMS += tail.Milliseconds()
		}
	}
	if st.ResolvedSample > 0 {
		st.MTTRMS /= float64(st.ResolvedSample)
	}
	var bugs int64
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Bug{}).
		Where("created_at >= ? AND created_at < ?", start, end).Count(&bugs).Error; err != nil {
		return nil, err
	}
	st.Bugs = bugs
	st.Total = st.Incidents + bugs
	st.RecurrenceChains = countRecurrenceChains(rows, start, end)
	return st, nil
}

// scanIncidents scans the columns needed for aggregation for incidents inside
// the time window.
func (s *Service) scanIncidents(ctx context.Context, start, end time.Time) ([]incidentAggRow, error) {
	rows := []incidentAggRow{}
	err := s.svcCtx.DB.WithContext(ctx).Model(&model.Incident{}).
		Select("detected_at, resolved_at, source, ref_type, ref_id, category_id, subcategory, responsible_type, responsible_id").
		Where("detected_at >= ? AND detected_at < ?", start, end).
		Order("detected_at ASC").
		Limit(reportScanLimit).
		Scan(&rows).Error
	return rows, err
}

// signatureOf builds the recurrence signature (§5.3): automated sources derive
// it from source + RefID; manual registration uses category + subcategory
// (category alone when there is no subcategory).
func signatureOf(r incidentAggRow) string {
	if r.RefType != "" && r.RefID != "" {
		return r.Source + "|" + r.RefID
	}
	sig := "cat:" + strconv.FormatUint(uint64(r.CategoryID), 10)
	if r.Subcategory != "" {
		sig += "/" + r.Subcategory
	}
	return sig
}

// recurrenceHeads returns the head incidents of the recurrence chains that fall
// inside this period (§5.3: a same-signature incident re-firing within 7 days
// after a resolve counts as a recurrence; a run of several consecutive
// occurrences is counted once at the chain head). rows must also include the
// 7-day window preceding this period.
func recurrenceHeads(rows []incidentAggRow, start, end time.Time) []incidentAggRow {
	bySig := map[string][]incidentAggRow{}
	for _, r := range rows {
		sig := signatureOf(r)
		bySig[sig] = append(bySig[sig], r)
	}
	var heads []incidentAggRow
	for _, group := range bySig {
		sort.Slice(group, func(i, j int) bool { return group[i].DetectedAt.Before(group[j].DetectedAt) })
		var lastResolved *time.Time
		chainActive := false
		for _, r := range group {
			recurrence := lastResolved != nil &&
				r.DetectedAt.After(*lastResolved) &&
				r.DetectedAt.Before(lastResolved.Add(recurrenceWindow))
			if recurrence {
				if !chainActive {
					if !r.DetectedAt.Before(start) && r.DetectedAt.Before(end) {
						heads = append(heads, r)
					}
					chainActive = true
				}
			} else {
				chainActive = false
			}
			if r.ResolvedAt != nil {
				lastResolved = r.ResolvedAt
			}
		}
	}
	return heads
}

func countRecurrenceChains(rows []incidentAggRow, start, end time.Time) int64 {
	return int64(len(recurrenceHeads(rows, start, end)))
}

// dataEpoch returns the data epoch (the earlier of incidents.detected_at and
// bugs.created_at). A comparison period that lies entirely after the epoch
// (no data at all for that period) yields missing (§5.4) — it is not coerced
// to 0, which would fabricate numbers.
func (s *Service) dataEpoch(ctx context.Context) (time.Time, bool) {
	incMin := oldestTime(ctx, s.svcCtx.DB, &model.Incident{}, "detected_at")
	bugMin := oldestTime(ctx, s.svcCtx.DB, &model.Bug{}, "created_at")
	switch {
	case incMin != nil && bugMin != nil:
		if bugMin.Before(*incMin) {
			return *bugMin, true
		}
		return *incMin, true
	case incMin != nil:
		return *incMin, true
	case bugMin != nil:
		return *bugMin, true
	default:
		return time.Time{}, false
	}
}

// oldestTime returns the earliest timestamp in a table. It uses a model-based
// Order+Limit scan instead of a MIN() aggregate: the sqlite driver returns
// aggregate columns as strings, which a NullTime scan does not support
// (same precedent as ops/logs.go).
func oldestTime(ctx context.Context, db *gorm.DB, m interface{}, column string) *time.Time {
	var row struct {
		T time.Time
	}
	// column name mapped via an AS alias to the T field of the anonymous struct
	if err := db.WithContext(ctx).Model(m).Select(column + " AS t").
		Order(column + " ASC").Limit(1).Scan(&row).Error; err != nil {
		return nil
	}
	if row.T.IsZero() {
		return nil
	}
	return &row.T
}
