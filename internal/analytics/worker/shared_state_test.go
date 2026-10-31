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

// recordingConn 是并发安全的 mock ClickHouse 连接：每次 PrepareBatch 返回
// 新 batch，Send 时把 daily_revenue 的行记录到 conn 统计，供 DATA-014 的
// 「固定样本核对总量」验证（已 flush + revAgg 剩余 = 总写入）。
type recordingConn struct {
	mu      sync.Mutex
	revSent uint64 // 已 Send 的 revenue_cents 累计
	refunds uint64
	failed  uint64
	dailyN  int // daily_revenue 行数
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
	// daily_revenue 行格式：(d, game_id, env, revenue_cents, refunds_cents, failed, version)
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

// TestSharedState_ConcurrentRevAggNoLoss 验证 DATA-014 的「固定样本核对总量」：
// 并发 commitRevenue（写 revAgg）与 flush（读+删）下，已发送 revenue 与
// revAgg 剩余之和恒等于总写入，不丢不重。
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
	const total = writers * perWriter // 每笔 success 100 分 → 期望 revenue = total*100

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
	// 并发 flush 读+删，验证同步所有权下不丢不重。
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

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}
