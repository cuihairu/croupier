package worker

import (
	"context"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
)

// A failed message requeues into the SOURCE stream: the retry write must not
// apply a retention cap of its own, or a backlog over the cap silently evicts
// still-unread entries (DATA-019). Stream retention is the publisher's config;
// retry growth is bounded by retry_count / maxPendingRetries instead.
func TestProcessMessage_RequeueDoesNotTrimSourceBacklog(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	stream := w.streamPayments // exercise the payment failure -> retry branch

	// Unread backlog well past the old MaxLen: 1000 cap (e.g. accumulated
	// while the worker was down).
	const seeds = 1500
	var firstID string
	for i := 0; i < seeds; i++ {
		id := w.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"data": fmt.Sprintf(`{"game_id":"seed","n":%d}`, i)},
		}).Val()
		if i == 0 {
			firstID = id
		}
	}
	// Group starts at "$" so the seeded backlog stays unread and replayable.
	w.rdb.XGroupCreateMkStream(ctx, stream, w.group, "$")

	// Invalid payment (no amount_cents) -> insertPayment errors -> retry
	// branch requeues a copy into the same source stream.
	poisonedID := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"data": `{"ts":"2025-06-15T10:30:00Z","game_id":"g1","env":"prod","user_id":"u1","order_id":"ord-invalid","currency":"USD","status":"success"}`},
	}).Val()
	claimed := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Count: 1,
		Streams: []string{stream, ">"},
	}).Val()
	if len(claimed) != 1 || len(claimed[0].Messages) != 1 {
		t.Fatalf("XReadGroup returned %+v, want one message", claimed)
	}
	msg := claimed[0].Messages[0]
	if msg.ID != poisonedID {
		t.Fatalf("claimed ID %s, want poisoned %s", msg.ID, poisonedID)
	}

	if err := w.processMessage(ctx, stream, msg); err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	// The retry copy is appended, not swapped in under a 1000-entry cap:
	// all 1500 unread entries plus the poisoned original and its retry copy
	// must remain replayable.
	wantLen := int64(seeds + 2)
	if got := w.rdb.XLen(ctx, stream).Val(); got != wantLen {
		t.Fatalf("stream length = %d, want %d (retry requeue trimmed the source)", got, wantLen)
	}
	entries, err := w.rdb.XRange(ctx, stream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRANGE: %v", err)
	}
	if len(entries) != int(wantLen) {
		t.Fatalf("replayable entries = %d, want %d", len(entries), wantLen)
	}
	if entries[0].ID != firstID {
		t.Fatalf("oldest backlog entry lost: first=%s, want %s", entries[0].ID, firstID)
	}
	if entries[len(entries)-1].ID == poisonedID {
		t.Fatal("retry copy was not appended to the source stream")
	}
}
