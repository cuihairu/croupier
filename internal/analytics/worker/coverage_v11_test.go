package worker

// worker.go 补测：asRetryCount 三型分支（int/json.Number/坏 json.Number）
// 与缺键兜底、revenueDeltaFromPaymentArgs 全校验臂（长度/时间戳/游戏/env/
// 金额/状态类型与未知状态）与三状态臂（success/refunded/failed 大小写归一）、
// remergeTouched{Minute,Day} 已重建键幂等、remergeRev 累加合并、
// commitRevenue 的 nil rev 跳过、processMessage 重试耗尽且死信投递失败的
// 「不 ack 交还 reclaim 重试」分支。
//
// 诚实跳过（不可稳定触发）：Run 停机路径的 reclaim goroutine 超时告警
// （264-265，需 reclaim 真实慢于 shutdownTimeout 的调度时序）与
// skipFinalFlush 返回（286，取消竞速）；Run 消费循环 241 属 miniredis 不
// 支持 XGROUP SETID 的 harness 限制（coverage_v9_test.go:374 注释同源）。

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

// pfFailHook 只对 hll:new 键的 pfcount 强制失败（dau 键照常成功）。
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
	// 坏 json.Number → Int64 失败 → 兜底 0（每次重试按 0 处理，不死循环）
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

	// refunded / failed 状态臂（failed 走大小写归一）
	_, d, ok := revenueDeltaFromPaymentArgs(base("refunded"))
	require.True(t, ok)
	require.Equal(t, uint64(500), d.refunds)
	_, d, ok = revenueDeltaFromPaymentArgs(base("FAILED"))
	require.True(t, ok)
	require.Equal(t, uint64(1), d.failed)

	// 校验臂：长度/类型逐项不合法
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

	// 已重建键幂等（写方新值不被快照旧值覆盖）
	w.remergeTouchedMinute("m1")
	w.remergeTouchedMinute("m1")
	w.remergeTouchedDay("d1")
	w.remergeTouchedDay("d1")
	require.Len(t, w.touchedMinutes, 1)
	require.Len(t, w.touchedDays, 1)

	// rev 累加合并（失败重试不丢增量）
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

	// retry_count 已耗尽 + 死信投递失败（redis 不可达）→ 返回死信错误、
	// 不 ack：reclaim 侧稍后重试死信投递（:1054-1055）
	msg := redis.XMessage{ID: "1-1", Values: map[string]interface{}{
		"data":        `{"ts":"2026-11-09T10:00:00Z","game_id":"demo","env":"prod","user_id":"u1","retry_count":5}`,
		"retry_count": float64(5),
	}}
	err := w.processMessage(context.Background(), "events", msg)
	require.ErrorContains(t, err, "connection refused",
		"dead-letter send failure must be returned without ack")

	// miniredis 健在时同型消息 → 死信投递成功后 ack（对照组）
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
			"hll:online:g1:prod:" + future: {}, // 当前/未来分钟 → 保留
			"hll:online:g1:prod:gARBAGE":   {}, // 坏时间串 → 丢弃
			"junk":                         {}, // 段数不足 → 丢弃
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

	// hll:new 计数失败不得以 0 覆写当日 → day 键合并回共享 map 重试
	w.aggMu.Lock()
	_, remerged := w.touchedDays["2025-06-15|g1|e1"]
	w.aggMu.Unlock()
	require.True(t, remerged, "new-users pfcount failure must remerge the day key")
}

// addPendingInvalidMessageV11 造一条无效 JSON 的 pending 消息（PEL 内、
// 已按 miniredis 时钟拨快超过 pendingIdleTimeout）。
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
	// 无效 JSON → 死信投递成功 → ackImmediate 被 hook 强制失败 →
	// processMessage 返回错误 → 认领循环与孤儿循环都走告警分支
	w.rdb.AddHook(hookV10{failAck: true})
	addPendingInvalidMessageV11(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	// ack 失败 → 消息仍在 PEL，等待下一轮 reclaim
	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), p.Count)
}

func TestReclaimOrphanedProcessErrorWarns(t *testing.T) {
	w, mr := testWorker(t)
	// XPendingExt 失败 → 按消费者循环跳过；孤儿循环（XAutoClaim 0-0）认领后
	// processMessage 出错（无效 JSON + ack 强制失败）→ 孤儿告警分支
	w.rdb.AddHook(hookV10{failAck: true, failPending: true})
	addPendingInvalidMessageV11(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), p.Count)
}
