// 覆盖目标：WaitWithTimeout 的 Allow 错误（closed limiter）、cleanup 的空窗口
// 回收、Distributed AllowN 的本地错误/store 回退/Reset 错误、TokenBucket
// 最小 burst 与满桶 default 分支。
package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestV8SlidingWindow_WaitWithTimeout_AllowError(t *testing.T) {
	sw := NewSlidingWindowLimiter(10, time.Minute)
	if err := sw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	err := sw.WaitWithTimeout(context.Background(), "k", time.Second)
	if !errors.Is(err, ErrLimiterClosed) {
		t.Fatalf("expected ErrLimiterClosed, got %v", err)
	}
}

func TestV8SlidingWindow_CleanupRemovesEmptyWindow(t *testing.T) {
	sw := NewSlidingWindowLimiter(5, 20*time.Millisecond)
	defer func() { _ = sw.Close() }()

	// n=0 不追加时间戳，制造空窗口；cleanupInterval=40ms，等待回收。
	if _, err := sw.AllowN(context.Background(), "empty-key", 0); err != nil {
		t.Fatalf("AllowN(n=0): %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sw.mu.RLock()
		_, exists := sw.windows["empty-key"]
		sw.mu.RUnlock()
		if !exists {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("empty window was not cleaned up")
}

// v8ClosedLimiter 实现 AdvancedLimiter，所有调用返回 ErrLimiterClosed。
type v8ClosedLimiter struct{}

func (v8ClosedLimiter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	return nil, ErrLimiterClosed
}
func (v8ClosedLimiter) AllowN(ctx context.Context, key string, n int) (*RateLimitResult, error) {
	return nil, ErrLimiterClosed
}
func (v8ClosedLimiter) Wait(ctx context.Context, key string) error { return ErrLimiterClosed }
func (v8ClosedLimiter) WaitWithTimeout(ctx context.Context, key string, timeout time.Duration) error {
	return ErrLimiterClosed
}
func (v8ClosedLimiter) Reset(ctx context.Context, key string) error { return nil }
func (v8ClosedLimiter) GetStats(key string) *RateLimitStats         { return nil }

func TestV8Distributed_AllowN_LocalError(t *testing.T) {
	d := NewDistributedRateLimiter(newMemStore(), 5, time.Minute)
	d.local = v8ClosedLimiter{}
	_, err := d.AllowN(context.Background(), "k", 1)
	if err == nil {
		t.Fatal("expected local limiter error to propagate")
	}
}

func TestV8Distributed_AllowN_StoreErrorFallsBackLocal(t *testing.T) {
	store := newMemStore()
	store.err = errors.New("store down")
	d := NewDistributedRateLimiter(store, 5, time.Minute)

	res, err := d.AllowN(context.Background(), "k", 1)
	if err != nil {
		t.Fatalf("AllowN: %v", err)
	}
	if !res.Allowed {
		t.Fatal("expected local decision fallback to allow")
	}
}

func TestV8Distributed_Reset_StoreError(t *testing.T) {
	store := newMemStore()
	store.err = errors.New("store down")
	d := NewDistributedRateLimiter(store, 5, time.Minute)
	err := d.Reset(context.Background(), "k")
	if err == nil {
		t.Fatal("expected store reset error to propagate")
	}
}

func TestV8Distributed_AllowN_DistributedDenySetsRetryAfter(t *testing.T) {
	store := newMemStore()
	store.inc["k"] = 10 // 分布式计数已超限
	d := NewDistributedRateLimiter(store, 5, time.Minute)

	res, err := d.AllowN(context.Background(), "k", 1)
	if err != nil {
		t.Fatalf("AllowN: %v", err)
	}
	if res.Allowed {
		t.Fatal("expected distributed count to deny the request")
	}
	if res.RetryAfter <= 0 {
		t.Fatalf("expected positive RetryAfter, got %v", res.RetryAfter)
	}
}

func TestV8SlidingWindow_CleanupRemovesExpiredWindow(t *testing.T) {
	sw := NewSlidingWindowLimiter(5, 20*time.Millisecond)
	defer func() { _ = sw.Close() }()

	if _, err := sw.Allow(context.Background(), "stale-key"); err != nil {
		t.Fatalf("Allow: %v", err)
	}

	// 等待时间戳过期（>20ms）后由 cleanup（40ms 间隔）按 allOld 分支回收。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sw.mu.RLock()
		_, exists := sw.windows["stale-key"]
		sw.mu.RUnlock()
		if !exists {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expired window was not cleaned up")
}

func TestV8TokenBucket_MinimalBurst(t *testing.T) {
	tb := NewTokenBucket(30, 0)
	// burst 归一化：0 → max(rpm/60,1)=1（与旧 channel 桶容量语义一致）。
	if tb.lim.Burst() != 1 {
		t.Fatalf("expected minimal burst capacity 1, got %d", tb.lim.Burst())
	}
	// 桶初始有一个令牌，可立即取到。
	if err := tb.Wait(context.Background()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestV8TokenBucket_LimitAndBurstMapping(t *testing.T) {
	// 旧实现靠 refill goroutine 的 default 丢弃分支封顶桶容量；x/time/rate 以
	// burst 参数硬封顶、按 limit 连续记账，映射为 rpm/60 每秒速率。
	tb := NewTokenBucket(60000, 5)
	if got := tb.lim.Limit(); got != 1000 {
		t.Fatalf("expected limit 1000/s (60000rpm), got %v", got)
	}
	if tb.lim.Burst() != 5 {
		t.Fatalf("unexpected burst capacity %d", tb.lim.Burst())
	}
}
