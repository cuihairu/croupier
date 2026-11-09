package incident

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// periodStats 汇总 [start,end) 的指标。now 用于未解决事故的拖尾截断
// （§5.2：期末仍 open 的计到 min(now, 期末)）。
func (s *Service) periodStats(ctx context.Context, start, end, now time.Time) (*periodStats, error) {
	rows, err := s.scanIncidents(ctx, start.Add(-recurrenceWindow), end)
	if err != nil {
		return nil, err
	}
	st := &periodStats{}
	for _, r := range rows {
		if r.DetectedAt.Before(start) || !r.DetectedAt.Before(end) {
			continue // 仅复发窗口需要的期前行，不计入本期指标
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

// scanIncidents 扫时间窗内 incidents 的聚合所需列。
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

// signatureOf 复发签名（§5.3）：自动源 = source + RefID 派生；人工登记 =
// category + subcategory（无子类用 category）。
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

// recurrenceHeads 返回本期内的复发链链首事故（§5.3：同签名 resolve 后 7
// 天内再发 = 复发；连续多次按链首记 1 次）。rows 须含本期前 7 天窗口的行。
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

// dataEpoch 数据起点（incidents.detected_at 与 bugs.created_at 的较早者）。
// 对照期整体晚于数据起点（该期完全无数据）→ missing（§5.4，不置 0 不算假数）。
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

// oldestTime 取表内最早时间戳。走 Order+Limit 模型化扫描而非 MIN() 聚合：
// sqlite 驱动对聚合列返回 string，NullTime 扫描不支持（同 ops/logs.go 先例）。
func oldestTime(ctx context.Context, db *gorm.DB, m interface{}, column string) *time.Time {
	var row struct {
		T time.Time
	}
	// 列名经 AS 别名映射到匿名 struct 的 T 字段
	if err := db.WithContext(ctx).Model(m).Select(column + " AS t").
		Order(column + " ASC").Limit(1).Scan(&row).Error; err != nil {
		return nil
	}
	if row.T.IsZero() {
		return nil
	}
	return &row.T
}
