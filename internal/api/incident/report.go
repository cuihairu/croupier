package incident

import (
	"time"
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

func pickTotal(st *periodStats) (float64, bool) { return float64(st.Total), true }

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
