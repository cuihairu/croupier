package dispatch

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int32

const (
	// CircuitClosed means the circuit is closed and requests are allowed
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen means the circuit is open and requests are blocked
	CircuitOpen
	// CircuitHalfOpen means the circuit is half-open and testing if the agent has recovered
	CircuitHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// AgentHealthState tracks the health state of a single agent
type AgentHealthState struct {
	// Immutable fields
	AgentID string
	Addr    string

	// Mutable fields (use atomic operations for int32)
	activeConnections   int32
	consecutiveFailures int32
	circuitState        atomic.Int32 // Stores CircuitBreakerState as int32
	healthScore         atomic.Int64 // Stores float64 as int64 bits

	// Circuit breaker state
	circuitOpenedAt atomic.Int64 // Unix timestamp in nanoseconds
	lastStateChange atomic.Int64 // Unix timestamp in nanoseconds

	// Statistics (atomic)
	totalRequests      atomic.Int64
	successfulRequests atomic.Int64
	failedRequests     atomic.Int64
	lastSuccessTime    atomic.Int64
	lastFailureTime    atomic.Int64

	// Configuration
	config *HealthCheckConfig

	mu sync.RWMutex
}

// HealthCheckConfig configures the health check behavior
type HealthCheckConfig struct {
	// Score decay rate per tick (0-1)
	ScoreDecayRate float64
	// Score bonus on successful request
	ScoreSuccessBonus float64
	// Score penalty on failed request
	ScoreFailurePenalty float64
	// Minimum health score (0-100)
	MinScore float64
	// Maximum health score (0-100)
	MaxScore float64
	// Score decay interval
	DecayInterval time.Duration
	// Circuit breaker failure threshold
	FailureThreshold int32
	// Circuit breaker open timeout
	CircuitOpenTimeout time.Duration
	// Half-open max requests before transitioning
	HalfOpenMaxRequests int32
}

// DefaultHealthCheckConfig returns the default health check configuration
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		ScoreDecayRate:      0.1,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		DecayInterval:       30 * time.Second,
		FailureThreshold:    5,
		CircuitOpenTimeout:  30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
}

// NewAgentHealthState creates a new agent health state
func NewAgentHealthState(agentID, addr string, config *HealthCheckConfig) *AgentHealthState {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	state := &AgentHealthState{
		AgentID: agentID,
		Addr:    addr,
		config:  config,
	}
	state.circuitState.Store(int32(CircuitClosed))
	state.healthScore.Store(int64(math.Float64bits(config.MaxScore)))
	state.circuitOpenedAt.Store(0)
	state.lastStateChange.Store(time.Now().UnixNano())
	state.lastSuccessTime.Store(time.Now().UnixNano())
	state.lastFailureTime.Store(0)

	return state
}

// ActiveConnections returns the current number of active connections
func (s *AgentHealthState) ActiveConnections() int32 {
	return atomic.LoadInt32(&s.activeConnections)
}

// IncrementConnections increments the active connection count
func (s *AgentHealthState) IncrementConnections() {
	atomic.AddInt32(&s.activeConnections, 1)
}

// DecrementConnections decrements the active connection count
func (s *AgentHealthState) DecrementConnections() {
	atomic.AddInt32(&s.activeConnections, -1)
}

// ConsecutiveFailures returns the current consecutive failure count
func (s *AgentHealthState) ConsecutiveFailures() int32 {
	return atomic.LoadInt32(&s.consecutiveFailures)
}

// HealthScore returns the current health score (0-100)
func (s *AgentHealthState) HealthScore() float64 {
	return math.Float64frombits(uint64(s.healthScore.Load()))
}

// CircuitState returns the current circuit breaker state
func (s *AgentHealthState) CircuitState() CircuitBreakerState {
	return CircuitBreakerState(s.circuitState.Load())
}

// IsCircuitOpen returns true if the circuit is open
func (s *AgentHealthState) IsCircuitOpen() bool {
	return s.CircuitState() == CircuitOpen
}

// IsAvailable returns true if the agent is available for routing
func (s *AgentHealthState) IsAvailable() bool {
	state := s.CircuitState()
	if state == CircuitOpen {
		return false
	}
	return s.HealthScore() > s.config.MinScore
}

// RecordSuccess records a successful request
func (s *AgentHealthState) RecordSuccess() {
	now := time.Now()

	atomic.AddInt32(&s.consecutiveFailures, -1)
	if atomic.LoadInt32(&s.consecutiveFailures) < 0 {
		atomic.StoreInt32(&s.consecutiveFailures, 0)
	}

	atomic.AddInt64(&s.totalRequests, 1)
	atomic.AddInt64(&s.successfulRequests, 1)
	atomic.StoreInt64(&s.lastSuccessTime, now.UnixNano())

	// Update health score
	s.updateHealthScore(s.config.ScoreSuccessBonus)

	s.transitionOnSuccess()
}

// RecordFailure records a failed request
func (s *AgentHealthState) RecordFailure() {
	now := time.Now()

	atomic.AddInt32(&s.consecutiveFailures, 1)

	atomic.AddInt64(&s.totalRequests, 1)
	atomic.AddInt64(&s.failedRequests, 1)
	atomic.StoreInt64(&s.lastFailureTime, now.UnixNano())

	// Update health score
	s.updateHealthScore(-s.config.ScoreFailurePenalty)

	s.transitionOnFailure()
}

// updateHealthScore updates the health score with the given delta
func (s *AgentHealthState) updateHealthScore(delta float64) {
	for {
		oldBits := s.healthScore.Load()
		oldScore := math.Float64frombits(uint64(oldBits))
		newScore := oldScore + delta

		// Clamp to [MinScore, MaxScore]
		if newScore > s.config.MaxScore {
			newScore = s.config.MaxScore
		}
		if newScore < s.config.MinScore {
			newScore = s.config.MinScore
		}

		newBits := int64(math.Float64bits(newScore))
		if s.healthScore.CompareAndSwap(oldBits, newBits) {
			return
		}
	}
}

// transitionOnSuccess handles state transitions on success
func (s *AgentHealthState) transitionOnSuccess() {
	currentState := s.CircuitState()

	switch currentState {
	case CircuitHalfOpen:
		// In half-open, successful requests may close the circuit
		s.mu.Lock()
		defer s.mu.Unlock()

		// Re-check state under lock
		if CircuitBreakerState(s.circuitState.Load()) != CircuitHalfOpen {
			return
		}

		s.circuitState.Store(int32(CircuitClosed))
		s.lastStateChange.Store(time.Now().UnixNano())
		s.circuitOpenedAt.Store(0)

	case CircuitOpen:
		// Check if we should transition to half-open
		openedAt := time.Unix(0, s.circuitOpenedAt.Load())
		if time.Since(openedAt) >= s.config.CircuitOpenTimeout {
			s.mu.Lock()
			defer s.mu.Unlock()

			// Re-check state under lock
			if CircuitBreakerState(s.circuitState.Load()) != CircuitOpen {
				return
			}

			s.circuitState.Store(int32(CircuitHalfOpen))
			s.lastStateChange.Store(time.Now().UnixNano())
		}
	}
}

// transitionOnFailure handles state transitions on failure
func (s *AgentHealthState) transitionOnFailure() {
	currentState := s.CircuitState()

	switch currentState {
	case CircuitClosed:
		// Check if we should open the circuit
		failures := atomic.LoadInt32(&s.consecutiveFailures)
		if failures >= s.config.FailureThreshold {
			s.mu.Lock()
			defer s.mu.Unlock()

			// Re-check state under lock
			if CircuitBreakerState(s.circuitState.Load()) != CircuitClosed {
				return
			}

			s.circuitState.Store(int32(CircuitOpen))
			s.circuitOpenedAt.Store(time.Now().UnixNano())
			s.lastStateChange.Store(time.Now().UnixNano())
		}

	case CircuitHalfOpen:
		// In half-open, any failure immediately opens the circuit
		s.mu.Lock()
		defer s.mu.Unlock()

		// Re-check state under lock
		if CircuitBreakerState(s.circuitState.Load()) != CircuitHalfOpen {
			return
		}

		s.circuitState.Store(int32(CircuitOpen))
		s.circuitOpenedAt.Store(time.Now().UnixNano())
		s.lastStateChange.Store(time.Now().UnixNano())
	}
}

// GetStatistics returns the current statistics
func (s *AgentHealthState) GetStatistics() HealthStatistics {
	return HealthStatistics{
		TotalRequests:       s.totalRequests.Load(),
		SuccessfulRequests:  s.successfulRequests.Load(),
		FailedRequests:      s.failedRequests.Load(),
		ActiveConnections:   atomic.LoadInt32(&s.activeConnections),
		ConsecutiveFailures: atomic.LoadInt32(&s.consecutiveFailures),
		HealthScore:         s.HealthScore(),
		CircuitState:        s.CircuitState(),
		LastSuccessTime:     time.Unix(0, s.lastSuccessTime.Load()),
		LastFailureTime:     time.Unix(0, s.lastFailureTime.Load()),
		CircuitOpenedAt:     time.Unix(0, s.circuitOpenedAt.Load()),
		LastStateChange:     time.Unix(0, s.lastStateChange.Load()),
	}
}

// HealthStatistics contains the current health statistics
type HealthStatistics struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	ActiveConnections   int32
	ConsecutiveFailures int32
	HealthScore         float64
	CircuitState        CircuitBreakerState
	LastSuccessTime     time.Time
	LastFailureTime     time.Time
	CircuitOpenedAt     time.Time
	LastStateChange     time.Time
}

// HealthTracker manages health states for all agents
type HealthTracker struct {
	mu     sync.RWMutex
	states map[string]*AgentHealthState // agentID -> state
	config *HealthCheckConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// For round-robin load balancing
	lastSelectedIndex atomic.Int32
}

// NewHealthTracker creates a new health tracker
func NewHealthTracker(config *HealthCheckConfig) *HealthTracker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthTracker{
		states: make(map[string]*AgentHealthState),
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the health tracker background routines
func (t *HealthTracker) Start() {
	t.wg.Add(1)
	go t.scoreDecayLoop()
}

// Stop stops the health tracker
func (t *HealthTracker) Stop() {
	t.cancel()
	t.wg.Wait()
}

// scoreDecayLoop periodically decays health scores
func (t *HealthTracker) scoreDecayLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.config.DecayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.decayScores()
		}
	}
}

// decayScores decays all health scores
func (t *HealthTracker) decayScores() {
	t.mu.RLock()
	states := make([]*AgentHealthState, 0, len(t.states))
	for _, state := range t.states {
		states = append(states, state)
	}
	t.mu.RUnlock()

	decay := -(t.config.ScoreDecayRate * t.config.MaxScore)
	for _, state := range states {
		state.updateHealthScore(decay)
	}
}

// RegisterAgent registers a new agent or updates an existing one
func (t *HealthTracker) RegisterAgent(agentID, addr string) *AgentHealthState {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, ok := t.states[agentID]; ok {
		// Update address if changed
		if state.Addr != addr {
			state.mu.Lock()
			state.Addr = addr
			state.mu.Unlock()
		}
		return state
	}

	state := NewAgentHealthState(agentID, addr, t.config)
	t.states[agentID] = state
	return state
}

// UnregisterAgent removes an agent from tracking
func (t *HealthTracker) UnregisterAgent(agentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.states, agentID)
}

// GetState returns the health state for an agent
func (t *HealthTracker) GetState(agentID string) (*AgentHealthState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state, ok := t.states[agentID]
	return state, ok
}

// GetOrCreateState returns the health state for an agent, creating it if necessary
func (t *HealthTracker) GetOrCreateState(agentID, addr string) *AgentHealthState {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, ok := t.states[agentID]; ok {
		return state
	}

	state := NewAgentHealthState(agentID, addr, t.config)
	t.states[agentID] = state
	return state
}

// GetAvailableAgents returns all agents that are available for routing
func (t *HealthTracker) GetAvailableAgents() []*AgentHealthState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	available := make([]*AgentHealthState, 0, len(t.states))
	for _, state := range t.states {
		if state.IsAvailable() {
			available = append(available, state)
		}
	}
	return available
}

// GetAllStates returns a snapshot of all agent states
func (t *HealthTracker) GetAllStates() map[string]*AgentHealthState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*AgentHealthState, len(t.states))
	for k, v := range t.states {
		result[k] = v
	}
	return result
}

// RecordSuccess records a successful request for an agent
func (t *HealthTracker) RecordSuccess(agentID string) {
	if state, ok := t.GetState(agentID); ok {
		state.RecordSuccess()
	}
}

// RecordFailure records a failed request for an agent
func (t *HealthTracker) RecordFailure(agentID string) {
	if state, ok := t.GetState(agentID); ok {
		state.RecordFailure()
	}
}

// IncrementConnections increments the connection count for an agent
func (t *HealthTracker) IncrementConnections(agentID string) {
	if state, ok := t.GetState(agentID); ok {
		state.IncrementConnections()
	}
}

// DecrementConnections decrements the connection count for an agent
func (t *HealthTracker) DecrementConnections(agentID string) {
	if state, ok := t.GetState(agentID); ok {
		state.DecrementConnections()
	}
}

// GetStatistics returns statistics for a specific agent
func (t *HealthTracker) GetStatistics(agentID string) (HealthStatistics, error) {
	state, ok := t.GetState(agentID)
	if !ok {
		return HealthStatistics{}, fmt.Errorf("agent not found: %s", agentID)
	}
	return state.GetStatistics(), nil
}

// GetAllStatistics returns statistics for all agents
func (t *HealthTracker) GetAllStatistics() map[string]HealthStatistics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]HealthStatistics, len(t.states))
	for id, state := range t.states {
		result[id] = state.GetStatistics()
	}
	return result
}

// Reset resets the health state for an agent (e.g., after agent re-registration)
func (t *HealthTracker) Reset(agentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, ok := t.states[agentID]; ok {
		state.mu.Lock()
		atomic.StoreInt32(&state.consecutiveFailures, 0)
		state.healthScore.Store(int64(math.Float64bits(t.config.MaxScore)))
		state.circuitState.Store(int32(CircuitClosed))
		state.circuitOpenedAt.Store(0)
		state.lastStateChange.Store(time.Now().UnixNano())
		state.mu.Unlock()
	}
}

// GetNextRoundRobinIndex returns the next index for round-robin selection
func (t *HealthTracker) GetNextRoundRobinIndex(count int) int32 {
	if count <= 0 {
		return 0
	}
	next := t.lastSelectedIndex.Add(1)
	return (next - 1) % int32(count)
}
