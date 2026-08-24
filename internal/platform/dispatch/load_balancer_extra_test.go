package dispatch

import (
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoadBalancer_EmptyStrategyDefaultsToMinID(t *testing.T) {
	lb := NewLoadBalancer("", nil)
	assert.Equal(t, StrategyMinID, lb.GetStrategy())
}

func TestLoadBalancer_SelectNoCandidates(t *testing.T) {
	lb := NewLoadBalancer(StrategyMinID, nil)
	_, err := lb.Select("fn", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no available agents")
}

func TestLoadBalancer_SelectFiltersUnavailableWithTracker(t *testing.T) {
	tracker := NewHealthTracker(nil)
	sick := tracker.RegisterAgent("sick-agent", "")
	for i := 0; i < int(DefaultHealthCheckConfig().FailureThreshold); i++ {
		sick.RecordFailure()
	}
	require.False(t, sick.IsAvailable())

	lb := NewLoadBalancer(StrategyMinID, tracker)
	_, err := lb.Select("fn", []*Candidate{
		{AgentID: "sick-agent", Health: sick},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy agents available")

	// Candidates without health state are still considered available.
	selected, err := lb.Select("fn", []*Candidate{{AgentID: "opaque-agent"}})
	require.NoError(t, err)
	assert.Equal(t, "opaque-agent", selected.AgentID)
}

func TestLoadBalancer_SelectUnknownStrategyFallsBackToMinID(t *testing.T) {
	lb := NewLoadBalancer("bogus", nil)
	selected, err := lb.Select("fn", []*Candidate{
		{AgentID: "b"},
		{AgentID: "a"},
	})
	require.NoError(t, err)
	assert.Equal(t, "a", selected.AgentID)
}

func TestLoadBalancer_SelectMinIDNoCandidatesAfterFilter(t *testing.T) {
	lb := NewLoadBalancer(StrategyMinID, nil)
	_, err := lb.selectMinID(nil)
	require.Error(t, err)
}

func TestLoadBalancer_SelectLeastConnTieBreaksByAgentID(t *testing.T) {
	tracker := NewHealthTracker(nil)
	busy := tracker.RegisterAgent("a-busy", "")
	busy.IncrementConnections()

	lb := NewLoadBalancer(StrategyLeastConn, tracker)
	selected, err := lb.selectLeastConn([]*Candidate{
		{AgentID: "z-free", Health: tracker.RegisterAgent("z-free", "")},
		{AgentID: "a-busy", Health: busy},
	})
	require.NoError(t, err)
	assert.Equal(t, "z-free", selected.AgentID)

	// Equal connection counts prefer the lower agent ID.
	selected, err = lb.selectLeastConn([]*Candidate{
		{AgentID: "z-free", Health: tracker.RegisterAgent("z-free", "")},
		{AgentID: "a-free", Health: tracker.RegisterAgent("a-free", "")},
	})
	require.NoError(t, err)
	assert.Equal(t, "a-free", selected.AgentID)

	_, err = lb.selectLeastConn(nil)
	require.Error(t, err)
}

func TestLoadBalancer_SelectWeightedVariants(t *testing.T) {
	tracker := NewHealthTracker(nil)
	lb := NewLoadBalancer(StrategyWeighted, tracker)

	selected, err := lb.selectWeighted([]*Candidate{
		{AgentID: "no-health"},
	})
	require.NoError(t, err)
	assert.Equal(t, "no-health", selected.AgentID)

	// Empty candidate list: total weight is zero, falls back to least-conn error.
	_, err = lb.selectWeighted(nil)
	require.Error(t, err)
}

func TestLoadBalancer_BuildCandidatesSkipsInvalidSessions(t *testing.T) {
	tracker := NewHealthTracker(nil)
	lb := NewLoadBalancer(StrategyMinID, tracker)

	sessions := []*reg.AgentSession{
		nil,
		{AgentID: ""},
		{
			AgentID: "disabled-agent",
			Functions: map[string]reg.FunctionMeta{
				"fn": {Enabled: false},
			},
		},
		{
			AgentID: "enabled-agent",
			Functions: map[string]reg.FunctionMeta{
				"fn": {Enabled: true},
			},
		},
	}

	candidates := lb.BuildCandidates(sessions, "fn")
	require.Len(t, candidates, 1)
	assert.Equal(t, "enabled-agent", candidates[0].AgentID)
	assert.NotNil(t, candidates[0].Health)
	assert.True(t, candidates[0].Available)

	// Registered agents reuse their existing health state.
	existing, ok := tracker.GetState("enabled-agent")
	require.True(t, ok)
	assert.Same(t, existing, lb.BuildCandidates(sessions, "fn")[0].Health)
}

func TestLoadBalancer_StatsReportStrategy(t *testing.T) {
	lb := NewLoadBalancer(StrategyRoundRobin, nil)
	stats := lb.GetStatistics()
	assert.Equal(t, StrategyRoundRobin, stats.Strategy)
}

func TestHealthTracker_RegisterAgentUpdatesRouteHint(t *testing.T) {
	tracker := NewHealthTracker(nil)

	first := tracker.RegisterAgent("agent-x", "hint-1")
	require.NotNil(t, first)

	updated := tracker.RegisterAgent("agent-x", "hint-2")
	assert.Same(t, first, updated)
	first.mu.RLock()
	hint := first.RouteHint
	addr := first.Addr
	first.mu.RUnlock()
	assert.Equal(t, "hint-2", hint)
	assert.Equal(t, "hint-2", addr)

	// Same hint returns without mutation.
	again := tracker.RegisterAgent("agent-x", "hint-2")
	assert.Same(t, first, again)
}

func TestAgentHealthState_TransitionOnFailureOpensCircuit(t *testing.T) {
	config := DefaultHealthCheckConfig()
	config.FailureThreshold = 2
	state := NewAgentHealthState("agent-f", "", config)

	state.RecordFailure()
	assert.Equal(t, CircuitClosed, state.CircuitState())

	state.RecordFailure()
	assert.Equal(t, CircuitOpen, state.CircuitState())
	assert.True(t, state.IsCircuitOpen())
}

func TestAgentHealthState_TransitionOnFailureFromHalfOpen(t *testing.T) {
	state := NewAgentHealthState("agent-ho", "", nil)
	state.circuitState.Store(int32(CircuitHalfOpen))

	state.RecordFailure()
	assert.Equal(t, CircuitOpen, state.CircuitState())
}

func TestAgentHealthState_TransitionOnSuccessFromOpen(t *testing.T) {
	config := DefaultHealthCheckConfig()
	config.CircuitOpenTimeout = time.Millisecond
	state := NewAgentHealthState("agent-o", "", config)

	state.circuitState.Store(int32(CircuitOpen))
	state.circuitOpenedAt.Store(time.Now().Add(-time.Hour).UnixNano())

	state.RecordSuccess()
	assert.Equal(t, CircuitHalfOpen, state.CircuitState())

	// A success in half-open closes the circuit again.
	state.RecordSuccess()
	assert.Equal(t, CircuitClosed, state.CircuitState())
}

func TestAgentHealthState_TransitionOnSuccessOpenNotExpired(t *testing.T) {
	state := NewAgentHealthState("agent-strict", "", nil)
	state.circuitState.Store(int32(CircuitOpen))
	state.circuitOpenedAt.Store(time.Now().UnixNano())

	state.RecordSuccess()
	assert.Equal(t, CircuitOpen, state.CircuitState())
}

func TestPickAgentByHashStableSelection(t *testing.T) {
	candidates := []*reg.AgentSession{
		{AgentID: "a"},
		{AgentID: "b"},
		{AgentID: "c"},
	}
	assert.Nil(t, pickAgentByHash(nil, "k"))
	assert.Equal(t, "a", pickAgentByHash(candidates, "k").AgentID)
	assert.Equal(t, "a", pickAgentByHash(candidates, "k").AgentID)

	single := []*reg.AgentSession{{AgentID: "only"}}
	assert.Equal(t, "only", pickAgentByHash(single, "k").AgentID)
	assert.Equal(t, "a", pickAgentByHash(candidates, "").AgentID)
	assert.Equal(t, "a", pickAgentByHash(candidates, "   ").AgentID)
}

func TestAgentHasServiceNilAgent(t *testing.T) {
	assert.False(t, agentHasService(nil, "provider", "fn"))
	assert.False(t, agentHasService(&reg.AgentSession{}, "", "fn"))
}
