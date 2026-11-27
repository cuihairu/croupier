package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	redis "github.com/redis/go-redis/v9"
)

func (w *Worker) flushEventBatch(ctx context.Context) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.flushEventBatchLocked(ctx)
}

// heldRow is a deferred-ACK ref whose ClickHouse row is kept for retry after
// a failed batch Send (DATA-053), together with the wire payload needed to
// re-derive the row.
type heldRow struct {
	ref  ackRef
	data string
}

// holdKey namespaces per-message counters across the two streams.
func holdKey(stream, id string) string { return stream + "\x1f" + id }

// holdEventData records the wire payload for a message whose row is entering
// an unsent batch (DATA-053), so a failed flush can hold and retry the row
// without a reclaim round-trip.
func (w *Worker) holdEventData(stream, id, data string) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	switch stream {
	case w.streamEvents:
		if w.eventData == nil {
			w.eventData = map[string]string{}
		}
		w.eventData[id] = data
	case w.streamPayments:
		if w.paymentData == nil {
			w.paymentData = map[string]string{}
		}
		w.paymentData[id] = data
	}
}

// flushEventBatchLocked retries rows held from previous failed flush rounds
// (DATA-053), then Send()s the batch. On Send failure the rows and their ACK
// refs are HELD for the next round instead of dropped; a record whose flush
// has failed maxPendingRetries rounds is dead-lettered through the idempotent
// transfer (DATA-052), which also ACKs the source so reclaim stops cycling it.
func (w *Worker) flushEventBatchLocked(ctx context.Context) error {
	w.retryHeldEventsLocked(ctx)
	if w.eventBatch == nil || w.eventBatchRows == 0 {
		w.eventAcks = nil
		return nil
	}
	if err := w.eventBatch.Send(); err != nil {
		w.holdFailedRefsLocked(ctx, &w.eventHeld, w.eventAcks, w.eventData, w.streamEvents, err)
		// A failed Send burns the batch object: clickhouse-go batches cannot be
		// resent, so the next append opens a fresh one. The rows live on in the
		// held list and are re-derived from their wire payloads (DATA-053).
		w.eventBatch = nil
		w.eventBatchRows = 0
		w.eventAcks = nil
		return err
	}
	w.eventBatch = nil
	w.eventBatchRows = 0
	acks := w.eventAcks
	w.eventAcks = nil
	for _, ref := range acks {
		delete(w.eventData, ref.id)
		delete(w.holdFails, holdKey(ref.stream, ref.id))
	}
	w.ackSent(ctx, acks)
	return nil
}

func (w *Worker) flushPaymentBatch(ctx context.Context) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.flushPaymentBatchLocked(ctx)
}

func (w *Worker) flushPaymentBatchLocked(ctx context.Context) error {
	w.retryHeldPaymentsLocked(ctx)
	if w.paymentBatch == nil || w.paymentBatchRows == 0 {
		w.paymentAcks = nil
		return nil
	}
	if err := w.paymentBatch.Send(); err != nil {
		w.holdFailedRefsLocked(ctx, &w.paymentHeld, w.paymentAcks, w.paymentData, w.streamPayments, err)
		// Same as events: the burned batch object is dropped and the rows live
		// on in the held list (DATA-053).
		w.paymentBatch = nil
		w.paymentBatchRows = 0
		w.paymentAcks = nil
		return err
	}
	w.paymentBatch = nil
	w.paymentBatchRows = 0
	acks := w.paymentAcks
	w.paymentAcks = nil
	for _, ref := range acks {
		delete(w.paymentData, ref.id)
		delete(w.holdFails, holdKey(ref.stream, ref.id))
	}
	// DATA-021: advance the durable cumulative revenue progress BEFORE the
	// ACK. On Redis failure the advance returns an error, the messages stay
	// pending and reclaim redelivers them; the per-message seen markers make
	// the retry skip anything the failed round never advanced, so the
	// cumulative total cannot double count. The tradeoff is that a redelivery
	// re-appends the payments detail row — a visible, recoverable duplication
	// in analytics.payments, the same exposure the crash window between Send
	// and ackSent already had. Corrupting daily_revenue is strictly worse.
	if err := w.commitRevenueDurable(ctx, acks); err != nil {
		slog.Warn("durable revenue commit failed; leaving messages pending for reclaim", "n", len(acks), "err", err)
		return err
	}
	w.ackSent(ctx, acks)
	return nil
}

// holdFailedRefsLocked moves a failed flush's ACK refs into the held list
// with their wire payloads (DATA-053). Records whose holdFails counter has
// reached maxPendingRetries are isolated to the dead letter instead. Held
// entries stay unique per source message id.
func (w *Worker) holdFailedRefsLocked(ctx context.Context, held *[]heldRow, refs []ackRef, data map[string]string, stream string, sendErr error) {
	if w.holdFails == nil {
		w.holdFails = map[string]int{}
	}
	for _, ref := range refs {
		key := holdKey(ref.stream, ref.id)
		w.holdFails[key]++
		payload := data[ref.id]
		if w.holdFails[key] >= maxPendingRetries {
			w.deadLetterHeldLocked(ctx, ref, payload,
				fmt.Sprintf("flush failed %d times: %v", w.holdFails[key], sendErr),
				"max_flush_retries_exceeded")
			continue
		}
		found := false
		for i := range *held {
			if (*held)[i].ref.id == ref.id {
				found = true
				break
			}
		}
		if !found {
			*held = append(*held, heldRow{ref: ref, data: payload})
		}
	}
	attempts := 0
	for _, ref := range refs {
		if n := w.holdFails[holdKey(ref.stream, ref.id)]; n > attempts {
			attempts = n
		}
	}
	slog.Warn("batch send failed; rows held for retry",
		"stream", stream, "held", len(*held), "attempts", attempts, "err", sendErr)
}

// retryHeldEventsLocked re-derives rows from held payloads and appends them
// to the fresh batch before the Send attempt (DATA-053). Items whose message
// was meanwhile redelivered by reclaim are skipped — the fresh row and its
// own deferred ref supersede the held copy. Payloads that no longer decode
// are isolated to the dead letter immediately (permanent bad records).
func (w *Worker) retryHeldEventsLocked(ctx context.Context) {
	if len(w.eventHeld) == 0 {
		return
	}
	held := w.eventHeld
	w.eventHeld = nil
	for _, h := range held {
		if w.hasRefLocked(w.eventAcks, h.ref.id) {
			continue
		}
		var m map[string]any
		dec := json.NewDecoder(strings.NewReader(h.data))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			w.deadLetterHeldLocked(ctx, h.ref, h.data, err.Error(), "retry_decode_failed")
			continue
		}
		if w.eventData == nil {
			w.eventData = map[string]string{}
		}
		w.eventData[h.ref.id] = h.data
		if err := w.appendEventRowLocked(ctx, h.ref.id, eventRowArgs(m)...); err != nil {
			// A batch-size auto-flush inside the append may already have
			// re-held (or dead-lettered) this ref; only isolate when the ref
			// did not land anywhere.
			if !w.hasRefLocked(w.eventAcks, h.ref.id) && !w.heldHas(w.eventHeld, h.ref.id) {
				w.deadLetterHeldLocked(ctx, h.ref, h.data, err.Error(), "retry_append_failed")
			}
		}
	}
}

func (w *Worker) retryHeldPaymentsLocked(ctx context.Context) {
	if len(w.paymentHeld) == 0 {
		return
	}
	held := w.paymentHeld
	w.paymentHeld = nil
	for _, h := range held {
		if w.hasRefLocked(w.paymentAcks, h.ref.id) {
			continue
		}
		var m map[string]any
		dec := json.NewDecoder(strings.NewReader(h.data))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			w.deadLetterHeldLocked(ctx, h.ref, h.data, err.Error(), "retry_decode_failed")
			continue
		}
		if w.paymentData == nil {
			w.paymentData = map[string]string{}
		}
		w.paymentData[h.ref.id] = h.data
		args, err := paymentRowArgs(m)
		if err == nil {
			err = w.appendPaymentRowLocked(ctx, h.ref.id, args...)
		}
		if err != nil {
			if !w.hasRefLocked(w.paymentAcks, h.ref.id) && !w.heldHas(w.paymentHeld, h.ref.id) {
				w.deadLetterHeldLocked(ctx, h.ref, h.data, err.Error(), "retry_append_failed")
			}
		}
	}
}

func (w *Worker) hasRefLocked(refs []ackRef, id string) bool {
	for i := range refs {
		if refs[i].id == id {
			return true
		}
	}
	return false
}

func (w *Worker) heldHas(held []heldRow, id string) bool {
	for i := range held {
		if held[i].ref.id == id {
			return true
		}
	}
	return false
}

// deadLetterHeldLocked isolates a persistently failing held record through the
// idempotent dead-letter transfer (DATA-052), which ACKs the pending source
// message so reclaim stops cycling it. On transfer failure the message stays
// pending (nothing was written); reclaim owns it from here on.
func (w *Worker) deadLetterHeldLocked(ctx context.Context, ref ackRef, data, details, reason string) {
	msg := redis.XMessage{ID: ref.id, Values: map[string]any{"data": data}}
	if err := w.sendToDeadLetter(ctx, ref.stream, msg, reason, details); err != nil {
		slog.Warn("held row dead-letter transfer failed; leaving pending for reclaim", "id", ref.id, "err", err)
		return
	}
	delete(w.holdFails, holdKey(ref.stream, ref.id))
	delete(w.eventData, ref.id)
	delete(w.paymentData, ref.id)
}

// forgetMessageBookkeeping drops a message's flush bookkeeping when it leaves
// the flush pipeline without a Send (requeue / dead-letter transfers): the
// payload maps, the failure counter, and any held row. The stream retry copy
// or the dead letter supersedes the in-memory held row — keeping both would
// write the record twice.
func (w *Worker) forgetMessageBookkeeping(stream, id string) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	w.dropHeldLocked(&w.eventHeld, id)
	w.dropHeldLocked(&w.paymentHeld, id)
	delete(w.eventData, id)
	delete(w.paymentData, id)
	delete(w.holdFails, holdKey(stream, id))
}

// dropHeldLocked removes every held row for a message id.
func (w *Worker) dropHeldLocked(held *[]heldRow, id string) {
	out := (*held)[:0]
	for _, h := range *held {
		if h.ref.id != id {
			out = append(out, h)
		}
	}
	*held = out
}

func revenueDeltaFromPaymentArgs(args []any) (string, *revRow, bool) {
	if len(args) < 9 {
		return "", nil, false
	}
	ts, ok := args[0].(string)
	if !ok || len(ts) < 10 {
		return "", nil, false
	}
	game, ok := args[1].(string)
	if !ok {
		return "", nil, false
	}
	env, ok := args[2].(string)
	if !ok {
		return "", nil, false
	}
	amount, ok := args[5].(uint64)
	if !ok {
		return "", nil, false
	}
	status, ok := args[7].(string)
	if !ok {
		return "", nil, false
	}
	delta := &revRow{}
	switch strings.ToLower(status) {
	case "success":
		delta.revenue = amount
	case "refunded":
		delta.refunds = amount
	case "failed":
		delta.failed = 1
	default:
		return "", nil, false
	}
	return fmt.Sprintf("%s|%s|%s", ts[:10], game, env), delta, true
}

// commitRevenue folds the payment rows' revenue deltas into revAgg. A source
// message redelivered by reclaim can appear twice in one Send's refs; the
// delta must be committed exactly once per message (DATA-053).
func (w *Worker) commitRevenue(acks []ackRef) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	seen := make(map[string]struct{}, len(acks))
	for _, ack := range acks {
		if ack.rev == nil {
			continue
		}
		key := holdKey(ack.stream, ack.id)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		current := w.revAgg[ack.revKey]
		if current == nil {
			current = &revRow{}
			w.revAgg[ack.revKey] = current
		}
		current.revenue += ack.rev.revenue
		current.refunds += ack.rev.refunds
		current.failed += ack.rev.failed
	}
}

// flushBatches flushes both streams independently (DATA-050): an event Send
// failure must not strand payments — each stream's rows land and its refs
// ACK on its own, and both failures are reported in the joined error. This
// is the single flush path for the periodic and final flushes.
func (w *Worker) flushBatches(ctx context.Context) error {
	errEvents := w.flushEventBatch(ctx)
	errPayments := w.flushPaymentBatch(ctx)
	return errors.Join(errEvents, errPayments)
}
