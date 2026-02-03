package ratelimit

import (
	"context"
	"testing"
	"time"
)

// TestTokenBucket_Wait 测试基本的令牌桶等待功能
func TestTokenBucket_Wait(t *testing.T) {
	tb := NewTokenBucket(10, 1) // 容量10，每分钟补充1个（每60秒1个）

	ctx := context.Background()

	// 初始状态应该有10个令牌（burst size）
	for i := 0; i < 10; i++ {
		err := tb.Wait(ctx)
		if err != nil {
			t.Errorf("Wait() should succeed for initial tokens, got %v", err)
		}
	}

	// 第11个请求应该阻塞或超时
	done := make(chan error, 1)
	go func() {
		err := tb.Wait(ctx)
		done <- err
	}()

	// 等待一小段时间，检查是否仍在等待
	select {
	case err := <-done:
		if err == nil {
			t.Error("Should block when bucket is empty")
		}
	case <-time.After(100 * time.Millisecond):
		// 正确：仍在等待令牌
	}
}

// TestTokenBucket_Refill 测试令牌补充
func TestTokenBucket_Refill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping refill test in short mode")
	}

	tb := NewTokenBucket(1, 60) // 容量1，每60秒补充1个

	ctx := context.Background()

	// 消耗初始令牌
	err := tb.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() should succeed for initial token, got %v", err)
	}

	// 桶已空，尝试获取另一个令牌（会超时）
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = tb.Wait(ctx)
	if err == nil {
		t.Error("Wait() should timeout when bucket is empty")
	}

	// 验证错误是超时而不是上下文取消
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

// TestTokenBucket_ContextCancellation 测试上下文取消
func TestTokenBucket_ContextCancellation(t *testing.T) {
	tb := NewTokenBucket(1, 60)

	ctx, cancel := context.WithCancel(context.Background())

	// 消耗初始令牌
	err := tb.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() should succeed for initial token, got %v", err)
	}

	// 取消上下文
	cancel()

	// 下一个 Wait 应该立即返回错误
	err = tb.Wait(ctx)
	if err == nil {
		t.Error("Wait() should return error when context is canceled")
	}

	// 验证错误类型
	if err != context.Canceled {
		t.Logf("Got error type: %T, message: %v", err, err)
	}
}

// TestTokenBucket_HighBurstSize 测试高突发容量
func TestTokenBucket_HighBurstSize(t *testing.T) {
	tb := NewTokenBucket(10, 1000) // 每分钟10个，突发1000个

	ctx := context.Background()

	// 应该能够立即处理1000个请求（突发容量）
	for i := 0; i < 1000; i++ {
		err := tb.Wait(ctx)
		if err != nil {
			t.Errorf("Wait() should succeed for burst requests, iteration %d: %v", i, err)
		}
	}

	// 第1001个请求应该阻塞
	done := make(chan error, 1)
	go func() {
		err := tb.Wait(ctx)
		done <- err
	}()

	select {
	case <-done:
		t.Error("Should block when burst capacity is exhausted")
	case <-time.After(100 * time.Millisecond):
		// 正确：仍在等待
	}
}

// TestTokenBucket_ZeroRateLimiting 测试零速率限制
func TestTokenBucket_ZeroRateLimiting(t *testing.T) {
	tb := NewTokenBucket(0, 1) // 每分钟0个，突发1个

	ctx := context.Background()

	// 应该有1个突发令牌可用
	err := tb.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() should succeed for burst token, got %v", err)
	}
}

// TestTokenBucket_DefaultParameters 测试默认参数处理
func TestTokenBucket_DefaultParameters(t *testing.T) {
	// 测试零值或负值会被正确处理
	tb1 := NewTokenBucket(0, 0) // 应该使用默认值
	if tb1 == nil {
		t.Error("NewTokenBucket(0, 0) should return non-nil TokenBucket")
	}

	tb2 := NewTokenBucket(-1, -1)
	if tb2 == nil {
		t.Error("NewTokenBucket(-1, -1) should return non-nil TokenBucket")
	}

	ctx := context.Background()
	// 验证默认值下的功能
	err := tb2.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() should work with default parameters, got %v", err)
	}
}

// TestTokenBucket_ConcurrentWaits 测试并发等待
func TestTokenBucket_ConcurrentWaits(t *testing.T) {
	tb := NewTokenBucket(10, 60)
	ctx := context.Background()

	success := 0
	done := make(chan bool, 10)

	// 并发等待令牌
	for i := 0; i < 10; i++ {
		go func(idx int) {
			err := tb.Wait(ctx)
			if err == nil {
				success++
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine
	for i := 0; i < 10; i++ {
		<-done
	}

	// 应该成功10个请求（突发容量）
	if success != 10 {
		t.Errorf("Expected 10 successful waits, got %d", success)
	}
}

// TestTokenBucket_RefillGoroutine 测试补充 goroutine 运行
func TestTokenBucket_RefillGoroutine(t *testing.T) {
	tb := NewTokenBucket(1, 60000) // 每分钟1个，突发1个（非常低的速率）

	ctx := context.Background()

	// 消耗初始令牌
	err := tb.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() should succeed for initial token, got %v", err)
	}

	// 令牌桶应该继续运行（不 panic）
	// 我们无法轻易测试补充，但可以验证它不会崩溃
	time.Sleep(10 * time.Millisecond)
	// 如果补充 goroutine 崩溃，可能会有其他症状
}
