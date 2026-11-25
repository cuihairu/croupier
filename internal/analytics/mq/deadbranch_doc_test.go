package mq

import (
	"testing"
)

// The tests below document the unreachable defensive branches in
// factory.go / kafka_pub.go:
//
//  1. factory.go:26/29: newRedisFromEnv's only return statement is
//     `return NewRedis(...), nil`, so the error is always nil; NewRedis degrades to
//     NewNoop() (non-nil) when parsing fails, and returns &redisQueue{} on
//     success. Therefore neither the (err != nil) nor the
//     (q == nil) branch can ever be triggered.
//  2. factory.go:35/38: newKafkaFromEnv's only return statement is `return q, nil`;
//     NewKafka returns NewNoop() (non-nil) when brokers is empty and
//     otherwise returns &kafkaQueue{} (non-nil), so both branches are equally unreachable.
//  3. kafka_pub.go:50/55: kafka-go v0.4.50's Writer.Close unconditionally
//     `return nil`s, so the Close error branch cannot be triggered.
func TestNewRedisFromEnv_NeverReturnsErrorOrNil(t *testing.T) {
	t.Setenv("REDIS_URL", "://not-a-redis-url")
	q, err := newRedisFromEnv()
	if err != nil {
		t.Fatalf("newRedisFromEnv must never return an error, got %v", err)
	}
	if q == nil {
		t.Fatal("newRedisFromEnv must never return a nil queue with nil error")
	}
	if _, ok := q.(*Noop); !ok {
		t.Fatalf("invalid REDIS_URL should degrade to Noop inside NewRedis, got %T", q)
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
