package mq

type Noop struct{}

func NewNoop() *Noop                                { return &Noop{} }
func (n *Noop) PublishEvent(map[string]any) error   { return nil }
func (n *Noop) PublishPayment(map[string]any) error { return nil }
func (n *Noop) Close() error                        { return nil }
func (n *Noop) PendingEvents() (int64, error)       { return 0, nil }
func (n *Noop) PendingPayments() (int64, error)     { return 0, nil }
