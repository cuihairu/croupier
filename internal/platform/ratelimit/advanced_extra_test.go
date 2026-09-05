package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 内存版 RateLimitStore，可注入错误。
type memStore struct {
	err error
	inc map[string]int
}

func newMemStore() *memStore { return &memStore{inc: map[string]int{}} }

func (m *memStore) Increment(ctx context.Context, key string, window time.Duration) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.inc[key]++
	return m.inc[key], nil
}
func (m *memStore) Get(ctx context.Context, key string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.inc[key], nil
}
func (m *memStore) Reset(ctx context.Context, key string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.inc, key)
	return nil
}
func (m *memStore) SetNX(ctx context.Context, key string, value int, ttl time.Duration) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return true, nil
}

func TestDistributedRateLimiterWithStore(t *testing.T) {
	store := newMemStore()
	d := NewDistributedRateLimiter(store, 2, time.Minute)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		res, err := d.Allow(ctx, "k")
		if err != nil || !res.Allowed {
			t.Fatalf("request %d should pass: %+v %v", i, res, err)
		}
	}
	res, err := d.Allow(ctx, "k")
	if err != nil || res.Allowed {
		t.Fatalf("third request should be blocked: %+v %v", res, err)
	}
	if err := d.Reset(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if res, err = d.Allow(ctx, "k"); err != nil || !res.Allowed {
		t.Fatalf("after reset should pass: %+v %v", res, err)
	}

	// store 错误 → 回落本地判定（不透传错误）
	store.err = errors.New("store boom")
	if res, err := d.Allow(ctx, "k"); err != nil {
		t.Fatalf("store failure should fall back to local decision: %v", err)
	} else if !res.Allowed {
		t.Fatalf("after reset+store failure local should allow: %+v", res)
	}
	store.err = nil

	// 分布式 Wait / WaitWithTimeout 委托本地限流器
	if err := d.Wait(ctx, "dw"); err != nil {
		t.Fatal(err)
	}
	if err := d.WaitWithTimeout(ctx, "dw", time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AllowN(ctx, "dn", 1); err != nil {
		t.Fatalf("AllowN error: %v", err)
	}
}

func TestSlidingWindowWaitVariants(t *testing.T) {
	sw := NewSlidingWindowLimiter(1, 50*time.Millisecond)
	ctx := context.Background()
	if err := sw.Wait(ctx, "w"); err != nil {
		t.Fatal(err)
	}
	if err := sw.Wait(ctx, "w"); err != nil {
		t.Fatalf("second Wait should block until window passes: %v", err)
	}
	if err := sw.WaitWithTimeout(ctx, "w", 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// 窗口未过 + 超时小于 RetryAfter → ErrRateLimitExceeded
	if err := sw.WaitWithTimeout(ctx, "w", 5*time.Millisecond); err == nil {
		t.Fatal("insufficient timeout should fail when window full")
	}
}

func TestMultiTierUnknownTierUsesDefault(t *testing.T) {
	mt := NewMultiTierRateLimiter(
		TierConfig{MaxRequests: 2, WindowSize: 40 * time.Millisecond},
		[]TierConfig{{Name: "pro", MaxRequests: 5, WindowSize: 40 * time.Millisecond}},
	)
	ctx := context.Background()
	// 未知 tier → 回落 default limiter
	for i := 0; i < 2; i++ {
		res, err := mt.Allow(ctx, "ghost", "k")
		if err != nil || !res.Allowed {
			t.Fatalf("ghost tier req %d should pass: %+v %v", i, res, err)
		}
	}
	if res, err := mt.Allow(ctx, "ghost", "k"); err != nil || res.Allowed {
		t.Fatalf("ghost tier third req should block: %+v %v", res, err)
	}
	if _, err := mt.AllowN(ctx, "ghost", "k", 1); err != nil {
		t.Fatalf("AllowN error: %v", err)
	}
	// Wait 会阻塞等待窗口滑过，最终成功
	if err := mt.Wait(ctx, "ghost", "k"); err != nil {
		t.Fatalf("Wait should eventually succeed: %v", err)
	}
	if err := mt.Reset(ctx, "ghost", "k"); err != nil {
		t.Fatal(err)
	}
	if err := mt.WaitWithTimeout(ctx, "ghost", "k", time.Second); err != nil {
		t.Fatalf("after reset WaitWithTimeout should succeed: %v", err)
	}
	if st := mt.GetStats("ghost", "k"); st == nil {
		t.Fatal("GetStats should never be nil")
	}
}

func TestAdaptiveRateLimiterAllowNAndWait(t *testing.T) {
	base := NewSlidingWindowLimiter(10, time.Minute)
	a := NewAdaptiveRateLimiter(base, 8, 2)
	ctx := context.Background()

	res, err := a.Allow(ctx, "k")
	if err != nil || !res.Allowed {
		t.Fatalf("first Allow should pass: %+v %v", res, err)
	}
	if _, err := a.AllowN(ctx, "an", 1); err != nil {
		t.Fatalf("AllowN error: %v", err)
	}
	if res, err = a.AllowN(ctx, "k", 2); err != nil {
		t.Fatalf("AllowN error: %v", err)
	}
	_ = res
	if err := a.Wait(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := a.WaitWithTimeout(ctx, "k", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := a.Reset(ctx, "k"); err != nil {
		t.Fatal(err)
	}
}
