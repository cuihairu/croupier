package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// V10 helpers: go-redis ProcessHook 注入，伪造服务端异常响应
// ---------------------------------------------------------------------------

// hookV10 按命令名注入故障：
//   - failAck:     XACK 恒失败（模拟 ack 异常，覆盖 Run/reclaim 内 processMessage 错误分支）
//   - okSetID:     XGROUP SETID 恒成功（miniredis 未实现该命令，伪造成功以覆盖 lastIDs 赋值）
//   - failPending: XPENDING EXT 恒失败
//   - failClaim:   XAUTOCLAIM 恒失败
//   - pfCountNeg:  PFCOUNT 恒返回 -5（伪造服务端异常值，覆盖 n<0 防御分支）
type hookV10 struct {
	failAck     bool
	okSetID     bool
	failPending bool
	failClaim   bool
	pfCountNeg  bool
}

var errForcedV10 = errors.New("forced failure for coverage")

func (h hookV10) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h hookV10) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h hookV10) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		switch cmd.Name() {
		case "xack":
			if h.failAck {
				return errForcedV10
			}
		case "pfcount":
			if c, ok := cmd.(*redis.IntCmd); ok && h.pfCountNeg {
				c.SetVal(-5)
				return nil
			}
		}
		if h.failPending {
			if _, ok := cmd.(*redis.XPendingExtCmd); ok {
				return errForcedV10
			}
		}
		if h.failClaim {
			if _, ok := cmd.(*redis.XAutoClaimCmd); ok {
				return errForcedV10
			}
		}
		if h.okSetID && cmd.Name() == "xgroup" && len(cmd.Args()) > 1 && cmd.Args()[1] == "setid" {
			return nil
		}
		return next(ctx, cmd)
	}
}

// addPendingMessageV10 在 stream 上制造一条属于 stale 消费者的 pending 消息，
// 并把 miniredis 时钟推进 10 分钟（idle ≥ pendingIdleTimeout）。
func addPendingMessageV10(t *testing.T, w *Worker, mr *miniredis.Miniredis) {
	t.Helper()
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u9","ts":"2025-06-15T10:00:00Z"}`,
	}})
	w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: "stale-consumer",
		Streams: []string{w.streamEvents, ">"}, Count: 10,
	})
	mr.SetTime(time.Now().Add(10 * time.Minute))
}

// ---------------------------------------------------------------------------
// Run 循环：processMessage 返回错误（XAck 失败）→ slog.Warn 分支
// ---------------------------------------------------------------------------

func TestRunProcessMessageErrorWarnV10(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.AddHook(hookV10{failAck: true})
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "0")
	w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "0")
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u1","ts":"2025-06-15T10:00:00Z"}`,
	}})

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()

	// XAck 恒失败 → 消息处理完成后留在 PEL（即 Run 内 processMessage 已返回错误）。
	deadline := time.Now().Add(5 * time.Second)
	for {
		p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
		if err == nil && p.Count >= 1 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("message was not processed (pending) within timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 失败路径不写 checkpoint。
	if n, _ := w.rdb.Exists(context.Background(), w.checkpointPrefix+":"+w.streamEvents).Result(); n != 0 {
		t.Error("checkpoint must not be persisted when ack fails")
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// ---------------------------------------------------------------------------
// restoreCheckpoints：XGroupSetID 成功 → lastIDs[stream] = id
// ---------------------------------------------------------------------------

func TestRestoreCheckpointSetIDSuccessV10(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.AddHook(hookV10{okSetID: true})
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")
	w.rdb.Set(ctx, w.checkpointPrefix+":"+w.streamEvents, "5-1", 0)

	w.restoreCheckpoints(ctx, w.streamEvents)
	if got := w.lastIDs[w.streamEvents]; got != "5-1" {
		t.Errorf("lastIDs[%s] = %q, want 5-1", w.streamEvents, got)
	}
}

// ---------------------------------------------------------------------------
// reclaim：XPendingExt 失败 + 孤儿消息认领后 XAck 失败
// ---------------------------------------------------------------------------

func TestReclaimPendingExtFailThenOrphanAckFailV10(t *testing.T) {
	w, mr := testWorker(t)
	w.rdb.AddHook(hookV10{failPending: true, failAck: true})
	addPendingMessageV10(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	// 孤儿路径认领成功但 ack 失败 → 消息仍在 PEL。
	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 1 {
		t.Errorf("pending = %d, want 1 (ack failed)", p.Count)
	}
}

// ---------------------------------------------------------------------------
// reclaim：XAutoClaim 失败（pending 循环内 + 孤儿路径）
// ---------------------------------------------------------------------------

func TestReclaimAutoClaimFailV10(t *testing.T) {
	w, mr := testWorker(t)
	w.rdb.AddHook(hookV10{failClaim: true})
	addPendingMessageV10(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	// 认领全部失败 → 消息保留在 PEL，且不写 checkpoint。
	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 1 {
		t.Errorf("pending = %d, want 1 (claim failed)", p.Count)
	}
	if n, _ := w.rdb.Exists(context.Background(), w.checkpointPrefix+":"+w.streamEvents).Result(); n != 0 {
		t.Error("checkpoint must not be persisted when claim fails")
	}
}

// ---------------------------------------------------------------------------
// reclaim：认领成功后 processMessage 失败（XAck 失败）→ claimed 循环 Warn
// ---------------------------------------------------------------------------

func TestReclaimClaimedProcessErrorV10(t *testing.T) {
	w, mr := testWorker(t)
	w.rdb.AddHook(hookV10{failAck: true})
	addPendingMessageV10(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 1 {
		t.Errorf("pending = %d, want 1 (ack failed after claim)", p.Count)
	}
}

// ---------------------------------------------------------------------------
// flush：PFCount 返回负数 → n<0 防御分支（置 0 后仍成功写入）
// ---------------------------------------------------------------------------

func TestFlushNegativePFCountV10(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.AddHook(hookV10{pfCountNeg: true})
	k := pastMinuteKeyV9()
	w.touchedMinutes[k] = struct{}{}

	if err := w.flush(context.Background()); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if _, ok := w.touchedMinutes[k]; ok {
		t.Error("minute key should be flushed (deleted) despite negative PFCount")
	}
	mc := getMockConn(t, w)
	if mc.batch == nil || mc.batch.Rows() != 1 {
		t.Fatalf("minute_online batch rows = %d, want 1", func() int {
			if mc.batch == nil {
				return -1
			}
			return mc.batch.Rows()
		}())
	}
}
