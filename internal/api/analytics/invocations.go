package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/svc"
)

var errInvocationsUnavailable = errors.New("invocation analytics unavailable")

// ---------------------------------------------------------------------------
// Invocation analytics: aggregates over audit_records (function.invoke).
//
// Every governed console invocation already writes an audit record; this
// turns that stream into an analytics view (volume / success rate /
// latency / Jaeger hop) without any new collection pipeline. Latency and
// trace ids are only present once auditFunctionInvoke started recording
// them into details; older records still count toward volume/success.
// ---------------------------------------------------------------------------

const (
	invocationEventType = string(audit.EventFunctionInvoke)
	// Console binding executions audit as page.execute with the same
	// outcome/duration semantics; both count as "invocations".
	pageExecuteEventType  = string(audit.EventPageExecute)
	invocationEventTypes  = "('" + invocationEventType + "','" + pageExecuteEventType + "')"
	defaultInvocationsCap = 1000
)

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

// invocationScope filters constrain aggregation to the audit rows written
// for function invokes. game/env scoping relies on the details JSON written
// by auditFunctionInvoke (details.game_id / details.env); rows without those
// keys (legacy) are only included in unscoped queries.
func invocationTimeWindow(interval string) (layout string, since time.Time) {
	if interval == "day" {
		return "2006-01-02", time.Now().UTC().AddDate(0, 0, -30)
	}
	return "2006-01-02 15:00:00", time.Now().UTC().Add(-24 * time.Hour)
}

func invocationsTrend(ctx context.Context, svcCtx *svc.ServiceContext, req *InvocationsTrendRequest) (*InvocationsTrendResponse, error) {
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errInvocationsUnavailable
	}
	layout, since := invocationTimeWindow(req.Interval)

	// Bucket in Go instead of SQL: DATE_FORMAT/strftime differ across the
	// supported drivers (postgres / sqlite), and audit volume per bucket is
	// small enough that scanning raw timestamps is cheap.
	type rawRow struct {
		Timestamp time.Time
		Outcome   string
	}
	raw := []rawRow{}
	if err := svcCtx.DB.WithContext(ctx).Table("audit_records").
		Select("timestamp, outcome").
		Where("event_type IN "+invocationEventTypes+" AND timestamp >= ?", since).
		Order("timestamp").Limit(10000).
		Scan(&raw).Error; err != nil {
		return nil, err
	}

	buckets := map[string]*InvocationsTrendPoint{}
	order := []string{}
	for _, r := range raw {
		key := r.Timestamp.UTC().Format(layout)
		p, ok := buckets[key]
		if !ok {
			p = &InvocationsTrendPoint{Bucket: key}
			buckets[key] = p
			order = append(order, key)
		}
		p.Total++
		if r.Outcome != "success" {
			p.Failed++
		}
	}
	points := make([]InvocationsTrendPoint, 0, len(order))
	for _, key := range order {
		points = append(points, *buckets[key])
	}
	return &InvocationsTrendResponse{Points: points}, nil
}

type invocationLatencyRow struct {
	Duration float64 `json:"duration_ms"`
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

	resp := &InvocationsSummaryResponse{TopFunctions: []InvocationFunctionStats{}}

	// Volume + failures. CASE WHEN is portable across postgres/sqlite;
	// SUM(boolean) is postgres-invalid.
	totals := struct {
		Total  int64
		Failed int64
	}{}
	if err := svcCtx.DB.WithContext(ctx).Table("audit_records").
		Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN outcome <> 'success' THEN 1 ELSE 0 END), 0) AS failed").
		Where("event_type IN "+invocationEventTypes+" AND timestamp >= ?", since).
		Scan(&totals).Error; err != nil {
		return nil, err
	}
	resp.Total, resp.Failed = totals.Total, totals.Failed
	if resp.Total > 0 {
		resp.SuccessRate = float64(resp.Total-resp.Failed) / float64(resp.Total)
	}

	// Latency + per-function stats: parse details JSON in Go so the query
	// stays dialect-neutral (postgres/sqlite have no JSON_EXTRACT).
	type detailRow struct {
		Timestamp   time.Time
		Outcome     string
		DetailsJSON []byte
	}
	detailRows := []detailRow{}
	if err := svcCtx.DB.WithContext(ctx).Table("audit_records").
		Select("timestamp, outcome, details_json").
		Where("event_type IN "+invocationEventTypes+" AND timestamp >= ?", since).
		Order("timestamp").Limit(defaultInvocationsCap).
		Scan(&detailRows).Error; err != nil {
		return nil, err
	}

	latencies := []float64{}
	type fnAgg struct {
		total, failed int64
		durSum        float64
		durCount      int64
	}
	fnAggs := map[string]*fnAgg{}
	fnOrder := []string{}
	for _, r := range detailRows {
		var details map[string]interface{}
		if len(r.DetailsJSON) > 0 {
			_ = json.Unmarshal(r.DetailsJSON, &details)
		}
		fnID, _ := details["function_id"].(string)
		agg, ok := fnAggs[fnID]
		if !ok {
			agg = &fnAgg{}
			fnAggs[fnID] = agg
			fnOrder = append(fnOrder, fnID)
		}
		agg.total++
		if r.Outcome != "success" {
			agg.failed++
		}
		if d, ok := numericValue(details["duration_ms"]); ok {
			latencies = append(latencies, d)
			agg.durSum += d
			agg.durCount++
		} else if d, ok := numericValue(details["elapsed_ms"]); ok {
			latencies = append(latencies, d)
			agg.durSum += d
			agg.durCount++
		}
	}
	if len(latencies) > 0 {
		sum := 0.0
		for _, d := range latencies {
			sum += d
		}
		resp.AvgDurationMs = sum / float64(len(latencies))
		sorted := append([]float64(nil), latencies...)
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
		idx := int(float64(len(sorted))*0.95) - 1
		if idx < 0 {
			idx = len(sorted) - 1
		}
		resp.P95DurationMs = sorted[idx]
	}
	sort.Strings(fnOrder)
	// order top functions by total desc
	type fnPair struct {
		id  string
		agg *fnAgg
	}
	pairs := make([]fnPair, 0, len(fnOrder))
	for _, id := range fnOrder {
		pairs = append(pairs, fnPair{id, fnAggs[id]})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].agg.total > pairs[j].agg.total })
	for i, pr := range pairs {
		if i >= 10 {
			break
		}
		stats := InvocationFunctionStats{FunctionID: pr.id, Total: pr.agg.total, Failed: pr.agg.failed}
		if pr.agg.durCount > 0 {
			stats.AvgDurMs = pr.agg.durSum / float64(pr.agg.durCount)
		}
		resp.TopFunctions = append(resp.TopFunctions, stats)
	}
	return resp, nil
}

// numericValue coerces JSON numbers (float64 after unmarshal) and ints.
func numericValue(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
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

	query := svcCtx.DB.WithContext(ctx).Table("audit_records").
		Where("event_type IN " + invocationEventTypes)
	if req.Outcome != "" {
		query = query.Where("outcome = ?", req.Outcome)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Dialect-neutral: fetch raw columns and parse the JSON payloads in Go
	// (postgres/sqlite have no JSON_EXTRACT). functionId filtering also
	// happens here; when set we over-fetch a bounded page window first.
	fetchLimit := size
	if req.FunctionId != "" {
		fetchLimit = defaultInvocationsCap
	}
	type rawRow struct {
		Timestamp    time.Time
		Outcome      string
		DetailsJSON  []byte
		ActorJSON    []byte
		ErrorMessage string
	}
	rawRows := []rawRow{}
	if err := query.Select("timestamp, outcome, details_json, actor_json, error_message").
		Order("timestamp DESC").
		Offset((page - 1) * fetchLimit).Limit(fetchLimit).
		Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	items := make([]InvocationItem, 0, size)
	for _, r := range rawRows {
		var details, actor map[string]interface{}
		if len(r.DetailsJSON) > 0 {
			_ = json.Unmarshal(r.DetailsJSON, &details)
		}
		if len(r.ActorJSON) > 0 {
			_ = json.Unmarshal(r.ActorJSON, &actor)
		}
		fnID, _ := details["function_id"].(string)
		if req.FunctionId != "" && fnID != req.FunctionId {
			continue
		}
		item := InvocationItem{
			Timestamp:  r.Timestamp.UTC().Format(time.RFC3339),
			FunctionID: fnID,
			Outcome:    r.Outcome,
			Error:      r.ErrorMessage,
		}
		if id, ok := actor["id"].(string); ok {
			item.Actor = id
		}
		if traceID, ok := details["trace_id"].(string); ok {
			item.TraceID = traceID
		}
		if gameID, ok := details["game_id"].(string); ok {
			item.GameId = gameID
		}
		if env, ok := details["env"].(string); ok {
			item.Env = env
		}
		if d, ok := numericValue(details["duration_ms"]); ok {
			item.DurationMs = int64(d)
		} else if d, ok := numericValue(details["elapsed_ms"]); ok {
			item.DurationMs = int64(d)
		}
		items = append(items, item)
		if len(items) >= size {
			break
		}
	}
	return &InvocationsListResponse{Items: items, Total: total, Page: page, Size: size}, nil
}
