package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func (w *Worker) restoreCheckpoints(ctx context.Context, stream string) {
	if w.checkpointPrefix == "" {
		return
	}
	key := fmt.Sprintf("%s:%s", w.checkpointPrefix, stream)
	id, err := w.rdb.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("checkpoint get", "stream", stream, "err", err)
		}
		return
	}
	if id == "" {
		return
	}
	if err := w.rdb.XGroupSetID(ctx, stream, w.group, id).Err(); err != nil {
		slog.Warn("checkpoint setid", "stream", stream, "err", err)
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
	key := fmt.Sprintf("%s:%s", w.checkpointPrefix, stream)
	if err := w.rdb.Set(ctx, key, id, 0).Err(); err != nil {
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
// unique. A message that is re-queued or dead-lettered is ACKed only after
// the re-queue / dead-letter XAdd succeeds — otherwise it stays pending.
func (w *Worker) processMessage(ctx context.Context, stream string, msg redis.XMessage) error {
	data := string(fmtAny(msg.Values["data"]))
	if data == "" {
		// Empty data, just acknowledge
		return w.ackImmediate(ctx, stream, msg)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		// Invalid JSON, move to dead letter
		if derr := w.sendToDeadLetter(ctx, stream, msg, "invalid_json", err.Error()); derr != nil {
			return derr // do not ack: reclaim retries the dead-letter
		}
		return w.ackImmediate(ctx, stream, msg)
	}

	// Add retry count if not present
	if m["retry_count"] == nil {
		m["retry_count"] = float64(0) // JSON numbers are float64
	}

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
			return w.ackImmediate(ctx, stream, msg)
		}
		// Increment retry count and re-queue
		// 归一化：无论原值类型，重试计数都写为 float64（JSON 数字），避免
		// 下一次 processMessage 再走非 float64 分支（asRetryCount 已兜底，
		// 但保持消息自洽，重试计数真实推进）。
		m["retry_count"] = float64(retries + 1)
		retryData, _ := json.Marshal(m)
		if rerr := w.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			MaxLen: 1000, // Keep retry queue bounded
			Approx: true,
			ID:     "*",
			Values: map[string]interface{}{"data": string(retryData)},
		}).Err(); rerr != nil {
			// Re-queue failed: do NOT ack. The message stays pending so the
			// reclaim path retries it; acking here would lose it.
			slog.Warn("message requeue failed; leaving pending for reclaim", "stream", stream, "id", msg.ID, "retries", retries+1, "err", rerr)
			return rerr
		}
		slog.Warn("message processing failed, re-queued", "stream", stream, "id", msg.ID, "retries", retries+1, "err", err)
		return w.ackImmediate(ctx, stream, msg)
	}

	// Success: the row is in an unsent ClickHouse batch. XACK is deferred
	// until the batch Send()s; do not ack or advance the checkpoint here.
	return nil
}

// sendToDeadLetter sends a message to the dead letter stream. It returns the
// XAdd error so the caller can avoid ACKing a message whose dead-letter entry
// was not enqueued (ARCH-011: unchecked re-queue/dead-letter error would
// otherwise lose the message).
func (w *Worker) sendToDeadLetter(ctx context.Context, stream string, msg redis.XMessage, reason, details string) error {
	deadStream := w.deadEventsStream
	if stream == w.streamPayments {
		deadStream = w.deadPaymentsStream
	}

	deadEntry := map[string]interface{}{
		"original_stream": stream,
		"original_id":     msg.ID,
		"reason":          reason,
		"details":         details,
		"failed_at":       time.Now().Unix(),
		"retry_count":     msg.Values["retry_count"],
		"original_data":   msg.Values["data"],
	}

	if err := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: deadStream,
		MaxLen: 10000, // Keep dead letter queue bounded
		Approx: true,
		ID:     "*",
		Values: deadEntry,
	}).Err(); err != nil {
		slog.Error("failed to send to dead letter", "stream", deadStream, "original_id", msg.ID, "err", err)
		return err
	}
	slog.Warn("message sent to dead letter", "stream", deadStream, "original_stream", stream, "original_id", msg.ID, "reason", reason)
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
