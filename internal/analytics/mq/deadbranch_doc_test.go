package mq

import (
	"testing"
)

// The tests below document the reachability of the factory branches.
//
//  1. newRedisFromEnv returns a real error for an unparseable REDIS_URL
//     (DATA-043 fail-fast), so factory.go's (err != nil) branch is reachable
//     in production; the (q == nil) guard stays defensive.
//  2. factory.go:35/38: newKafkaFromEnv's only return statement is `return q, nil`;
//     NewKafka returns NewNoop() (non-nil) when brokers is empty and
//     otherwise returns &kafkaQueue{} (non-nil), so both branches are equally unreachable.
//  3. kafka_pub.go:50/55: kafka-go v0.4.50's Writer.Close unconditionally
//     `return nil`s, so the Close error branch cannot be triggered.
//
// TestNewRedisFromEnv_InvalidURLFails pins the DATA-043 contract: an
// unparseable address surfaces as an initialization error from the publisher
// factory instead of a silent noop that would acknowledge publishes while
// dropping every event. Empty-write mode is only reachable through the
// explicit ANALYTICS_MQ_TYPE=noop factory branch (factory_test.go).
func TestNewRedisFromEnv_InvalidURLFails(t *testing.T) {
	t.Setenv("REDIS_URL", "://not-a-redis-url")
	q, err := newRedisFromEnv()
	if err == nil {
		t.Fatalf("invalid REDIS_URL must fail construction, got queue %T", q)
	}
	if q != nil {
		t.Fatalf("error path must return nil queue, got %T", q)
	}
}

func TestNewKafkaFromEnv_NeverReturnsErrorOrNil(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "   ")
	q, err := newKafkaFromEnv()
	if err != nil {
		t.Fatalf("newKafkaFromEnv must never return an error, got %v", err)
	}
	if q == nil {
		t.Fatal("newKafkaFromEnv must never return a nil queue with nil error")
	}
	if _, ok := q.(*kafkaQueue); !ok {
		t.Fatalf("blank brokers fall back to default brokers, want *kafkaQueue, got %T", q)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("cleanup Close: %v", err)
	}
}

func TestKafkaQueue_CloseAlwaysNil(t *testing.T) {
	q := NewKafka([]string{"127.0.0.1:19092"}, "t.events", "t.payments")
	if err := q.Close(); err != nil {
		t.Fatalf("kafka Writer.Close in kafka-go v0.4.50 always returns nil, got %v", err)
	}
	// A second Close is still nil: the closeOnce semantics of Writer.Close.
	if err := q.Close(); err != nil {
		t.Fatalf("second Close should also be nil, got %v", err)
	}
}
