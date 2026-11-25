package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// recordingConn is a concurrency-safe mock ClickHouse connection: each
// PrepareBatch returns a fresh batch, and Send records the daily_revenue
// rows into the conn counters, feeding DATA-014's "fixed sample versus total"
// cross-check (flushed + revAgg remainder == total written).
type recordingConn struct {
	mu      sync.Mutex
	revSent uint64 // revenue_cents already Sent
	refunds uint64
	failed  uint64
	dailyN  int // number of daily_revenue rows
}

func (c *recordingConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &recordingBatch{conn: c, query: query}, nil
}
func (c *recordingConn) Contributors() []string                        { return nil }
func (c *recordingConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }
func (c *recordingConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return nil
}
func (c *recordingConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, nil
}
func (c *recordingConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return nil
}
func (c *recordingConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (c *recordingConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (c *recordingConn) Ping(ctx context.Context) error { return nil }
func (c *recordingConn) Stats() driver.Stats            { return driver.Stats{} }
func (c *recordingConn) Close() error                   { return nil }

type recordingBatch struct {
	conn  *recordingConn
	query string
	rows  []any
}

func (b *recordingBatch) Abort() error                  { return nil }
func (b *recordingBatch) Append(v ...any) error         { b.rows = append(b.rows, v); return nil }
func (b *recordingBatch) AppendStruct(v any) error      { return nil }
func (b *recordingBatch) Column(int) driver.BatchColumn { return nil }
func (b *recordingBatch) Flush() error                  { return nil }
func (b *recordingBatch) IsSent() bool                  { return true }
func (b *recordingBatch) Rows() int                     { return len(b.rows) }
func (b *recordingBatch) Columns() []column.Interface   { return nil }
func (b *recordingBatch) Close() error                  { return nil }

func (b *recordingBatch) Send() error {
	// daily_revenue row shape: (d, game_id, env, revenue_cents, refunds_cents, failed, version)
	if b.query == "INSERT INTO analytics.daily_revenue (d, game_id, env, revenue_cents, refunds_cents, failed, version)" {
		b.conn.mu.Lock()
		for _, r := range b.rows {
			row, ok := r.([]any)
			if !ok || len(row) < 6 {
				continue
			}
			b.conn.revSent += toUint64(row[3])
			b.conn.refunds += toUint64(row[4])
			b.conn.failed += toUint64(row[5])
			b.conn.dailyN++
		}
		b.conn.mu.Unlock()
	}
	return nil
}

func toUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case uint32:
		return uint64(n)
	case int:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	}
	return 0
}

// TestSharedState_ConcurrentRevAggNoLoss verifies DATA-014's "fixed sample
// versus total" cross-check: with concurrent commitRevenue (writing revAgg)
// and flush (read+delete), sent revenue plus the revAgg remainder always
// equals the total written — nothing lost, nothing duplicated.
func TestSharedState_ConcurrentRevAggNoLoss(t *testing.T) {
	w := &Worker{
		rdb:            newTestRedisClient(t),
		ch:             &recordingConn{},
		touchedMinutes: map[string]struct{}{},
		touchedDays:    map[string]struct{}{},
		revAgg:         map[string]*revRow{},
		clickBatchSize: 500,
	}
	rec := w.ch.(*recordingConn)

	const writers = 8
	const perWriter = 25
	const total = writers * perWriter // 100 cents per success -> expected revenue = total*100

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				w.commitRevenue([]ackRef{{revKey: "2025-06-15|g1|e1", rev: &revRow{revenue: 100}}})
			}
		}()
	}
	// Concurrent flush read+delete, checking for no loss and no duplication
	// under synchronized ownership.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < writers; i++ {
			_ = w.flush(context.Background())
		}
	}()
	wg.Wait()

	rec.mu.Lock()
	sent := rec.revSent
	rec.mu.Unlock()

	w.aggMu.Lock()
	remaining := uint64(0)
	if rv := w.revAgg["2025-06-15|g1|e1"]; rv != nil {
		remaining = rv.revenue
	}
	w.aggMu.Unlock()

	if got := sent + remaining; got != uint64(total*100) {
		t.Fatalf("total revenue mismatch: sent=%d remaining=%d got=%d want=%d", sent, remaining, got, total*100)
	}
}

// #656: when Redis PFCount errors out during the daily_users flush, the real
// DAU of the day must not be overwritten with 0 (ReplacingMergeTree replaces
// by version); the day should be merged back into the shared map and retried
// next time.
func TestFlush_DailyUsersPFCntErrorRemerges(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rec := &recordingConn{}
	w := &Worker{
		rdb:            rdb,
		ch:             rec,
		touchedMinutes: map[string]struct{}{},
		touchedDays:    map[string]struct{}{"2025-06-15|g1|e1": {}},
		revAgg:         map[string]*revRow{},
		clickBatchSize: 500,
	}
	// Simulate a PFCount failure (miniredis returns 0 for a missing key, so
	// instead Close is used to make Redis unreachable and trigger the error
	// path). A closed client stands in for the Redis error.
	bad := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	_ = bad.Close()
	w.rdb = bad

	_ = w.flush(context.Background())

	// The failure path should merge the day back into the shared map
	// (no loss, and no 0 row written).
	w.aggMu.Lock()
	_, remerged := w.touchedDays["2025-06-15|g1|e1"]
	w.aggMu.Unlock()
	if !remerged {
		t.Fatalf("expected daily key to be remerged for retry, but touchedDays is empty")
	}
}

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}
