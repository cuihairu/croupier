package dispatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatcher_SetHAEnabled(t *testing.T) {
	d := NewDispatcher(nil)

	// Enable HA
	d.SetHAEnabled(true)
	assert.True(t, d.haEnabled)
	assert.NotNil(t, d.GetHealthTracker())
	assert.NotNil(t, d.GetLoadBalancer())

	// Disable HA
	d.SetHAEnabled(false)
	assert.False(t, d.haEnabled)
	assert.Nil(t, d.GetHealthTracker())
	assert.Nil(t, d.GetLoadBalancer())
}

func TestDispatcher_SetHAEnabled_Idempotent(t *testing.T) {
	d := NewDispatcher(nil)

	d.SetHAEnabled(true)
	d.SetHAEnabled(true) // should be no-op
	assert.NotNil(t, d.GetHealthTracker())

	d.SetHAEnabled(false)
	d.SetHAEnabled(false) // should be no-op
	assert.Nil(t, d.GetHealthTracker())
}

func TestDispatcher_SetLoadBalanceStrategy(t *testing.T) {
	d := NewDispatcher(nil)
	d.SetHAEnabled(true)
	defer d.SetHAEnabled(false)

	d.SetLoadBalanceStrategy(StrategyRoundRobin)
	assert.Equal(t, StrategyRoundRobin, d.GetLoadBalanceStrategy())
}

func TestDispatcher_GetLoadBalanceStrategy_Default(t *testing.T) {
	d := NewDispatcher(nil)
	// Without HA, should return default
	strategy := d.GetLoadBalanceStrategy()
	assert.Equal(t, StrategyMinID, strategy)
}

func TestDispatcher_SetSessionResolver(t *testing.T) {
	d := NewDispatcher(nil)
	d.SetSessionResolver(nil)
	// Should not panic
}

func TestDispatcher_SetTaskEventQuery(t *testing.T) {
	d := NewDispatcher(nil)
	d.SetTaskEventQuery(nil)
	// Should not panic
}

func TestCircuitBreakerState_String(t *testing.T) {
	tests := []struct {
		state CircuitBreakerState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half_open"},
		{CircuitBreakerState(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestHealthTracker_UnregisterAgent(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.UnregisterAgent("agent-1")

	// After unregister, GetState should not find it
	_, ok := tracker.GetState("agent-1")
	assert.False(t, ok)
}

func TestHealthTracker_GetAvailableAgents(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.RegisterAgent("agent-2", "addr-2")

	agents := tracker.GetAvailableAgents()
	assert.NotNil(t, agents)
	assert.Len(t, agents, 2)
}

func TestHealthTracker_GetAllStates(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.RegisterAgent("agent-2", "addr-2")

	states := tracker.GetAllStates()
	assert.Len(t, states, 2)
}

func TestHealthTracker_GetStatistics(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	stats, err := tracker.GetStatistics("agent-1")
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalRequests)
}

func TestHealthTracker_GetStatistics_NotFound(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	_, err := tracker.GetStatistics("nonexistent")
	assert.Error(t, err)
}

func TestHealthTracker_GetAllStatistics(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	stats := tracker.GetAllStatistics()
	assert.Len(t, stats, 1)
}

func TestHealthTracker_Reset(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.Reset("agent-1")

	state, ok := tracker.GetState("agent-1")
	assert.True(t, ok)
	assert.Equal(t, CircuitClosed, state.CircuitState())
}

func TestHealthTracker_GetNextRoundRobinIndex(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	idx := tracker.GetNextRoundRobinIndex(3)
	assert.True(t, idx >= 0 && idx < 3)
}

func TestHealthTracker_GetNextRoundRobinIndex_Zero(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	idx := tracker.GetNextRoundRobinIndex(0)
	assert.Equal(t, int32(0), idx)
}

func TestLoadBalancer_GetStrategy(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	lb := NewLoadBalancer(StrategyRoundRobin, tracker)
	assert.Equal(t, StrategyRoundRobin, lb.GetStrategy())
}

func TestLoadBalancer_SetStrategy(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	lb := NewLoadBalancer(StrategyMinID, tracker)
	lb.SetStrategy(StrategyLeastConn)
	assert.Equal(t, StrategyLeastConn, lb.GetStrategy())
}

func TestNewDispatcherWithHA(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyRoundRobin, nil)

	assert.NotNil(t, d)
	assert.True(t, d.haEnabled)
	assert.NotNil(t, d.GetHealthTracker())
	assert.NotNil(t, d.GetLoadBalancer())
}

func TestHealthTracker_GetOrCreateState(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	// First call creates
	state1 := tracker.GetOrCreateState("agent-1", "addr-1")
	assert.NotNil(t, state1)
	assert.Equal(t, "agent-1", state1.AgentID)

	// Second call returns existing
	state2 := tracker.GetOrCreateState("agent-1", "addr-2")
	assert.Equal(t, state1, state2)
}

func TestHealthTracker_StartStop(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.Start()
	tracker.Stop()
	// Should not panic
}

func TestHealthTracker_RecordSuccess(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.RecordSuccess("agent-1")

	stats, err := tracker.GetStatistics("agent-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.SuccessfulRequests)
}

func TestHealthTracker_RecordFailure(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.RecordFailure("agent-1")

	stats, err := tracker.GetStatistics("agent-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.FailedRequests)
}

func TestHealthTracker_IncrementDecrementConnections(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)

	tracker.RegisterAgent("agent-1", "addr-1")
	tracker.IncrementConnections("agent-1")
	tracker.IncrementConnections("agent-1")

	stats, err := tracker.GetStatistics("agent-1")
	require.NoError(t, err)
	assert.Equal(t, int32(2), stats.ActiveConnections)

	tracker.DecrementConnections("agent-1")
	stats, _ = tracker.GetStatistics("agent-1")
	assert.Equal(t, int32(1), stats.ActiveConnections)
}

func TestAgentHealthState_RecordSuccess(t *testing.T) {
	config := DefaultHealthCheckConfig()
	state := NewAgentHealthState("agent-1", "addr-1", config)

	state.RecordSuccess()
	assert.Equal(t, int64(1), state.GetStatistics().TotalRequests)
	assert.Equal(t, int64(1), state.GetStatistics().SuccessfulRequests)
}

func TestAgentHealthState_RecordFailure(t *testing.T) {
	config := DefaultHealthCheckConfig()
	state := NewAgentHealthState("agent-1", "addr-1", config)

	state.RecordFailure()
	assert.Equal(t, int64(1), state.GetStatistics().TotalRequests)
	assert.Equal(t, int64(1), state.GetStatistics().FailedRequests)
}

func TestAgentHealthState_Connections(t *testing.T) {
	config := DefaultHealthCheckConfig()
	state := NewAgentHealthState("agent-1", "addr-1", config)

	assert.Equal(t, int32(0), state.ActiveConnections())
	state.IncrementConnections()
	assert.Equal(t, int32(1), state.ActiveConnections())
	state.DecrementConnections()
	assert.Equal(t, int32(0), state.ActiveConnections())
}

func TestAgentHealthState_IsAvailable(t *testing.T) {
	config := DefaultHealthCheckConfig()
	state := NewAgentHealthState("agent-1", "addr-1", config)

	assert.True(t, state.IsAvailable())
}

func TestAgentHealthState_IsCircuitOpen(t *testing.T) {
	config := DefaultHealthCheckConfig()
	state := NewAgentHealthState("agent-1", "addr-1", config)

	assert.False(t, state.IsCircuitOpen())
}

func TestLoadBalancer_Select_Empty(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	lb := NewLoadBalancer(StrategyMinID, tracker)

	_, err := lb.Select("func-1", nil)
	assert.Error(t, err)
}

func TestLoadBalancer_Select_SingleCandidate(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	lb := NewLoadBalancer(StrategyRoundRobin, tracker)

	candidate := &Candidate{
		AgentID:   "agent-1",
		Available: true,
	}
	selected, err := lb.Select("func-1", []*Candidate{candidate})
	require.NoError(t, err)
	assert.Equal(t, "agent-1", selected.AgentID)
}

func TestLoadBalancer_Select_RoundRobin(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	lb := NewLoadBalancer(StrategyRoundRobin, tracker)

	candidates := []*Candidate{
		{AgentID: "agent-1", Available: true},
		{AgentID: "agent-2", Available: true},
	}

	// Select multiple times to exercise round-robin
	s1, err := lb.Select("func-1", candidates)
	require.NoError(t, err)
	s2, err := lb.Select("func-1", candidates)
	require.NoError(t, err)
	assert.NotEqual(t, s1.AgentID, s2.AgentID)
}

func TestLoadBalancer_Select_LeastConn(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	lb := NewLoadBalancer(StrategyLeastConn, tracker)

	h1 := tracker.RegisterAgent("agent-1", "addr-1")
	h2 := tracker.RegisterAgent("agent-2", "addr-2")
	h1.IncrementConnections()
	h1.IncrementConnections()
	// agent-1 has 2 connections, agent-2 has 0

	candidates := []*Candidate{
		{AgentID: "agent-1", Health: h1, Available: true},
		{AgentID: "agent-2", Health: h2, Available: true},
	}

	selected, err := lb.Select("func-1", candidates)
	require.NoError(t, err)
	assert.Equal(t, "agent-2", selected.AgentID)
}

func TestLoadBalancer_Select_Weighted(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	lb := NewLoadBalancer(StrategyWeighted, tracker)

	candidates := []*Candidate{
		{AgentID: "agent-1", Available: true, Health: tracker.RegisterAgent("agent-1", "addr-1")},
		{AgentID: "agent-2", Available: true, Health: tracker.RegisterAgent("agent-2", "addr-2")},
	}

	selected, err := lb.Select("func-1", candidates)
	require.NoError(t, err)
	assert.NotNil(t, selected)
}

func TestLoadBalancer_GetStatistics(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	lb := NewLoadBalancer(StrategyRoundRobin, tracker)

	stats := lb.GetStatistics()
	assert.Equal(t, StrategyRoundRobin, stats.Strategy)
}

func TestIsTaskRunDone(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"succeeded", true},
		{"success", true},
		{"done", true},
		{"completed", true},
		{"failed", true},
		{"error", true},
		{"cancelled", true},
		{"canceled", true},
		{"timed_out", true},
		{"timeout", true},
		{"running", false},
		{"pending", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.want, isTaskRunDone(tt.status))
		})
	}
}

func TestIsTaskEventTypeDone(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{"completed", true},
		{"success", true},
		{"succeeded", true},
		{"failed", true},
		{"error", true},
		{"cancelled", true},
		{"canceled", true},
		{"progress", false},
		{"started", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			assert.Equal(t, tt.want, isTaskEventTypeDone(tt.eventType))
		})
	}
}

func TestHealthTracker_RecordSuccess_NotFound(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	// Should not panic for unknown agent
	tracker.RecordSuccess("unknown")
}

func TestHealthTracker_RecordFailure_NotFound(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	// Should not panic for unknown agent
	tracker.RecordFailure("unknown")
}

func TestHealthTracker_IncrementConnections_NotFound(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	// Should not panic for unknown agent
	tracker.IncrementConnections("unknown")
}

func TestHealthTracker_DecrementConnections_NotFound(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	// Should not panic for unknown agent
	tracker.DecrementConnections("unknown")
}

func TestHealthTracker_Reset_NotFound(t *testing.T) {
	config := DefaultHealthCheckConfig()
	tracker := NewHealthTracker(config)
	// Should not panic for unknown agent
	tracker.Reset("unknown")
}

func TestDefaultHealthCheckConfig(t *testing.T) {
	config := DefaultHealthCheckConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 0.1, config.ScoreDecayRate)
	assert.Equal(t, 10.0, config.ScoreSuccessBonus)
	assert.Equal(t, 20.0, config.ScoreFailurePenalty)
	assert.Equal(t, 0.0, config.MinScore)
	assert.Equal(t, 100.0, config.MaxScore)
	assert.Equal(t, int32(5), config.FailureThreshold)
}

func TestNewAgentHealthState_NilConfig(t *testing.T) {
	state := NewAgentHealthState("agent-1", "addr-1", nil)
	assert.NotNil(t, state)
	assert.Equal(t, 100.0, state.HealthScore())
}

func TestAgentHealthState_CircuitBreakerTransitions(t *testing.T) {
	config := &HealthCheckConfig{
		ScoreDecayRate:      0.1,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		FailureThreshold:    3,
		CircuitOpenTimeout:  0, // Immediate transition to half-open
		HalfOpenMaxRequests: 1,
	}

	state := NewAgentHealthState("agent-1", "addr-1", config)

	// Record failures to trigger circuit open
	for i := 0; i < 3; i++ {
		state.RecordFailure()
	}

	assert.Equal(t, CircuitOpen, state.CircuitState())
	assert.False(t, state.IsAvailable())

	// Record success should transition to half-open (since timeout is 0)
	state.RecordSuccess()
	assert.Equal(t, CircuitHalfOpen, state.CircuitState())

	// Another success should close the circuit
	state.RecordSuccess()
	assert.Equal(t, CircuitClosed, state.CircuitState())
	assert.True(t, state.IsAvailable())
}

func TestDispatcher_Close_WithHA(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
	err := d.Close()
	assert.NoError(t, err)
}
