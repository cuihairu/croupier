package incident

import (
	"time"
)

// Report aggregation core (docs/design/incident-reports.md §5): metric
// definitions §5.2, recurrence signatures §5.3, three-column comparison §5.4.
// Bucketing and aggregation are always done Go-side (precedent:
// analytics/invocations) -- no DATE_FORMAT/strftime, safe across postgres and
// sqlite dialects.

const (
	// recurrenceWindow is the recurrence decision window (fixed at 7 days in
	// v1, §5.2/§5.3).
	recurrenceWindow = 7 * 24 * time.Hour
	// reportScanLimit is the upper bound of a single aggregation scan (guards
	// memory against an extreme database; under the report scope the incident
	// count per period is far below this).
	reportScanLimit = 20000
	// Leaderboard types (§5.5).
	BoardIncidents = "incidents"
	BoardDuration  = "duration"
	BoardMTTR      = "mttr"
	// Leaderboard views.
	ViewResponsible = "responsible"
	ViewCategory    = "category"
	// Trend bucket granularity.
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// ---- Comparison column DTOs (§5.4) ----

// CompareDelta is the delta and percentage against a comparison period
// (period-over-period / year-over-year).
type CompareDelta struct {
	Value   *float64 `json:"value"`             // comparison period value; null when no sample
	Delta   *float64 `json:"delta"`             // current - comparison; null when the comparison period has no sample
	Pct     *float64 `json:"pct"`               // percentage; null when the comparison period is 0 or has no sample
	Missing bool     `json:"missing,omitempty"` // comparison period has no data (no zero, no fabricated number)
}

// CompareValue is the three columns of one metric: current value + PoP + YoY.
type CompareValue struct {
	Value *float64      `json:"value"`
	Prev  *CompareDelta `json:"prev"`
	Yoy   *CompareDelta `json:"yoy"`
}

// ---- Summary (stat cards, §5.2/§5.4) ----

// ReportMetrics is the four stat-card metrics, each with a three-column
// comparison.
type ReportMetrics struct {
	Total      CompareValue `json:"total"`      // incident count N (incidents + bugs)
	Duration   CompareValue `json:"duration"`   // impact duration (ms)
	MTTR       CompareValue `json:"mttr"`       // mean time to repair (ms; null with no sample)
	Recurrence CompareValue `json:"recurrence"` // recurrence rate (0-1; null with no sample)
}

// CategoryShare is one category share row.
type CategoryShare struct {
	CategoryID uint     `json:"categoryId"`
	Slug       string   `json:"slug,omitempty"`
	Name       string   `json:"name,omitempty"`
	Count      int64    `json:"count"`
	SharePct   *float64 `json:"sharePct"`
}

// ReportSummaryResponse is the stat-card aggregate (GET
// /incident-reports/summary).
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

// ---- Trend (§7) ----

// TrendPoint is one time bucket's counts (split by category).
type TrendPoint struct {
	Bucket     string           `json:"t"`
	Total      int64            `json:"total"`
	ByCategory map[string]int64 `json:"byCategory"`
}

// TrendCategory is a trend-chart category legend entry.
type TrendCategory struct {
	ID   uint   `json:"id"`
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

// ReportTrendResponse is the trend series (GET /incident-reports/trend).
type ReportTrendResponse struct {
	Bucket     string          `json:"bucket"`
	Start      time.Time       `json:"start"`
	End        time.Time       `json:"end"`
	Categories []TrendCategory `json:"categories"`
	Points     []TrendPoint    `json:"points"`
	YoyPoints  []TrendPoint    `json:"yoyPoints"`
	YoyMissing bool            `json:"yoyMissing"`
}

// ---- Leaderboard (§5.5) ----

// LeaderboardRow is one leaderboard row (owner view or category view).
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

// ReportLeaderboardResponse is the leaderboard (GET
// /incident-reports/leaderboard).
type ReportLeaderboardResponse struct {
	Period    string           `json:"period"`
	PeriodKey string           `json:"periodKey"`
	View      string           `json:"view"`
	Board     string           `json:"board"`
	Rows      []LeaderboardRow `json:"rows"`
}

// ---- Owner report (§5.6) ----

// ResponsibilityOverview is the report header overview (three-column metrics).
type ResponsibilityOverview struct {
	Total      CompareValue `json:"total"`
	Duration   CompareValue `json:"duration"`
	MTTR       CompareValue `json:"mttr"`
	Recurrence CompareValue `json:"recurrence"`
}

// ResponsibilityCategoryRow is one row of the category x owner matrix.
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

// ResponsibilityRow is one owner detail row.
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

// UnattributedIncident is one row of the unattributed list (a reminder to fill
// in attribution).
type UnattributedIncident struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	CategoryID uint      `json:"categoryId"`
	DetectedAt time.Time `json:"detectedAt"`
	Severity   string    `json:"severity"`
}

// ResponsibilityReportResponse is the owner report (GET
// /incident-reports/responsibility).
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

// ---- Internal aggregation shapes ----

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

// periodStats is the metric aggregate for one period (§5.2).
type periodStats struct {
	Incidents        int64
	Bugs             int64
	Total            int64
	DurationMS       int64
	ResolvedSample   int64
	MTTRMS           float64
	RecurrenceChains int64
	rows             []incidentAggRow // incidents within this period (reused by grouping / unattributed list)
}

// metricPick extracts one metric's value and sample presence from period
// stats (MTTR/recurrence with no sample = no value).
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
