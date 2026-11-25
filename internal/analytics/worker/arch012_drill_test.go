package worker

// ARCH-012 — crash-recovery fault drill for the analytics pipeline.
//
// Acceptance (todo.md): interrupt after Append/before Send, after Send/before
// ACK, and on requeue failure; compare source messages, the PEL, the dead letter,
// and the ClickHouse rows to prove there is no silent loss.
//
// This drill uses real Redis Streams (redis-server, or ARCH012_REDIS_ADDR) and
// real ClickHouse (ARCH012_CLICKHOUSE_DSN, with the analytics database and the
// events/payments tables created by 001_init.sql) to verify the closed loop of
// "crash -> restart -> reclaim redelivery -> reconciliation" at each of the
// three fault points. It calls t.Skip when no controllable environment is
// available (ARCH-012 boundary: needs a controllable Redis and ClickHouse);
// only local runs / CI with the environment execute it for real.
//
// Test-fixture game-side effects are not treated as real game recovery
// credentials: this only verifies the analytics pipeline's own "no silent loss"
// invariant (the source message must end up in exactly one of the events table,
// the dead-letter stream, or still PENDING) and does not assert game-side
// exactly-once (GAME-010 remains NOT RUN).

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// arch012Redis returns a real Redis addr. If ARCH012_REDIS_ADDR is set it is
// used (and must be reachable — a CI misconfiguration must fail, not skip);
// otherwise a redis-server is spawned on a free port. Skips only when no
// binary is available and no addr was provided (no controllable Redis).
func arch012Redis(t *testing.T) string {
	t.Helper()
	if addr := os.Getenv("ARCH012_REDIS_ADDR"); addr != "" {
		c := redis.NewClient(&redis.Options{Addr: addr})
		if err := c.Ping(context.Background()).Err(); err != nil {
			_ = c.Close()
			t.Fatalf("ARCH-012: ARCH012_REDIS_ADDR %s unreachable: %v", addr, err)
		}
		_ = c.Close()
		return addr
	}
	bin, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("ARCH-012: redis-server binary not found (set ARCH012_REDIS_ADDR to use a running Redis)")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("ARCH-012: no free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	cmd := exec.Command(bin, "--port", fmt.Sprint(port), "--save", "", "--appendonly", "no")
	if err := cmd.Start(); err != nil {
		t.Skipf("ARCH-012: redis-server start failed: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for i := 0; i < 40; i++ {
		c := redis.NewClient(&redis.Options{Addr: addr})
		if err := c.Ping(context.Background()).Err(); err == nil {
			_ = c.Close()
			return addr
		}
		_ = c.Close()
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("ARCH-012: redis-server did not become ready at %s", addr)
	return ""
}

// arch012CH opens a ClickHouse connection from ARCH012_CLICKHOUSE_DSN and
// verifies the analytics schema is reachable. Skips only when the env is unset
// (no ClickHouse requested); a set-but-unreachable DSN fails (CI must surface
// a down ClickHouse, not silently pass).
func arch012CH(t *testing.T) clickhouse.Conn {
	t.Helper()
	dsn := os.Getenv("ARCH012_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("ARCH-012: ARCH012_CLICKHOUSE_DSN not set (needs controllable ClickHouse with analytics.events/payments)")
	}
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ARCH-012: bad DSN: %v", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		t.Fatalf("ARCH-012: open CH: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM analytics.events").Scan(&n); err != nil {
		t.Fatalf("ARCH-012: CH unreachable or analytics.events missing (need initdb 001_init.sql): %v", err)
	}
	return conn
}

// arch012Strs holds the per-test Redis stream names (unique so a shared
// ARCH012_REDIS_ADDR cannot contaminate scenarios).
type arch012Strs struct {
	events, payments, deadEvents, deadPayments, group, consumer string
}

func arch012Streams(t *testing.T, suffix string) arch012Strs {
	// Stream names must be unique per run: a shared ARCH012_REDIS_ADDR persists
	// streams/groups across runs, and a retry-copy left unclaimed by a prior run
	// would be the next XReadGroup(">") delivery, breaking the claim-is-the-XADD
	// assertion. Same rationale as the per-run uuid event IDs.
	base := "arch012:" + strings.ToLower(strings.ReplaceAll(suffix, "/", "-")) + ":" + uuid.NewString()[:8]
	return arch012Strs{
		events:       base + ":events",
		payments:     base + ":payments",
		deadEvents:   base + ":events:dead",
		deadPayments: base + ":payments:dead",
		group:        base + ":group",
		consumer:     base + ":consumer",
	}
}

// arch012Worker builds a Worker bound to the given real Redis+ClickHouse.
func arch012Worker(t *testing.T, redisAddr string, sts arch012Strs) *Worker {
	t.Helper()
	t.Setenv("REDIS_URL", "redis://"+redisAddr+"/0")
	t.Setenv("CLICKHOUSE_DSN", os.Getenv("ARCH012_CLICKHOUSE_DSN"))
	t.Setenv("ANALYTICS_REDIS_STREAM_EVENTS", sts.events)
	t.Setenv("ANALYTICS_REDIS_STREAM_PAYMENTS", sts.payments)
	t.Setenv("WORKER_GROUP", sts.group)
	t.Setenv("WORKER_CONSUMER", sts.consumer)
	t.Setenv("ANALYTICS_DEAD_EVENTS_STREAM", sts.deadEvents)
	t.Setenv("ANALYTICS_DEAD_PAYMENTS_STREAM", sts.deadPayments)
	t.Setenv("ANALYTICS_CHECKPOINT_PREFIX", sts.events+":cp")
	w, err := NewWorker()
	require.NoError(t, err)
	w.ensureGroups(context.Background())
	return w
}

// arch012Event is a valid event payload for the analytics.events table.
func arch012Event(ts, eventID string) map[string]any {
	return map[string]any{
		"ts": ts, "game_id": "arch012-game", "env": "dev",
		"event": "login", "user_id": "u1", "session_id": "s1",
		"channel": "direct", "platform": "linux", "country": "US",
		"event_id": eventID, "props": map[string]any{},
	}
}

// arch012XAdd writes one source message to the stream and returns its ID.
func arch012XAdd(ctx context.Context, t *testing.T, rdb *redis.Client, stream string, payload map[string]any) string {
	t.Helper()
	data, _ := json.Marshal(payload)
	id, err := rdb.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"data": string(data)}}).Result()
	require.NoError(t, err)
	return id
}

// arch012Claim reads the just-XADDed new message into the group's PEL
// (simulating the worker's XReadGroup with ">", which claims new messages).
func arch012Claim(ctx context.Context, t *testing.T, rdb *redis.Client, stream, group, consumer, id string) redis.XMessage {
	t.Helper()
	res, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: group, Consumer: consumer,
		Streams: []string{stream, ">"}, Count: 10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.NotEmpty(t, res[0].Messages)
	msg := res[0].Messages[0]
	require.Equal(t, id, msg.ID, "claimed message must be the one XADDed")
	return msg
}

// arch012PendingLen returns the number of PEL entries in a group.
func arch012PendingLen(ctx context.Context, t *testing.T, rdb *redis.Client, stream, group string) int {
	t.Helper()
	pend, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{Stream: stream, Group: group, Start: "-", End: "+", Count: 1000}).Result()
	require.NoError(t, err)
	return len(pend)
}

// arch012EventRows returns how many analytics.events rows carry the eventID.
func arch012EventRows(ctx context.Context, t *testing.T, conn clickhouse.Conn, eventID string) int {
	t.Helper()
	var n uint64
	err := conn.QueryRow(ctx, "SELECT count() FROM analytics.events WHERE event_id = ?", eventID).Scan(&n)
	require.NoError(t, err)
	return int(n)
}

// arch012Reclaim claims every PEL entry immediately (MinIdle=0) and reprocesses
// it, driving the same reclaim path the worker's periodic loop uses without
// waiting pendingIdleTimeout (a 5-minute const).
func arch012Reclaim(ctx context.Context, t *testing.T, w *Worker, rdb *redis.Client, stream string) {
	t.Helper()
	claimed, _, err := rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: stream, Group: w.group, Consumer: w.consumer,
		MinIdle: 0, Start: "-", Count: 1000,
	}).Result()
	require.NoError(t, err)
	for _, msg := range claimed {
		require.NoError(t, w.processMessage(ctx, stream, msg))
	}
}

// --- Fault point 1: crash after Append / before Send ---

// The message has been appended into the in-memory batch and the crash happens
// before the batch Send: the in-memory batch is lost and the message stays in
// the Redis PEL (deferred XACK). After a restart, reclaim redelivers ->
// re-append -> flush -> the ClickHouse row appears + ACK. No silent loss.
func TestDrill_AppendThenCrash_BeforeSend_ReclaimNoLoss(t *testing.T) {
	redisAddr := arch012Redis(t)
	conn := arch012CH(t)
	ctx := context.Background()
	sts := arch012Streams(t, t.Name())
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	eventID := uuid.NewString() // unique per run; the CH database persists across runs
	wA := arch012Worker(t, redisAddr, sts)

	// The source message enters the stream and is claimed into the PEL, then goes
	// through the real processMessage (deferred XACK).
	srcID := arch012XAdd(ctx, t, rdb, sts.events, arch012Event("2025-06-15 10:00:00", eventID))
	msg := arch012Claim(ctx, t, rdb, sts.events, sts.group, sts.consumer, srcID)
	require.NoError(t, wA.processMessage(ctx, sts.events, msg))

	// Not flushed: the message is in the PEL, ClickHouse has no row, the batch has not been Sent.
	require.Equal(t, 1, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "message must stay pending until batch Send")
	require.Equal(t, 0, arch012EventRows(ctx, t, conn, eventID), "no ClickHouse row before Send")

	// Crash: drop wA (the in-memory batch is lost); the message stays in the PEL.
	_ = wA

	// Restart: the new worker's reclaim redelivers -> re-append -> flush -> CH row + ACK.
	wB := arch012Worker(t, redisAddr, sts)
	arch012Reclaim(ctx, t, wB, rdb, sts.events)
	require.NoError(t, wB.flushBatches(ctx))

	require.Equal(t, 1, arch012EventRows(ctx, t, conn, eventID), "row must land after reclaim (no silent loss)")
	require.Equal(t, 0, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "PEL drained after reclaim+flush")
	require.Zero(t, rdb.XLen(ctx, sts.deadEvents).Val(), "no dead-letter on the crash path")
}

// --- Fault point 2: crash after Send / before ACK ---

// The batch Send already succeeded (the row landed in ClickHouse) but the ACK
// was not executed when the crash happens: the message is still in the PEL.
// After a restart, reclaim redelivers -> re-append -> Send again -> ClickHouse
// may gain an extra row (same event_id); the consumer dedups by event_id, so
// the message is not lost. The PEL drains and there is no dead letter.
func TestDrill_SendThenCrash_BeforeAck_ReclaimNoLoss(t *testing.T) {
	redisAddr := arch012Redis(t)
	conn := arch012CH(t)
	ctx := context.Background()
	sts := arch012Streams(t, t.Name())
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	eventID := uuid.NewString() // unique per run; the CH database persists across runs
	wA := arch012Worker(t, redisAddr, sts)

	srcID := arch012XAdd(ctx, t, rdb, sts.events, arch012Event("2025-06-15 10:05:00", eventID))
	msg := arch012Claim(ctx, t, rdb, sts.events, sts.group, sts.consumer, srcID)
	require.NoError(t, wA.processMessage(ctx, sts.events, msg))

	// Send succeeded (the row landed) but ackSent was not executed: the message stays in the PEL.
	require.NoError(t, wA.eventBatch.Send())
	require.Equal(t, 1, arch012EventRows(ctx, t, conn, eventID), "Send must land the row")
	require.Equal(t, 1, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "message pending (ACK lost)")

	// Crash: drop wA (the batch was already consumed).
	_ = wA

	// Restart: reclaim redelivers -> re-append -> Send again -> row (possibly duplicated, event_id dedup) + ACK.
	wB := arch012Worker(t, redisAddr, sts)
	arch012Reclaim(ctx, t, wB, rdb, sts.events)
	require.NoError(t, wB.flushBatches(ctx))

	rows := arch012EventRows(ctx, t, conn, eventID)
	require.GreaterOrEqual(t, rows, 1, "row must survive (no silent loss)")
	t.Logf("ARCH-012: event_id %s rows = %d (>=1; duplicates allowed, consumer dedups by event_id)", eventID, rows)
	require.Equal(t, 0, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "PEL drained after reclaim+flush")
	require.Zero(t, rdb.XLen(ctx, sts.deadEvents).Val(), "no dead-letter on the crash path")
}

// --- Fault point 3: requeue failure ---

// processMessage fails to process (the row is rejected by ClickHouse) and then
// the requeue XAdd fails: the message must not be ACKed and must stay in the
// PEL for reclaim to retry (otherwise it is silently lost). Here, using the
// real Redis PEL/stream state, the requeue failure is injected by swapping the
// worker's rdb for a client that rejects XAdd.
func TestDrill_RequeueFailure_MessageStaysPending(t *testing.T) {
	redisAddr := arch012Redis(t)
	conn := arch012CH(t)
	ctx := context.Background()
	sts := arch012Streams(t, t.Name())
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	wA := arch012Worker(t, redisAddr, sts)

	// The source message carries an invalid timestamp -> insertEvent's
	// batch.Append is rejected by ClickHouse -> processMessage enters the retry
	// branch -> requeue.
	eventID := uuid.NewString() // unique per run; the CH database persists across runs
	srcID := arch012XAdd(ctx, t, rdb, sts.events, arch012Event("not-a-date", eventID))
	msg := arch012Claim(ctx, t, rdb, sts.events, sts.group, sts.consumer, srcID)

	// Inject the requeue failure: swap rdb for a client that rejects XAdd.
	wA.rdb = deadRedisClientV9()
	err := wA.processMessage(ctx, sts.events, msg)
	require.Error(t, err, "requeue failure must surface (message not acked)")

	// Invariant: the message stays in the real PEL, no ClickHouse row, no dead
	// letter — it was not silently dropped.
	require.Equal(t, 1, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "message must stay pending on requeue failure")
	require.Equal(t, 0, arch012EventRows(ctx, t, conn, eventID), "no row for rejected payload")
	require.Zero(t, rdb.XLen(ctx, sts.deadEvents).Val(), "not dead-lettered (retryable)")

	// Recovery: reclaim against the real Redis can pick it back up (no more
	// silent loss once the requeue succeeds).
	wA.rdb = rdb
	wB := arch012Worker(t, redisAddr, sts)
	arch012Reclaim(ctx, t, wB, rdb, sts.events)
	// The invalid payload is rejected again -> the requeue succeeds -> the source
	// is ACKed + a retry copy enters the stream; nothing is lost in the end.
	require.Zero(t, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "reclaim recovered the pending message")
}
