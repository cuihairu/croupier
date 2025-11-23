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