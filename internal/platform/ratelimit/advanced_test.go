package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRateLimitStore is a mock implementation of RateLimitStore
type MockRateLimitStore struct {
	mu     map[string]int
	ttl    map[string]time.Time
	muLock sync.Mutex
	calls  map[string]int
}

func NewMockRateLimitStore() *MockRateLimitStore {
	return &MockRateLimitStore{
		mu:    make(map[string]int),
		ttl:   make(map[string]time.Time),
		calls: make(map[string]int),
	}
}

func (m *MockRateLimitStore) Increment(ctx context.Context, key string, window time.Duration) (int, error) {
	m.muLock.Lock()
	defer m.muLock.Unlock()

	m.calls["Increment"]++
	now := time.Now()

	// Check if TTL expired
	if expiry, ok := m.ttl[key]; ok && now.After(expiry) {
		m.mu[key] = 0
	}

	m.mu[key]++
	return m.mu[key], nil
}

func (m *MockRateLimitStore) Get(ctx context.Context, key string) (int, error) {
	m.muLock.Lock()
	defer m.muLock.Unlock()

	m.calls["Get"]++
	now := time.Now()

	// Check if TTL expired
	if expiry, ok := m.ttl[key]; ok && now.After(expiry) {
		m.mu[key] = 0
	}

	return m.mu[key], nil
}

func (m *MockRateLimitStore) Reset(ctx context.Context, key string) error {
	m.muLock.Lock()
	defer m.muLock.Unlock()

	m.calls["Reset"]++
	delete(m.mu, key)
	delete(m.ttl, key)
	return nil
}

func (m *MockRateLimitStore) SetNX(ctx context.Context, key string, value int, ttl time.Duration) (bool, error) {
	m.muLock.Lock()
	defer m.muLock.Unlock()

	m.calls["SetNX"]++
	if _, exists := m.mu[key]; !exists {
		m.mu[key] = value
		m.ttl[key] = time.Now().Add(ttl)
		return true, nil
	}
	return false, nil
}

func (m *MockRateLimitStore) SetExpiry(key string, ttl time.Duration) {
	m.muLock.Lock()
	defer m.muLock.Unlock()
	m.ttl[key] = time.Now().Add(ttl)
}

func (m *MockRateLimitStore) GetCount(key string) int {
	m.muLock.Lock()
	defer m.muLock.Unlock()
	return m.mu[key]
}

func (m *MockRateLimitStore) GetCalls(method string) int {
	m.muLock.Lock()
	defer m.muLock.Unlock()
	return m.calls[method]
}

// SlidingWindowLimiter Tests

func TestNewSlidingWindowLimiter(t *testing.T) {
	sw := NewSlidingWindowLimiter(10, time.Second)
	assert.NotNil(t, sw)
	assert.Equal(t, 10, sw.maxRequests)
	assert.Equal(t, time.Second, sw.windowSize)
	assert.False(t, sw.closed)

	sw.Close()
}

func TestSlidingWindowLimiter_Allow_Basic(t *testing.T) {
	sw := NewSlidingWindowLimiter(3, time.Second)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		result, err := sw.Allow(ctx, key)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
		assert.Equal(t, 2-i, result.Remaining)
		assert.Equal(t, 3, result.Limit)
	}

	// 4th request should be denied
	result, err := sw.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	// When denied, remaining can be negative (indicates how many over limit)
	assert.LessOrEqual(t, result.Remaining, 0)
	assert.Greater(t, result.RetryAfter.Milliseconds(), int64(0))
}

func TestSlidingWindowLimiter_AllowN(t *testing.T) {
	sw := NewSlidingWindowLimiter(10, time.Second)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// Allow 5 requests at once
	result, err := sw.AllowN(ctx, key, 5)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 5, result.Remaining)

	// Allow 3 more
	result, err = sw.AllowN(ctx, key, 3)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 2, result.Remaining)

	// Try to allow 5 more (should fail)
	result, err = sw.AllowN(ctx, key, 5)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestSlidingWindowLimiter_AllowN_ExceedsLimit(t *testing.T) {
	sw := NewSlidingWindowLimiter(5, time.Second)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// First use some quota
	result, err := sw.AllowN(ctx, key, 3)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	// Try to allow more than remaining at once
	result, err = sw.AllowN(ctx, key, 5)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	// When denied with AllowN, remaining can be negative
	assert.LessOrEqual(t, result.Remaining, 0)
}

func TestSlidingWindowLimiter_Reset(t *testing.T) {
	sw := NewSlidingWindowLimiter(3, time.Second)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// Use up all requests
	for i := 0; i < 3; i++ {
		sw.Allow(ctx, key)
	}

	// Should be denied
	result, _ := sw.Allow(ctx, key)
	assert.False(t, result.Allowed)

	// Reset
	err := sw.Reset(ctx, key)
	require.NoError(t, err)

	// Should be allowed again
	result, err = sw.Allow(ctx, key)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSlidingWindowLimiter_GetStats(t *testing.T) {
	sw := NewSlidingWindowLimiter(10, time.Second)
	defer sw.Close()

	key := "user1"

	// No stats for non-existent key
	stats := sw.GetStats(key)
	assert.Nil(t, stats)

	// Make some requests
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		sw.Allow(ctx, key)
	}

	// Get stats
	stats = sw.GetStats(key)
	require.NotNil(t, stats)
	assert.Equal(t, key, stats.Key)
	assert.Equal(t, 7, stats.CurrentTokens)
	assert.False(t, stats.LastRequest.IsZero())
}

func TestSlidingWindowLimiter_Wait(t *testing.T) {
	sw := NewSlidingWindowLimiter(2, time.Millisecond*100)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// Use up quota
	sw.Allow(ctx, key)
	sw.Allow(ctx, key)

	// Wait should block until quota available
	done := make(chan error)
	go func() {
		done <- sw.Wait(ctx, key)
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestSlidingWindowLimiter_Wait_ContextCanceled(t *testing.T) {
	sw := NewSlidingWindowLimiter(1, time.Second)
	defer sw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	key := "user1"

	// Use up quota
	sw.Allow(ctx, key)

	// Cancel context and wait
	cancel()
	err := sw.Wait(ctx, key)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestSlidingWindowLimiter_WaitWithTimeout(t *testing.T) {
	sw := NewSlidingWindowLimiter(1, time.Second)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// Use up quota
	sw.Allow(ctx, key)

	// Wait with short timeout - should exceed
	err := sw.WaitWithTimeout(ctx, key, time.Millisecond*10)
	assert.Error(t, err)
	assert.Equal(t, ErrRateLimitExceeded, err)
}

func TestSlidingWindowLimiter_Allow_Closed(t *testing.T) {
	sw := NewSlidingWindowLimiter(10, time.Second)
	sw.Close()

	ctx := context.Background()
	_, err := sw.Allow(ctx, "key")
	assert.Error(t, err)
	assert.Equal(t, ErrLimiterClosed, err)
}

func TestSlidingWindowLimiter_SlidingWindowExpiration(t *testing.T) {
	sw := NewSlidingWindowLimiter(3, time.Millisecond*100)
	defer sw.Close()

	ctx := context.Background()
	key := "user1"

	// Use all quota
	for i := 0; i < 3; i++ {
		result, err := sw.Allow(ctx, key)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	}

	// Should be denied
	result, _ := sw.Allow(ctx, key)
	assert.False(t, result.Allowed)

	// Wait for window to slide
	time.Sleep(time.Millisecond * 150)

	// Should be allowed again
	result, err := sw.Allow(ctx, key)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSlidingWindowLimiter_MultipleKeys(t *testing.T) {
	sw := NewSlidingWindowLimiter(2, time.Second)
	defer sw.Close()

	ctx := context.Background()

	// Each key should have independent quota
	for _, key := range []string{"user1", "user2", "user3"} {
		for i := 0; i < 2; i++ {
			result, err := sw.Allow(ctx, key)
			require.NoError(t, err)
			assert.True(t, result.Allowed, "Key %s request %d should be allowed", key, i+1)
		}
	}
}

// DistributedRateLimiter Tests

func TestNewDistributedRateLimiter(t *testing.T) {
	store := NewMockRateLimitStore()
	d := NewDistributedRateLimiter(store, 10, time.Second)
	assert.NotNil(t, d)
	assert.NotNil(t, d.local)
	assert.Equal(t, store, d.store)
}

func TestDistributedRateLimiter_Allow_StoreFallback(t *testing.T) {
	store := NewMockRateLimitStore()
	d := NewDistributedRateLimiter(store, 5, time.Second)
	d.keyPrefix = "test:"

	ctx := context.Background()
	key := "user1"

	// First request - should check local then distributed
	result, err := d.Allow(ctx, key)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 1, store.GetCalls("Increment"))
}

func TestDistributedRateLimiter_Allow_StoreIncrement(t *testing.T) {
	store := NewMockRateLimitStore()
	d := NewDistributedRateLimiter(store, 3, time.Second)
	d.keyPrefix = "test:"
	store.SetExpiry("test:user1", time.Second*10)

	ctx := context.Background()
	key := "user1"

	// Make multiple requests
	for i := 0; i < 3; i++ {
		store.mu["test:"+key] = i
		result, err := d.Allow(ctx, key)
		require.NoError(t, err)
		if i < 2 {
			assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
		}
	}
}

func TestDistributedRateLimiter_Reset(t *testing.T) {
	store := NewMockRateLimitStore()
	d := NewDistributedRateLimiter(store, 10, time.Second)
	d.keyPrefix = "test:"

	ctx := context.Background()
	key := "user1"

	// Make a request
	d.Allow(ctx, key)

	// Reset
	err := d.Reset(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 1, store.GetCalls("Reset"))
}

func TestDistributedRateLimiter_Wait(t *testing.T) {
	store := NewMockRateLimitStore()
	d := NewDistributedRateLimiter(store, 2, time.Millisecond*100)
	d.keyPrefix = "test:"

	ctx := context.Background()
	key := "user1"

	// Use up quota
	d.Allow(ctx, key)
	d.Allow(ctx, key)

	// Wait should eventually succeed
	done := make(chan error)
	go func() {
		done <- d.Wait(ctx, key)
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestDistributedRateLimiter_GetStats(t *testing.T) {
	store := NewMockRateLimitStore()
	d := NewDistributedRateLimiter(store, 10, time.Second)
	d.keyPrefix = "test:"

	ctx := context.Background()
	key := "user1"

	// Make a request
	d.Allow(ctx, key)

	// Get stats
	stats := d.GetStats(key)
	assert.NotNil(t, stats)
	assert.Equal(t, key, stats.Key)
}

// MultiTierRateLimiter Tests

func TestNewMultiTierRateLimiter(t *testing.T) {
	defaultConfig := TierConfig{
		Name:        "default",
		MaxRequests: 10,
		WindowSize:  time.Second,
	}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
		{Name: "free", MaxRequests: 5, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)
	assert.NotNil(t, mt)
	assert.NotNil(t, mt.defaultLimiter)
	assert.Len(t, mt.limiters, 2)
}

func TestMultiTierRateLimiter_Allow_KnownTier(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 10, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()

	// Premium tier should have higher limit
	for i := 0; i < 50; i++ {
		result, err := mt.Allow(ctx, "premium", "user1")
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Premium request %d should be allowed", i+1)
	}
}

func TestMultiTierRateLimiter_Allow_DefaultTier(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 5, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()

	// Unknown tier should use default
	for i := 0; i < 5; i++ {
		result, err := mt.Allow(ctx, "unknown", "user1")
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
	}

	// 6th request should be denied
	result, err := mt.Allow(ctx, "unknown", "user1")
	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestMultiTierRateLimiter_AllowN(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 10, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()

	result, err := mt.AllowN(ctx, "premium", "user1", 50)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 50, result.Remaining)
}

func TestMultiTierRateLimiter_Reset(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 5, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()

	// Use up quota
	for i := 0; i < 5; i++ {
		mt.Allow(ctx, "unknown", "user1")
	}

	// Should be denied
	result, _ := mt.Allow(ctx, "unknown", "user1")
	assert.False(t, result.Allowed)

	// Reset
	err := mt.Reset(ctx, "unknown", "user1")
	require.NoError(t, err)

	// Should be allowed again
	result, err = mt.Allow(ctx, "unknown", "user1")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestMultiTierRateLimiter_GetStats(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 10, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()
	mt.Allow(ctx, "premium", "user1")

	stats := mt.GetStats("premium", "user1")
	assert.NotNil(t, stats)
	assert.Contains(t, stats.Key, "premium")
}

func TestMultiTierRateLimiter_AddTier(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 10, WindowSize: time.Second}
	mt := NewMultiTierRateLimiter(defaultConfig, nil)

	ctx := context.Background()

	// Tier doesn't exist yet
	mt.Allow(ctx, "new", "user1")
	mt.Allow(ctx, "new", "user1")

	// Add tier
	mt.AddTier("new", TierConfig{MaxRequests: 50, WindowSize: time.Second})

	// Now should use new tier
	for i := 0; i < 50; i++ {
		result, err := mt.Allow(ctx, "new", "user1")
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed with new tier", i+1)
	}
}

func TestMultiTierRateLimiter_RemoveTier(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 5, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()

	// Use premium tier
	for i := 0; i < 50; i++ {
		mt.Allow(ctx, "premium", "user1")
	}

	// Remove tier
	mt.RemoveTier("premium")

	// Should now use default (5 requests)
	for i := 0; i < 5; i++ {
		result, err := mt.Allow(ctx, "premium", "user1")
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d with default tier should be allowed", i+1)
	}

	// 6th should be denied
	result, _ := mt.Allow(ctx, "premium", "user1")
	assert.False(t, result.Allowed)
}

func TestMultiTierRateLimiter_Wait(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 1, WindowSize: time.Millisecond * 100}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 100, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()
	mt.Allow(ctx, "premium", "user1")

	// Wait should eventually succeed
	done := make(chan error)
	go func() {
		done <- mt.Wait(ctx, "premium", "user1")
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestMultiTierRateLimiter_WaitWithTimeout(t *testing.T) {
	defaultConfig := TierConfig{MaxRequests: 1, WindowSize: time.Second}
	tiers := []TierConfig{
		{Name: "premium", MaxRequests: 1, WindowSize: time.Second},
	}

	mt := NewMultiTierRateLimiter(defaultConfig, tiers)

	ctx := context.Background()
	// Use up the only quota
	mt.Allow(ctx, "premium", "user1")

	err := mt.WaitWithTimeout(ctx, "premium", "user1", time.Millisecond*10)
	assert.Error(t, err)
	assert.Equal(t, ErrRateLimitExceeded, err)
}

// AdaptiveRateLimiter Tests

func TestNewAdaptiveRateLimiter(t *testing.T) {
	base := NewSlidingWindowLimiter(100, time.Second)
	a := NewAdaptiveRateLimiter(base, 100, 10)
	assert.NotNil(t, a)
	assert.Equal(t, 100, a.maxLimit)
	assert.Equal(t, 10, a.minLimit)
	assert.Equal(t, 100, a.currentLimit)
	assert.Equal(t, 0.1, a.adjustmentFactor)
}

func TestAdaptiveRateLimiter_Allow_NoHealthCheck(t *testing.T) {
	base := NewSlidingWindowLimiter(10, time.Second)
	a := NewAdaptiveRateLimiter(base, 10, 5)
	defer base.Close()

	ctx := context.Background()

	// Without health check, should use base limiter normally
	for i := 0; i < 10; i++ {
		result, err := a.Allow(ctx, "user1")
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
	}
}

func TestAdaptiveRateLimiter_Allow_WithHealthCheck(t *testing.T) {
	base := NewSlidingWindowLimiter(100, time.Second)
	a := NewAdaptiveRateLimiter(base, 100, 10)
	defer base.Close()

	// Set health check to return 0.5 (50% health)
	healthCalled := false
	a.SetHealthCheck(func() float64 {
		healthCalled = true
		return 0.5
	})

	ctx := context.Background()

	// Make a request - should trigger health check
	a.Allow(ctx, "user1")
	assert.True(t, healthCalled)
}

func TestAdaptiveRateLimiter_AdjustLimit_Increase(t *testing.T) {
	base := NewSlidingWindowLimiter(100, time.Second)
	a := NewAdaptiveRateLimiter(base, 100, 10)
	defer base.Close()

	// Start at minimum
	a.currentLimit = 10

	// Health improves to 100%
	a.SetHealthCheck(func() float64 {
		return 1.0
	})

	ctx := context.Background()

	// Make multiple requests to allow gradual adjustment
	for i := 0; i < 20; i++ {
		a.Allow(ctx, "user1")
	}

	// Limit should have increased
	assert.Greater(t, a.currentLimit, 10)
}

func TestAdaptiveRateLimiter_AdjustLimit_Decrease(t *testing.T) {
	base := NewSlidingWindowLimiter(100, time.Second)
	a := NewAdaptiveRateLimiter(base, 100, 10)
	defer base.Close()

	// Start at maximum
	a.currentLimit = 100

	// Health degrades to 20%
	a.SetHealthCheck(func() float64 {
		return 0.2
	})

	ctx := context.Background()

	// Make multiple requests to allow gradual adjustment
	for i := 0; i < 20; i++ {
		a.Allow(ctx, "user1")
	}

	// Limit should have decreased
	assert.Less(t, a.currentLimit, 100)
}

func TestAdaptiveRateLimiter_AdjustLimit_Minimum(t *testing.T) {
	base := NewSlidingWindowLimiter(100, time.Second)
	a := NewAdaptiveRateLimiter(base, 100, 50)
	defer base.Close()

	a.currentLimit = 60

	// Very poor health
	a.SetHealthCheck(func() float64 {
		return 0.1
	})

	ctx := context.Background()

	// Make many requests
	for i := 0; i < 30; i++ {
		a.Allow(ctx, "user1")
	}

	// Limit should not go below minimum
	assert.GreaterOrEqual(t, a.currentLimit, a.minLimit)
}

func TestAdaptiveRateLimiter_Wait(t *testing.T) {
	base := NewSlidingWindowLimiter(1, time.Millisecond*100)
	a := NewAdaptiveRateLimiter(base, 10, 1)
	defer base.Close()

	ctx := context.Background()
	a.Allow(ctx, "user1")

	// Wait should use base limiter
	err := a.Wait(ctx, "user1")
	assert.NoError(t, err)
}

func TestAdaptiveRateLimiter_Reset(t *testing.T) {
	base := NewSlidingWindowLimiter(10, time.Second)
	a := NewAdaptiveRateLimiter(base, 10, 5)
	defer base.Close()

	ctx := context.Background()

	// Use quota
	for i := 0; i < 10; i++ {
		a.Allow(ctx, "user1")
	}

	// Reset
	err := a.Reset(ctx, "user1")
	assert.NoError(t, err)

	// Should be allowed again
	result, err := a.Allow(ctx, "user1")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestAdaptiveRateLimiter_GetStats(t *testing.T) {
	base := NewSlidingWindowLimiter(10, time.Second)
	a := NewAdaptiveRateLimiter(base, 10, 5)
	defer base.Close()

	ctx := context.Background()
	a.Allow(ctx, "user1")

	stats := a.GetStats("user1")
	assert.NotNil(t, stats)
}

// RateLimitResult Tests

func TestRateLimitResult(t *testing.T) {
	result := &RateLimitResult{
		Allowed:    true,
		Remaining:  5,
		ResetAt:    time.Now().Add(time.Second),
		RetryAfter: time.Millisecond * 100,
		Limit:      10,
	}

	assert.True(t, result.Allowed)
	assert.Equal(t, 5, result.Remaining)
	assert.Equal(t, 10, result.Limit)
	assert.Greater(t, result.ResetAt.Unix(), time.Now().Unix())
}

// RateLimitStats Tests

func TestRateLimitStats(t *testing.T) {
	stats := &RateLimitStats{
		Key:             "user1",
		TotalRequests:   100,
		AllowedRequests: 95,
		DeniedRequests:  5,
		CurrentTokens:   5,
		LastRequest:     time.Now(),
	}

	assert.Equal(t, "user1", stats.Key)
	assert.Equal(t, int64(100), stats.TotalRequests)
	assert.Equal(t, int64(95), stats.AllowedRequests)
	assert.Equal(t, int64(5), stats.DeniedRequests)
	assert.Equal(t, 5, stats.CurrentTokens)
}

// Error Tests

func TestErrors(t *testing.T) {
	assert.Equal(t, "rate limit exceeded", ErrRateLimitExceeded.Error())
	assert.Equal(t, "quota exceeded", ErrQuotaExceeded.Error())
	assert.Equal(t, "limiter is closed", ErrLimiterClosed.Error())
}

// Benchmark Tests

func BenchmarkSlidingWindowLimiter_Allow(b *testing.B) {
	sw := NewSlidingWindowLimiter(10000, time.Second)
	defer sw.Close()

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "user" + string(rune(i%100))
			sw.Allow(ctx, key)
			i++
		}
	})
}

func BenchmarkSlidingWindowLimiter_AllowN(b *testing.B) {
	sw := NewSlidingWindowLimiter(10000, time.Second)
	defer sw.Close()

	ctx := context.Background()
	keys := make([]string, 100)
	for i := range keys {
		keys[i] = "user" + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.AllowN(ctx, keys[i%100], 10)
	}
}
