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

	// Bucket in Go instead of SQL: DATE_FORMAT/strftime differ across the
	// supported drivers (postgres / sqlite), and audit volume per bucket is
	// small enough that scanning raw timestamps is cheap.
	type rawRow struct {
		Timestamp time.Time
		Outcome   string
	}
	raw := []rawRow{}
	if err := scopedInvocationQuery(svcCtx.DB.WithContext(ctx), req.GameId, req.Env).
		Select("timestamp, outcome").
		Where("timestamp >= ?", since).
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

	// Volume + failures over the full window. CASE WHEN is portable across
	// postgres/sqlite; SUM(boolean) is postgres-invalid.
	totals := struct {
		Total  int64
		Failed int64
	}{}
	if err := scopedInvocationQuery(db, req.GameId, req.Env).
		Where("timestamp >= ?", since).
		Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN outcome <> 'success' THEN 1 ELSE 0 END), 0) AS failed").
		Scan(&totals).Error; err != nil {
		return nil, err
	}
	resp.Total, resp.Failed = totals.Total, totals.Failed
	if resp.Total > 0 {
		resp.SuccessRate = float64(resp.Total-resp.Failed) / float64(resp.Total)
	}

	// Latency: durations live in the promoted duration_ms column, so a
	// single-column scan (no JSON parsing) feeds avg + p95 in Go. p95 has
	// no portable SQL form across postgres/sqlite.
	latencyRows := []struct {
		DurationMs int64
	}{}
	if err := scopedInvocationQuery(db, req.GameId, req.Env).
		Where("timestamp >= ? AND duration_ms > 0", since).
		Select("duration_ms").
		Scan(&latencyRows).Error; err != nil {
		return nil, err
	}
	if len(latencyRows) > 0 {
		durations := make([]int64, 0, len(latencyRows))
		sum := int64(0)
		for _, r := range latencyRows {
			durations = append(durations, r.DurationMs)
			sum += r.DurationMs
		}
		resp.AvgDurationMs = float64(sum) / float64(len(durations))
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		idx := int(float64(len(durations))*0.95) - 1
		if idx < 0 {
			idx = 0
		}
		resp.P95DurationMs = float64(durations[idx])
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
