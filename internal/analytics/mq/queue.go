package mq

import "context"

// Queue defines a minimal interface to publish analytics messages to MQ.
// Implementations can be backed by Kafka, Redis Streams, or a no-op for dev.
type Queue interface {
	PublishEvent(evt map[string]any) error
	PublishPayment(pay map[string]any) error
	Close() error
}

// StreamBacklog is the DATA-047 split reading behind one stream's "backlog":
// Retained is the physical entry count still kept by the broker (Redis XLEN —
// consumed entries stay until the trim policy drops them, so the physical
// length is not the backlog); Undelivered counts entries not yet handed to the
// consumer group (lag); PendingAck counts entries delivered but not
// acknowledged (PEL). A nil pointer means the broker could not provide a
// valid reading — displayed as unknown, never collapsed into a fake zero.
type StreamBacklog struct {
	Retained    int64
	Undelivered *int64
	PendingAck  *int64
}

// QueueBacklog carries the per-stream readings for the two ingest streams.
type QueueBacklog struct {
	Events   StreamBacklog
	Payments StreamBacklog
}

// BacklogReporter is implemented by queues that can report consumer-group
// progress (DATA-047). The ingest backlog sampler prefers it; queues without
// it report an unknown backlog instead of a fake zero.
type BacklogReporter interface {
	Backlog(ctx context.Context) (QueueBacklog, error)
}
