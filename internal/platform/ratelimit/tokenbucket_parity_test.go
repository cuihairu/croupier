package ratelimit

import (
	"context"
	"testing"
	"time"
)

// legacyTokenBucket 是换库前的 channel 令牌桶原样副本，仅作对拍基线（能力矩阵
// #2 Batch B 验收项：新旧放行序列对拍）。生产实现已换 x/time/rate
// （tokenbucket.go），此副本禁止回迁生产代码。
type legacyTokenBucket struct {
	tokens     chan struct{}
	refillRate time.Duration
}

func newLegacyTokenBucket(requestsPerMinute, burstSize int) *legacyTokenBucket {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burstSize <= 0 {
		burstSize = requestsPerMinute / 60
		if burstSize < 1 {
			burstSize = 1
		}
	}
	tb := &legacyTokenBucket{
		tokens:     make(chan struct{}, burstSize),
		refillRate: time.Minute / time.Duration(requestsPerMinute),
	}
	for i := 0; i < burstSize; i++ {
		tb.tokens <- struct{}{}
	}
	go tb.refill()
	return tb
}

func (tb *legacyTokenBucket) refill() {
	ticker := time.NewTicker(tb.refillRate)
	defer ticker.Stop()
	for range ticker.C {
		select {
		case tb.tokens <- struct{}{}:
		default:
			// Bucket is full, discard token
		}
	}
}

func (tb *legacyTokenBucket) tryAcquire() bool {
	select {
	case <-tb.tokens:
		return true
	default:
		return false
	}
}

// TestTokenBucketParity 相同 (rpm,burst) 下新旧放行序列语义一致：
// 即时突发 burst 个 → 突发耗尽后一个 refill 间隔内不放行 → 间隔过后恢复 ≥1 个。
// 断言只取慢机安全的方向——时间流逝只会让「更多令牌可用」，不会让已耗尽的桶
// 提前回满；拒绝方向的余量为 refill 间隔 1s 与探测窗 50ms 之差（950ms 调度量级）。
func TestTokenBucketParity(t *testing.T) {
	legacy := newLegacyTokenBucket(60, 3)
	tb := NewTokenBucket(60, 3)
	ctx := context.Background()

	// 即时突发：burst=3 个全部放行（两实现一致）。
	for i := 0; i < 3; i++ {
		if err := tb.Wait(ctx); err != nil {
			t.Fatalf("new bucket burst wait %d: %v", i, err)
		}
		if !legacy.tryAcquire() {
			t.Fatalf("legacy bucket burst token %d missing", i)
		}
	}

	// 突发耗尽后 50ms 探测窗内两实现都不许回满（refill 间隔 1s）。
	ctx50, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := tb.Wait(ctx50); err == nil {
		t.Fatal("new bucket should not refill within 50ms at 60rpm")
	}
	if legacy.tryAcquire() {
		t.Fatal("legacy bucket should not refill within 50ms at 60rpm")
	}

	// 一个 refill 间隔过后（≥1.15s）两实现都恢复 ≥1 个令牌。
	time.Sleep(1150 * time.Millisecond)
	if !tb.Allow() {
		t.Fatal("new bucket should have a token after 1.15s at 60rpm")
	}
	if !legacy.tryAcquire() {
		t.Fatal("legacy bucket should have a token after 1.15s at 60rpm")
	}
}
