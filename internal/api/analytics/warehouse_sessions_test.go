package analytics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routingConn dispatches canned rows by the queried event family: the
// sessions endpoint issues one end-scan and one start-scan per request, so a
// single fakeConn (one shared row set) cannot drive both.
type routingConn struct {
	ends     *fakeRows
	starts   *fakeRows
	queries  []string
	queryErr error
	lastArgs []any
}

func (f *routingConn) Query(_ context.Context, query string, args ...any) (warehouseRows, error) {
	f.queries = append(f.queries, query)
	f.lastArgs = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	switch {
	case containsAll(query, "session_end", "session.start"):
		// defensive: a query touching both families has no scripted answer
		return nil, errors.New("unexpected mixed query")
	case containsAll(query, "session_end"):
		return f.ends, nil
	case containsAll(query, "session_start"):
		return f.starts, nil
	}
	return nil, errors.New("unexpected query")
}

func containsAll(s string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}

func withRoutingConn(t *testing.T, conn *routingConn) {
	t.Helper()
	orig := warehouseConnect
	warehouseConnect = func() (warehouseConn, error) { return conn, nil }
	t.Cleanup(func() { warehouseConnect = orig })
}

func sessionEndRow(at time.Time, sessionID, props string) []any {
	return []any{at, sessionID, props}
}

func sessionStartRow(sessionID string, count uint64) []any {
	return []any{sessionID, count}
}

// TestWarehouseSessions_DistributionCoversDisconnectDuplicateAndUnresolved
// pins the fixed-sample contract: a disconnect session, a session with a
// duplicate end (latest wins), a 26-hour session, and a start-only session
// that must surface as unresolved instead of an assumed duration.
func TestWarehouseSessions_DistributionCoversDisconnectDuplicateAndUnresolved(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	conn := &routingConn{
		ends: &fakeRows{rows: [][]any{
			sessionEndRow(base.Add(0), "s1", `{"duration_ms":45000,"cause_end":"disconnect"}`),
			sessionEndRow(base.Add(5*time.Minute), "s2", `{"duration_ms":60000,"cause_end":"normal"}`),
			sessionEndRow(base.Add(6*time.Minute), "s2", `{"duration_ms":90000,"cause_end":"normal"}`),
			sessionEndRow(base.Add(10*time.Minute), "s3", `{"duration_ms":93600000,"cause_end":"normal"}`),
		}},
		starts: &fakeRows{rows: [][]any{
			sessionStartRow("s1", 1),
			sessionStartRow("s2", 3), // duplicate starts do not inflate anything
			sessionStartRow("s3", 1),
			sessionStartRow("s4", 1), // start without any end -> unresolved
		}},
	}
	withRoutingConn(t, conn)

	resp, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
	require.NoError(t, err)

	require.Len(t, resp.Buckets, len(sessionBucketEdgesMs))
	byLabel := map[string]WarehouseSessionBucket{}
	for _, b := range resp.Buckets {
		byLabel[b.Label] = b
	}
	assert.EqualValues(t, 1, byLabel["30s-1m"].Count, "45s disconnect session")
	assert.EqualValues(t, 1, byLabel["1-5m"].Count, "90s session (latest end wins)")
	assert.EqualValues(t, 1, byLabel["24h+"].Count, "26h session")
	var total int64
	for _, b := range resp.Buckets {
		total += b.Count
	}
	assert.EqualValues(t, resp.Sessions, total, "buckets must partition the sessions")

	assert.EqualValues(t, 3, resp.Sessions)
	assert.EqualValues(t, 4, resp.EndEvents)
	assert.EqualValues(t, 1, resp.DuplicateEnds)
	assert.EqualValues(t, 1, resp.Unresolved)
	assert.Equal(t, []WarehouseSessionCause{
		{Cause: "normal", Count: 2},
		{Cause: "disconnect", Count: 1},
	}, resp.Causes)

	// sorted durations [45000, 90000, 93600000]; nearest-rank p50 is the 2nd
	assert.InDelta(t, 90000, resp.P50Ms, 0.001)
	assert.InDelta(t, 93600000, resp.P95Ms, 0.001)
	avg := (45000 + 90000 + 93600000) / 3
	assert.InDelta(t, avg, resp.AvgMs, 0.001)

	assert.EqualValues(t, 14, resp.Meta.WindowDays)
	assert.EqualValues(t, 4, resp.Meta.ScannedEnds)
	assert.EqualValues(t, 4, resp.Meta.ScannedStarts)
	assert.False(t, resp.Meta.Sampled)
	assert.EqualValues(t, 0, resp.Meta.InvalidDurations)
}

// TestWarehouseSessions_CrossDaySessionCountsOnce pins the window-attribute
// rule: the distribution is attributed by end time, so a session that started
// the day before the window but ends inside it counts exactly once and does
// not become unresolved.
func TestWarehouseSessions_CrossDaySessionCountsOnce(t *testing.T) {
	endToday := time.Date(2026, 11, 20, 0, 30, 0, 0, time.UTC)
	conn := &routingConn{
		ends: &fakeRows{rows: [][]any{
			sessionEndRow(endToday, "sx", `{"duration_ms":7200000,"cause_end":"normal"}`),
		}},
		starts: &fakeRows{rows: [][]any{
			sessionStartRow("sx", 1), // start lies before the window's first day
		}},
	}
	withRoutingConn(t, conn)

	resp, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
	require.NoError(t, err)

	assert.EqualValues(t, 1, resp.Sessions)
	assert.EqualValues(t, 0, resp.Unresolved)
	byLabel := map[string]WarehouseSessionBucket{}
	for _, b := range resp.Buckets {
		byLabel[b.Label] = b
	}
	assert.EqualValues(t, 1, byLabel["1-3h"].Count)
}

// TestWarehouseSessions_InvalidDurationsExcluded pins the numeric contract:
// string or negative duration values never enter the distribution; if the
// session has no valid end it degrades to unresolved, never to an assumed 0.
func TestWarehouseSessions_InvalidDurationsExcluded(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	conn := &routingConn{
		ends: &fakeRows{rows: [][]any{
			sessionEndRow(base, "s1", `{"duration_ms":"oops","cause_end":"normal"}`),
			sessionEndRow(base, "s2", `{"duration_ms":-5,"cause_end":"normal"}`),
			sessionEndRow(base, "s3", `not-json`),
			sessionEndRow(base, "s4", `{"duration_ms":120000,"cause_end":"normal"}`),
		}},
		starts: &fakeRows{rows: [][]any{
			sessionStartRow("s1", 1),
			sessionStartRow("s2", 1),
			sessionStartRow("s3", 1),
			sessionStartRow("s4", 1),
		}},
	}
	withRoutingConn(t, conn)

	resp, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
	require.NoError(t, err)

	assert.EqualValues(t, 1, resp.Sessions, "only the valid-duration session resolves")
	assert.EqualValues(t, 3, resp.Meta.InvalidDurations)
	assert.EqualValues(t, 3, resp.Unresolved, "invalid ends degrade to unresolved")
	assert.InDelta(t, 120000, resp.P50Ms, 0.001)
	assert.EqualValues(t, 1, resp.EndEvents, "invalid rows leave the end-event count too")
	assert.EqualValues(t, 0, resp.DuplicateEnds)
}

// TestWarehouseSessions_SampledDisclosure pins the bounded scan: hitting the
// row cap marks the response sampled instead of pretending completeness.
func TestWarehouseSessions_SampledDisclosure(t *testing.T) {
	origCap := maxWarehouseSessionRows
	maxWarehouseSessionRows = 2
	t.Cleanup(func() { maxWarehouseSessionRows = origCap })

	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	conn := &routingConn{
		ends: &fakeRows{rows: [][]any{
			sessionEndRow(base, "s1", `{"duration_ms":1000,"cause_end":"normal"}`),
			sessionEndRow(base, "s2", `{"duration_ms":2000,"cause_end":"normal"}`),
		}},
		starts: &fakeRows{rows: [][]any{
			sessionStartRow("s1", 1),
			sessionStartRow("s2", 1),
		}},
	}
	withRoutingConn(t, conn)

	resp, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Meta.Sampled, "ends scan reached the cap")
	assert.EqualValues(t, 2, resp.Meta.ScannedEnds)
}

// TestWarehouseSessions_EmptyWindow returns the canonical empty shape: every
// bucket present at zero, no sessions, no causes.
func TestWarehouseSessions_EmptyWindow(t *testing.T) {
	conn := &routingConn{
		ends:   &fakeRows{rows: [][]any{}},
		starts: &fakeRows{rows: [][]any{}},
	}
	withRoutingConn(t, conn)

	resp, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"), &WarehouseSessionsRequest{})
	require.NoError(t, err)

	require.Len(t, resp.Buckets, len(sessionBucketEdgesMs))
	for _, b := range resp.Buckets {
		assert.EqualValues(t, 0, b.Count)
		assert.InDelta(t, 0, b.Share, 0.0001)
	}
	assert.EqualValues(t, 0, resp.Sessions)
	assert.Empty(t, resp.Causes)
	assert.InDelta(t, 0, resp.AvgMs, 0.0001)
}

// TestWarehouseSessions_QueryBoundedAndScoped pins the wire: both scans carry
// the dotted/underscore spellings, the current scope suffix and the row cap.
func TestWarehouseSessions_QueryBoundedAndScoped(t *testing.T) {
	conn := &routingConn{
		ends:   &fakeRows{rows: [][]any{}},
		starts: &fakeRows{rows: [][]any{}},
	}
	withRoutingConn(t, conn)

	_, err := NewService(nil).WarehouseSessions(warehouseScopeCtx("game-a", "prod"),
		&WarehouseSessionsRequest{GameId: "game-a", Env: "prod", Days: 30})
	require.NoError(t, err)

	require.Len(t, conn.queries, 2)
	for _, q := range conn.queries {
		assert.Contains(t, q, "game_id = ?")
		assert.Contains(t, q, "env = ?")
		assert.Contains(t, q, "LIMIT ?")
	}
	assert.Contains(t, conn.queries[0], "'session_end'")
	assert.Contains(t, conn.queries[0], "'session.end'")
	assert.Contains(t, conn.queries[0], "JSONHas(props_json, 'duration_ms')")
	assert.Contains(t, conn.queries[1], "'session_start'")
	assert.Contains(t, conn.queries[1], "'session.start'")
	// args order: since, scope args..., cap
	require.Len(t, conn.lastArgs, 4)
	assert.EqualValues(t, 50000, conn.lastArgs[len(conn.lastArgs)-1])
}

// TestWarehouseSessions_HandlerAndErrors covers the HTTP surface: scoped 200
// with the distribution body, disabled warehouse 503, query failure 500.
func TestWarehouseSessions_HandlerAndErrors(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	conn := &routingConn{
		ends: &fakeRows{rows: [][]any{
			sessionEndRow(base, "s1", `{"duration_ms":45000,"cause_end":"disconnect"}`),
		}},
		starts: &fakeRows{rows: [][]any{
			sessionStartRow("s1", 1),
		}},
	}
	withRoutingConn(t, conn)
	handler := NewHandler(NewService(nil), config.SSEConfig{})

	ctx, rec := newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/sessions?days=7", "")
	withWarehouseScope(ctx, "game-a", "prod")
	handler.WarehouseSessions(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"sessions":1`)
	assert.Contains(t, rec.Body.String(), `"cause":"disconnect"`)
	assert.Contains(t, rec.Body.String(), `"windowDays":7`)

	withDisabledWarehouse(t)
	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/sessions", "")
	withWarehouseScope(ctx, "game-a", "prod")
	handler.WarehouseSessions(ctx)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	conn2 := &routingConn{queryErr: fmt.Errorf("dial refused")}
	withRoutingConn(t, conn2)
	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/sessions", "")
	withWarehouseScope(ctx, "game-a", "prod")
	handler.WarehouseSessions(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
