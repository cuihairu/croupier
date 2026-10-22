package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokenBucket_BasicAcquire(t *testing.T) {
	tb := NewTokenBucket(60, 5)

	// Should be able to acquire up to burstSize tokens immediately
	for i := 0; i < 5; i++ {
		err := tb.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected to acquire token %d, got error: %v", i, err)
		}
	}
}

func TestTokenBucket_WaitFailFastWhenDeadlineEarly(t *testing.T) {
	tb := NewTokenBucket(60, 1)

	// Acquire the only token
	err := tb.Wait(context.Background())
	if err != nil {
		t.Fatalf("Expected to acquire first token, got error: %v", err)
	}

	// x/time/rate 语义（与旧 channel 桶的差异点）：ctx 截止早于下一枚令牌时刻
	// 时 Wait 不空等、快速失败（旧实现干等 10ms 后返回 DeadlineExceeded）。
	// 断言不碰墙上时延（慢机不安全）：若实现退化为「阻塞到截止」必然以
	// context.DeadlineExceeded 收场，故以错误类型区分两种语义。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = tb.Wait(ctx)
	if err == nil {
		t.Error("Expected fail-fast error, got nil")
	} else if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("x/time/rate should fail fast, not block until deadline: %v", err)
	}
}

func TestTokenBucket_ContextCancellation(t *testing.T) {
	tb := NewTokenBucket(60, 1)

	// Acquire the only token
	err := tb.Wait(context.Background())
	if err != nil {
		t.Fatalf("Expected to acquire first token, got error: %v", err)
	}

	// Cancel context while waiting
	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine that will cancel after a short delay
	done := make(chan error, 1)
	go func() {
		time.AfterFunc(10*time.Millisecond, func() {
			cancel()
		})
		done <- tb.Wait(ctx)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected error when context is canceled, got nil")
		} else if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v (type: %T)", err, err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Test timed out waiting for Wait to return")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	// Create a bucket with 1 token per second
	tb := NewTokenBucket(60, 1)

	// Acquire the only token
	err := tb.Wait(context.Background())
	if err != nil {
		t.Fatalf("Expected to acquire first token, got error: %v", err)
	}

	// Wait for refill (should refill within ~1 second for 60 requests/min)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = tb.Wait(ctx)
	if err != nil {
		t.Errorf("Expected to acquire token after refill, got error: %v", err)
	}
}

func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	tb := NewTokenBucket(60, 10)

	// Try to acquire more tokens than available
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	successCount := 0
	for i := 0; i < 20; i++ {
		if tb.Wait(ctx) == nil {
			successCount++
		}
	}

	// Should only acquire up to burstSize
	if successCount != 10 {
		t.Errorf("Expected to acquire 10 tokens, got %d", successCount)
	}
}

func TestNewTokenBucket_ZeroParameters(t *testing.T) {
	tb := NewTokenBucket(0, 0)

	// Should use default values
	if tb == nil {
		t.Fatal("Expected non-nil TokenBucket")
	}

	// Should be able to acquire at least 1 token
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := tb.Wait(ctx)
	if err != nil {
		t.Errorf("Expected to acquire token with defaults, got error: %v", err)
	}
}
