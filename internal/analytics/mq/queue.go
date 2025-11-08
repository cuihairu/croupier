package mq

// Queue defines a minimal interface to publish analytics messages to MQ.
// Implementations can be backed by Kafka, Redis Streams, or a no-op for dev.
type Queue interface {
    PublishEvent(evt map[string]any) error
    PublishPayment(pay map[string]any) error
    Close() error
}

