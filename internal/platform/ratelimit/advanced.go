package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Advanced rate limiting errors
var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrQuotaExceeded     = errors.New("quota exceeded")
	ErrLimiterClosed     = errors.New("limiter is closed")
)

// RateLimitStrategy defines the rate limiting strategy
type RateLimitStrategy string

const (
	StrategyTokenBucket   RateLimitStrategy = "token_bucket"
	StrategySlidingWindow RateLimitStrategy = "sliding_window"
	StrategyFixedWindow   RateLimitStrategy = "fixed_window"
	StrategyLeakyBucket   RateLimitStrategy = "leaky_bucket"
)

// RateLimitConfig holds rate limiter configuration
type RateLimitConfig struct {
	Strategy       RateLimitStrategy `json:"strategy"`
	RequestsPerSec float64           `json:"requestsPerSec"`
	BurstSize      int               `json:"burstSize"`
	WindowDuration time.Duration     `json:"windowDuration"`
	KeyFunc        func(ctx context.Context) string
	OnLimited      func(ctx context.Context, key string)
}

// RateLimitResult holds rate limit check result
type RateLimitResult struct {
	Allowed    bool          `json:"allowed"`
	Remaining  int           `json:"remaining"`
	ResetAt    time.Time     `json:"resetAt"`
	RetryAfter time.Duration `json:"retryAfter"`
	Limit      int           `json:"limit"`
}

// AdvancedLimiter provides advanced rate limiting capabilities
type AdvancedLimiter interface {
	Allow(ctx context.Context, key string) (*RateLimitResult, error)
	AllowN(ctx context.Context, key string, n int) (*RateLimitResult, error)
	Wait(ctx context.Context, key string) error
	WaitWithTimeout(ctx context.Context, key string, timeout time.Duration) error
	Reset(ctx context.Context, key string) error
	GetStats(key string) *RateLimitStats
}

// RateLimitStats holds rate limit statistics
type RateLimitStats struct {
	Key             string    `json:"key"`
	TotalRequests   int64     `json:"totalRequests"`
	AllowedRequests int64     `json:"allowedRequests"`
	DeniedRequests  int64     `json:"deniedRequests"`
	CurrentTokens   int       `json:"currentTokens"`
	LastRequest     time.Time `json:"lastRequest"`
}

// SlidingWindowLimiter implements sliding window rate limiting
type SlidingWindowLimiter struct {
	mu              sync.RWMutex
	windows         map[string]*windowState
	maxRequests     int
	windowSize      time.Duration
	cleanupInterval time.Duration
	closed          bool
}

type windowState struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter
func NewSlidingWindowLimiter(maxRequests int, windowSize time.Duration) *SlidingWindowLimiter {
	sw := &SlidingWindowLimiter{
		windows:         make(map[string]*windowState),
		maxRequests:     maxRequests,
		windowSize:      windowSize,
		cleanupInterval: windowSize * 2,
	}

	// Start cleanup goroutine
	go sw.cleanup()

	return sw
}

// Allow checks if a request is allowed
func (sw *SlidingWindowLimiter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	return sw.AllowN(ctx, key, 1)
}

// AllowN checks if n requests are allowed
func (sw *SlidingWindowLimiter) AllowN(ctx context.Context, key string, n int) (*RateLimitResult, error) {
	sw.mu.RLock()
	if sw.closed {
		sw.mu.RUnlock()
		return nil, ErrLimiterClosed
	}
	sw.mu.RUnlock()

	sw.mu.Lock()
	window, exists := sw.windows[key]
	if !exists {
		window = &windowState{
			timestamps: make([]time.Time, 0),
		}
		sw.windows[key] = window
	}
	sw.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.windowSize)

	// Remove old timestamps
	validIdx := 0
	for _, ts := range window.timestamps {
		if ts.After(windowStart) {
			window.timestamps[validIdx] = ts
			validIdx++
		}
	}
	window.timestamps = window.timestamps[:validIdx]

	// Check if allowed
	remaining := sw.maxRequests - len(window.timestamps)
	result := &RateLimitResult{
		Limit:     sw.maxRequests,
		Remaining: remaining - n,
		ResetAt:   now.Add(sw.windowSize),
	}

	if remaining >= n {
		// Add timestamps for the requests
		for i := 0; i < n; i++ {
			window.timestamps = append(window.timestamps, now)
		}
		result.Allowed = true
	} else {
		result.Allowed = false
		result.RetryAfter = sw.windowSize - now.Sub(windowStart)
		if len(window.timestamps) > 0 {
			oldest := window.timestamps[0]
			result.RetryAfter = oldest.Add(sw.windowSize).Sub(now)
			if result.RetryAfter < 0 {
				result.RetryAfter = 0
			}
		}
	}

	return result, nil
}

// Wait waits until a request is allowed
func (sw *SlidingWindowLimiter) Wait(ctx context.Context, key string) error {
	return sw.WaitWithTimeout(ctx, key, 0)
}

// WaitWithTimeout waits until a request is allowed or timeout
func (sw *SlidingWindowLimiter) WaitWithTimeout(ctx context.Context, key string, timeout time.Duration) error {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		result, err := sw.Allow(ctx, key)
		if err != nil {
			return err
		}
		if result.Allowed {
			return nil
		}

		waitTime := result.RetryAfter
		if waitTime <= 0 {
			waitTime = time.Millisecond * 10
		}

		if !deadline.IsZero() {
			if time.Now().Add(waitTime).After(deadline) {
				return ErrRateLimitExceeded
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

// Reset resets the rate limit for a key
func (sw *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.windows, key)
	return nil
}

// GetStats gets rate limit stats for a key
func (sw *SlidingWindowLimiter) GetStats(key string) *RateLimitStats {
	sw.mu.RLock()
	window, exists := sw.windows[key]
	sw.mu.RUnlock()

	if !exists {
		return nil
	}

	window.mu.Lock()
	defer window.mu.Unlock()

	return &RateLimitStats{
		Key:           key,
		CurrentTokens: sw.maxRequests - len(window.timestamps),
		LastRequest:   window.timestamps[len(window.timestamps)-1],
	}
}

// cleanup periodically removes old windows
func (sw *SlidingWindowLimiter) cleanup() {
	ticker := time.NewTicker(sw.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		sw.mu.Lock()
		if sw.closed {
			sw.mu.Unlock()
			return
		}

		now := time.Now()
		for key, window := range sw.windows {
			window.mu.Lock()
			if len(window.timestamps) == 0 {
				delete(sw.windows, key)
			} else {
				// Check if all timestamps are old
				allOld := true
				for _, ts := range window.timestamps {
					if ts.Add(sw.windowSize).After(now) {
						allOld = false
						break
					}
				}
				if allOld {
					delete(sw.windows, key)
				}
			}
			window.mu.Unlock()
		}
		sw.mu.Unlock()
	}
}

// Close closes the limiter
func (sw *SlidingWindowLimiter) Close() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.closed = true
	sw.windows = nil
	return nil
}

// DistributedRateLimiter provides rate limiting across multiple instances
type DistributedRateLimiter struct {
	local        AdvancedLimiter
	store        RateLimitStore
	keyPrefix    string
	replicaCount int
}

// RateLimitStore interface for distributed rate limit storage
type RateLimitStore interface {
	Increment(ctx context.Context, key string, window time.Duration) (int, error)
	Get(ctx context.Context, key string) (int, error)
	Reset(ctx context.Context, key string) error
	SetNX(ctx context.Context, key string, value int, ttl time.Duration) (bool, error)
}

// NewDistributedRateLimiter creates a new distributed rate limiter
func NewDistributedRateLimiter(store RateLimitStore, maxRequests int, windowSize time.Duration) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		local: NewSlidingWindowLimiter(maxRequests, windowSize),
		store: store,
	}
}

// Allow checks if a request is allowed using distributed state
func (d *DistributedRateLimiter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	return d.AllowN(ctx, key, 1)
}

// AllowN checks if n requests are allowed
func (d *DistributedRateLimiter) AllowN(ctx context.Context, key string, n int) (*RateLimitResult, error) {
	// First check local limiter for fast path
	localResult, err := d.local.Allow(ctx, key)
	if err != nil {
		return nil, err
	}

	if !localResult.Allowed {
		return localResult, nil
	}

	// Then check distributed store
	fullKey := d.keyPrefix + key
	count, err := d.store.Increment(ctx, fullKey, localResult.ResetAt.Sub(time.Now()))
	if err != nil {
		// If store fails, fall back to local decision
		return localResult, nil
	}

	// Re-check limit with distributed count
	result := &RateLimitResult{
		Allowed:   count <= localResult.Limit,
		Remaining: localResult.Limit - count,
		ResetAt:   localResult.ResetAt,
		Limit:     localResult.Limit,
	}

	if !result.Allowed {
		result.RetryAfter = localResult.ResetAt.Sub(time.Now())
	}

	return result, nil
}

// Wait waits until a request is allowed
func (d *DistributedRateLimiter) Wait(ctx context.Context, key string) error {
	return d.local.Wait(ctx, key)
}

// WaitWithTimeout waits with timeout
func (d *DistributedRateLimiter) WaitWithTimeout(ctx context.Context, key string, timeout time.Duration) error {
	return d.local.WaitWithTimeout(ctx, key, timeout)
}

// Reset resets the rate limit
func (d *DistributedRateLimiter) Reset(ctx context.Context, key string) error {
	fullKey := d.keyPrefix + key
	if err := d.store.Reset(ctx, fullKey); err != nil {
		return err
	}
	return d.local.Reset(ctx, key)
}

// GetStats gets rate limit stats
func (d *DistributedRateLimiter) GetStats(key string) *RateLimitStats {
	return d.local.GetStats(key)
}

// MultiTierRateLimiter provides multi-tier rate limiting
type MultiTierRateLimiter struct {
	limiters       map[string]AdvancedLimiter
	defaultLimiter AdvancedLimiter
	mu             sync.RWMutex
}

// TierConfig holds configuration for a rate limit tier
type TierConfig struct {
	Name        string
	MaxRequests int
	WindowSize  time.Duration
	BurstSize   int
}

// NewMultiTierRateLimiter creates a new multi-tier rate limiter
func NewMultiTierRateLimiter(defaultConfig TierConfig, tiers []TierConfig) *MultiTierRateLimiter {
	mt := &MultiTierRateLimiter{
		limiters:       make(map[string]AdvancedLimiter),
		defaultLimiter: NewSlidingWindowLimiter(defaultConfig.MaxRequests, defaultConfig.WindowSize),
	}

	for _, tier := range tiers {
		mt.limiters[tier.Name] = NewSlidingWindowLimiter(tier.MaxRequests, tier.WindowSize)
	}

	return mt
}

// Allow checks if request is allowed for a tier
func (mt *MultiTierRateLimiter) Allow(ctx context.Context, tier, key string) (*RateLimitResult, error) {
	mt.mu.RLock()
	limiter, exists := mt.limiters[tier]
	mt.mu.RUnlock()

	if !exists {
		limiter = mt.defaultLimiter
	}

	return limiter.Allow(ctx, mt.buildKey(tier, key))
}

// AllowN checks if n requests are allowed
func (mt *MultiTierRateLimiter) AllowN(ctx context.Context, tier, key string, n int) (*RateLimitResult, error) {
	mt.mu.RLock()
	limiter, exists := mt.limiters[tier]
	mt.mu.RUnlock()

	if !exists {
		limiter = mt.defaultLimiter
	}

	return limiter.AllowN(ctx, mt.buildKey(tier, key), n)
}

// Wait waits for a request to be allowed
func (mt *MultiTierRateLimiter) Wait(ctx context.Context, tier, key string) error {
	mt.mu.RLock()
	limiter, exists := mt.limiters[tier]
	mt.mu.RUnlock()

	if !exists {
		limiter = mt.defaultLimiter
	}

	return limiter.Wait(ctx, mt.buildKey(tier, key))
}

// WaitWithTimeout waits with timeout
func (mt *MultiTierRateLimiter) WaitWithTimeout(ctx context.Context, tier, key string, timeout time.Duration) error {
	mt.mu.RLock()
	limiter, exists := mt.limiters[tier]
	mt.mu.RUnlock()

	if !exists {
		limiter = mt.defaultLimiter
	}

	return limiter.WaitWithTimeout(ctx, mt.buildKey(tier, key), timeout)
}

// Reset resets rate limit for a tier
func (mt *MultiTierRateLimiter) Reset(ctx context.Context, tier, key string) error {
	mt.mu.RLock()
	limiter, exists := mt.limiters[tier]
	mt.mu.RUnlock()

	if !exists {
		limiter = mt.defaultLimiter
	}

	return limiter.Reset(ctx, mt.buildKey(tier, key))
}

// GetStats gets stats for a tier
func (mt *MultiTierRateLimiter) GetStats(tier, key string) *RateLimitStats {
	mt.mu.RLock()
	limiter, exists := mt.limiters[tier]
	mt.mu.RUnlock()

	if !exists {
		limiter = mt.defaultLimiter
	}

	return limiter.GetStats(mt.buildKey(tier, key))
}

// AddTier adds a new rate limit tier
func (mt *MultiTierRateLimiter) AddTier(name string, config TierConfig) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.limiters[name] = NewSlidingWindowLimiter(config.MaxRequests, config.WindowSize)
}

// RemoveTier removes a rate limit tier
func (mt *MultiTierRateLimiter) RemoveTier(name string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	delete(mt.limiters, name)
}

func (mt *MultiTierRateLimiter) buildKey(tier, key string) string {
	return fmt.Sprintf("%s:%s", tier, key)
}

// AdaptiveRateLimiter adjusts rate limits based on system health
type AdaptiveRateLimiter struct {
	base             AdvancedLimiter
	mu               sync.RWMutex
	currentLimit     int
	maxLimit         int
	minLimit         int
	adjustmentFactor float64
	healthCheck      func() float64 // Returns health score 0-1
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(base AdvancedLimiter, maxLimit, minLimit int) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		base:             base,
		currentLimit:     maxLimit,
		maxLimit:         maxLimit,
		minLimit:         minLimit,
		adjustmentFactor: 0.1,
	}
}

// SetHealthCheck sets the health check function
func (a *AdaptiveRateLimiter) SetHealthCheck(fn func() float64) {
	a.healthCheck = fn
}

// Allow checks if request is allowed with adaptive limit
func (a *AdaptiveRateLimiter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	a.adjustLimit()
	return a.base.Allow(ctx, key)
}

// AllowN checks if n requests are allowed
func (a *AdaptiveRateLimiter) AllowN(ctx context.Context, key string, n int) (*RateLimitResult, error) {
	a.adjustLimit()
	return a.base.AllowN(ctx, key, n)
}

// Wait waits for a request
func (a *AdaptiveRateLimiter) Wait(ctx context.Context, key string) error {
	return a.base.Wait(ctx, key)
}

// WaitWithTimeout waits with timeout
func (a *AdaptiveRateLimiter) WaitWithTimeout(ctx context.Context, key string, timeout time.Duration) error {
	return a.base.WaitWithTimeout(ctx, key, timeout)
}

// Reset resets the rate limit
func (a *AdaptiveRateLimiter) Reset(ctx context.Context, key string) error {
	return a.base.Reset(ctx, key)
}

// GetStats gets rate limit stats
func (a *AdaptiveRateLimiter) GetStats(key string) *RateLimitStats {
	return a.base.GetStats(key)
}

// adjustLimit adjusts the current limit based on health
func (a *AdaptiveRateLimiter) adjustLimit() {
	if a.healthCheck == nil {
		return
	}

	health := a.healthCheck()
	a.mu.Lock()
	defer a.mu.Unlock()

	// Adjust limit based on health score
	// Higher health = higher limit
	targetLimit := int(float64(a.maxLimit) * health)
	if targetLimit < a.minLimit {
		targetLimit = a.minLimit
	}

	// Gradual adjustment
	delta := float64(targetLimit-a.currentLimit) * a.adjustmentFactor
	a.currentLimit += int(delta)
}
