package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/svc"
)

var errInvocationsUnavailable = errors.New("invocation analytics unavailable")

// ---------------------------------------------------------------------------
// Invocation analytics: aggregates over audit_records (function.invoke).
//
// Every governed console invocation already writes an audit record; this
// turns that stream into an analytics view (volume / success rate /
// latency / Jaeger hop) without any new collection pipeline.
//
// game_id / env / function_id / duration_ms are first-class columns on
// audit_records (promoted at write time, backfilled for legacy rows), so
// scoping and aggregation happen in plain, dialect-neutral SQL. Rows
// written before the promotion (empty game_id) are only visible in
// unscoped queries.
// ---------------------------------------------------------------------------

const (
	invocationEventType = string(audit.EventFunctionInvoke)
	// Console binding executions audit as page.execute with the same
	// outcome/duration semantics; both count as "invocations".
	pageExecuteEventType = string(audit.EventPageExecute)
)

var invocationEventTypeList = []string{invocationEventType, pageExecuteEventType}

// Scan budgets (DATA-049): memory and query time stay flat no matter how
// large the window's backlog grows. Vars so tests can shrink them; when a
// window exceeds the cap the newest rows are scanned and the read is
// disclosed as sampled via the response's sample/latency objects.
var (
	invocationLatencyRowCap = int64(50000)
	invocationTrendRowCap   = int64(10000)
)

// invocationLatencyMethod is the published methodology string: the p95 rank
// is computed as ceil(0.95*n) (1-based) in pure integer arithmetic, so small
// samples are never rounded down to a lower observation than the rank
// demands (the previous int(n*0.95)-1 returned the minimum for n=2).
const invocationLatencyMethod = "nearest-rank p95 over recorded durations (ceil(0.95*n), 1-based)"

// InvocationsTrendRequest buckets invocation volume by hour/day.
type InvocationsTrendRequest struct {
	GameId string `form:"gameId"`
	Env    string `form:"env"`
	// "hour" (default) or "day"; window is fixed to the last 24h/30d.
	Interval string `form:"interval"`
}

type InvocationsTrendPoint struct {
	Bucket string `json:"bucket"`
	Total  int64  `json:"total"`
	Failed int64  `json:"failed"`
}

type InvocationsTrendResponse struct {
	Points []InvocationsTrendPoint `json:"points"`
	// Sample discloses the bounded scan (DATA-049): a window larger than
	// invocationTrendRowCap scans only the newest rows and says so, never
	// presenting a truncated chart as the full window.
	Sample *InvocationSampleMeta `json:"sample,omitempty"`
}

// InvocationsSummaryRequest aggregates overall health.
type InvocationsSummaryRequest struct {
	GameId string `form:"gameId"`
	Env    string `form:"env"`
	Hours  int    `form:"hours"` // lookback window, default 24
}

type InvocationsSummaryResponse struct {
	Total         int64                     `json:"total"`
	Failed        int64                     `json:"failed"`
	SuccessRate   float64                   `json:"successRate"`
	AvgDurationMs float64                   `json:"avgDurationMs"`
	P95DurationMs float64                   `json:"p95DurationMs"`
	TopFunctions  []InvocationFunctionStats `json:"topFunctions"`
	// Latency publishes the aggregation methodology (DATA-049): the p95
	// algorithm, the sample scope (recorded durations only; missing ones
	// counted, never imputed) and the scan budget actually used.
	Latency *InvocationLatencyMeta `json:"latency"`
}

// InvocationSampleMeta discloses a bounded aggregation scan: total is the
// full window count, scanned the rows that actually fed the aggregation,
// sampled flags the cap being hit and coverage the fraction covered
// (scanned/total). Partial reads are never presented as exact.
type InvocationSampleMeta struct {
	TotalEvents   int64   `json:"totalEvents"`
	ScannedEvents int64   `json:"scannedEvents"`
	Sampled       bool    `json:"sampled"`
	Coverage      float64 `json:"coverage"`
	// MissingDuration counts window rows without a recorded duration; they
	// are excluded from avg/p95 and never imputed. Not applicable to the
	// trend sample, which counts rows regardless of duration.
	MissingDuration *int64 `json:"missingDuration,omitempty"`
}

// InvocationLatencyMeta publishes the latency methodology (DATA-049).
type InvocationLatencyMeta struct {
	Method string               `json:"method"`
	Sample InvocationSampleMeta `json:"sample"`
}

type InvocationFunctionStats struct {
	FunctionID string  `json:"functionId"`
	Total      int64   `json:"total"`
	Failed     int64   `json:"failed"`
	AvgDurMs   float64 `json:"avgDurationMs"`
}

// InvocationsListRequest pages raw invocation records.
type InvocationsListRequest struct {
	GameId     string `form:"gameId"`
	Env        string `form:"env"`
	FunctionId string `form:"functionId"`
	Outcome    string `form:"outcome"` // success|failure
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
}

type InvocationItem struct {
	Timestamp  string `json:"timestamp"`
	FunctionID string `json:"functionId"`
	Actor      string `json:"actor"`
	Outcome    string `json:"outcome"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	TraceID    string `json:"traceId,omitempty"`
	GameId     string `json:"gameId,omitempty"`
	Env        string `json:"env,omitempty"`
}

type InvocationsListResponse struct {
	Items []InvocationItem `json:"items"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Size  int              `json:"pageSize"`
}

// ---------------------------------------------------------------------------
// Service methods
// ---------------------------------------------------------------------------

func (s *Service) InvocationsTrend(ctx context.Context, req *InvocationsTrendRequest) (*InvocationsTrendResponse, error) {
	return invocationsTrend(ctx, s.svcCtx, req)
}

func (s *Service) InvocationsSummary(ctx context.Context, req *InvocationsSummaryRequest) (*InvocationsSummaryResponse, error) {
	return invocationsSummary(ctx, s.svcCtx, req)
}

func (s *Service) InvocationsList(ctx context.Context, req *InvocationsListRequest) (*InvocationsListResponse, error) {
	return invocationsList(ctx, s.svcCtx, req)
}

// ---------------------------------------------------------------------------
// Implementations (raw SQL over audit_records in the meta database)
// ---------------------------------------------------------------------------

func invocationTimeWindow(interval string) (layout string, since time.Time) {
	if interval == "day" {
		return "2006-01-02", time.Now().UTC().AddDate(0, 0, -30)
	}
	return "2006-01-02 15:00:00", time.Now().UTC().Add(-24 * time.Hour)
}

// scopedInvocationQuery applies the event-type filter plus optional
// game/env scoping. Scope params filter on the promoted columns, so a
// scoped view only contains rows explicitly tagged with that game/env.
func scopedInvocationQuery(db *gorm.DB, gameID, env string) *gorm.DB {
	q := db.Table("audit_records").Where("event_type IN ?", invocationEventTypeList)
	if gameID != "" {
		q = q.Where("game_id = ?", gameID)
	}
	if env != "" {
		q = q.Where("env = ?", env)
	}
	return q
}

func invocationsTrend(ctx context.Context, svcCtx *svc.ServiceContext, req *InvocationsTrendRequest) (*InvocationsTrendResponse, error) {
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errInvocationsUnavailable
	}
	layout, since := invocationTimeWindow(req.Interval)
	db := svcCtx.DB.WithContext(ctx)

	// Window count first (DATA-049): the bounded scan below must be able to
	// disclose whether it covered the whole window.
	var trendTotals struct {
		Total int64
	}
	if err := scopedInvocationQuery(db, req.GameId, req.Env).
		Where("timestamp >= ?", since).
		Select("COUNT(*) AS total").
		Scan(&trendTotals).Error; err != nil {
		return nil, err
	}

	// Bucket in Go instead of SQL: DATE_FORMAT/strftime differ across the
	// supported drivers (postgres / sqlite). The scan is capped at
	// invocationTrendRowCap NEWEST rows (timestamp DESC, id DESC as the
	// deterministic tiebreak) — under high load the chart degrades toward
	// recency instead of silently freezing on the oldest prefix, and the
	// truncation is disclosed via Sample.
	type rawRow struct {
		Timestamp time.Time
		Outcome   string
	}
	raw := []rawRow{}
	if err := scopedInvocationQuery(db, req.GameId, req.Env).
		Select("timestamp, outcome").
		Where("timestamp >= ?", since).
		Order("timestamp DESC, id DESC").Limit(int(invocationTrendRowCap)).
		Scan(&raw).Error; err != nil {
		return nil, err
	}

	buckets := map[string]*InvocationsTrendPoint{}
	order := []string{}
	// Reverse the newest-first scan so bucket order stays chronological.
	for i := len(raw) - 1; i >= 0; i-- {
		key := raw[i].Timestamp.UTC().Format(layout)
		p, ok := buckets[key]
		if !ok {
			p = &InvocationsTrendPoint{Bucket: key}
			buckets[key] = p
			order = append(order, key)
		}
		p.Total++
		if raw[i].Outcome != "success" {
			p.Failed++
		}
	}
	points := make([]InvocationsTrendPoint, 0, len(order))
	for _, key := range order {
		points = append(points, *buckets[key])
	}
	sample := &InvocationSampleMeta{
		TotalEvents:   trendTotals.Total,
		ScannedEvents: int64(len(raw)),
		Coverage:      1,
	}
	if trendTotals.Total > int64(len(raw)) {
		sample.Sampled = true
		sample.Coverage = float64(len(raw)) / float64(trendTotals.Total)
	}
	return &InvocationsTrendResponse{Points: points, Sample: sample}, nil
}

func invocationsSummary(ctx context.Context, svcCtx *svc.ServiceContext, req *InvocationsSummaryRequest) (*InvocationsSummaryResponse, error) {
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errInvocationsUnavailable
	}
	hours := req.Hours
	if hours <= 0 || hours > 24*30 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	db := svcCtx.DB.WithContext(ctx)

	resp := &InvocationsSummaryResponse{TopFunctions: []InvocationFunctionStats{}}

	// Volume + failures over the full window, plus the latency sample book
	// (DATA-049): rows with a recorded duration and rows without one, in the
	// same scan. CASE WHEN is portable across postgres/sqlite; SUM(boolean)
	// is postgres-invalid; COUNT(CASE WHEN ... THEN 1 END) counts non-nulls.
	totals := struct {
		Total        int64
		Failed       int64
		WithDuration int64
		Missing      int64
	}{}
	if err := scopedInvocationQuery(db, req.GameId, req.Env).
		Where("timestamp >= ?", since).
		Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN outcome <> 'success' THEN 1 ELSE 0 END), 0) AS failed, COUNT(CASE WHEN duration_ms > 0 THEN 1 END) AS with_duration, COUNT(*) - COUNT(CASE WHEN duration_ms > 0 THEN 1 END) AS missing").
		Scan(&totals).Error; err != nil {
		return nil, err
	}
	resp.Total, resp.Failed = totals.Total, totals.Failed
	if resp.Total > 0 {
		resp.SuccessRate = float64(resp.Total-resp.Failed) / float64(resp.Total)
	}

	// Latency (DATA-049): deterministic scope and bounded resources. The
	// sample is every window row with a recorded duration (duration_ms > 0);
	// rows without one are counted as missing and never imputed. The scan is
	// capped at invocationLatencyRowCap newest rows (timestamp DESC, id DESC
	// as the deterministic tiebreak) so memory and query time stay flat under
	// high load; a capped read is disclosed as sampled with its coverage.
	// p95 uses the integer nearest-rank: ceil(0.95*n)-1, never the float
	// rounding that returned a lower observation for small n.
	latencyMeta := &InvocationLatencyMeta{Method: invocationLatencyMethod}
	latencyMeta.Sample.TotalEvents = totals.WithDuration
	latencyMeta.Sample.MissingDuration = &totals.Missing
	latencyMeta.Sample.Coverage = 1
	resp.Latency = latencyMeta
	durations := []int64{}
	latQuery := scopedInvocationQuery(db, req.GameId, req.Env).
		Where("timestamp >= ? AND duration_ms > 0", since)
	if totals.WithDuration > invocationLatencyRowCap {
		latQuery = latQuery.Order("timestamp DESC, id DESC").Limit(int(invocationLatencyRowCap))
	}
	if err := latQuery.Pluck("duration_ms", &durations).Error; err != nil {
		return nil, err
	}
	latencyMeta.Sample.ScannedEvents = int64(len(durations))
	if int64(len(durations)) < totals.WithDuration {
		latencyMeta.Sample.Sampled = true
		latencyMeta.Sample.Coverage = float64(len(durations)) / float64(totals.WithDuration)
	}
	if len(durations) > 0 {
		sum := int64(0)
		for _, d := range durations {
			sum += d
		}
		resp.AvgDurationMs = float64(sum) / float64(len(durations))
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		resp.P95DurationMs = float64(durations[nearestRankIndex(len(durations), 95, 100)])
	}

	// Top functions: group on the promoted function_id column; AVG over a
	// NULL-producing CASE keeps "no duration recorded" out of the mean and
	// stays portable.
	type fnRow struct {
		FunctionID string
		Total      int64
		Failed     int64
		AvgDurMs   *float64
	}
	fnRows := []fnRow{}
	if err := scopedInvocationQuery(db, req.GameId, req.Env).
		Where("timestamp >= ? AND function_id <> ''", since).
		Select("function_id, COUNT(*) AS total, COALESCE(SUM(CASE WHEN outcome <> 'success' THEN 1 ELSE 0 END), 0) AS failed, AVG(CASE WHEN duration_ms > 0 THEN duration_ms END) AS avg_dur_ms").
		Group("function_id").
		Order("total DESC").
		Limit(10).
		Scan(&fnRows).Error; err != nil {
		return nil, err
	}
	for _, r := range fnRows {
		stats := InvocationFunctionStats{FunctionID: r.FunctionID, Total: r.Total, Failed: r.Failed}
		if r.AvgDurMs != nil {
			stats.AvgDurMs = *r.AvgDurMs
		}
		resp.TopFunctions = append(resp.TopFunctions, stats)
	}
	return resp, nil
}

// nearestRankIndex returns the 0-based nearest-rank index for a quantile
// given as num/den (e.g. 95/100): ceil(num*n/den)-1 in pure integer
// arithmetic, clamped to [0, n-1]. It never returns -1 for n >= 1, so small
// samples keep their full rank (the previous float int(n*0.95)-1 dropped to
// index 0 for n <= 2, reporting the minimum as p95).
func nearestRankIndex(n int, num, den int64) int {
	if n <= 0 {
		return 0
	}
	idx := int((int64(n)*num+den-1)/den) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return idx
}

func invocationsList(ctx context.Context, svcCtx *svc.ServiceContext, req *InvocationsListRequest) (*InvocationsListResponse, error) {
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errInvocationsUnavailable
	}
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// All filters push down to SQL now (promoted columns), so COUNT and the
	// page window share one consistent filtered set.
	query := scopedInvocationQuery(svcCtx.DB.WithContext(ctx), req.GameId, req.Env)
	if req.Outcome != "" {
		query = query.Where("outcome = ?", req.Outcome)
	}
	if req.FunctionId != "" {
		query = query.Where("function_id = ?", req.FunctionId)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	type rawRow struct {
		Timestamp    time.Time
		Outcome      string
		GameID       string
		Env          string
		FunctionID   string
		DurationMs   int64
		ActorJSON    []byte
		DetailsJSON  []byte
		ErrorMessage string
	}
	rawRows := []rawRow{}
	if err := query.Select("timestamp, outcome, game_id, env, function_id, duration_ms, actor_json, details_json, error_message").
		Order("timestamp DESC").
		Offset((page - 1) * size).Limit(size).
		Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	items := make([]InvocationItem, 0, len(rawRows))
	for _, r := range rawRows {
		var actor, details map[string]interface{}
		if len(r.ActorJSON) > 0 {
			_ = json.Unmarshal(r.ActorJSON, &actor)
		}
		if len(r.DetailsJSON) > 0 {
			_ = json.Unmarshal(r.DetailsJSON, &details)
		}
		item := InvocationItem{
			Timestamp:  r.Timestamp.UTC().Format(time.RFC3339),
			FunctionID: r.FunctionID,
			Outcome:    r.Outcome,
			Error:      r.ErrorMessage,
			DurationMs: r.DurationMs,
			GameId:     r.GameID,
			Env:        r.Env,
		}
		if id, ok := actor["id"].(string); ok {
			item.Actor = id
		}
		if traceID, ok := details["trace_id"].(string); ok {
			item.TraceID = traceID
		}
		items = append(items, item)
	}
	return &InvocationsListResponse{Items: items, Total: total, Page: page, Size: size}, nil
}
