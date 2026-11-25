package worker

// Additional coverage for worker.go: the processMessage error-warning arm (241) of the
// Run consume loop — using a redis hook that fails only the dead-letter stream XAdd
// (XReadGroup and the remaining commands behave normally), a pending message whose
// retries are exhausted returns an error at dead-letter delivery, which drives the
// slog.Warn("process message", ...) branch. The earlier honest skip recorded this arm as a
// miniredis harness limitation; a "targeted failure hook" reaches it reliably instead.
//
// Honest skips (keeping the earlier judgment — the timing race cannot be triggered
// stably): the reclaim goroutine timeout warning (264-265) on Run's shutdown path and
// the two final-flush timeout arms (286-287, 289-290) — both need the child goroutine to
// actually run slower than shutdownTimeout/finalFlushTimeout scheduling-wise, and the test
// duration and stability are uncontrollable.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// deadStreamFailHook forces failure only for xadd on the dead-letter stream (other
// commands pass through).
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
	// ClickHouse write failure + retries exhausted -> dead-letter delivery (XAdd refused by
	// the hook) -> processMessage returns an error (not an internal warning), taking the
	// Run loop's branch at 241.
	w.ch = &stubConnV9{mockConn: &mockConn{}, prepareErr: errors.New("ch down")}

	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "0")
	w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "0")
	// retries exhausted -> dead-letter delivery -> XAdd refused by the hook -> processMessage
	// returns an error
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data":        `{"ts":"2026-11-09T10:00:00Z","game_id":"demo","env":"prod","user_id":"u1","retry_count":5}`,
		"retry_count": float64(5),
	}})

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()

	// Dead-letter delivery fails -> no ack -> the message stays in the PEL (i.e.
	// processMessage inside Run has already returned an error)
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
