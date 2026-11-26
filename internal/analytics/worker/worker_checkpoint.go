package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// checkpointKey is the persistent key of a stream checkpoint. The consumer
// group is part of the key (DATA-055): two groups sharing one stream each
// keep their own high-water mark instead of overwriting each other's with an
// older progress value.
func (w *Worker) checkpointKey(stream string) string {
	return fmt.Sprintf("%s:%s:%s", w.checkpointPrefix, w.group, stream)
}

// legacyCheckpointKey is the pre-DATA-055 key shape (no group component).
// Migration rule: legacy keys are read once as the adoptCheckpoint monotonic
// floor and never written again — persistCheckpoint only writes the
// group-scoped key. The floor path is the only consumer: group creation and
// WORKER_GROUP_RESET read the group-scoped key exclusively, so a second
// group added after an upgrade cannot inherit another group's progress and
// silently skip its own backlog. A group recreated after a pre-upgrade Redis
// loss falls back to its declared start position; the replay is absorbed by
// the ClickHouse natural-key dedup contract (event_id / order_id).
func legacyCheckpointKey(prefix, stream string) string {
	return fmt.Sprintf("%s:%s", prefix, stream)
}

// checkpointFor returns the persisted checkpoint id for a stream and this
// worker's consumer group, or "" when none is stored (or the store read
// fails). Reads the group-scoped key only (DATA-055).
func (w *Worker) checkpointFor(ctx context.Context, stream string) string {
	if w.checkpointPrefix == "" {
		return ""
	}
	id, err := w.rdb.Get(ctx, w.checkpointKey(stream)).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("checkpoint get", "stream", stream, "err", err)
		}
		return ""
	}
	return id
}

// checkpointFloorFor is checkpointFor plus the legacy pre-DATA-055 key as a
// read-only migration fallback. Only the adoptCheckpoint floor uses it.
func (w *Worker) checkpointFloorFor(ctx context.Context, stream string) string {
	if id := w.checkpointFor(ctx, stream); id != "" {
		return id
	}
	if w.checkpointPrefix == "" {
		return ""
	}
	id, err := w.rdb.Get(ctx, legacyCheckpointKey(w.checkpointPrefix, stream)).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("legacy checkpoint get", "stream", stream, "err", err)
		}
		return ""
	}
	return id
}

// adoptCheckpoint loads the persisted checkpoint into lastIDs as the
// monotonic floor for advanceCheckpoint without touching the group cursor.
// On an existing group the Redis-native cursor is the source of truth
// (DATA-023): a stale application checkpoint must never rewind it, so this
// only influences in-memory regression guarding. The read includes the
// legacy no-group key as a migration fallback (DATA-055).
func (w *Worker) adoptCheckpoint(ctx context.Context, stream string) {
	id := w.checkpointFloorFor(ctx, stream)
	if id == "" {
		return
	}
	w.lastMu.Lock()
	w.lastIDs[stream] = id
	w.lastMu.Unlock()
}

func (w *Worker) persistCheckpoint(ctx context.Context, stream, id string) {
	if w.checkpointPrefix == "" || id == "" {
		return
	}
	// The group-scoped key only (DATA-055); the legacy key is never written.
	if err := w.rdb.Set(ctx, w.checkpointKey(stream), id, 0).Err(); err != nil {
		slog.Warn("checkpoint set", "stream", stream, "err", err)
	}
}

// processMessage handles a single message with error handling and dead letter
// queue.
//
// Durability contract (ARCH-011): a message whose row is appended to an
// in-memory ClickHouse batch is NOT ACKed here — XACK is deferred until the
// batch durably Send()s (see flushEventBatch/flushPaymentBatch). A crash
// between append and Send therefore redelivers the message via the PEL
// reclaim path instead of silently dropping the row. Duplicates can occur if
// a crash lands between Send and XACK; the ClickHouse natural key
// (event_id for events, order_id for payments) is the dedup point, and the
// raw tables are plain MergeTree so consumers must treat those keys as
// unique. A message that is re-queued or dead-lettered is ACKed in the same
// atomic, idempotent script that writes the target stream (DATA-052) — the
// target write and the source ACK cannot split, so reclaim never duplicates
// the retry copy or the dead letter.
func (w *Worker) processMessage(ctx context.Context, stream string, msg redis.XMessage) error {
	data := string(fmtAny(msg.Values["data"]))
	if data == "" {
		// Empty data, just acknowledge
		return w.ackImmediate(ctx, stream, msg)
	}

	var m map[string]any
	// UseNumber keeps JSON numbers as json.Number instead of float64: the
	// payment amount path (DATA-018) parses amount_cents exactly and refuses
	// values that a float64 round-trip would silently corrupt.
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		// Invalid JSON, move to dead letter
		if derr := w.sendToDeadLetter(ctx, stream, msg, "invalid_json", err.Error()); derr != nil {
			return derr // do not ack: reclaim retries the dead-letter
		}
		// sendToDeadLetter ACKed the source in the same atomic script (DATA-052).
		w.forgetMessageBookkeeping(stream, msg.ID)
		return nil
	}

	// Add retry count if not present
	if m["retry_count"] == nil {
		m["retry_count"] = float64(0) // JSON numbers are float64
	}

	// DATA-053: keep the wire payload so a failed flush can hold and retry
	// the row without a reclaim round-trip.
	w.holdEventData(stream, msg.ID, data)

	var err error
	switch stream {
	case w.streamEvents:
		// Update Redis HLL for minute online, DAU/new_users
		w.touchAgg(ctx, m)
		err = w.insertEvent(ctx, m, msg.ID)
	case w.streamPayments:
		err = w.insertPayment(ctx, m, msg.ID)
	}

	if err != nil {
		retries := asRetryCount(m)
		if retries >= maxPendingRetries {
			// Max retries exceeded, move to dead letter
			if derr := w.sendToDeadLetter(ctx, stream, msg, "max_retries_exceeded", err.Error()); derr != nil {
				return derr // do not ack: reclaim retries the dead-letter
			}
			// sendToDeadLetter ACKed the source in the same atomic script (DATA-052).
			w.forgetMessageBookkeeping(stream, msg.ID)
			return nil
		}
		// Increment retry count and re-queue
		// Normalization: regardless of the original value type, the retry
		// count is written back as float64 (a JSON number) so the next
		// processMessage does not take the non-float64 branch again
		// (asRetryCount already falls back, but keeping the message
		// self-consistent makes the retry count genuinely advance).
		m["retry_count"] = float64(retries + 1)
		retryData, _ := json.Marshal(m)
		// Re-queue WITHOUT a MaxLen cap (DATA-019): this is the producer's
		// source stream and its retention is the publisher's config (e.g.
		// ANALYTICS_REDIS_MAXLEN). Trimming here at an arbitrary 1000 evicts
		// unread and pending entries once the backlog exceeds the cap —
		// silent data loss. The retry path cannot grow the stream without
		// bound: retry_count rides in the payload and dead-letters after
		// maxPendingRetries requeues.
		// The copy and the source ACK are one atomic idempotent script
		// (DATA-052): a failed XAdd aborts before the ACK (the message stays
		// pending so the reclaim path retries it), and a repeated attempt
		// never writes a second retry copy.
		if rerr := w.requeueMessage(ctx, stream, msg, retryData, retries+1); rerr != nil {
			slog.Warn("message requeue failed; leaving pending for reclaim", "stream", stream, "id", msg.ID, "retries", retries+1, "err", rerr)
			return rerr
		}
		slog.Warn("message processing failed, re-queued", "stream", stream, "id", msg.ID, "retries", retries+1, "err", err)
		w.forgetMessageBookkeeping(stream, msg.ID)
		return nil
	}

	// Success: the row is in an unsent ClickHouse batch. XACK is deferred
	// until the batch Send()s; do not ack or advance the checkpoint here.
	return nil
}

// reclaimPendingMessages periodically reclaims pending messages that have been idle too long
func (w *Worker) reclaimPendingMessages(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(pendingReclaimInterval)
	defer wg.Done()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reclaimPendingFromStream(ctx, w.streamEvents)
			w.reclaimPendingFromStream(ctx, w.streamPayments)
		}
	}
}

// reclaimPendingFromStream reclaims pending messages from a specific stream
func (w *Worker) reclaimPendingFromStream(ctx context.Context, stream string) {
	// Get list of consumers in the group
	consumers, err := w.rdb.XInfoConsumers(ctx, stream, w.group).Result()
	if err != nil {
		slog.Warn("failed to get consumers", "stream", stream, "err", err)
		return
	}

	// Check each consumer for pending messages
	for _, consumer := range consumers {
		// Get pending messages for this consumer
		pending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream:   stream,
			Group:    w.group,
			Consumer: consumer.Name,
			Start:    "-",
			End:      "+",
			Count:    100,
		}).Result()
		if err != nil {
			slog.Warn("failed to get pending", "stream", stream, "consumer", consumer.Name, "err", err)
			continue
		}

		for _, p := range pending {
			// Check if message has been idle too long
			// p.Idle is time.Duration representing how long the message has been idle
			if p.Idle >= pendingIdleTimeout {
				// Try to claim the message
				cmd := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
					Stream:   stream,
					Group:    w.group,
					Consumer: w.consumer,
					MinIdle:  pendingIdleTimeout,
					Start:    p.ID,
					Count:    100,
				})
				claimed, _, err := cmd.Result()
				if err != nil && err != redis.Nil {
					slog.Warn("failed to auto-claim", "stream", stream, "id", p.ID, "err", err)
					continue
				}

				// Process claimed messages - claimed is []redis.XMessage
				for _, msg := range claimed {
					slog.Info("reclaimed pending message", "stream", stream, "id", msg.ID, "idle_time", p.Idle)
					if err := w.processMessage(ctx, stream, msg); err != nil {
						slog.Warn("failed to process reclaimed message", "stream", stream, "id", msg.ID, "err", err)
					}
				}
			}
		}
	}

	// Also check for any orphaned messages (no consumer)
	cmd := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    w.group,
		Consumer: w.consumer,
		MinIdle:  pendingIdleTimeout,
		Start:    "0-0",
		Count:    100,
	})
	orphaned, _, err := cmd.Result()
	if err != nil && err != redis.Nil {
		slog.Warn("failed to claim orphaned", "stream", stream, "err", err)
		return
	}

	for _, msg := range orphaned {
		slog.Info("reclaimed orphaned message", "stream", stream, "id", msg.ID)
		if err := w.processMessage(ctx, stream, msg); err != nil {
			slog.Warn("failed to process orphaned message", "stream", stream, "id", msg.ID, "err", err)
		}
	}
}
