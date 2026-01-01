package mq

import (
	"testing"
)

// TestNewNoop 测试创建 Noop 发布者
func TestNewNoop(t *testing.T) {
	n := NewNoop()

	if n == nil {
		t.Fatal("NewNoop() should return non-nil instance")
	}
}

// TestNoop_PublishEvent 测试发布事件
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

// TestNoop_PublishEvent_Nil 测试发布 nil 事件
func TestNoop_PublishEvent_Nil(t *testing.T) {
	n := NewNoop()

	err := n.PublishEvent(nil)
	if err != nil {
		t.Errorf("PublishEvent(nil) should not return error, got %v", err)
	}
}

// TestNoop_PublishEvent_Empty 测试发布空事件
func TestNoop_PublishEvent_Empty(t *testing.T) {
	n := NewNoop()

	err := n.PublishEvent(map[string]any{})
	if err != nil {
		t.Errorf("PublishEvent({}) should not return error, got %v", err)
	}
}

// TestNoop_PublishPayment 测试发布支付
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

// TestNoop_PublishPayment_Nil 测试发布 nil 支付
func TestNoop_PublishPayment_Nil(t *testing.T) {
	n := NewNoop()

	err := n.PublishPayment(nil)
	if err != nil {
		t.Errorf("PublishPayment(nil) should not return error, got %v", err)
	}
}

// TestNoop_Close 测试关闭
func TestNoop_Close(t *testing.T) {
	n := NewNoop()

	err := n.Close()
	if err != nil {
		t.Errorf("Close() should not return error, got %v", err)
	}

	// 多次关闭应该安全
	err = n.Close()
	if err != nil {
		t.Errorf("Close() called twice should not return error, got %v", err)
	}
}

// TestNoop_PendingEvents 测试获取待处理事件数
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

// TestNoop_PendingPayments 测试获取待处理支付数
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

// TestNoop_ConcurrentOperations 测试并发操作
func TestNoop_ConcurrentOperations(t *testing.T) {
	n := NewNoop()
	done := make(chan bool, 10)

	// 并发发布事件
	for i := 0; i < 5; i++ {
		go func() {
			n.PublishEvent(map[string]any{"id": i})
			done <- true
		}()
	}

	// 并发发布支付
	for i := 0; i < 5; i++ {
		go func() {
			n.PublishPayment(map[string]any{"id": i})
			done <- true
		}()
	}

	// 等待所有 goroutine
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证没有错误
	if count, _ := n.PendingEvents(); count != 0 {
		t.Errorf("PendingEvents() should return 0, got %d", count)
	}

	if count, _ := n.PendingPayments(); count != 0 {
		t.Errorf("PendingPayments() should return 0, got %d", count)
	}
}

// TestNoop_SequentialOperations 测试顺序操作
func TestNoop_SequentialOperations(t *testing.T) {
	n := NewNoop()

	// 发布多个事件
	for i := 0; i < 100; i++ {
		err := n.PublishEvent(map[string]any{"seq": i})
		if err != nil {
			t.Errorf("PublishEvent() iteration %d should not return error, got %v", i, err)
		}
	}

	// 发布多个支付
	for i := 0; i < 100; i++ {
		err := n.PublishPayment(map[string]any{"seq": i})
		if err != nil {
			t.Errorf("PublishPayment() iteration %d should not return error, got %v", i, err)
		}
	}

	// 验证计数
	if count, _ := n.PendingEvents(); count != 0 {
		t.Errorf("PendingEvents() should return 0 after 100 publishes, got %d", count)
	}

	if count, _ := n.PendingPayments(); count != 0 {
		t.Errorf("PendingPayments() should return 0 after 100 publishes, got %d", count)
	}
}

// BenchmarkPublishEvent 性能基准测试
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
		n.PublishEvent(event)
	}
}

// BenchmarkPublishPayment 性能基准测试
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
		n.PublishPayment(payment)
	}
}

// BenchmarkPendingEvents 性能基准测试
func BenchmarkPendingEvents(b *testing.B) {
	n := NewNoop()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.PendingEvents()
	}
}

// BenchmarkConcurrentPublish 性能基准测试 - 并发
func BenchmarkConcurrentPublish(b *testing.B) {
	n := NewNoop()
	event := map[string]any{"id": 1}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n.PublishEvent(event)
		}
	})
}
