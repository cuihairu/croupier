package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Recording ClickHouse conn: captures every appended+sent row with its table,
// so the tests can assert exactly what the flush wrote (values AND versions).
// ---------------------------------------------------------------------------

type revSentRow struct {
	query string
	vals  []any
}

type revRecordingConn struct {
	sent     []revSentRow
	failSend bool
}

func (c *revRecordingConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &revRecordingBatch{conn: c, query: query}, nil
}
func (c *revRecordingConn) Contributors() []string                        { return nil }
func (c *revRecordingConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }
func (c *revRecordingConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return nil
}
func (c *revRecordingConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, nil
}
func (c *revRecordingConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return nil
}
func (c *revRecordingConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (c *revRecordingConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (c *revRecordingConn) Ping(ctx context.Context) error { return nil }
func (c *revRecordingConn) Stats() driver.Stats            { return driver.Stats{} }
func (c *revRecordingConn) Close() error                   { return nil }

func (c *revRecordingConn) rowsFor(table string) []revSentRow {
	var out []revSentRow
	for _, r := range c.sent {
		if strings.Contains(r.query, table) {
			out = append(out, r)
		}
	}
	return out
}

type revRecordingBatch struct {
	conn  *revRecordingConn
	query string
	rows  [][]any
}

func (b *revRecordingBatch) Append(v ...any) error { b.rows = append(b.rows, v); return nil }
func (b *revRecordingBatch) Send() error {
	if b.conn.failSend {
		return errors.New("clickhouse down")
	}
	for _, row := range b.rows {
		b.conn.sent = append(b.conn.sent, revSentRow{query: b.query, vals: row})
	}
	return nil
}
func (b *revRecordingBatch) Abort() error                  { return nil }
func (b *revRecordingBatch) AppendStruct(v any) error      { return nil }
func (b *revRecordingBatch) Column(int) driver.BatchColumn { return nil }
func (b *revRecordingBatch) Flush() error                  { return nil }
func (b *revRecordingBatch) IsSent() bool                  { return false }
func (b *revRecordingBatch) Rows() int                     { return len(b.rows) }
func (b *revRecordingBatch) Columns() []column.Interface   { return nil }
func (b *revRecordingBatch) Close() error                  { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const data021PaymentPayload = `{"ts":"2025-06-15T10:30:00Z","game_id":"g1","env":"e1","status":"success","amount_cents":100,"order_id":"o1"}`

const data021RevKey = "2025-06-15|g1|e1"

// data021Revenue accumulates the daily_revenue rows across a set of recording
// conns (the ReplacingMergeTree FINAL view: the surviving row per (d,game,env)
// is the one with the greatest version).
func data021Final(conn *revRecordingConn) (rev float64, ver uint64, found bool) {
	for _, r := range conn.rowsFor("daily_revenue") {
		// (d, game_id, env, revenue_cents, refunds_cents, failed, version)
		v, _ := r.vals[6].(uint64)
		if !found || v > ver {
			rev, ver, found = float64(r.vals[3].(uint64)), v, true
		}
	}
	return
}

// data021DirtyRevenue reports the "r:"-prefixed dirty members.
func data021DirtyRevenue(t *testing.T, w *Worker) []string {
	t.Helper()
	members, err := w.rdb.SMembers(context.Background(), w.aggDirtySet).Result()
	require.NoError(t, err)
	var out []string
	for _, m := range members {
		if len(m) > 2 && m[:2] == "r:" {
			out = append(out, m)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Acceptance (DATA-021): the payment detail row is Send()ed and its message
// ACKed, the process dies before the daily_revenue insert — the restarted
// worker restores the owed cumulative total from Redis and writes it, exactly
// once.
// ---------------------------------------------------------------------------

func TestRevenueRecovery_CrashAfterPaymentSendAndAck(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	ch := &revRecordingConn{}
	w.ch = ch
	w.clickBatchSize = 0

	// Payment delivered to the consumer (pending), appended, then the batch
	// flushes: detail row Send + durable advance + XACK. This is the state a
	// crash right after ackSent leaves behind.
	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	assert.Empty(t, mustPending(t, w, w.streamPayments), "the payment message is ACKed after the flush")
	assert.Empty(t, ch.rowsFor("daily_revenue"), "the process dies before the daily_revenue insert")

	// The owed total is durable in Redis: cumulative progress + dirty marker.
	h, err := w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	require.NoError(t, err)
	assert.Equal(t, "100", h[data021RevKey+":rev"], "cumulative progress advanced")
	require.Len(t, data021DirtyRevenue(t, w), 1, "the owed daily_revenue flush is durably marked")

	// Restart: a brand-new worker on the same Redis, empty in-memory state.
	w2 := testWorkerOn(t, mr)
	ch2 := &revRecordingConn{}
	w2.ch = ch2
	w2.ensureGroups(ctx)
	rv := w2.revAgg[data021RevKey]
	require.NotNil(t, rv, "restart must restore the owed revenue total")
	assert.EqualValues(t, 100, rv.revenue)
	assert.Greater(t, rv.version, uint64(1_000_000_000), "version is seeded above legacy wall-clock versions")

	// The recovery flush writes the cumulative total.
	require.NoError(t, w2.flush(ctx))
	rows := ch2.rowsFor("daily_revenue")
	require.Len(t, rows, 1, "exactly one daily_revenue row")
	assert.EqualValues(t, 100, rows[0].vals[3])
	assert.EqualValues(t, rv.version, rows[0].vals[6], "the row carries the durable version")
	assert.Empty(t, data021DirtyRevenue(t, w2), "marker cleared after the row is durable")
}

// The scenario the durable version exists for: a crash between the
// daily_revenue Send and the marker clear replays the SAME cumulative total
// after restart. The replayed row must carry the same values (never
// 100+100), so ReplacingMergeTree FINAL collapses the duplicates.
func TestRevenueRecovery_ReplayedCumulativeFlushDoesNotDoubleCount(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	ch := &revRecordingConn{}
	w.ch = ch
	w.clickBatchSize = 0

	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	require.NoError(t, w.flush(ctx))
	rev, ver, found := data021Final(ch)
	require.True(t, found)
	require.EqualValues(t, 100, rev)

	// Crash between the Send and clearRevenueProgress: the row is in
	// ClickHouse but the dirty marker survived. Re-adding the marker is
	// exactly the state the restarted worker observes.
	require.NoError(t, w.rdb.SAdd(ctx, w.aggDirtySet, aggDirtyRevenuePrefix+data021RevKey).Err())

	w2 := testWorkerOn(t, mr)
	ch2 := &revRecordingConn{}
	w2.ch = ch2
	w2.ensureGroups(ctx)
	require.NoError(t, w2.flush(ctx))
	rev2, ver2, found2 := data021Final(ch2)
	require.True(t, found2)
	// The FINAL view still shows 100 — the replay wrote the same cumulative
	// total, never an increment on top of it.
	assert.EqualValues(t, 100, rev2, "replay of an already-flushed total must not double count")
	assert.GreaterOrEqual(t, ver2, ver, "the replayed row never regresses the version")
}

// New payments arriving after a restart fold into the restored cumulative
// total: the next flush carries 100+25, not just the new delta.
func TestRevenueRecovery_NewPaymentsAfterRestart(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	ch := &revRecordingConn{}
	w.ch = ch
	w.clickBatchSize = 0

	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	_, ver1, _ := data021Final(ch)
	_ = ver1

	w2 := testWorkerOn(t, mr)
	ch2 := &revRecordingConn{}
	w2.ch = ch2
	w2.clickBatchSize = 0
	w2.ensureGroups(ctx)

	const later = `{"ts":"2025-06-15T11:00:00Z","game_id":"g1","env":"e1","status":"success","amount_cents":25,"order_id":"o2"}`
	id2 := w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamPayments, Values: map[string]any{"data": later}}).Val()
	require.NoError(t, w2.processMessage(ctx, w.streamPayments, redis.XMessage{ID: id2, Values: map[string]any{"data": []byte(later)}}))
	require.NoError(t, w2.flushPaymentBatch(ctx))
	require.NoError(t, w2.flush(ctx))

	rev, ver2, found := data021Final(ch2)
	require.True(t, found)
	assert.EqualValues(t, 125, rev, "restored total plus the new payment")
	assert.Greater(t, ver2, uint64(1_000_000_000))
}

// A durable advance failure (Redis unavailable) must leave the source
// messages pending and advance nothing; the reclaim redelivery then advances
// the total exactly once.
func TestRevenueRecovery_AdvanceFailureLeavesPendingAndDedups(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	ch := &revRecordingConn{}
	w.ch = ch
	w.clickBatchSize = 0

	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))

	// Simulate Redis being down: a canceled context fails the EVAL before it
	// executes. Send itself is a mock and succeeds.
	failCtx, cancel := context.WithCancel(ctx)
	cancel()
	err := w.flushPaymentBatch(failCtx)
	require.Error(t, err, "the durable advance failure must surface")
	require.Len(t, mustPending(t, w, w.streamPayments), 1, "the message stays pending for reclaim")
	h, herr := w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	require.NoError(t, herr)
	assert.Empty(t, h, "nothing advanced on a failed round")
	assert.Empty(t, data021DirtyRevenue(t, w))

	// Reclaim redelivers the message; the retry advances the total exactly
	// once (the failed round left no half-state behind).
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	assert.Empty(t, mustPending(t, w, w.streamPayments))
	h, herr = w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	require.NoError(t, herr)
	assert.Equal(t, "100", h[data021RevKey+":rev"], "the total advanced exactly once across the retry")
}

// A reclaim redelivery of an already-ACKed... rather: an already-advanced
// message (seen marker present) must not advance the cumulative total again.
func TestRevenueRecovery_RedeliveryDedupedBySeenMarker(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	ch := &revRecordingConn{}
	w.ch = ch
	w.clickBatchSize = 0

	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	h, _ := w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	require.Equal(t, "100", h[data021RevKey+":rev"])

	// Same message again (reclaim redelivery racing the ACK): the seen
	// marker makes the second advance a no-op.
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	h, _ = w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	assert.Equal(t, "100", h[data021RevKey+":rev"], "redelivery must not double count")

	// A second flush on the same cumulative state writes nothing new.
	require.NoError(t, w.flush(ctx))
	require.NoError(t, w.flush(ctx))
	assert.Len(t, ch.rowsFor("daily_revenue"), 1, "repeat flushes add no rows")
}

// Healthy path: with no crash, a fresh worker restores nothing and pays no
// rebuild cost — the durable progress only exists for the owed window.
func TestRevenueRecovery_HealthyPathRestoresNothing(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	ch := &revRecordingConn{}
	w.ch = ch
	w.clickBatchSize = 0

	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	require.NoError(t, w.flush(ctx))
	assert.Empty(t, data021DirtyRevenue(t, w), "healthy flush leaves no owed revenue")

	w2 := testWorkerOn(t, mr)
	w2.ensureGroups(ctx)
	assert.Empty(t, w2.revAgg, "a fully flushed state restores nothing on restart")
}

// The progress hash keeps no per-message history beyond the reclaim window:
// seen markers expire, the cumulative totals persist (TTL refreshed on every
// advance).
func TestRevenueRecovery_SeenMarkerExpiryAndProgressTTL(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	w := testWorkerOn(t, mr)
	w.ch = &revRecordingConn{}
	w.revProgressSeenTTL = 50 * time.Millisecond
	ctx := context.Background()

	id := data053Pending(t, w, data021PaymentPayload)
	msg := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(data021PaymentPayload)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))

	assert.True(t, mr.Exists(revProgressSeenMarker(w.streamPayments, id)), "seen marker written")
	mr.FastForward(60 * time.Millisecond)
	assert.False(t, mr.Exists(revProgressSeenMarker(w.streamPayments, id)), "seen marker expires")

	ttl, terr := w.rdb.TTL(ctx, w.revProgressSet).Result()
	require.NoError(t, terr)
	assert.Greater(t, ttl, 100*24*time.Hour, "cumulative progress keeps a long TTL")

	// After the marker is gone the same message CAN advance again — that is
	// why the marker TTL must exceed the reclaim window (7d >> 5min idle).
	require.NoError(t, w.processMessage(ctx, w.streamPayments, msg))
	require.NoError(t, w.flushPaymentBatch(ctx))
	h, _ := w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	assert.Equal(t, "200", h[data021RevKey+":rev"], "post-expiry redelivery advances (documented boundary)")
}

// A corrupt progress hash (fields gone, marker left) must not be restored as
// zeros — writing them would overwrite real ClickHouse rows.
func TestRevenueRecovery_CorruptProgressNotRestored(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	require.NoError(t, w.rdb.SAdd(ctx, w.aggDirtySet, aggDirtyRevenuePrefix+data021RevKey).Err())

	w2 := testWorkerOn(t, mr)
	w2.ensureGroups(ctx)
	assert.Nil(t, w2.revAgg[data021RevKey], "missing progress fields must not restore zeros")
	assert.Len(t, data021DirtyRevenue(t, w2), 1, "the marker stays (inert) for observability")
}

// remergeRev: two durable rows are totals, not increments — the higher
// version wins wholesale and the stale snapshot is dropped; legacy
// versionless rows keep the additive merge.
func TestRemergeRev_VersionedRowsReplaceNotAdd(t *testing.T) {
	w := &Worker{revAgg: map[string]*revRow{}}
	// A newer advance landed while the flush was in flight: the shared map
	// holds v5/150, the failed snapshot was v3/125 (already contained).
	w.revAgg[data021RevKey] = &revRow{revenue: 150, version: 5}
	w.remergeRev(data021RevKey, &revRow{revenue: 125, version: 3})
	assert.EqualValues(t, 150, w.revAgg[data021RevKey].revenue, "stale snapshot must not add onto the newer total")
	assert.EqualValues(t, 5, w.revAgg[data021RevKey].version)

	// Reverse order: the snapshot is newer than what a concurrent legacy
	// write produced — the versioned snapshot replaces.
	w.revAgg[data021RevKey] = &revRow{revenue: 125, version: 3}
	w.remergeRev(data021RevKey, &revRow{revenue: 150, version: 5})
	assert.EqualValues(t, 150, w.revAgg[data021RevKey].revenue)

	// Legacy versionless rows keep the additive merge.
	w.revAgg[data021RevKey] = &revRow{revenue: 100}
	w.remergeRev(data021RevKey, &revRow{revenue: 50})
	assert.EqualValues(t, 150, w.revAgg[data021RevKey].revenue, "legacy increments still accumulate")
}
