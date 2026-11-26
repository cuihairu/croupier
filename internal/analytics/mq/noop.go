package mq

import "context"

type Noop struct{}

func NewNoop() *Noop                                { return &Noop{} }
func (n *Noop) PublishEvent(map[string]any) error   { return nil }
func (n *Noop) PublishPayment(map[string]any) error { return nil }
func (n *Noop) Close() error                        { return nil }

// Backlog reports true zeros (DATA-047): the noop queue never enqueues
// anything, so empty streams are a real reading, not a placeholder.
func (n *Noop) Backlog(ctx context.Context) (QueueBacklog, error) {
	zero := int64(0)
	known := StreamBacklog{Retained: 0, Undelivered: &zero, PendingAck: &zero}
	return QueueBacklog{Events: known, Payments: known}, nil
}
