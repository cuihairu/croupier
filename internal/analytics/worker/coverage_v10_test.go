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
// V10 helpers: go-redis ProcessHook injection, faking server-side error responses
// ---------------------------------------------------------------------------

// hookV10 injects faults by command name:
//   - failAck:     XACK always fails (simulates an ack anomaly, covering the
//     processMessage error branches inside Run/reclaim)
//   - okSetID:     XGROUP SETID always succeeds (miniredis does not implement this
//     command; faking success covers the lastIDs assignment)
//   - failPending: XPENDING EXT always fails
//   - failClaim:   XAUTOCLAIM always fails
//   - pfCountNeg:  PFCOUNT always returns -5 (a faked server-side error value,
//     covering the n<0 defensive branch)
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

// addPendingMessageV10 creates a pending message owned by a stale consumer on
// the stream and advances the miniredis clock by 10 minutes (idle >= pendingIdleTimeout).
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
// Run loop: processMessage returns an error (XAck fails) -> slog.Warn branch
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

	// XAck always fails -> the message stays in the PEL after processing
	// (i.e. processMessage inside Run has already returned an error).
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
	// The failure path does not write a checkpoint.
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
// adoptCheckpoint: persisted checkpoint -> lastIDs[stream] (no group cursor
// mutation; DATA-023 split the two concerns apart)
// ---------------------------------------------------------------------------

func TestRestoreCheckpointSetIDSuccessV10(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	w.rdb.Set(ctx, w.checkpointPrefix+":"+w.streamEvents, "5-1", 0)

	w.adoptCheckpoint(ctx, w.streamEvents)
	if got := w.lastIDs[w.streamEvents]; got != "5-1" {
		t.Errorf("lastIDs[%s] = %q, want 5-1", w.streamEvents, got)
	}
}

// ---------------------------------------------------------------------------
// reclaim: XPendingExt fails + XAck fails after claiming an orphaned message
// ---------------------------------------------------------------------------

func TestReclaimPendingExtFailThenOrphanAckFailV10(t *testing.T) {
	w, mr := testWorker(t)
	w.rdb.AddHook(hookV10{failPending: true, failAck: true})
	addPendingMessageV10(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	// The orphan path claimed successfully but ack failed -> the message is still in the PEL.
	p, err := w.rdb.XPending(context.Background(), w.streamEvents, w.group).Result()
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 1 {
		t.Errorf("pending = %d, want 1 (ack failed)", p.Count)
	}
}

// ---------------------------------------------------------------------------
// reclaim: XAutoClaim fails (inside the pending loop + the orphan path)
// ---------------------------------------------------------------------------

func TestReclaimAutoClaimFailV10(t *testing.T) {
	w, mr := testWorker(t)
	w.rdb.AddHook(hookV10{failClaim: true})
	addPendingMessageV10(t, w, mr)

	w.reclaimPendingFromStream(context.Background(), w.streamEvents)

	// All claims failed -> the message stays in the PEL, and no checkpoint is written.
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
// reclaim: after a successful claim, processMessage fails (XAck fails) -> Warn in
// the claimed loop
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
// flush: PFCount returns a negative number -> the n<0 defensive branch (writing
// still succeeds after it is clamped to 0)
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
