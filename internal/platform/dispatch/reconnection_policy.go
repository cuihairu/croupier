package dispatch

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ReconnectionPolicy defines how reconnection attempts are made
type ReconnectionPolicy struct {
	// MaxRetries is the maximum number of reconnection attempts (-1 for infinite)
	MaxRetries int
	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the exponential backoff multiplier (e.g., 2.0 for doubling)
	Multiplier float64
	// Jitter is the random jitter factor (0-1) to add to delays
	Jitter float64
	// EnableAutoReconnect enables automatic reconnection
	EnableAutoReconnect bool

	mu   sync.Mutex
	rand *rand.Rand
}

// DefaultReconnectionPolicy returns the default reconnection policy
func DefaultReconnectionPolicy() *ReconnectionPolicy {
	return &ReconnectionPolicy{
		MaxRetries:          5,
		InitialDelay:        500 * time.Millisecond,
		MaxDelay:            30 * time.Second,
		Multiplier:          2.0,
		Jitter:              0.1, // 10% jitter
		EnableAutoReconnect: true,
		rand:                rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewReconnectionPolicy creates a new reconnection policy with custom settings
func NewReconnectionPolicy(maxRetries int, initialDelay, maxDelay time.Duration, multiplier, jitter float64, enableAuto bool) *ReconnectionPolicy {
	if initialDelay <= 0 {
		initialDelay = 500 * time.Millisecond
	}
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	if multiplier <= 1.0 {
		multiplier = 2.0
	}
	if jitter < 0 {
		jitter = 0
	}
	if jitter > 1 {
		jitter = 1
	}

	return &ReconnectionPolicy{
		MaxRetries:          maxRetries,
		InitialDelay:        initialDelay,
		MaxDelay:            maxDelay,
		Multiplier:          multiplier,
		Jitter:              jitter,
		EnableAutoReconnect: enableAuto,
		rand:                rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NextDelay returns the delay before the next reconnection attempt
func (p *ReconnectionPolicy) NextDelay(attempt int) (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.MaxRetries >= 0 && attempt > p.MaxRetries {
		return 0, fmt.Errorf("max retries exceeded: %d", p.MaxRetries)
	}

	// Calculate base delay using exponential backoff
	delay := float64(p.InitialDelay) * powFloat64(p.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	// Add jitter
	if p.Jitter > 0 {
		jitterRange := delay * p.Jitter
		jitterOffset := (p.rand.Float64()*2 - 1) * jitterRange // [-jitterRange, +jitterRange]
		delay += jitterOffset
	}

	// Ensure non-negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay), nil
}

// ShouldRetry returns whether another reconnection attempt should be made
func (p *ReconnectionPolicy) ShouldRetry(attempt int) bool {
	if p.MaxRetries < 0 {
		return true // Infinite retries
	}
	return attempt <= p.MaxRetries
}

// powFloat64 calculates base^exp for float64 values
func powFloat64(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	if base == 0 {
		return 0
	}

	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}

// ReconnectionState tracks the reconnection state for a connection
type ReconnectionState struct {
	policy      *ReconnectionPolicy
	attempt     int
	lastAttempt time.Time
	lastError   error
	mu          sync.Mutex
	enabled     bool
}

// NewReconnectionState creates a new reconnection state
func NewReconnectionState(policy *ReconnectionPolicy) *ReconnectionState {
	if policy == nil {
		policy = DefaultReconnectionPolicy()
	}

	return &ReconnectionState{
		policy:  policy,
		attempt: 0,
		enabled: policy.EnableAutoReconnect,
	}
}

// ShouldRetry returns whether another reconnection attempt should be made
func (s *ReconnectionState) ShouldRetry() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return false
	}
	return s.policy.ShouldRetry(s.attempt)
}

// NextDelay returns the delay before the next reconnection attempt
func (s *ReconnectionState) NextDelay() (time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.policy.NextDelay(s.attempt)
}

// RecordAttempt records a reconnection attempt
func (s *ReconnectionState) RecordAttempt(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attempt++
	s.lastAttempt = time.Now()
	s.lastError = err
}

// Reset resets the reconnection state (e.g., after successful connection)
func (s *ReconnectionState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attempt = 0
	s.lastError = nil
}

// GetAttempt returns the current attempt count
func (s *ReconnectionState) GetAttempt() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempt
}

// GetLastError returns the last error encountered
func (s *ReconnectionState) GetLastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// SetEnabled enables or disables automatic reconnection
func (s *ReconnectionState) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// IsEnabled returns whether automatic reconnection is enabled
func (s *ReconnectionState) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// ReconnectionManager manages reconnection states for multiple connections
type ReconnectionManager struct {
	mu     sync.Mutex
	states map[string]*ReconnectionState // key -> state
	policy *ReconnectionPolicy
}

// NewReconnectionManager creates a new reconnection manager
func NewReconnectionManager(policy *ReconnectionPolicy) *ReconnectionManager {
	if policy == nil {
		policy = DefaultReconnectionPolicy()
	}

	return &ReconnectionManager{
		states: make(map[string]*ReconnectionState),
		policy: policy,
	}
}

// GetOrCreateState gets or creates a reconnection state for the given key
func (m *ReconnectionManager) GetOrCreateState(key string) *ReconnectionState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.states[key]; ok {
		return state
	}

	state := NewReconnectionState(m.policy)
	m.states[key] = state
	return state
}

// RemoveState removes the reconnection state for the given key
func (m *ReconnectionManager) RemoveState(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, key)
}

// ResetAll resets all reconnection states
func (m *ReconnectionManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, state := range m.states {
		state.Reset()
	}
}

// GetPolicy returns the reconnection policy
func (m *ReconnectionManager) GetPolicy() *ReconnectionPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.policy
}

// SetPolicy updates the reconnection policy
func (m *ReconnectionManager) SetPolicy(policy *ReconnectionPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.policy = policy
	// Reset all states with new policy
	for key := range m.states {
		m.states[key] = NewReconnectionState(policy)
	}
}

// Stats returns reconnection statistics
func (m *ReconnectionManager) Stats() map[string]ReconnectionStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := make(map[string]ReconnectionStats, len(m.states))
	for key, state := range m.states {
		state.mu.Lock()
		stats[key] = ReconnectionStats{
			Attempt:     state.attempt,
			LastAttempt: state.lastAttempt,
			LastError:   state.lastError,
			Enabled:     state.enabled,
		}
		state.mu.Unlock()
	}
	return stats
}

// ReconnectionStats contains reconnection statistics for a single connection
type ReconnectionStats struct {
	Attempt     int       `json:"attempt"`
	LastAttempt time.Time `json:"lastAttempt"`
	LastError   error     `json:"lastError,omitempty"`
	Enabled     bool      `json:"enabled"`
}
