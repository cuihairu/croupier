package incident

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
)

// ReportTrend 时间序列（GET /incident-reports/trend）。
func (s *Service) ReportTrend(ctx context.Context, bucket string, from, to *time.Time) (*ReportTrendResponse, error) {
	if bucket == "" {
		bucket = BucketDay
	}
	if bucket != BucketDay && bucket != BucketWeek && bucket != BucketMonth {
		return nil, errorx.NewBadRequest("无效的桶粒度: " + bucket + "（支持 day/week/month）")
	}
	now := time.Now()
	start, end := trendWindow(bucket, from, to, now)
	yoyStart := start.AddDate(-1, 0, 0)
	yoyEnd := end.AddDate(-1, 0, 0)

	points, cats, err := s.trendPoints(ctx, bucket, start, end)
	if err != nil {
		return nil, err
	}
	epoch, epochOK := s.dataEpoch(ctx)
	resp := &ReportTrendResponse{
		Bucket: bucket, Start: start, End: end,
		Categories: cats, Points: points, YoyPoints: []TrendPoint{},
	}
	if !epochOK || !yoyEnd.After(epoch) {
		resp.YoyMissing = true
		return resp, nil
	}
	yoyPoints, _, err := s.trendPoints(ctx, bucket, yoyStart, yoyEnd)
	if err != nil {
		return nil, err
	}
	resp.YoyPoints = yoyPoints
	return resp, nil
}

// trendWindow 解析趋势时间窗：显式 from/to 优先；缺省按桶粒度给近 30 天/
// 12 周/12 个月。
func trendWindow(bucket string, from, to *time.Time, now time.Time) (time.Time, time.Time) {
	end := now
	if to != nil {
		end = *to
	}
	if from != nil {
		return *from, end
	}
	switch bucket {
	case BucketWeek:
		return end.AddDate(0, 0, -7*12), end
	case BucketMonth:
		return end.AddDate(0, -12, 0), end
	default:
		return end.AddDate(0, 0, -30), end
	}
}

// trendPoints 把窗口内 incidents + bugs 按桶聚合（Go 侧分桶，§7）。
func (s *Service) trendPoints(ctx context.Context, bucket string, start, end time.Time) ([]TrendPoint, []TrendCategory, error) {
	type row struct {
		T          time.Time
		CategoryID uint
	}
	raw := []row{}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Incident{}).
		Select("detected_at AS t, category_id").
		Where("detected_at >= ? AND detected_at < ?", start, end).
		Order("detected_at ASC").Limit(reportScanLimit).Scan(&raw).Error; err != nil {
		return nil, nil, err
	}
	bugRaw := []row{}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Bug{}).
		Select("created_at AS t, category_id").
		Where("created_at >= ? AND created_at < ?", start, end).
		Order("created_at ASC").Limit(reportScanLimit).Scan(&bugRaw).Error; err != nil {
		return nil, nil, err
	}
	raw = append(raw, bugRaw...)

	buckets := map[string]*TrendPoint{}
	order := []string{}
	seenCat := map[uint]bool{}
	for _, r := range raw {
		key := bucketKey(bucket, r.T)
		p, ok := buckets[key]
		if !ok {
			p = &TrendPoint{Bucket: key, ByCategory: map[string]int64{}}
			buckets[key] = p
			order = append(order, key)
		}
		p.Total++
		p.ByCategory[strconv.FormatUint(uint64(r.CategoryID), 10)]++
		seenCat[r.CategoryID] = true
	}
	sort.Strings(order)
	points := make([]TrendPoint, 0, len(order))
	for _, k := range order {
		points = append(points, *buckets[k])
	}
	names, err := s.categoryIndex(ctx)
	if err != nil {
		return nil, nil, err
	}
	cats := make([]TrendCategory, 0, len(seenCat))
	for id := range seenCat {
		tc := TrendCategory{ID: id}
		if cat, ok := names[id]; ok {
			tc.Slug, tc.Name = cat.slug, cat.name
		}
		cats = append(cats, tc)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].ID < cats[j].ID })
	return points, cats, nil
}

// bucketKey 桶键：day=日期；week=该 ISO 周周一日期；month=年月。
func bucketKey(bucket string, t time.Time) string {
	switch bucket {
	case BucketWeek:
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return t.AddDate(0, 0, 1-wd).Format("2006-01-02")
	case BucketMonth:
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// ---- Service：排行榜 ----
