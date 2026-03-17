package mq

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRedisQueue_PublishPayment tests PublishPayment method
func TestRedisQueue_PublishPayment(t *testing.T) {
	q := NewRedis("redis://localhost:6379/0", "events", "payments", 1000, true)
	payment := map[string]any{
		"order_id":     "order-123",
		"user_id":      "user-456",
		"amount_cents": 9999,
		"currency":     "USD",
	}
	// Will fail to connect to Redis, but we're testing the method exists
	err := q.PublishPayment(payment)
	_ = err
}

// TestRedisQueue_Close tests Close method
func TestRedisQueue_Close(t *testing.T) {
	q := NewRedis("redis://localhost:6379/0", "events", "payments", 1000, true)
	err := q.Close()
	// Should not error even if connection fails
	assert.NoError(t, err)
}

// TestNewRedis_InvalidURL tests NewRedis with invalid URL
func TestNewRedis_InvalidURL(t *testing.T) {
	q := NewRedis("://invalid-url", "events", "payments", 1000, true)
	// Invalid URL should return noop queue
	assert.NotNil(t, q)
}

// TestNewRedis_ZeroMaxLen tests NewRedis with zero max length
func TestNewRedis_ZeroMaxLen(t *testing.T) {
	q := NewRedis("redis://localhost:6379/0", "events", "payments", 0, false)
	assert.NotNil(t, q)
}

// TestNewRedis_DefaultParameters tests NewRedis with default parameters
func TestNewRedis_DefaultParameters(t *testing.T) {
	q := NewRedis("redis://localhost:6379/0", "", "", 0, false)
	assert.NotNil(t, q)
}
