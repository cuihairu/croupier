package mq

import (
	"testing"
)

// TestNewNoop verifies that creating a Noop publisher works
func TestNewNoop(t *testing.T) {
	n := NewNoop()

	if n == nil {
		t.Fatal("NewNoop() should return non-nil instance")
	}
}

// TestNoop_PublishEvent verifies publishing an event
func TestNoop_PublishEvent(t *testing.T) {
	n := NewNoop()

	event := map[string]any{
		"event_id": "test-123",
		"user_id":  "user-456",
		"event":    "test_event",
	}

	err := n.PublishEvent(event)
	if err != nil {
		t.Errorf("PublishEvent() should not return error, got %v", err)
	}
}

// TestNoop_PublishEvent_Nil verifies publishing a nil event
func TestNoop_PublishEvent_Nil(t *testing.T) {
	n := NewNoop()

	err := n.PublishEvent(nil)
	if err != nil {
		t.Errorf("PublishEvent(nil) should not return error, got %v", err)
	}
}

// TestNoop_PublishEvent_Empty verifies publishing an empty event
func TestNoop_PublishEvent_Empty(t *testing.T) {
	n := NewNoop()

	err := n.PublishEvent(map[string]any{})
	if err != nil {
		t.Errorf("PublishEvent({}) should not return error, got %v", err)
	}
}

// TestNoop_PublishPayment verifies publishing a payment
func TestNoop_PublishPayment(t *testing.T) {
	n := NewNoop()

	payment := map[string]any{
		"order_id":     "order-789",
		"user_id":      "user-456",
		"amount_cents": 9999,
		"currency":     "USD",
	}

	err := n.PublishPayment(payment)
	if err != nil {
		t.Errorf("PublishPayment() should not return error, got %v", err)
	}
}

// TestNoop_PublishPayment_Nil verifies publishing a nil payment
func TestNoop_PublishPayment_Nil(t *testing.T) {
	n := NewNoop()

	err := n.PublishPayment(nil)
	if err != nil {
		t.Errorf("PublishPayment(nil) should not return error, got %v", err)
	}
}

// TestNoop_Close verifies closing
func TestNoop_Close(t *testing.T) {
	n := NewNoop()

	err := n.Close()
	if err != nil {
		t.Errorf("Close() should not return error, got %v", err)
	}

	// Multiple closes should be safe
	err = n.Close()
	if err != nil {
		t.Errorf("Close() called twice should not return error, got %v", err)
	}
}

// TestNoop_PendingEvents verifies getting the pending events count
func TestNoop_PendingEvents(t *testing.T) {
	n := NewNoop()

	count, err := n.PendingEvents()
	if err != nil {
		t.Errorf("PendingEvents() should not return error, got %v", err)
	}

	if count != 0 {
		t.Errorf("PendingEvents() should return 0, got %d", count)
	}
}

// TestNoop_PendingPayments verifies getting the pending payments count
func TestNoop_PendingPayments(t *testing.T) {
	n := NewNoop()

	count, err := n.PendingPayments()
	if err != nil {
		t.Errorf("PendingPayments() should not return error, got %v", err)
	}

	if count != 0 {
		t.Errorf("PendingPayments() should return 0, got %d", count)
	}
}

// TestNoop_ConcurrentOperations verifies concurrent operations
func TestNoop_ConcurrentOperations(t *testing.T) {
	n := NewNoop()
	done := make(chan bool, 10)

	// Publish events concurrently
	for i := 0; i < 5; i++ {
		go func() {
			_ = n.PublishEvent(map[string]any{"id": i})
			done <- true
		}()
	}

	// Publish payments concurrently
	for i := 0; i < 5; i++ {
		go func() {
			_ = n.PublishPayment(map[string]any{"id": i})
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no errors occurred
	if count, _ := n.PendingEvents(); count != 0 {
		t.Errorf("PendingEvents() should return 0, got %d", count)
	}

	if count, _ := n.PendingPayments(); count != 0 {
		t.Errorf("PendingPayments() should return 0, got %d", count)
	}
}

// TestNoop_SequentialOperations verifies sequential operations
func TestNoop_SequentialOperations(t *testing.T) {
	n := NewNoop()

	// Publish multiple events
	for i := 0; i < 100; i++ {
		err := n.PublishEvent(map[string]any{"seq": i})
		if err != nil {
			t.Errorf("PublishEvent() iteration %d should not return error, got %v", i, err)
		}
	}

	// Publish multiple payments
	for i := 0; i < 100; i++ {
		err := n.PublishPayment(map[string]any{"seq": i})
		if err != nil {
			t.Errorf("PublishPayment() iteration %d should not return error, got %v", i, err)
		}
	}

	// Verify the counts
	if count, _ := n.PendingEvents(); count != 0 {
		t.Errorf("PendingEvents() should return 0 after 100 publishes, got %d", count)
	}

	if count, _ := n.PendingPayments(); count != 0 {
		t.Errorf("PendingPayments() should return 0 after 100 publishes, got %d", count)
	}
}

// BenchmarkPublishEvent is a benchmark for its performance
func BenchmarkPublishEvent(b *testing.B) {
	n := NewNoop()
	event := map[string]any{
		"event_id": "bench-123",
		"user_id":  "bench-user",
		"event":    "bench_event",
		"data":     make([]byte, 256),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = n.PublishEvent(event)
	}
}

// BenchmarkPublishPayment is a benchmark for its performance
func BenchmarkPublishPayment(b *testing.B) {
	n := NewNoop()
	payment := map[string]any{
		"order_id":     "bench-order",
		"user_id":      "bench-user",
		"amount_cents": 10000,
		"currency":     "USD",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = n.PublishPayment(payment)
	}
}

// BenchmarkPendingEvents is a benchmark for its performance
func BenchmarkPendingEvents(b *testing.B) {
	n := NewNoop()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = n.PendingEvents()
	}
}

// BenchmarkConcurrentPublish is a benchmark for its performance - concurrent
func BenchmarkConcurrentPublish(b *testing.B) {
	n := NewNoop()
	event := map[string]any{"id": 1}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = n.PublishEvent(event)
		}
	})
}
