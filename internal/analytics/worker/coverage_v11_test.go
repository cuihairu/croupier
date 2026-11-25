package worker

// Supplemental coverage for worker.go: asRetryCount's three type branches
// (int/json.Number/bad json.Number) and the missing-key fallback, all of
// revenueDeltaFromPaymentArgs's validation arms (length/timestamp/game/env/
// amount/status type and unknown status) plus its three status arms (success/
// refunded/failed, case-normalized), remergeTouched{Minute,Day} idempotence for
// already-rebuilt keys, remergeRev's accumulative merge, commitRevenue's nil
// rev skip, and processMessage's "do not ack, hand back to reclaim for retry"
// branch on retry exhaustion with a failed dead-letter delivery.
//
// Honest skips (not stably triggerable): the Run shutdown path's reclaim
// goroutine timeout warning (264-265, requires reclaim genuinely slower than
// shutdownTimeout scheduling) and the skipFinalFlush return (286, a cancel race);
// the Run consume loop at 241 is a harness limitation of miniredis not
// supporting XGROUP SETID (same origin as the coverage_v9_test.go:374 comment).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// pfFailHook forces pfcount to fail only for hll:new keys (dau keys still succeed).
type pfFailHook struct{ err error }

func (h pfFailHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h pfFailHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h pfFailHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "pfcount" && strings.Contains(fmt.Sprint(cmd.Args()), "hll:new") {
			return h.err
		}
		return next(ctx, cmd)
	}
}

func TestAsRetryCountTypeArms(t *testing.T) {
	require.Equal(t, 2, asRetryCount(map[string]any{"retry_count": float64(2)}))
	require.Equal(t, 5, asRetryCount(map[string]any{"retry_count": 5}))
	require.Equal(t, 7, asRetryCount(map[string]any{"retry_count": json.Number("7")}))
	// Bad json.Number -> Int64 fails -> fall back to 0 (each retry is treated as
	// 0, so there is no infinite loop)
	require.Equal(t, 0, asRetryCount(map[string]any{"retry_count": json.Number("abc")}))
	require.Equal(t, 0, asRetryCount(map[string]any{"other": 1}))
	require.Equal(t, 0, asRetryCount(nil))
}

func TestRevenueDeltaFromPaymentArgsArms(t *testing.T) {
	base := func(status any) []any {
		return []any{"2026-11-09T10:00:00Z", "demo", "prod", nil, nil, uint64(500), nil, status, nil}
	}

	key, delta, ok := revenueDeltaFromPaymentArgs(base("success"))
	require.True(t, ok)
	require.Equal(t, "2026-11-09|demo|prod", key)
	require.Equal(t, uint64(500), delta.revenue)

	// refunded / failed status arms (failed goes through case normalization)
	_, d, ok := revenueDeltaFromPaymentArgs(base("refunded"))
	require.True(t, ok)
	require.Equal(t, uint64(500), d.refunds)
	_, d, ok = revenueDeltaFromPaymentArgs(base("FAILED"))
	require.True(t, ok)
	require.Equal(t, uint64(1), d.failed)

	// Validation arms: length/type individually invalid
	_, _, ok = revenueDeltaFromPaymentArgs(base("success")[:8])
	require.False(t, ok, "short args must be rejected")

	mut := func(i int, v any) []any {
		args := base("success")
		args[i] = v
		return args
	}
	_, _, ok = revenueDeltaFromPaymentArgs(mut(0, 42))
	require.False(t, ok, "non-string ts must be rejected")
	_, _, ok = revenueDeltaFromPaymentArgs(mut(0, "2026"))
	require.False(t, ok, "short ts must be rejected")
	_, _, ok = revenueDeltaFromPaymentArgs(mut(1, 42))
	require.False(t, ok, "non-string game must be rejected")
	_, _, ok = revenueDeltaFromPaymentArgs(mut(2, 42))
	require.False(t, ok, "non-string env must be rejected")
	_, _, ok = revenueDeltaFromPaymentArgs(mut(5, "500"))
	require.False(t, ok, "non-uint64 amount must be rejected")
	_, _, ok = revenueDeltaFromPaymentArgs(mut(7, 42))
	require.False(t, ok, "non-string status must be rejected")
	_, _, ok = revenueDeltaFromPaymentArgs(base("pending"))
	require.False(t, ok, "unknown status must be rejected")
}

func TestRemergeFamilyKeepsWriterState(t *testing.T) {
	w := &Worker{
		touchedMinutes: map[string]struct{}{},
		touchedDays:    map[string]struct{}{},
		revAgg:         map[string]*revRow{},
	}

	// Idempotence for already-rebuilt keys (the writer's new value is not
	// overwritten by a stale snapshot value)
	w.remergeTouchedMinute("m1")
	w.remergeTouchedMinute("m1")
	w.remergeTouchedDay("d1")
	w.remergeTouchedDay("d1")
	require.Len(t, w.touchedMinutes, 1)
	require.Len(t, w.touchedDays, 1)

	// rev accumulative merge (a failed retry does not lose the delta)
	w.remergeRev("r1", &revRow{revenue: 10})
	w.remergeRev("r1", &revRow{revenue: 5, refunds: 2, failed: 1})
	cur := w.revAgg["r1"]
	require.Equal(t, uint64(15), cur.revenue)
	require.Equal(t, uint64(2), cur.refunds)
	require.Equal(t, uint64(1), cur.failed)
}

func TestCommitRevenueSkipsNilRev(t *testing.T) {
	w := &Worker{revAgg: map[string]*revRow{}}
	w.commitRevenue([]ackRef{
		{stream: "payments", id: "1-1", revKey: "k", rev: nil},
		{stream: "payments", id: "1-2", revKey: "k", rev: &revRow{revenue: 3}},
	})
	require.Equal(t, uint64(3), w.revAgg["k"].revenue)
}

func TestProcessMessageDeadLetterSendFailNoAck(t *testing.T) {
	stub := &stubConnV9{mockConn: &mockConn{}, prepareErr: errors.New("ch down")}
	w := &Worker{
		rdb:                deadRedisClientV9(),
		ch:                 stub,
		streamEvents:       "events",
		streamPayments:     "payments",
		deadEventsStream:   "events:dead",
		deadPaymentsStream: "payments:dead",
		touchedMinutes:     map[string]struct{}{},
		touchedDays:        map[string]struct{}{},
		revAgg:             map[string]*revRow{},
		lastIDs:            map[string]string{},
	}

	// retry_count exhausted + dead-letter delivery failed (redis unreachable) ->
	// return the dead-letter error and do not ack: the reclaim side retries the
	// dead-letter delivery later (:1054-1055)
	msg := redis.XMessage{ID: "1-1", Values: map[string]interface{}{
		"data":        `{"ts":"2026-11-09T10:00:00Z","game_id":"demo","env":"prod","user_id":"u1","retry_count":5}`,
		"retry_count": float64(5),
	}}
	err := w.processMessage(context.Background(), "events", msg)
	require.ErrorContains(t, err, "connection refused",
		"dead-letter send failure must be returned without ack")

	// With a healthy miniredis the same message -> acked after a successful
	// dead-letter delivery (control group)
	mr := miniredis.RunT(t)
	w.rdb = redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	err = w.processMessage(context.Background(), "events", msg)
	require.NoError(t, err)
}

func TestFlushMinuteKeyShapeGuards(t *testing.T) {
	mr := miniredis.RunT(t)
	future := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Minute).Format("200601021504")
	w := &Worker{
		rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		ch:  &mockConn{},
		touchedMinutes: map[string]struct{}{
			"hll:online:g1:prod:" + future: {}, // current/future minute -> kept
			"hll:online:g1:prod:gARBAGE":   {}, // bad time string -> dropped
			"junk":                         {}, // too few segments -> dropped
		},
		touchedDays:    map[string]struct{}{},
		revAgg:         map[string]*revRow{},
		clickBatchSize: 500,
	}
	_ = w.flush(context.Background())

	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	require.Len(t, w.touchedMinutes, 1, "only the future-minute key may be kept")
	_, kept := w.touchedMinutes["hll:online:g1:prod:"+future]
	require.True(t, kept, "future-minute key must wait for its turn")
}

func TestFlushDailyUsersNewPFCountFailRemerges(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdb.AddHook(pfFailHook{err: errors.New("pfcount new down")})
	w := &Worker{
		rdb:            rdb,
		ch:             &mockConn{},
		touchedMinutes: map[string]struct{}{},
		touchedDays:    map[string]struct{}{"2025-06-15|g1|e1": {}},
		revAgg:         map[string]*revRow{},
		clickBatchSize: 500,
	}
	_ = w.flush(context.Background())

	// A failed hll:new count must not overwrite today's value with 0 -> the day
	// key is merged back into the shared map for a retry
	w.aggMu.Lock()
	_, remerged := w.touchedDays["2025-06-15|g1|e1"]
	w.aggMu.Unlock()
	require.True(t, remerged, "new-users pfcount failure must remerge the day key")
}

// addPendingInvalidMessageV11 creates a pending message with invalid JSON (in the
// PEL, with the miniredis clock already advanced past pendingIdleTimeout).
func addPendingInvalidMessageV11(t *testing.T, w *Worker, mr *miniredis.Miniredis) {
	t.Helper()
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{invalid-json`,
	}})
	w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: "stale-consumer",
		Streams: []string{w.streamEvents, ">"}, Count: 10,
	})
	mr.SetTime(time.Now().Add(10 * time.Minute))
}

func TestReclaimProcessErrorWarnsClaimedAndOrphaned(t *testing.T) {
	w, mr := testWorker(t)
	// Invalid JSON -> dead-letter delivery succeeds -> ackImmediate is forced to
	// fail by the hook -> processMessage returns an error -> both the claim loop
	// and the orphan loop take the warning branch
	w.rdb.AddHook(hookV10{failAck: true})
	addPendingInvalidMessageV11(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	// The ack failed -> the message is still in the PEL, waiting for the next reclaim round
	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), p.Count)
}

func TestReclaimOrphanedProcessErrorWarns(t *testing.T) {
	w, mr := testWorker(t)
	// XPendingExt fails -> the per-consumer loop is skipped; the orphan loop
	// (XAutoClaim 0-0) claims the message and then processMessage errors (invalid
	// JSON + ack forced to fail) -> the orphan warning branch
	w.rdb.AddHook(hookV10{failAck: true, failPending: true})
	addPendingInvalidMessageV11(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), p.Count)
}
