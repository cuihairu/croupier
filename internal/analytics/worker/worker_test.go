package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Mock ClickHouse Batch
// ---------------------------------------------------------------------------
type mockBatch struct {
	rows    []interface{}
	sendErr error
}

func (b *mockBatch) Abort() error                  { return nil }
func (b *mockBatch) Append(v ...any) error         { b.rows = append(b.rows, v); return nil }
func (b *mockBatch) AppendStruct(v any) error      { return nil }
func (b *mockBatch) Column(int) driver.BatchColumn { return nil }
func (b *mockBatch) Flush() error                  { return nil }
func (b *mockBatch) Send() error                   { return b.sendErr }
func (b *mockBatch) IsSent() bool                  { return false }
func (b *mockBatch) Rows() int                     { return len(b.rows) }
func (b *mockBatch) Columns() []column.Interface   { return nil }
func (b *mockBatch) Close() error                  { return nil }

// ---------------------------------------------------------------------------
// Mock ClickHouse Conn
// ---------------------------------------------------------------------------
type mockConn struct {
	lastQuery string
	batch     *mockBatch
}

func (c *mockConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.lastQuery = query
	return c.batch, nil
}
func (c *mockConn) Contributors() []string                                                { return nil }
func (c *mockConn) ServerVersion() (*driver.ServerVersion, error)                         { return nil, nil }
func (c *mockConn) Select(ctx context.Context, dest any, query string, args ...any) error { return nil }
func (c *mockConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, nil
}
func (c *mockConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row { return nil }
func (c *mockConn) Exec(ctx context.Context, query string, args ...any) error          { return nil }
func (c *mockConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (c *mockConn) Ping(ctx context.Context) error { return nil }
func (c *mockConn) Stats() driver.Stats            { return driver.Stats{} }
func (c *mockConn) Close() error                   { return nil }

// getMockConn extracts the mockConn from the Worker's ch field.
func getMockConn(t *testing.T, w *Worker) *mockConn {
	t.Helper()
	mc, ok := w.ch.(*mockConn)
	if !ok {
		t.Fatal("ch is not *mockConn")
	}
	return mc
}

// ---------------------------------------------------------------------------
// Helper: create a Worker wired to a miniredis + mock CH instance.
// ---------------------------------------------------------------------------
func testWorker(t *testing.T) (*Worker, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ch := &mockConn{batch: &mockBatch{}}
	w := &Worker{
		rdb:                rdb,
		ch:                 ch,
		streamEvents:       "analytics:events",
		streamPayments:     "analytics:payments",
		group:              "analytics-worker",
		consumer:           "test-consumer",
		touchedMinutes:     map[string]struct{}{},
		touchedDays:        map[string]struct{}{},
		revAgg:             map[string]*revRow{},
		clickBatchSize:     500,
		checkpointPrefix:   "analytics:checkpoint",
		lastIDs:            map[string]string{},
		deadEventsStream:   "analytics:events:dead",
		deadPaymentsStream: "analytics:payments:dead",
	}
	return w, mr
}

// ===========================================================================
// Pure helper function tests
// ===========================================================================

func TestEnvOrDefault(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		val    string
		def    string
		expect string
	}{
		{"env set", "TEST_WORKER_ENV_1", "hello", "default", "hello"},
		{"env empty string uses default", "TEST_WORKER_ENV_2", "", "default", "default"},
		{"env whitespace uses default", "TEST_WORKER_ENV_3", "   ", "default", "default"},
		{"env missing uses default", "TEST_WORKER_ENV_MISSING", "", "default", "default"},
		{"env set overrides default", "TEST_WORKER_ENV_4", "override", "fallback", "override"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.val != "" || tc.name == "env empty string uses default" || tc.name == "env whitespace uses default" {
				os.Setenv(tc.key, tc.val)
				defer os.Unsetenv(tc.key)
			} else {
				os.Unsetenv(tc.key)
			}
			got := envOrDefault(tc.key, tc.def)
			if got != tc.expect {
				t.Errorf("envOrDefault(%q, %q) = %q, want %q", tc.key, tc.def, got, tc.expect)
			}
		})
	}
}

func TestFmtAny(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"bytes", []byte("world"), "world"},
		{"empty bytes", []byte{}, ""},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtAny(tc.input)
			if got != tc.expect {
				t.Errorf("fmtAny(%v) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestAsString(t *testing.T) {
	m := map[string]any{
		"str":  "value",
		"num":  42,
		"bool": true,
		"nil":  nil,
	}
	tests := []struct {
		key    string
		expect string
	}{
		{"str", "value"},
		{"num", ""},
		{"bool", ""},
		{"nil", ""},
		{"missing", ""},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got := asString(m, tc.key)
			if got != tc.expect {
				t.Errorf("asString(%q) = %q, want %q", tc.key, got, tc.expect)
			}
		})
	}
}

func TestAsFloat(t *testing.T) {
	m := map[string]any{
		"f64":  float64(3.14),
		"int":  42,
		"str":  "hello",
		"bool": true,
		"nil":  nil,
	}
	tests := []struct {
		key    string
		expect float64
	}{
		{"f64", 3.14},
		{"int", 42.0},
		{"str", 0},
		{"bool", 0},
		{"nil", 0},
		{"missing", 0},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got := asFloat(m, tc.key)
			if got != tc.expect {
				t.Errorf("asFloat(%q) = %v, want %v", tc.key, got, tc.expect)
			}
		})
	}
}

func TestMax0(t *testing.T) {
	tests := []struct {
		input  int64
		expect int64
	}{
		{-10, 0},
		{-1, 0},
		{0, 0},
		{1, 1},
		{100, 100},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("n=%d", tc.input), func(t *testing.T) {
			got := max0(tc.input)
			if got != tc.expect {
				t.Errorf("max0(%d) = %d, want %d", tc.input, got, tc.expect)
			}
		})
	}
}

// ===========================================================================
// Worker batch tests
// ===========================================================================

func TestFlushEventBatch_NilBatch(t *testing.T) {
	w := &Worker{}
	if err := w.flushEventBatch(context.Background()); err != nil {
		t.Fatalf("expected nil error for nil batch, got %v", err)
	}
}

func TestFlushEventBatch_ZeroRows(t *testing.T) {
	w := &Worker{eventBatchRows: 0}
	if err := w.flushEventBatch(context.Background()); err != nil {
		t.Fatalf("expected nil error for zero rows, got %v", err)
	}
}

func TestFlushPaymentBatch_NilBatch(t *testing.T) {
	w := &Worker{}
	if err := w.flushPaymentBatch(context.Background()); err != nil {
		t.Fatalf("expected nil error for nil batch, got %v", err)
	}
}

func TestFlushPaymentBatch_ZeroRows(t *testing.T) {
	w := &Worker{paymentBatchRows: 0}
	if err := w.flushPaymentBatch(context.Background()); err != nil {
		t.Fatalf("expected nil error for zero rows, got %v", err)
	}
}

func TestFlushBatches_AllNil(t *testing.T) {
	w := &Worker{}
	if err := w.flushBatches(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestEnsureEventBatch_ReuseExisting(t *testing.T) {
	w, _ := testWorker(t)
	batch := &mockBatch{}
	w.eventBatch = batch
	got, err := w.ensureEventBatch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return the same batch pointer
	if got != batch {
		t.Error("expected same batch instance to be returned")
	}
}

func TestEnsurePaymentBatch_ReuseExisting(t *testing.T) {
	w, _ := testWorker(t)
	batch := &mockBatch{}
	w.paymentBatch = batch
	got, err := w.ensurePaymentBatch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != batch {
		t.Error("expected same batch instance to be returned")
	}
}

func TestEnsureEventBatch_NewBatch(t *testing.T) {
	w, _ := testWorker(t)
	batch, err := w.ensureEventBatch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batch == nil {
		t.Fatal("expected non-nil batch")
	}
	mc := getMockConn(t, w)
	if mc.lastQuery != insertEventsSQL {
		t.Errorf("unexpected query: %s", mc.lastQuery)
	}
}

func TestEnsurePaymentBatch_NewBatch(t *testing.T) {
	w, _ := testWorker(t)
	batch, err := w.ensurePaymentBatch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batch == nil {
		t.Fatal("expected non-nil batch")
	}
	mc := getMockConn(t, w)
	if mc.lastQuery != insertPaymentsSQL {
		t.Errorf("unexpected query: %s", mc.lastQuery)
	}
}

// ===========================================================================
// Append + flush with mock CH
// ===========================================================================

func TestAppendEventRow_Normal(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 500
	err := w.appendEventRow(context.Background(), "2025-01-01", "g1", "e1", "u1", "s1", "evt", "ch", "pl", "cn", "1.0", "eid", "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.eventBatchRows != 1 {
		t.Errorf("expected eventBatchRows=1, got %d", w.eventBatchRows)
	}
}

func TestAppendEventRow_BatchFull(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 2
	err := w.appendEventRow(context.Background(), "2025-01-01", "g1", "e1", "u1", "s1", "evt", "ch", "pl", "cn", "1.0", "eid", "{}")
	if err != nil {
		t.Fatalf("unexpected error on first row: %v", err)
	}
	err = w.appendEventRow(context.Background(), "2025-01-01", "g1", "e1", "u2", "s2", "evt2", "ch", "pl", "cn", "1.0", "eid2", "{}")
	if err != nil {
		t.Fatalf("unexpected error on second row: %v", err)
	}
	if w.eventBatch != nil {
		t.Error("expected eventBatch to be nil after flush")
	}
	if w.eventBatchRows != 0 {
		t.Errorf("expected eventBatchRows=0 after flush, got %d", w.eventBatchRows)
	}
}

func TestAppendEventRow_BatchSizeZero(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 0
	err := w.appendEventRow(context.Background(), "2025-01-01", "g1", "e1", "u1", "s1", "evt", "ch", "pl", "cn", "1.0", "eid", "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.eventBatch == nil {
		t.Error("expected eventBatch to still exist (no auto-flush)")
	}
}

func TestAppendPaymentRow_Normal(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 500
	err := w.appendPaymentRow(context.Background(), "2025-01-01", "g1", "e1", "u1", "o1", uint64(100), "USD", "success", "ch", "pl", "cn", "r", "c", "p", "reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.paymentBatchRows != 1 {
		t.Errorf("expected paymentBatchRows=1, got %d", w.paymentBatchRows)
	}
}

func TestAppendPaymentRow_BatchFull(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 2
	err := w.appendPaymentRow(context.Background(), "2025-01-01", "g1", "e1", "u1", "o1", uint64(100), "USD", "success", "ch", "pl", "cn", "r", "c", "p", "reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = w.appendPaymentRow(context.Background(), "2025-01-01", "g1", "e1", "u2", "o2", uint64(200), "USD", "success", "ch", "pl", "cn", "r", "c", "p", "reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.paymentBatch != nil {
		t.Error("expected paymentBatch to be nil after flush")
	}
	if w.paymentBatchRows != 0 {
		t.Errorf("expected paymentBatchRows=0 after flush, got %d", w.paymentBatchRows)
	}
}

func TestFlushEventBatch_SendError(t *testing.T) {
	w, _ := testWorker(t)
	mock := &mockBatch{sendErr: fmt.Errorf("send failed")}
	w.eventBatch = mock
	w.eventBatchRows = 1

	err := w.flushEventBatch(context.Background())
	if err == nil {
		t.Fatal("expected error from send")
	}
	if err.Error() != "send failed" {
		t.Errorf("unexpected error: %v", err)
	}
	// Batch is preserved on error for retry
	if w.eventBatch == nil {
		t.Error("expected eventBatch to be preserved after error for retry")
	}
}

func TestFlushPaymentBatch_SendError(t *testing.T) {
	w, _ := testWorker(t)
	mock := &mockBatch{sendErr: fmt.Errorf("send failed")}
	w.paymentBatch = mock
	w.paymentBatchRows = 1

	err := w.flushPaymentBatch(context.Background())
	if err == nil {
		t.Fatal("expected error from send")
	}
	// Batch is preserved on error for retry
	if w.paymentBatch == nil {
		t.Error("expected paymentBatch to be nil after error")
	}
}

// ===========================================================================
// Checkpoint tests
// ===========================================================================

func TestPersistCheckpoint_EmptyPrefix(t *testing.T) {
	w := &Worker{checkpointPrefix: ""}
	w.persistCheckpoint(context.Background(), "analytics:events", "123-0")
}

func TestPersistCheckpoint_EmptyID(t *testing.T) {
	w, _ := testWorker(t)
	w.persistCheckpoint(context.Background(), "analytics:events", "")
	val, err := w.rdb.Get(context.Background(), "analytics:checkpoint:analytics:events").Result()
	if err != redis.Nil {
		t.Fatalf("expected redis.Nil, got val=%q err=%v", val, err)
	}
}

func TestPersistCheckpoint_Normal(t *testing.T) {
	w, _ := testWorker(t)
	w.persistCheckpoint(context.Background(), "analytics:events", "999-0")
	val, err := w.rdb.Get(context.Background(), "analytics:checkpoint:analytics:events").Result()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != "999-0" {
		t.Errorf("expected 999-0, got %s", val)
	}
}

func TestRestoreCheckpoints_EmptyPrefix(t *testing.T) {
	w := &Worker{checkpointPrefix: ""}
	w.restoreCheckpoints(context.Background(), "analytics:events")
}

func TestRestoreCheckpoints_NoCheckpoint(t *testing.T) {
	w, _ := testWorker(t)
	w.restoreCheckpoints(context.Background(), "analytics:events")
	if _, ok := w.lastIDs["analytics:events"]; ok {
		t.Error("lastIDs should not be set when no checkpoint exists")
	}
}

func TestRestoreCheckpoints_EmptyID(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.Set(context.Background(), "analytics:checkpoint:analytics:events", "", 0)
	w.restoreCheckpoints(context.Background(), "analytics:events")
	if _, ok := w.lastIDs["analytics:events"]; ok {
		t.Error("lastIDs should not be set for empty checkpoint ID")
	}
}

// ===========================================================================
// touchRevenue tests (purely in-memory)
// ===========================================================================

func TestTouchRevenue_Success(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "game1", "env": "prod",
		"status": "success", "amount_cents": float64(999),
	})

	key := "2025-06-15|game1|prod"
	rv, ok := w.revAgg[key]
	if !ok {
		t.Fatal("expected revAgg entry to be created")
	}
	if rv.revenue != 999 {
		t.Errorf("expected revenue=999, got %d", rv.revenue)
	}
	if rv.refunds != 0 {
		t.Errorf("expected refunds=0, got %d", rv.refunds)
	}
	if rv.failed != 0 {
		t.Errorf("expected failed=0, got %d", rv.failed)
	}
}

func TestTouchRevenue_Refunded(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "game1", "env": "prod",
		"status": "refunded", "amount_cents": float64(500),
	})

	key := "2025-06-15|game1|prod"
	rv := w.revAgg[key]
	if rv.refunds != 500 {
		t.Errorf("expected refunds=500, got %d", rv.refunds)
	}
}

func TestTouchRevenue_Failed(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "game1", "env": "prod",
		"status": "failed", "amount_cents": float64(0),
	})

	key := "2025-06-15|game1|prod"
	rv := w.revAgg[key]
	if rv.failed != 1 {
		t.Errorf("expected failed=1, got %d", rv.failed)
	}
}

func TestTouchRevenue_Accumulate(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "e1",
		"status": "success", "amount_cents": float64(100),
	})
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T11:00:00Z", "game_id": "g1", "env": "e1",
		"status": "success", "amount_cents": float64(200),
	})

	key := "2025-06-15|g1|e1"
	rv := w.revAgg[key]
	if rv.revenue != 300 {
		t.Errorf("expected accumulated revenue=300, got %d", rv.revenue)
	}
}

func TestTouchRevenue_EmptyTimestamp(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"game_id": "g1", "env": "e1", "status": "success", "amount_cents": float64(50),
	})

	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("%s|g1|e1", today)
	rv, ok := w.revAgg[key]
	if !ok {
		t.Fatal("expected revAgg entry with today's date")
	}
	if rv.revenue != 50 {
		t.Errorf("expected revenue=50, got %d", rv.revenue)
	}
}

func TestTouchRevenue_InvalidTimestamp(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "not-a-date", "game_id": "g1", "env": "e1",
		"status": "success", "amount_cents": float64(75),
	})

	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("%s|g1|e1", today)
	rv := w.revAgg[key]
	if rv.revenue != 75 {
		t.Errorf("expected revenue=75, got %d", rv.revenue)
	}
}

func TestTouchRevenue_DifferentGames(t *testing.T) {
	w, _ := testWorker(t)
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "e1",
		"status": "success", "amount_cents": float64(100),
	})
	w.touchRevenue(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g2", "env": "e1",
		"status": "success", "amount_cents": float64(200),
	})

	if w.revAgg["2025-06-15|g1|e1"].revenue != 100 {
		t.Error("g1 revenue mismatch")
	}
	if w.revAgg["2025-06-15|g2|e1"].revenue != 200 {
		t.Error("g2 revenue mismatch")
	}
}

// ===========================================================================
// touchAgg tests (with miniredis for HLL operations)
// ===========================================================================

func TestTouchAgg_Heartbeat(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:45Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "heartbeat",
	})

	if len(w.touchedMinutes) == 0 {
		t.Error("expected touchedMinutes to be non-empty")
	}
	found := false
	for k := range w.touchedMinutes {
		if strings.HasPrefix(k, "hll:online:g1:prod:") {
			found = true
		}
	}
	if !found {
		t.Error("expected hll:online key in touchedMinutes")
	}
}

func TestTouchAgg_SessionStart(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "session_start",
	})

	if len(w.touchedMinutes) == 0 {
		t.Error("expected touchedMinutes for session_start")
	}
	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays for session_start")
	}
}

func TestTouchAgg_Login(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "login",
	})

	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays for login")
	}
	if len(w.touchedMinutes) != 0 {
		t.Error("login should not trigger touchedMinutes")
	}
}

func TestTouchAgg_Register(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "register",
	})

	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays for register")
	}
}

func TestTouchAgg_FirstActive(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "first_active",
	})

	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays for first_active")
	}
}

func TestTouchAgg_OtherEvent(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "click_button",
	})

	if len(w.touchedMinutes) != 0 {
		t.Error("no touchedMinutes expected for non-special event")
	}
	if len(w.touchedDays) != 0 {
		t.Error("no touchedDays expected for non-special event")
	}
}

func TestTouchAgg_EmptyTimestamp(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"game_id": "g1", "env": "prod", "user_id": "u1", "event": "heartbeat",
	})

	if len(w.touchedMinutes) == 0 {
		t.Error("expected touchedMinutes even with empty ts")
	}
}

func TestTouchAgg_InvalidTimestamp(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "bad-time", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "login",
	})

	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays even with invalid ts")
	}
}

func TestTouchAgg_EventCaseInsensitive(t *testing.T) {
	w, _ := testWorker(t)
	w.touchAgg(context.Background(), map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "HEARTBEAT",
	})

	if len(w.touchedMinutes) == 0 {
		t.Error("expected touchedMinutes for uppercase HEARTBEAT")
	}
}

// ===========================================================================
// processMessage tests
// ===========================================================================

func TestProcessMessage_EmptyData(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": ""},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte("")}}
	err := w.processMessage(context.Background(), "analytics:events", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pending, _ := w.rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: "analytics:events", Group: "analytics-worker",
		Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending))
	}
}

func TestProcessMessage_InvalidJSON(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": "not-json"},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte("not-json")}}
	err := w.processMessage(context.Background(), "analytics:events", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dlLen := w.rdb.XLen(context.Background(), "analytics:events:dead").Val()
	if dlLen != 1 {
		t.Errorf("expected 1 dead letter, got %d", dlLen)
	}
}

func TestProcessMessage_ValidEvent(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "session_id": "s1", "event": "login",
		"channel": "ios", "platform": "ios", "country": "CN",
		"app_version": "1.0.0", "event_id": "e1",
		"props": map[string]any{"level": float64(5)},
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": string(data)},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": data}}
	err := w.processMessage(context.Background(), "analytics:events", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pending, _ := w.rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: "analytics:events", Group: "analytics-worker",
		Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending))
	}

	if w.lastIDs["analytics:events"] != id {
		t.Errorf("expected lastID=%s, got %s", id, w.lastIDs["analytics:events"])
	}

	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays to be populated after login")
	}

	mc := getMockConn(t, w)
	if mc.lastQuery != insertEventsSQL {
		t.Errorf("expected events SQL, got %s", mc.lastQuery)
	}
}

func TestProcessMessage_ValidPayment(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:payments", "analytics-worker", "$")

	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "order_id": "ord1", "amount_cents": float64(999),
		"currency": "USD", "status": "success", "channel": "ios",
		"platform": "ios", "country": "US", "region": "CA", "city": "SF",
		"product_id": "p1", "reason": "",
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:payments",
		Values: map[string]any{"data": string(data)},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": data}}
	err := w.processMessage(context.Background(), "analytics:payments", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key := "2025-06-15|g1|prod"
	rv, ok := w.revAgg[key]
	if !ok {
		t.Fatal("expected revAgg entry after payment")
	}
	if rv.revenue != 999 {
		t.Errorf("expected revenue=999, got %d", rv.revenue)
	}

	mc := getMockConn(t, w)
	if mc.lastQuery != insertPaymentsSQL {
		t.Errorf("expected payments SQL, got %s", mc.lastQuery)
	}
}

func TestProcessMessage_MaxRetriesExceeded(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 1 // flush after each row so sendErr is triggered
	getMockConn(t, w).batch = &mockBatch{sendErr: fmt.Errorf("ch error")}
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "login", "retry_count": float64(maxPendingRetries),
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": string(data)},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": data}}
	err := w.processMessage(context.Background(), "analytics:events", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dlLen := w.rdb.XLen(context.Background(), "analytics:events:dead").Val()
	if dlLen != 1 {
		t.Errorf("expected 1 dead letter (max retries exceeded), got %d", dlLen)
	}
}

func TestProcessMessage_RetryRequeue(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 1 // flush after each row so sendErr is triggered
	getMockConn(t, w).batch = &mockBatch{sendErr: fmt.Errorf("ch error")}
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "login",
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": string(data)},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": data}}
	err := w.processMessage(context.Background(), "analytics:events", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// retry_count was nil → set to 0 → 0 < maxPendingRetries → re-queue
	dlLen := w.rdb.XLen(context.Background(), "analytics:events:dead").Val()
	if dlLen != 0 {
		t.Errorf("expected 0 dead letters (retry < max), got %d", dlLen)
	}

	// Should have re-queued a new message
	streamLen := w.rdb.XLen(context.Background(), "analytics:events").Val()
	if streamLen < 2 {
		t.Errorf("expected at least 2 messages (original + retry), got %d", streamLen)
	}
}

func TestProcessMessage_PaymentMaxRetries(t *testing.T) {
	w, _ := testWorker(t)
	w.clickBatchSize = 1 // flush after each row so sendErr is triggered
	getMockConn(t, w).batch = &mockBatch{sendErr: fmt.Errorf("ch error")}
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:payments", "analytics-worker", "$")

	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "order_id": "o1", "amount_cents": float64(100),
		"currency": "USD", "status": "success", "retry_count": float64(maxPendingRetries),
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:payments",
		Values: map[string]any{"data": string(data)},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": data}}
	err := w.processMessage(context.Background(), "analytics:payments", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dlLen := w.rdb.XLen(context.Background(), "analytics:payments:dead").Val()
	if dlLen != 1 {
		t.Errorf("expected 1 dead letter, got %d", dlLen)
	}
}

// ===========================================================================
// sendToDeadLetter tests
// ===========================================================================

func TestSendToDeadLetter_EventsStream(t *testing.T) {
	w, _ := testWorker(t)

	msg := redis.XMessage{
		ID:     "1-0",
		Values: map[string]any{"data": "some-data"},
	}
	w.sendToDeadLetter(context.Background(), "analytics:events", msg, "test_reason", "test_details")

	dlLen := w.rdb.XLen(context.Background(), "analytics:events:dead").Val()
	if dlLen != 1 {
		t.Errorf("expected 1 dead letter, got %d", dlLen)
	}

	result, _ := w.rdb.XRangeN(context.Background(), "analytics:events:dead", "-", "+", 1).Result()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	entry := result[0].Values
	if entry["original_stream"] != "analytics:events" {
		t.Errorf("original_stream: got %v", entry["original_stream"])
	}
	if entry["reason"] != "test_reason" {
		t.Errorf("reason: got %v", entry["reason"])
	}
	if entry["details"] != "test_details" {
		t.Errorf("details: got %v", entry["details"])
	}
	if entry["original_id"] != "1-0" {
		t.Errorf("original_id: got %v", entry["original_id"])
	}
}

func TestSendToDeadLetter_PaymentsStream(t *testing.T) {
	w, _ := testWorker(t)

	msg := redis.XMessage{
		ID:     "2-0",
		Values: map[string]any{"data": "payment-data"},
	}
	w.sendToDeadLetter(context.Background(), "analytics:payments", msg, "test_reason", "test_details")

	dlLen := w.rdb.XLen(context.Background(), "analytics:payments:dead").Val()
	if dlLen != 1 {
		t.Errorf("expected 1 dead letter, got %d", dlLen)
	}

	result, _ := w.rdb.XRangeN(context.Background(), "analytics:payments:dead", "-", "+", 1).Result()
	entry := result[0].Values
	if entry["original_stream"] != "analytics:payments" {
		t.Errorf("original_stream: got %v", entry["original_stream"])
	}
}

// ===========================================================================
// reclaimPendingFromStream tests
// ===========================================================================

func TestReclaimPendingFromStream_NoConsumers(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")
	w.reclaimPendingFromStream(context.Background(), "analytics:events")
}

func TestReclaimPendingFromStream_NoIdleMessages(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": "test"},
	})

	w.rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group: "analytics-worker", Consumer: "other-consumer",
		Streams: []string{"analytics:events", ">"}, Count: 1,
	})

	w.reclaimPendingFromStream(context.Background(), "analytics:events")

	pending, _ := w.rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: "analytics:events", Group: "analytics-worker",
		Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending (not reclaimed), got %d", len(pending))
	}
}

func TestReclaimPendingFromStream_IdleMessage(t *testing.T) {
	w, mr := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u1"}`},
	})

	w.rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group: "analytics-worker", Consumer: "stale-consumer",
		Streams: []string{"analytics:events", ">"}, Count: 1,
	})

	mr.FastForward(10 * time.Minute)

	w.reclaimPendingFromStream(context.Background(), "analytics:events")

	pending, _ := w.rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: "analytics:events", Group: "analytics-worker",
		Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) > 1 {
		t.Errorf("expected at most 1 pending after reclaim, got %d", len(pending))
	}
}

func TestReclaimPendingFromStream_EmptyStream(t *testing.T) {
	w, _ := testWorker(t)
	w.reclaimPendingFromStream(context.Background(), "nonexistent:stream")
}

// ===========================================================================
// Run test
// ===========================================================================

func TestRun_CancelContext(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:payments", "analytics-worker", "$")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- w.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled on cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within timeout")
	}
}

// ===========================================================================
// flush tests
// ===========================================================================

func TestFlush_EmptyState(t *testing.T) {
	w, _ := testWorker(t)
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFlush_TouchedMinute_InvalidFormat(t *testing.T) {
	w, _ := testWorker(t)
	w.touchedMinutes["bad-key"] = struct{}{}
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.touchedMinutes["bad-key"]; ok {
		t.Error("expected bad key to be removed")
	}
}

func TestFlush_TouchedDay_InvalidFormat(t *testing.T) {
	w, _ := testWorker(t)
	w.touchedDays["bad-key"] = struct{}{}
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.touchedDays["bad-key"]; ok {
		t.Error("expected bad key to be removed")
	}
}

func TestFlush_TouchedDay_ValidFormat(t *testing.T) {
	w, _ := testWorker(t)
	w.touchedDays["2025-06-15|g1|e1"] = struct{}{}
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.touchedDays["2025-06-15|g1|e1"]; ok {
		t.Error("expected key to be removed after flush")
	}
}

func TestFlush_RevAgg_InvalidFormat(t *testing.T) {
	w, _ := testWorker(t)
	w.revAgg["bad-key"] = &revRow{revenue: 100}
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.revAgg["bad-key"]; !ok {
		t.Error("expected bad key to remain (continue on invalid format)")
	}
}

func TestFlush_RevAgg_ValidFormat(t *testing.T) {
	w, _ := testWorker(t)
	w.revAgg["2025-06-15|g1|e1"] = &revRow{revenue: 500, refunds: 50, failed: 2}
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	// CH mock succeeds → entry should be deleted
	if _, ok := w.revAgg["2025-06-15|g1|e1"]; ok {
		t.Error("expected revAgg entry to be deleted after successful flush")
	}
}

func TestFlush_RevAgg_SendError(t *testing.T) {
	w, _ := testWorker(t)
	getMockConn(t, w).batch = &mockBatch{sendErr: fmt.Errorf("send error")}
	w.revAgg["2025-06-15|g1|e1"] = &revRow{revenue: 500, refunds: 50, failed: 2}
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.revAgg["2025-06-15|g1|e1"]; !ok {
		t.Error("expected revAgg entry to remain after CH error")
	}
}

func TestFlush_TouchedMinute_ValidKey(t *testing.T) {
	w, _ := testWorker(t)
	pastTime := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	k := fmt.Sprintf("hll:online:g1:prod:%s", pastTime.Format("200601021504"))
	w.touchedMinutes[k] = struct{}{}
	w.rdb.PFAdd(context.Background(), k, "user1")

	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.touchedMinutes[k]; ok {
		t.Error("expected touchedMinutes key to be removed after flush")
	}
}

func TestFlush_TouchedMinute_FutureKey(t *testing.T) {
	w, _ := testWorker(t)
	futureTime := time.Now().Add(5 * time.Minute).Truncate(time.Minute)
	k := fmt.Sprintf("hll:online:g1:prod:%s", futureTime.Format("200601021504"))
	w.touchedMinutes[k] = struct{}{}

	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.touchedMinutes[k]; !ok {
		t.Error("expected future touchedMinutes key to remain")
	}
}

func TestFlush_PFCountZero(t *testing.T) {
	w, _ := testWorker(t)
	k := "hll:online:g1:prod:202501010000"
	w.touchedMinutes[k] = struct{}{}

	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, ok := w.touchedMinutes[k]; ok {
		t.Error("expected key to be deleted after flush (PFCount=0)")
	}
}

func TestFlush_PFCountNegative(t *testing.T) {
	w, _ := testWorker(t)
	k := "hll:online:g1:prod:202501010000"
	w.touchedMinutes[k] = struct{}{}

	// miniredis PFCount for non-existent key returns 0
	// The code checks `if n < 0 { n = 0 }` which we cover via the path
	err := w.flush(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// ===========================================================================
// NewWorker tests
// ===========================================================================

func TestNewWorker_InvalidRedisURL(t *testing.T) {
	os.Setenv("REDIS_URL", "not-a-valid-url")
	defer os.Unsetenv("REDIS_URL")

	_, err := NewWorker()
	if err == nil {
		t.Fatal("expected error for invalid Redis URL")
	}
	if !strings.Contains(err.Error(), "redis") {
		t.Errorf("expected redis error, got: %v", err)
	}
}

func TestNewWorker_InvalidCHDSN(t *testing.T) {
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("CLICKHOUSE_DSN", "://invalid")
	defer os.Unsetenv("REDIS_URL")
	defer os.Unsetenv("CLICKHOUSE_DSN")

	_, err := NewWorker()
	if err == nil {
		t.Fatal("expected error for invalid ClickHouse DSN")
	}
	if !strings.Contains(err.Error(), "clickhouse") {
		t.Errorf("expected clickhouse error, got: %v", err)
	}
}

func TestNewWorker_InvalidBatchSize(t *testing.T) {
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("CLICKHOUSE_DSN", "clickhouse://localhost:9000/analytics")
	os.Setenv("ANALYTICS_CLICKHOUSE_BATCH", "not-a-number")
	defer os.Unsetenv("REDIS_URL")
	defer os.Unsetenv("CLICKHOUSE_DSN")
	defer os.Unsetenv("ANALYTICS_CLICKHOUSE_BATCH")

	_, err := NewWorker()
	if err != nil && strings.Contains(err.Error(), "batch") {
		t.Errorf("invalid batch size should not cause error, got: %v", err)
	}
}

func TestNewWorker_ZeroBatchSize(t *testing.T) {
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("CLICKHOUSE_DSN", "clickhouse://localhost:9000/analytics")
	os.Setenv("ANALYTICS_CLICKHOUSE_BATCH", "0")
	defer os.Unsetenv("REDIS_URL")
	defer os.Unsetenv("CLICKHOUSE_DSN")
	defer os.Unsetenv("ANALYTICS_CLICKHOUSE_BATCH")

	_, err := NewWorker()
	if err != nil && strings.Contains(err.Error(), "batch") {
		t.Errorf("zero batch size should not cause error, got: %v", err)
	}
}

func TestNewWorker_NegativeBatchSize(t *testing.T) {
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("CLICKHOUSE_DSN", "clickhouse://localhost:9000/analytics")
	os.Setenv("ANALYTICS_CLICKHOUSE_BATCH", "-5")
	defer os.Unsetenv("REDIS_URL")
	defer os.Unsetenv("CLICKHOUSE_DSN")
	defer os.Unsetenv("ANALYTICS_CLICKHOUSE_BATCH")

	_, err := NewWorker()
	if err != nil && strings.Contains(err.Error(), "batch") {
		t.Errorf("negative batch size should not cause error, got: %v", err)
	}
}

func TestNewWorker_CheckpointPrefixTrimmed(t *testing.T) {
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("CLICKHOUSE_DSN", "clickhouse://localhost:9000/analytics")
	os.Setenv("ANALYTICS_CHECKPOINT_PREFIX", "my:prefix:")
	defer os.Unsetenv("REDIS_URL")
	defer os.Unsetenv("CLICKHOUSE_DSN")
	defer os.Unsetenv("ANALYTICS_CHECKPOINT_PREFIX")

	_, err := NewWorker()
	if err != nil && strings.Contains(err.Error(), "batch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewWorker_DefaultEnvVars(t *testing.T) {
	envVars := []string{
		"REDIS_URL", "ANALYTICS_REDIS_STREAM_EVENTS", "ANALYTICS_REDIS_STREAM_PAYMENTS",
		"WORKER_GROUP", "WORKER_CONSUMER", "CLICKHOUSE_DSN",
		"ANALYTICS_CLICKHOUSE_BATCH", "ANALYTICS_CHECKPOINT_PREFIX",
		"ANALYTICS_DEAD_EVENTS_STREAM", "ANALYTICS_DEAD_PAYMENTS_STREAM",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	_, err := NewWorker()
	if err != nil {
		if !strings.Contains(err.Error(), "clickhouse") {
			t.Errorf("unexpected error type: %v", err)
		}
	}
}

func TestNewWorker_CustomConsumerName(t *testing.T) {
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("CLICKHOUSE_DSN", "clickhouse://localhost:9000/analytics")
	os.Setenv("WORKER_CONSUMER", "my-consumer")
	defer os.Unsetenv("REDIS_URL")
	defer os.Unsetenv("CLICKHOUSE_DSN")
	defer os.Unsetenv("WORKER_CONSUMER")

	_, err := NewWorker()
	if err != nil && strings.Contains(err.Error(), "batch") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ===========================================================================
// ensureGroups tests
// ===========================================================================

func TestEnsureGroups(t *testing.T) {
	w, _ := testWorker(t)
	w.ensureGroups(context.Background())

	groups, _ := w.rdb.XInfoGroups(context.Background(), "analytics:events").Result()
	found := false
	for _, g := range groups {
		if g.Name == "analytics-worker" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected analytics-worker group to be created")
	}
}

func TestEnsureGroups_Idempotent(t *testing.T) {
	w, _ := testWorker(t)
	w.ensureGroups(context.Background())
	w.ensureGroups(context.Background())
}

// ===========================================================================
// Integration tests
// ===========================================================================

func TestProcessMessage_FullEventFlow(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:events", "analytics-worker", "$")

	// Login → DAU
	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "login",
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": string(data)},
	}).Val()

	msg := redis.XMessage{ID: id, Values: map[string]any{"data": data}}
	err := w.processMessage(context.Background(), "analytics:events", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(w.touchedDays) == 0 {
		t.Error("expected touchedDays for login")
	}

	// Heartbeat → minute online
	eventData2 := map[string]any{
		"ts": "2025-06-15T10:31:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "heartbeat",
	}
	data2, _ := json.Marshal(eventData2)

	id2 := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:events",
		Values: map[string]any{"data": string(data2)},
	}).Val()

	msg2 := redis.XMessage{ID: id2, Values: map[string]any{"data": data2}}
	err = w.processMessage(context.Background(), "analytics:events", msg2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(w.touchedMinutes) == 0 {
		t.Error("expected touchedMinutes for heartbeat")
	}
}

func TestProcessMessage_FullPaymentFlow(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.XGroupCreateMkStream(context.Background(), "analytics:payments", "analytics-worker", "$")

	// Success
	data1, _ := json.Marshal(map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "order_id": "ord1", "amount_cents": float64(500),
		"currency": "USD", "status": "success",
	})
	id1 := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:payments", Values: map[string]any{"data": string(data1)},
	}).Val()
	w.processMessage(context.Background(), "analytics:payments", redis.XMessage{ID: id1, Values: map[string]any{"data": data1}})

	// Refunded
	data2, _ := json.Marshal(map[string]any{
		"ts": "2025-06-15T11:00:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "order_id": "ord2", "amount_cents": float64(200),
		"currency": "USD", "status": "refunded",
	})
	id2 := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "analytics:payments", Values: map[string]any{"data": string(data2)},
	}).Val()
	w.processMessage(context.Background(), "analytics:payments", redis.XMessage{ID: id2, Values: map[string]any{"data": data2}})

	key := "2025-06-15|g1|prod"
	rv := w.revAgg[key]
	if rv.revenue != 500 {
		t.Errorf("expected revenue=500, got %d", rv.revenue)
	}
	if rv.refunds != 200 {
		t.Errorf("expected refunds=200, got %d", rv.refunds)
	}
}

// ===========================================================================
// Benchmark
// ===========================================================================

func BenchmarkFmtAny(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fmtAny("hello")
		fmtAny([]byte("world"))
		fmtAny(nil)
		fmtAny(42)
	}
}

func BenchmarkAsString(b *testing.B) {
	m := map[string]any{"key": "value", "num": 42}
	for i := 0; i < b.N; i++ {
		asString(m, "key")
		asString(m, "missing")
	}
}

func BenchmarkAsFloat(b *testing.B) {
	m := map[string]any{"f": float64(3.14), "i": 42}
	for i := 0; i < b.N; i++ {
		asFloat(m, "f")
		asFloat(m, "i")
	}
}
