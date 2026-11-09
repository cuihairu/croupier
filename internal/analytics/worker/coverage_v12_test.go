package worker

// worker.go 补测：Run 消费循环的 processMessage 错误告警臂（241）——经
// redis hook 只让死信流 XAdd 失败（XReadGroup 与其余命令照常），令
// retry 耗尽的 pending 消息在死信投递处返回错误，从而驱动
// slog.Warn("process message", ...) 分支。先前诚实跳过记录该臂为
// miniredis harness 限制，实际以「定向失败 hook」可稳定触达。
//
// 诚实跳过（维持先前 judgment，时序竞速不可稳定触发）：Run 停机路径的
// reclaim 协程超时告警（264-265）与最终 flush 超时两臂（286-287、
// 289-290）——均需子协程真实慢于 shutdownTimeout/finalFlushTimeout 的
// 调度时序，测试时长与稳定性不可控。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// deadStreamFailHook 只对死信流的 xadd 强制失败（其余命令透传）。
type deadStreamFailHook struct {
	stream string
	err    error
}

func (h deadStreamFailHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h deadStreamFailHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h deadStreamFailHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "xadd" && len(cmd.Args()) > 1 && cmd.Args()[1] == h.stream {
			return h.err
		}
		return next(ctx, cmd)
	}
}

func TestRunProcessMessageErrorWarnViaDeadLetterFail(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb.AddHook(deadStreamFailHook{stream: w.deadEventsStream, err: errors.New("dead stream down")})
	// ClickHouse 落库失败 + retry 耗尽 → 死信投递（XAdd 被 hook 拒绝）→
	// processMessage 返回错误（非内部告警），Run 循环走 241 分支。
	w.ch = &stubConnV9{mockConn: &mockConn{}, prepareErr: errors.New("ch down")}

	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "0")
	w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "0")
	// retry 耗尽 → 死信投递 → XAdd 被 hook 拒绝 → processMessage 返回错误
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data":        `{"ts":"2026-11-09T10:00:00Z","game_id":"demo","env":"prod","user_id":"u1","retry_count":5}`,
		"retry_count": float64(5),
	}})

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()

	// 死信投递失败 → 不 ack → 消息留在 PEL（即 Run 内 processMessage 已返回错误）
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

	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
