package worker

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// #6 单调检查点：ackPendingLocked 不得让 lastIDs/检查点回退
// ---------------------------------------------------------------------------

func TestCompareStreamIDs(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// 变宽毫秒段：字典序会错序（"999" > "1000"），必须按数值比较。
		{"500-0", "600-0", -1},
		{"600-0", "500-0", 1},
		{"600-0", "600-0", 0},
		{"999-0", "1000-0", -1},
		{"1000-0", "999-0", 1},
		{"1000-0", "1000-5", -1},
		{"1000-5", "1000-0", 1},
		// 同毫秒、缺 seq 按 0。
		{"1000", "1000-0", 0},
		{"1000", "1000-5", -1},
		{"1000-5", "1000", 1},
	}
	for _, c := range cases {
		if got := compareStreamIDs(c.a, c.b); got != c.want {
			t.Errorf("compareStreamIDs(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAckPendingLocked_MonotonicCheckpoint(t *testing.T) {
	w, _ := testWorker(t)
	stream := w.streamEvents
	ctx := context.Background()

	// 建组并投递三条显式 ID 的消息，使它们进入 PEL（XAck 可成功）。
	w.rdb.XGroupCreateMkStream(ctx, stream, w.group, "$")
	for _, id := range []string{"500-0", "600-0", "700-0"} {
		if err := w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: stream, ID: id,
			Values: map[string]any{"data": `{"event":"x"}`}}).Err(); err != nil {
			t.Fatalf("xadd %s: %v", id, err)
		}
	}
	w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Count: 10,
		Streams: []string{stream, ">"},
	})

	// 检查点已推进到 "600-0"（600 之前已 ACK 落库）。
	w.lastIDs[stream] = "600-0"

	// 旧消息 "500-0"（纯 reclaim 回填，早于检查点）ACK 成功但不得回退检查点。
	if err := w.ackPendingLocked(ctx, stream, []string{"500-0"}); err != nil {
		t.Fatalf("ack old: %v", err)
	}
	if got := w.lastIDs[stream]; got != "600-0" {
		t.Fatalf("checkpoint regressed on old batch: got %q, want %q", got, "600-0")
	}

	// 等值消息不得误推进（compare==0）。
	if err := w.ackPendingLocked(ctx, stream, []string{"600-0"}); err != nil {
		t.Fatalf("ack equal: %v", err)
	}
	if got := w.lastIDs[stream]; got != "600-0" {
		t.Fatalf("checkpoint advanced on equal batch: got %q, want %q", got, "600-0")
	}

	// 新消息 "700-0" 晚于检查点 → 正常推进。
	if err := w.ackPendingLocked(ctx, stream, []string{"700-0"}); err != nil {
		t.Fatalf("ack new: %v", err)
	}
	if got := w.lastIDs[stream]; got != "700-0" {
		t.Fatalf("checkpoint did not advance on new batch: got %q, want %q", got, "700-0")
	}

	// 空批：no-op。
	if err := w.ackPendingLocked(ctx, stream, nil); err != nil {
		t.Fatalf("ack empty: %v", err)
	}
}
