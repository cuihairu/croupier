package incident

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// 报表聚合核心（docs/design/incident-reports.md §5）：指标口径 §5.2、复发
// 签名 §5.3、三列对比 §5.4。分桶与聚合一律 Go 侧做（analytics/invocations
// 先例）——不用 DATE_FORMAT/strftime，postgres/sqlite 方言安全。

const (
	// recurrenceWindow 复发判定窗口（v1 固定 7 天，§5.2/§5.3）。
	recurrenceWindow = 7 * 24 * time.Hour
	// reportScanLimit 单次聚合扫描上限（防极端库拖垮内存；报表口径下
	// 单周期事故量远低于此）。
	reportScanLimit = 20000
	// 榜单类型（§5.5）。
	BoardIncidents = "incidents"
	BoardDuration  = "duration"
	BoardMTTR      = "mttr"
	// 榜单视图。
	ViewResponsible = "responsible"
	ViewCategory    = "category"
	// 趋势桶粒度。
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// ---- 对比列 DTO（§5.4） ----

// CompareDelta 是一个对照期（环比/同比）的差值与百分比。
type CompareDelta struct {
	Value   *float64 `json:"value"`             // 对照期值；无样本时 null
	Delta   *float64 `json:"delta"`             // 本期 − 对照期；对照期无样本时 null
	Pct     *float64 `json:"pct"`               // 百分比；对照期为 0 或无样本时 null
	Missing bool     `json:"missing,omitempty"` // 对照期无数据（不置 0、不算假数）
}

// CompareValue 是指标三列：本期值 + 环比 + 同比。
type CompareValue struct {
	Value *float64      `json:"value"`
	Prev  *CompareDelta `json:"prev"`
	Yoy   *CompareDelta `json:"yoy"`
}

// ---- 汇总（统计卡，§5.2/§5.4） ----

// ReportMetrics 是统计卡四指标，每个带三列对比。
type ReportMetrics struct {
	Total      CompareValue `json:"total"`      // 事故数 N（incidents + bugs）
	Duration   CompareValue `json:"duration"`   // 影响时长（毫秒）
	MTTR       CompareValue `json:"mttr"`       // 平均修复时长（毫秒；空样本 null）
	Recurrence CompareValue `json:"recurrence"` // 复发率（0-1；空样本 null）
}

// CategoryShare 是分类占比行。
type CategoryShare struct {
	CategoryID uint     `json:"categoryId"`
	Slug       string   `json:"slug,omitempty"`
	Name       string   `json:"name,omitempty"`
	Count      int64    `json:"count"`
	SharePct   *float64 `json:"sharePct"`
}

// ReportSummaryResponse 是统计卡聚合（GET /incident-reports/summary）。
type ReportSummaryResponse struct {
	Period           string          `json:"period"`
	PeriodKey        string          `json:"periodKey"`
	Start            time.Time       `json:"start"`
	End              time.Time       `json:"end"`
	GeneratedAt      time.Time       `json:"generatedAt"`
	Metrics          ReportMetrics   `json:"metrics"`
	Incidents        int64           `json:"incidents"`
	Bugs             int64           `json:"bugs"`
	Breakdown        []CategoryShare `json:"categoryBreakdown"`
	ResolvedSample   int64           `json:"resolvedSample"`
	RecurrenceChains int64           `json:"recurrenceChains"`
}

// ---- 趋势（§7） ----

// TrendPoint 是一个时间桶的计数（按类别拆分）。
type TrendPoint struct {
	Bucket     string           `json:"t"`
	Total      int64            `json:"total"`
	ByCategory map[string]int64 `json:"byCategory"`
}

// TrendCategory 是趋势图的类别图例项。
type TrendCategory struct {
	ID   uint   `json:"id"`
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

// ReportTrendResponse 是趋势序列（GET /incident-reports/trend）。
type ReportTrendResponse struct {
	Bucket     string          `json:"bucket"`
	Start      time.Time       `json:"start"`
	End        time.Time       `json:"end"`
	Categories []TrendCategory `json:"categories"`
	Points     []TrendPoint    `json:"points"`
	YoyPoints  []TrendPoint    `json:"yoyPoints"`
	YoyMissing bool            `json:"yoyMissing"`
}

// ---- 排行榜（§5.5） ----

// LeaderboardRow 是榜单一行（责任人视图或类别视图）。
type LeaderboardRow struct {
	Key          string        `json:"key"`
	Label        string        `json:"label"`
	CategoryID   uint          `json:"categoryId,omitempty"`
	CategoryName string        `json:"categoryName,omitempty"`
	CategorySlug string        `json:"categorySlug,omitempty"`
	Value        float64       `json:"value"`
	Incidents    int64         `json:"incidents"`
	Bugs         int64         `json:"bugs"`
	DurationMS   int64         `json:"durationMs"`
	Resolved     int64         `json:"resolved"`
	MTTRMS       *float64      `json:"mttrMs"`
	Prev         *CompareDelta `json:"prev"`
	Yoy          *CompareDelta `json:"yoy"`
}

// ReportLeaderboardResponse 是排行榜（GET /incident-reports/leaderboard）。
type ReportLeaderboardResponse struct {
	Period    string           `json:"period"`
	PeriodKey string           `json:"periodKey"`
	View      string           `json:"view"`
	Board     string           `json:"board"`
	Rows      []LeaderboardRow `json:"rows"`
}

// ---- 责任人报告（§5.6） ----

// ResponsibilityOverview 是报告头总览（三列指标）。
type ResponsibilityOverview struct {
	Total      CompareValue `json:"total"`
	Duration   CompareValue `json:"duration"`
	MTTR       CompareValue `json:"mttr"`
	Recurrence CompareValue `json:"recurrence"`
}

// ResponsibilityCategoryRow 是类别×责任人矩阵一行。
type ResponsibilityCategoryRow struct {
	CategoryID    uint             `json:"categoryId"`
	Slug          string           `json:"slug,omitempty"`
	Name          string           `json:"name,omitempty"`
	Leader        string           `json:"leader,omitempty"`
	Count         int64            `json:"count"`
	Prev          *CompareDelta    `json:"prev"`
	Yoy           *CompareDelta    `json:"yoy"`
	ByResponsible map[string]int64 `json:"byResponsible"`
}

// ResponsibilityRow 是责任人明细行。
type ResponsibilityRow struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Total       int64            `json:"total"`
	Prev        *CompareDelta    `json:"prev"`
	Yoy         *CompareDelta    `json:"yoy"`
	ByCategory  map[string]int64 `json:"byCategory"`
	DurationMS  int64            `json:"durationMs"`
	MTTRMS      *float64         `json:"mttrMs"`
	Resolved    int64            `json:"resolved"`
	Recurrences int64            `json:"recurrences"`
}

// UnattributedIncident 是未归因清单行（提醒补全）。
type UnattributedIncident struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	CategoryID uint      `json:"categoryId"`
	DetectedAt time.Time `json:"detectedAt"`
	Severity   string    `json:"severity"`
}

// ResponsibilityReportResponse 是责任人报告（GET /incident-reports/responsibility）。
type ResponsibilityReportResponse struct {
	PeriodType   string                      `json:"periodType"`
	PeriodKey    string                      `json:"periodKey"`
	Start        time.Time                   `json:"start"`
	End          time.Time                   `json:"end"`
	GeneratedAt  time.Time                   `json:"generatedAt"`
	Overview     ResponsibilityOverview      `json:"overview"`
	Categories   []ResponsibilityCategoryRow `json:"categories"`
	Responsibles []ResponsibilityRow         `json:"responsibles"`
	Unattributed []UnattributedIncident      `json:"unattributed"`
}

// ---- 聚合内部形态 ----

type incidentAggRow struct {
	DetectedAt      time.Time
	ResolvedAt      *time.Time
	Source          string
	RefType         string
	RefID           string
	CategoryID      uint
	Subcategory     string
	ResponsibleType string
	ResponsibleID   string
}

// periodStats 是一个周期的指标聚合结果（§5.2）。
type periodStats struct {
	Incidents        int64
	Bugs             int64
	Total            int64
	DurationMS       int64
	ResolvedSample   int64
	MTTRMS           float64
	RecurrenceChains int64
	rows             []incidentAggRow // 本周期内 incidents（分组/未归因清单复用）
}

// metricPick 从周期统计取一个指标的值与是否有样本（MTTR/复发率空样本=无值）。
type metricPick func(*periodStats) (float64, bool)

func pickTotal(st *periodStats) (float64, bool)    { return float64(st.Total), true }
func pickDuration(st *periodStats) (float64, bool) { return float64(st.DurationMS), true }
func pickMTTR(st *periodStats) (float64, bool) {
	if st.ResolvedSample == 0 {
		return 0, false
	}
	return st.MTTRMS, true
}
func pickRecurrence(st *periodStats) (float64, bool) {
	if st.ResolvedSample == 0 {
		return 0, false
	}
	return float64(st.RecurrenceChains) / float64(st.ResolvedSample), true
}

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

// buildCompare 组装指标三列（§5.4）。
func buildCompare(cur *periodStats, prev, yoy *periodStats, pick metricPick,
	epochOK bool, epoch time.Time, prevEnd, yoyEnd time.Time, yoyForceMissing bool) CompareValue {
	curVal, curHas := pick(cur)
	cv := CompareValue{}
	if curHas {
		v := curVal
		cv.Value = &v
	}
	cv.Prev = buildDelta(curVal, curHas, prev, pick, prevEnd, epochOK, epoch)
	if yoyForceMissing || yoy == nil {
		cv.Yoy = &CompareDelta{Missing: true}
	} else {
		cv.Yoy = buildDelta(curVal, curHas, yoy, pick, yoyEnd, epochOK, epoch)
	}
	return cv
}

// buildDelta 计算单个对照期差值；对照期整体晚于数据起点（该期完全无数据）
// 或基期无样本 → missing（不置 0、不算假数，§5.4）。
func buildDelta(curVal float64, curHas bool, base *periodStats, pick metricPick,
	baseEnd time.Time, epochOK bool, epoch time.Time) *CompareDelta {
	if base == nil || !epochOK || !baseEnd.After(epoch) {
		return &CompareDelta{Missing: true}
	}
	baseVal, baseHas := pick(base)
	if !baseHas {
		return &CompareDelta{Missing: true}
	}
	d := &CompareDelta{}
	bv := baseVal
	d.Value = &bv
	if curHas {
		delta := curVal - baseVal
		d.Delta = &delta
		if baseVal != 0 {
			pct := delta / baseVal * 100
			d.Pct = &pct
		}
	}
	return d
}

// ---- Service：汇总 ----

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
