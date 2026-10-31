package worker

// ARCH-012 — 分析管道崩溃恢复故障钻机。
//
// 验收（todo.md）：在 Append 后/Send 前、Send 后/ACK 前、重入队失败时中断；
// 比对源消息、PEL、死信与 ClickHouse 行，证实无静默丢失。
//
// 本钻机用真实 Redis Streams（redis-server，或 ARCH012_REDIS_ADDR）与真实
// ClickHouse（ARCH012_CLICKHOUSE_DSN，需带 analytics 库与 001_init.sql 建出的
// events/payments 表）验证三条故障点下的「崩溃 → 重启 → reclaim 重投 →
// 对账」闭环。缺可控环境时 t.Skip（ARCH-012 边界：需 Redis 与 ClickHouse
// 可控环境）；本地/带环境的 CI 才实际执行。
//
// 不把测试 fixture 的游戏副作用结果当真实游戏恢复凭据：这里只核对分析管道
// 自身「无静默丢失」不变量（源消息最终落在 events 表 / 死信流 / 仍 PENDING 三
// 者之一），不断言游戏侧 exactly-once（GAME-010 仍 NOT RUN）。

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

// --- 故障点 1：Append 后 / Send 前崩溃 ---

// 消息被 append 进内存批次后、批次 Send 前崩溃：内存批次丢失，消息保持在
// Redis PEL（deferred XACK）。重启后 reclaim 重投 → 重新 append → flush →
// ClickHouse 行出现 + ACK。无静默丢失。
func TestDrill_AppendThenCrash_BeforeSend_ReclaimNoLoss(t *testing.T) {
	redisAddr := arch012Redis(t)
	conn := arch012CH(t)
	ctx := context.Background()
	sts := arch012Streams(t, t.Name())
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	eventID := uuid.NewString() // unique per run; the CH database persists across runs
	wA := arch012Worker(t, redisAddr, sts)

	// 源消息进流并 claim 进 PEL，然后走真实 processMessage（deferred XACK）。
	srcID := arch012XAdd(ctx, t, rdb, sts.events, arch012Event("2025-06-15 10:00:00", eventID))
	msg := arch012Claim(ctx, t, rdb, sts.events, sts.group, sts.consumer, srcID)
	require.NoError(t, wA.processMessage(ctx, sts.events, msg))

	// 未 flush：消息在 PEL、ClickHouse 无行、批次未 Send。
	require.Equal(t, 1, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "message must stay pending until batch Send")
	require.Equal(t, 0, arch012EventRows(ctx, t, conn, eventID), "no ClickHouse row before Send")

	// 崩溃：丢弃 wA（内存批次丢失），消息留在 PEL。
	_ = wA

	// 重启：新 worker reclaim 重投 → 重新 append → flush → CH 行 + ACK。
	wB := arch012Worker(t, redisAddr, sts)
	arch012Reclaim(ctx, t, wB, rdb, sts.events)
	require.NoError(t, wB.flushBatches(ctx))

	require.Equal(t, 1, arch012EventRows(ctx, t, conn, eventID), "row must land after reclaim (no silent loss)")
	require.Equal(t, 0, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "PEL drained after reclaim+flush")
	require.Zero(t, rdb.XLen(ctx, sts.deadEvents).Val(), "no dead-letter on the crash path")
}

// --- 故障点 2：Send 后 / ACK 前崩溃 ---

// 批次 Send 已成功（行落 ClickHouse）但 ACK 未执行时崩溃：消息仍在 PEL。
// 重启后 reclaim 重投 → 重新 append → 再次 Send → ClickHouse 可能多一行
// （同一 event_id），消费端以 event_id 去重；消息不丢。PEL 清空、无死信。
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

	// Send 成功（行已落库）但 ackSent 未执行：消息仍在 PEL。
	require.NoError(t, wA.eventBatch.Send())
	require.Equal(t, 1, arch012EventRows(ctx, t, conn, eventID), "Send must land the row")
	require.Equal(t, 1, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "message pending (ACK lost)")

	// 崩溃：丢弃 wA（批次已消费）。
	_ = wA

	// 重启：reclaim 重投 → 重新 append → 再次 Send → 行（可能重复，event_id 去重）+ ACK。
	wB := arch012Worker(t, redisAddr, sts)
	arch012Reclaim(ctx, t, wB, rdb, sts.events)
	require.NoError(t, wB.flushBatches(ctx))

	rows := arch012EventRows(ctx, t, conn, eventID)
	require.GreaterOrEqual(t, rows, 1, "row must survive (no silent loss)")
	t.Logf("ARCH-012: event_id %s rows = %d (>=1; duplicates allowed, consumer dedups by event_id)", eventID, rows)
	require.Equal(t, 0, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "PEL drained after reclaim+flush")
	require.Zero(t, rdb.XLen(ctx, sts.deadEvents).Val(), "no dead-letter on the crash path")
}

// --- 故障点 3：重入队失败 ---

// processMessage 处理失败（行被 ClickHouse 拒绝）后重入队 XAdd 失败：消息
// 不得被 ACK，必须留在 PEL 供 reclaim 重试（否则静默丢失）。这里用真实
// Redis 的 PEL/流态，把 worker 的 rdb 换成拒绝 XAdd 的客户端注入重入队失败。
func TestDrill_RequeueFailure_MessageStaysPending(t *testing.T) {
	redisAddr := arch012Redis(t)
	conn := arch012CH(t)
	ctx := context.Background()
	sts := arch012Streams(t, t.Name())
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	wA := arch012Worker(t, redisAddr, sts)

	// 源消息带非法时间戳 → insertEvent 的 batch.Append 被 ClickHouse 拒绝
	// → processMessage 进入重试分支 → 重入队。
	eventID := uuid.NewString() // unique per run; the CH database persists across runs
	srcID := arch012XAdd(ctx, t, rdb, sts.events, arch012Event("not-a-date", eventID))
	msg := arch012Claim(ctx, t, rdb, sts.events, sts.group, sts.consumer, srcID)

	// 注入重入队失败：换 rdb 为拒绝 XAdd 的客户端。
	wA.rdb = deadRedisClientV9()
	err := wA.processMessage(ctx, sts.events, msg)
	require.Error(t, err, "requeue failure must surface (message not acked)")

	// 不变量：消息留在真实 PEL、无 ClickHouse 行、无死信 —— 未被静默丢弃。
	require.Equal(t, 1, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "message must stay pending on requeue failure")
	require.Equal(t, 0, arch012EventRows(ctx, t, conn, eventID), "no row for rejected payload")
	require.Zero(t, rdb.XLen(ctx, sts.deadEvents).Val(), "not dead-lettered (retryable)")

	// 恢复：真实 redis 的 reclaim 能取回它（重入队成功后不再静默丢失）。
	wA.rdb = rdb
	wB := arch012Worker(t, redisAddr, sts)
	arch012Reclaim(ctx, t, wB, rdb, sts.events)
	// 非法 payload 会再次被拒 → 重入队成功 → 源 ACK + retry 副本入流，最终不丢。
	require.Zero(t, arch012PendingLen(ctx, t, rdb, sts.events, sts.group), "reclaim recovered the pending message")
}
