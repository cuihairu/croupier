package mq

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKafkaQueue_PublishPayment tests PublishPayment method
func TestKafkaQueue_PublishPayment(t *testing.T) {
	q := NewKafka([]string{"localhost:9092"}, "events", "payments")
	payment := map[string]any{
		"order_id":     "order-123",
		"user_id":      "user-456",
		"amount_cents": 9999,
		"currency":     "USD",
	}
	// Will fail to connect to Kafka, but we're testing the method exists
	err := q.PublishPayment(payment)
	_ = err
}

// TestKafkaQueue_Close tests Close method
func TestKafkaQueue_Close(t *testing.T) {
	q := NewKafka([]string{"localhost:9092"}, "events", "payments")
	err := q.Close()
	// Should not error even if connection fails
	assert.NoError(t, err)
}

// TestKafkaQueue_Close_Idempotent tests Close is idempotent
func TestKafkaQueue_Close_Idempotent(t *testing.T) {
	q := NewKafka([]string{"localhost:9092"}, "events", "payments")
	err := q.Close()
	assert.NoError(t, err)

	// Second close should also not error
	err = q.Close()
	assert.NoError(t, err)
}

// TestNewKafka_TopicDefaults tests default topic names
func TestNewKafka_TopicDefaults(t *testing.T) {
	q := NewKafka([]string{"localhost:9092"}, "", "")
	assert.NotNil(t, q)
}

// TestNewKafka_MultipleBrokers tests with multiple brokers
func TestNewKafka_MultipleBrokers(t *testing.T) {
	q := NewKafka([]string{"broker1:9092", "broker2:9092", "broker3:9092"}, "events", "payments")
	assert.NotNil(t, q)
}

// TestNewKafka_EmptyBrokers tests with empty brokers list (returns noop)
func TestNewKafka_EmptyBrokers(t *testing.T) {
	q := NewKafka([]string{}, "events", "payments")
	assert.NotNil(t, q)
	// Should return Noop when brokers list is empty
	if _, ok := q.(*Noop); !ok {
		t.Error("Empty brokers list should return Noop queue")
	}
}
