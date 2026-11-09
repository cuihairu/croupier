package worker

import (
	"context"
	"fmt"
	"strings"
)

func (w *Worker) flushEventBatch(ctx context.Context) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.flushEventBatchLocked(ctx)
}

// flushEventBatchLocked Send()s the event batch, then ACKs its deferred
// message refs. On Send failure the batch and its refs are DROPPED (not
// retried): the messages stay PENDING in Redis so reclaim redelivers them,
// and the triggering message takes the processMessage retry path — avoiding
// double-writing a row that was appended to a batch whose Send failed.
func (w *Worker) flushEventBatchLocked(ctx context.Context) error {
	if w.eventBatch == nil || w.eventBatchRows == 0 {
		w.eventAcks = nil
		return nil
	}
	if err := w.eventBatch.Send(); err != nil {
		w.eventBatch = nil
		w.eventBatchRows = 0
		w.eventAcks = nil
		return err
	}
	w.eventBatch = nil
	w.eventBatchRows = 0
	acks := w.eventAcks
	w.eventAcks = nil
	w.ackSent(ctx, acks)
	return nil
}

func (w *Worker) flushPaymentBatch(ctx context.Context) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.flushPaymentBatchLocked(ctx)
}

func (w *Worker) flushPaymentBatchLocked(ctx context.Context) error {
	if w.paymentBatch == nil || w.paymentBatchRows == 0 {
		w.paymentAcks = nil
		return nil
	}
	if err := w.paymentBatch.Send(); err != nil {
		w.paymentBatch = nil
		w.paymentBatchRows = 0
		w.paymentAcks = nil
		return err
	}
	w.paymentBatch = nil
	w.paymentBatchRows = 0
	acks := w.paymentAcks
	w.paymentAcks = nil
	w.commitRevenue(acks)
	w.ackSent(ctx, acks)
	return nil
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

func (w *Worker) commitRevenue(acks []ackRef) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	for _, ack := range acks {
		if ack.rev == nil {
			continue
		}
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

func (w *Worker) flushBatches(ctx context.Context) error {
	if err := w.flushEventBatch(ctx); err != nil {
		return err
	}
	if err := w.flushPaymentBatch(ctx); err != nil {
		return err
	}
	return nil
}
