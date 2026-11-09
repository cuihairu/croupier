package worker

import (
	"context"
	"encoding/json"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

func (w *Worker) insertEvent(ctx context.Context, m map[string]any, msgID string) error {
	// Minimal fields; props_json stays raw
	ts := normalizeTimestamp(asString(m, "ts"))
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	sid := asString(m, "session_id")
	evt := asString(m, "event")
	channel := asString(m, "channel")
	platform := asString(m, "platform")
	country := asString(m, "country")
	appv := asString(m, "app_version")
	eid := asString(m, "event_id")
	if _, err := uuid.Parse(eid); err != nil {
		// ClickHouse UUID column rejects empty/invalid values; producers that
		// do not assign event ids still get a dedup-key-grade identifier.
		eid = uuid.NewString()
	}
	propsBytes, _ := json.Marshal(m["props"]) // may be nil
	return w.appendEventRow(ctx, msgID, ts, game, env, uid, sid, evt, channel, platform, country, appv, eid, string(propsBytes))
}

func (w *Worker) insertPayment(ctx context.Context, m map[string]any, msgID string) error {
	ts := normalizeTimestamp(asString(m, "ts"))
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	oid := asString(m, "order_id")
	amount := asFloat(m, "amount_cents")
	curr := asString(m, "currency")
	status := asString(m, "status")
	channel := asString(m, "channel")
	platform := asString(m, "platform")
	country := asString(m, "country")
	region := asString(m, "region")
	city := asString(m, "city")
	product := asString(m, "product_id")
	reason := asString(m, "reason")
	return w.appendPaymentRow(ctx, msgID, ts, game, env, uid, oid, uint64(amount), curr, status, channel, platform, country, region, city, product, reason)
}

func (w *Worker) ensureEventBatch(ctx context.Context) (chdriver.Batch, error) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.ensureEventBatchLocked(ctx)
}

func (w *Worker) ensureEventBatchLocked(ctx context.Context) (chdriver.Batch, error) {
	if w.eventBatch != nil {
		return w.eventBatch, nil
	}
	batch, err := w.ch.PrepareBatch(ctx, insertEventsSQL)
	if err != nil {
		return nil, err
	}
	w.eventBatch = batch
	w.eventBatchRows = 0
	return batch, nil
}

func (w *Worker) ensurePaymentBatch(ctx context.Context) (chdriver.Batch, error) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.ensurePaymentBatchLocked(ctx)
}

func (w *Worker) ensurePaymentBatchLocked(ctx context.Context) (chdriver.Batch, error) {
	if w.paymentBatch != nil {
		return w.paymentBatch, nil
	}
	batch, err := w.ch.PrepareBatch(ctx, insertPaymentsSQL)
	if err != nil {
		return nil, err
	}
	w.paymentBatch = batch
	w.paymentBatchRows = 0
	return batch, nil
}

// appendEventRow appends one row to the event batch and records the source
// message in eventAcks so it is ACKed only when the batch durably Send()s
// (ARCH-011). The msgID is the Redis stream message the row came from.
func (w *Worker) appendEventRow(ctx context.Context, msgID string, args ...any) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	batch, err := w.ensureEventBatchLocked(ctx)
	if err != nil {
		return err
	}
	if err := batch.Append(args...); err != nil {
		return err
	}
	w.eventBatchRows++
	w.eventAcks = append(w.eventAcks, ackRef{stream: w.streamEvents, id: msgID})
	if w.clickBatchSize > 0 && w.eventBatchRows >= w.clickBatchSize {
		return w.flushEventBatchLocked(ctx)
	}
	return nil
}

// appendPaymentRow appends one row to the payment batch and records the
// source message in paymentAcks so it is ACKed only when the batch durably
// Send()s (ARCH-011).
func (w *Worker) appendPaymentRow(ctx context.Context, msgID string, args ...any) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	batch, err := w.ensurePaymentBatchLocked(ctx)
	if err != nil {
		return err
	}
	if err := batch.Append(args...); err != nil {
		return err
	}
	w.paymentBatchRows++
	ref := ackRef{stream: w.streamPayments, id: msgID}
	if key, delta, ok := revenueDeltaFromPaymentArgs(args); ok {
		ref.revKey = key
		ref.rev = delta
	}
	w.paymentAcks = append(w.paymentAcks, ref)
	if w.clickBatchSize > 0 && w.paymentBatchRows >= w.clickBatchSize {
		return w.flushPaymentBatchLocked(ctx)
	}
	return nil
}
