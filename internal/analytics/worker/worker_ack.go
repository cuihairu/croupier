package worker

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	redis "github.com/redis/go-redis/v9"
)

// ackSent ACKs message refs whose rows have been durably Send()ed to
// ClickHouse, then advances the per-stream checkpoint. A XAck failure is
// logged but not fatal: the message stays pending and reclaim redelivers it,
// and the ClickHouse natural key (event_id / order_id) deduplicates the
// re-insert.
func (w *Worker) ackSent(ctx context.Context, acks []ackRef) {
	for _, a := range acks {
		if err := w.rdb.XAck(ctx, a.stream, w.group, a.id).Err(); err != nil {
			slog.Warn("batch ack failed", "stream", a.stream, "id", a.id, "err", err)
			continue
		}
		w.advanceCheckpoint(ctx, a.stream, a.id)
	}
}

// ackImmediate ACKs a message resolved without a pending batch row (empty
// payload, dead-lettered, or re-queued) and advances the checkpoint.
func (w *Worker) ackImmediate(ctx context.Context, stream string, msg redis.XMessage) error {
	if err := w.rdb.XAck(ctx, stream, w.group, msg.ID).Err(); err != nil {
		slog.Warn("ack failed", "stream", stream, "id", msg.ID, "err", err)
		return err
	}
	w.advanceCheckpoint(ctx, stream, msg.ID)
	return nil
}

// advanceCheckpoint moves the per-stream high-water mark forward. Stream IDs
// are "<ms>-<seq>"; a later message may be ACKed immediately (dead-letter /
// re-queue) while an earlier one still sits in an unsent batch, so the mark
// must be monotonic rather than blindly overwritten — otherwise a checkpoint
// regression would re-read already-resolved messages after a restart.
func (w *Worker) advanceCheckpoint(ctx context.Context, stream, id string) {
	w.lastMu.Lock()
	cur := w.lastIDs[stream]
	if streamIDLess(id, cur) {
		w.lastMu.Unlock()
		return
	}
	w.lastIDs[stream] = id
	w.lastMu.Unlock()
	w.persistCheckpoint(ctx, stream, id)
}

// parseStreamID splits a "<ms>-<seq>" stream ID; malformed IDs parse as 0.
func parseStreamID(s string) (ms, seq uint64) {
	i := strings.IndexByte(s, '-')
	if i < 0 {
		ms, _ = strconv.ParseUint(s, 10, 64)
		return
	}
	ms, _ = strconv.ParseUint(s[:i], 10, 64)
	seq, _ = strconv.ParseUint(s[i+1:], 10, 64)
	return
}

// streamIDLess reports whether stream ID a sorts before b.
func streamIDLess(a, b string) bool {
	am, as := parseStreamID(a)
	bm, bs := parseStreamID(b)
	if am != bm {
		return am < bm
	}
	return as < bs
}

// --- Aggregation helpers ---
