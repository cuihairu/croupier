package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// maxWarehouseSessionRows bounds the raw event scan behind the session-length
// distribution (same pattern as the realtime series row cap). Hitting the cap
// marks the response sampled instead of pretending completeness.
var maxWarehouseSessionRows = 50000

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

type WarehouseSessionsRequest struct {
	GameId string `form:"gameId"`
	Env    string `form:"env"`
	Days   int    `form:"days"` // lookback window, default 14, max 90
}

type WarehouseSessionBucket struct {
	Label string  `json:"label"`
	Count int64   `json:"count"`
	Share float64 `json:"share"`
}

type WarehouseSessionCause struct {
	Cause string `json:"cause"`
	Count int64  `json:"count"`
}

type WarehouseSessionsMeta struct {
	WindowDays       int   `json:"windowDays"`
	ScannedEnds      int64 `json:"scannedEnds"`
	ScannedStarts    int64 `json:"scannedStarts"`
	Sampled          bool  `json:"sampled"`
	InvalidDurations int64 `json:"invalidDurations"`
}

type WarehouseSessionsResponse struct {
	Buckets       []WarehouseSessionBucket `json:"buckets"`
	Causes        []WarehouseSessionCause  `json:"causes"`
	Sessions      int64                    `json:"sessions"`
	EndEvents     int64                    `json:"endEvents"`
	DuplicateEnds int64                    `json:"duplicateEnds"`
	Unresolved    int64                    `json:"unresolved"`
	AvgMs         float64                  `json:"avgMs"`
	P50Ms         float64                  `json:"p50Ms"`
	P95Ms         float64                  `json:"p95Ms"`
	Meta          WarehouseSessionsMeta    `json:"meta"`
}

// ---------------------------------------------------------------------------
// Service method
// ---------------------------------------------------------------------------

// WarehouseSessions returns the session-length distribution for ended
// sessions (DATA-037). Input contract: only events carrying both a session
// id and a numeric duration_ms end prop count — the bridge's session.end
// telemetry (and its underscore alias) is the authoritative producer. Policy:
// the latest end event per session wins (duplicate ends from redelivery or
// repeated closes collapse onto one session), durations are never imputed —
// a session with a start but no valid end inside the window is reported as
// unresolved, not assigned an assumed length. Sessions are attributed by
// their end time, so a session that started the previous day but ends inside
// the window counts exactly once.
func (s *Service) WarehouseSessions(ctx context.Context, req *WarehouseSessionsRequest) (*WarehouseSessionsResponse, error) {
	days := clampDays(req.Days, 14, 90)
	since := time.Now().UTC().AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)

	gameID, env, err := warehouseScope(ctx, req.GameId, req.Env)
	if err != nil {
		return nil, err
	}

	// Ended sessions: bounded scan over session-end events that carry a
	// duration prop. JSONHas pre-filters rows without duration_ms so the row
	// cap is spent on usable rows; Go still validates the value (a string or
	// negative value lands in invalidDurations, never in the distribution).
	endsQuery := fmt.Sprintf(
		"SELECT event_time, session_id, props_json FROM analytics.events "+
			"WHERE event IN ('session_end', 'session.end') AND session_id != '' "+
			"AND JSONHas(props_json, 'duration_ms') AND event_time >= ?%s "+
			"ORDER BY event_time, session_id LIMIT ?",
		warehouseScopeSuffix(gameID, env),
	)
	endsArgs := append([]any{since}, warehouseScopeArgs(gameID, env)...)
	endsArgs = append(endsArgs, maxWarehouseSessionRows)
	endRows, err := warehouseQuery(ctx, endsQuery, endsArgs...)
	if err != nil {
		return nil, err
	}

	type sessionEnd struct {
		at    time.Time
		durMs float64
		cause string
	}
	sessions := map[string]*sessionEnd{}
	var scannedEnds, invalidDurations int64
	scanEnd := func() error {
		for endRows.Next() {
			var at time.Time
			var sessionID, props string
			if err := endRows.Scan(&at, &sessionID, &props); err != nil {
				return fmt.Errorf("%w: scan session end: %v", errWarehouseQuery, err)
			}
			scannedEnds++
			var payload struct {
				DurationMs float64 `json:"duration_ms"`
				CauseEnd   string  `json:"cause_end"`
			}
			if err := json.Unmarshal([]byte(props), &payload); err != nil ||
				math.IsNaN(payload.DurationMs) || math.IsInf(payload.DurationMs, 0) ||
				payload.DurationMs < 0 {
				invalidDurations++
				continue
			}
			prev, ok := sessions[sessionID]
			if !ok || at.After(prev.at) {
				sessions[sessionID] = &sessionEnd{at: at, durMs: payload.DurationMs, cause: payload.CauseEnd}
			}
		}
		return nil
	}
	if err := scanEnd(); err != nil {
		_ = endRows.Close()
		return nil, err
	}
	if err := endRows.Err(); err != nil {
		_ = endRows.Close()
		return nil, fmt.Errorf("%w: iterate session end: %v", errWarehouseQuery, err)
	}
	if err := endRows.Close(); err != nil {
		return nil, fmt.Errorf("%w: close session end scan: %v", errWarehouseQuery, err)
	}

	// Started sessions in the same window: used only to disclose starts that
	// never produced a valid end (unresolved), not to impute durations.
	startsQuery := fmt.Sprintf(
		"SELECT session_id, count() FROM analytics.events "+
			"WHERE event IN ('session_start', 'session.start') AND session_id != '' "+
			"AND event_time >= ?%s GROUP BY session_id ORDER BY session_id LIMIT ?",
		warehouseScopeSuffix(gameID, env),
	)
	startsArgs := append([]any{since}, warehouseScopeArgs(gameID, env)...)
	startsArgs = append(startsArgs, maxWarehouseSessionRows)
	startRows, err := warehouseQuery(ctx, startsQuery, startsArgs...)
	if err != nil {
		return nil, err
	}
	started := map[string]struct{}{}
	scanStart := func() error {
		for startRows.Next() {
			var sessionID string
			var count uint64
			if err := startRows.Scan(&sessionID, &count); err != nil {
				return fmt.Errorf("%w: scan session start: %v", errWarehouseQuery, err)
			}
			started[sessionID] = struct{}{}
		}
		return nil
	}
	if err := scanStart(); err != nil {
		_ = startRows.Close()
		return nil, err
	}
	if err := startRows.Err(); err != nil {
		_ = startRows.Close()
		return nil, fmt.Errorf("%w: iterate session start: %v", errWarehouseQuery, err)
	}
	if err := startRows.Close(); err != nil {
		return nil, fmt.Errorf("%w: close session start scan: %v", errWarehouseQuery, err)
	}

	var unresolved int64
	for sessionID := range started {
		if _, ended := sessions[sessionID]; !ended {
			unresolved++
		}
	}

	sessionCount := int64(len(sessions))
	endEvents := scannedEnds - invalidDurations
	durations := make([]float64, 0, len(sessions))
	causeCounts := map[string]int64{}
	for _, s := range sessions {
		durations = append(durations, s.durMs)
		causeCounts[s.cause]++
	}
	sort.Float64s(durations)

	resp := &WarehouseSessionsResponse{
		Buckets:       sessionBuckets(durations, sessionCount),
		Causes:        sessionCauses(causeCounts),
		Sessions:      sessionCount,
		EndEvents:     endEvents,
		DuplicateEnds: endEvents - sessionCount,
		Unresolved:    unresolved,
		AvgMs:         sessionPercentile(durations, -1),
		P50Ms:         sessionPercentile(durations, 0.50),
		P95Ms:         sessionPercentile(durations, 0.95),
		Meta: WarehouseSessionsMeta{
			WindowDays:       days,
			ScannedEnds:      scannedEnds,
			ScannedStarts:    int64(len(started)),
			Sampled:          scannedEnds >= int64(maxWarehouseSessionRows) || int64(len(started)) >= int64(maxWarehouseSessionRows),
			InvalidDurations: invalidDurations,
		},
	}
	return resp, nil
}

// sessionBucketEdgesMs are the fixed distribution edges (inclusive lower
// bound, exclusive upper bound except the last open bucket).
var sessionBucketEdgesMs = []struct {
	label string
	maxMs float64
}{
	{"0-30s", 30_000},
	{"30s-1m", 60_000},
	{"1-5m", 300_000},
	{"5-15m", 900_000},
	{"15-30m", 1_800_000},
	{"30m-1h", 3_600_000},
	{"1-3h", 10_800_000},
	{"3-6h", 21_600_000},
	{"6-12h", 43_200_000},
	{"12-24h", 86_400_000},
	{"24h+", math.Inf(1)},
}

// sessionBuckets materialises every bucket (empty ones included) and shares
// of the resolved session total.
func sessionBuckets(durations []float64, sessionCount int64) []WarehouseSessionBucket {
	buckets := make([]WarehouseSessionBucket, 0, len(sessionBucketEdgesMs))
	for _, edge := range sessionBucketEdgesMs {
		var count int64
		for _, d := range durations {
			if d < edge.maxMs {
				count++
			}
		}
		// Remove the durations already assigned to earlier buckets by
		// subtracting previous counts instead of mutating the slice.
		var prior int64
		for i := range buckets {
			prior += buckets[i].Count
		}
		share := 0.0
		if sessionCount > 0 {
			share = math.Round(float64(count)/float64(sessionCount)*1000) / 1000
		}
		buckets = append(buckets, WarehouseSessionBucket{
			Label: edge.label,
			Count: count - prior,
			Share: share,
		})
	}
	return buckets
}

// sessionCauses sorts end-cause counts by volume then name.
func sessionCauses(causeCounts map[string]int64) []WarehouseSessionCause {
	causes := make([]WarehouseSessionCause, 0, len(causeCounts))
	for cause, count := range causeCounts {
		causes = append(causes, WarehouseSessionCause{Cause: cause, Count: count})
	}
	sort.Slice(causes, func(i, j int) bool {
		if causes[i].Count != causes[j].Count {
			return causes[i].Count > causes[j].Count
		}
		return causes[i].Cause < causes[j].Cause
	})
	return causes
}

// sessionPercentile returns the nearest-rank percentile of the sorted
// durations; quantile < 0 means the mean. Empty input yields 0.
func sessionPercentile(sorted []float64, quantile float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if quantile < 0 {
		sum := 0.0
		for _, d := range sorted {
			sum += d
		}
		return math.Round(sum/float64(n)*1000) / 1000
	}
	idx := int(math.Ceil(quantile*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return math.Round(sorted[idx]*1000) / 1000
}
