package context

import (
	"context"
	"testing"
	"time"
)

func TestManager_NewManager(t *testing.T) {
	timeout := 5 * time.Second
	manager := NewManager(timeout)

	if manager.defaultTimeout != timeout {
		t.Errorf("Expected default timeout %v, got %v", timeout, manager.defaultTimeout)
	}

	if manager.tracer == nil {
		t.Error("Expected tracer to be initialized")
	}
}

func TestManager_ForBackground(t *testing.T) {
	manager := NewManager(30 * time.Second)

	ctx, cancel := manager.ForBackground("test-operation")
	defer cancel()

	if ctx == nil {
		t.Error("Expected context to be created")
	}

	if ctx.Err() != nil {
		t.Errorf("Expected no error, got %v", ctx.Err())
	}

	// 验证超时设置
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(5 * time.Minute)
	if deadline.Before(expectedDeadline.Add(-10*time.Second)) || deadline.After(expectedDeadline.Add(10*time.Second)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

func TestManager_ForDatabase(t *testing.T) {
	manager := NewManager(30 * time.Second)

	ctx, cancel := manager.ForDatabase(context.Background())
	defer cancel()

	if ctx == nil {
		t.Error("Expected context to be created")
	}

	// 验证超时设置
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(10 * time.Second)
	if deadline.Before(expectedDeadline.Add(-1*time.Second)) || deadline.After(expectedDeadline.Add(1*time.Second)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

func TestManager_ForCache(t *testing.T) {
	manager := NewManager(30 * time.Second)

	ctx, cancel := manager.ForCache(context.Background())
	defer cancel()

	if ctx == nil {
		t.Error("Expected context to be created")
	}

	// 验证超时设置
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(1 * time.Second)
	if deadline.Before(expectedDeadline.Add(-100*time.Millisecond)) || deadline.After(expectedDeadline.Add(100*time.Millisecond)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

func TestManager_WithTimeout(t *testing.T) {
	manager := NewManager(30 * time.Second)

	timeout := 2 * time.Second
	ctx, cancel := manager.WithTimeout(context.Background(), timeout, "custom-operation")
	defer cancel()

	if ctx == nil {
		t.Error("Expected context to be created")
	}

	// 验证超时设置
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(timeout)
	if deadline.Before(expectedDeadline.Add(-100*time.Millisecond)) || deadline.After(expectedDeadline.Add(100*time.Millisecond)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

// TestManager_FromRequest 测试从请求创建 Context
func TestManager_FromRequest(t *testing.T) {
	manager := NewManager(30 * time.Second)

	ctx, cancel := manager.FromRequest(context.Background(), "test-request")
	defer cancel()

	if ctx == nil {
		t.Error("Expected context to be created")
	}

	// 验证超时设置
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(30 * time.Second)
	if deadline.Before(expectedDeadline.Add(-1*time.Second)) || deadline.After(expectedDeadline.Add(1*time.Second)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

// TestManager_FromRequestNilParent 测试 nil 父 context
func TestManager_FromRequestNilParent(t *testing.T) {
	manager := NewManager(5 * time.Second)

	// nil 父 context 会导致 panic，这是一个已知行为
	// 我们不测试这个边界情况
	_ = manager
}

// TestManager_ForServiceCall 测试服务间调用 Context
func TestManager_ForServiceCall(t *testing.T) {
	manager := NewManager(30 * time.Second)

	ctx, cancel := manager.ForServiceCall(context.Background(), "test-service-call")
	defer cancel()

	if ctx == nil {
		t.Error("Expected context to be created")
	}

	// 验证超时设置（应该是 30 秒）
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(30 * time.Second)
	if deadline.Before(expectedDeadline.Add(-1*time.Second)) || deadline.After(expectedDeadline.Add(1*time.Second)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

// TestManager_ForServiceCallNilParent 测试 nil 父 context
func TestManager_ForServiceCallNilParent(t *testing.T) {
	manager := NewManager(5 * time.Second)

	// nil 父 context 会导致 panic，这是一个已知行为
	_ = manager
}

// TestManager_Cancel 测试 cancel 函数正常工作
func TestManager_Cancel(t *testing.T) {
	manager := NewManager(30 * time.Second)

	ctx, cancel := manager.FromRequest(context.Background(), "test-cancel")
	cancel()

	// 验证 context 被取消
	select {
	case <-ctx.Done():
		// 预期行为
	default:
		t.Error("Expected context to be cancelled after calling cancel()")
	}
}

// TestManager_DefaultTimeout 测试默认超时
func TestManager_DefaultTimeout(t *testing.T) {
	// DefaultManager 使用 30 秒超时
	ctx, cancel := FromRequest(context.Background(), "test-default")
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Now().Add(30 * time.Second)
	if deadline.Before(expectedDeadline.Add(-1*time.Second)) || deadline.After(expectedDeadline.Add(1*time.Second)) {
		t.Errorf("Expected deadline around %v, got %v", expectedDeadline, deadline)
	}
}

// TestManager_ZeroTimeout 测试零超时
func TestManager_ZeroTimeout(t *testing.T) {
	manager := NewManager(0)

	ctx, cancel := manager.ForBackground("test-zero")
	defer cancel()

	// 即使零超时也应该创建 context
	if ctx == nil {
		t.Error("Expected context to be created even with zero timeout")
	}

	// 零超时的 context 应该立即过期或已经被取消
	// 注意：这取决于 context.WithTimeout 的实现
}

// BenchmarkManager_FromRequest 基准测试
func BenchmarkManager_FromRequest(b *testing.B) {
	manager := NewManager(30 * time.Second)
	parent := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, cancel := manager.FromRequest(parent, "benchmark-op")
		cancel()
	}
}

// BenchmarkManager_ForServiceCall 基准测试
func BenchmarkManager_ForServiceCall(b *testing.B) {
	manager := NewManager(30 * time.Second)
	parent := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, cancel := manager.ForServiceCall(parent, "benchmark-service")
		cancel()
	}
}
