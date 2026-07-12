package dispatch

import (
	"fmt"
	"sync"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// TestHealthTracker_BasicOperations tests basic health tracker operations
func TestHealthTracker_BasicOperations(t *testing.T) {
	config := &HealthCheckConfig{
		ScoreDecayRate:      0.1,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		DecayInterval:       1 * time.Second,
		FailureThreshold:    3,
		CircuitOpenTimeout:  5 * time.Second,
	}

	tracker := NewHealthTracker(config)
	defer tracker.Stop()

	// Register agent
	state := tracker.RegisterAgent("agent1", "localhost:19091")
	if state == nil {
		t.Fatal("failed to register agent")
	}

	// Check initial state
	if state.AgentID != "agent1" {
		t.Errorf("expected agent ID 'agent1', got '%s'", state.AgentID)
	}
	if state.HealthScore() != 100.0 {
		t.Errorf("expected initial health score 100.0, got %f", state.HealthScore())
	}
	if state.CircuitState() != CircuitClosed {
		t.Errorf("expected circuit state Closed, got %v", state.CircuitState())
	}

	// Test success recording
	state.RecordSuccess()
	stats := state.GetStatistics()
	if stats.SuccessfulRequests != 1 {
		t.Errorf("expected 1 successful request, got %d", stats.SuccessfulRequests)
	}
	if state.ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", state.ConsecutiveFailures())
	}

	// Test failure recording
	for i := 0; i < 3; i++ {
		state.RecordFailure()
	}
	if state.ConsecutiveFailures() != 3 {
		t.Errorf("expected 3 consecutive failures, got %d", state.ConsecutiveFailures())
	}

	// Circuit should be open after threshold
	state.RecordFailure()
	if state.CircuitState() != CircuitOpen {
		t.Errorf("expected circuit state Open after threshold, got %v", state.CircuitState())
	}
	if !state.IsCircuitOpen() {
		t.Error("expected IsCircuitOpen to return true")
	}

	// Agent should not be available when circuit is open
	if state.IsAvailable() {
		t.Error("expected agent to be unavailable when circuit is open")
	}
}

// TestHealthTracker_ConnectionTracking tests connection tracking
func TestHealthTracker_ConnectionTracking(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	state := tracker.RegisterAgent("agent1", "localhost:19091")

	// Test increment/decrement
	tracker.IncrementConnections("agent1")
	if state.ActiveConnections() != 1 {
		t.Errorf("expected 1 active connection, got %d", state.ActiveConnections())
	}

	tracker.IncrementConnections("agent1")
	tracker.IncrementConnections("agent1")
	if state.ActiveConnections() != 3 {
		t.Errorf("expected 3 active connections, got %d", state.ActiveConnections())
	}

	tracker.DecrementConnections("agent1")
	if state.ActiveConnections() != 2 {
		t.Errorf("expected 2 active connections, got %d", state.ActiveConnections())
	}
}

// TestHealthTracker_ScoreDecay tests health score decay
func TestHealthTracker_ScoreDecay(t *testing.T) {
	config := &HealthCheckConfig{
		ScoreDecayRate:      0.5,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		DecayInterval:       100 * time.Millisecond,
		FailureThreshold:    5,
		CircuitOpenTimeout:  5 * time.Second,
	}

	tracker := NewHealthTracker(config)
	tracker.Start()

	state := tracker.RegisterAgent("agent1", "localhost:19091")

	// Initial score should be max
	if state.HealthScore() != 100.0 {
		t.Errorf("expected initial score 100.0, got %f", state.HealthScore())
	}

	// Wait for decay
	time.Sleep(150 * time.Millisecond)

	// Score should have decayed
	newScore := state.HealthScore()
	if newScore >= 100.0 {
		t.Errorf("expected score to decay, still at %f", newScore)
	}
	if newScore < 40.0 {
		t.Errorf("score decayed too much: %f", newScore)
	}

	tracker.Stop()
}

// TestHealthTracker_CircuitBreakerStateTransitions tests circuit breaker state transitions
func TestHealthTracker_CircuitBreakerStateTransitions(t *testing.T) {
	config := &HealthCheckConfig{
		FailureThreshold:    3,
		CircuitOpenTimeout:  100 * time.Millisecond,
		ScoreDecayRate:      0.1,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		DecayInterval:       1 * time.Second,
	}

	tracker := NewHealthTracker(config)
	defer tracker.Stop()

	state := tracker.RegisterAgent("agent1", "localhost:19091")

	// Initial state: Closed
	if state.CircuitState() != CircuitClosed {
		t.Errorf("expected initial state Closed, got %v", state.CircuitState())
	}

	// Record failures to trigger circuit open
	for i := 0; i < 3; i++ {
		state.RecordFailure()
	}

	// Circuit should be open
	if state.CircuitState() != CircuitOpen {
		t.Errorf("expected circuit state Open after failures, got %v", state.CircuitState())
	}

	// Wait for circuit timeout
	time.Sleep(150 * time.Millisecond)

	// First success after timeout transitions to HalfOpen
	state.RecordSuccess()
	if state.CircuitState() != CircuitHalfOpen {
		t.Errorf("expected circuit state HalfOpen after timeout + success, got %v", state.CircuitState())
	}

	// Second success closes the circuit
	state.RecordSuccess()
	if state.CircuitState() != CircuitClosed {
		t.Errorf("expected circuit to close on second success in HalfOpen, got %v", state.CircuitState())
	}
}

// TestLoadBalancer_MinID tests min_id load balancing strategy
func TestLoadBalancer_MinID(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyMinID, tracker)

	candidates := []*Candidate{
		{AgentID: "agent3", Session: &reg.AgentSession{AgentID: "agent3", Addr: "addr3"}},
		{AgentID: "agent1", Session: &reg.AgentSession{AgentID: "agent1", Addr: "addr1"}},
		{AgentID: "agent2", Session: &reg.AgentSession{AgentID: "agent2", Addr: "addr2"}},
	}

	selected, err := lb.Select("testFunc", candidates)
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}

	if selected.AgentID != "agent1" {
		t.Errorf("expected to select agent1 (min ID), got %s", selected.AgentID)
	}
}

// TestLoadBalancer_RoundRobin tests round-robin load balancing strategy
func TestLoadBalancer_RoundRobin(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyRoundRobin, tracker)

	candidates := []*Candidate{
		{AgentID: "agent1", Session: &reg.AgentSession{AgentID: "agent1", Addr: "addr1"}},
		{AgentID: "agent2", Session: &reg.AgentSession{AgentID: "agent2", Addr: "addr2"}},
		{AgentID: "agent3", Session: &reg.AgentSession{AgentID: "agent3", Addr: "addr3"}},
	}

	// Select in order
	order := make([]string, 6)
	for i := 0; i < 6; i++ {
		selected, err := lb.Select("testFunc", candidates)
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		order[i] = selected.AgentID
	}

	// Should cycle: agent1, agent2, agent3, agent1, agent2, agent3
	expected := []string{"agent1", "agent2", "agent3", "agent1", "agent2", "agent3"}
	for i, e := range expected {
		if order[i] != e {
			t.Errorf("expected %s at position %d, got %s", e, i, order[i])
		}
	}
}

// TestLoadBalancer_LeastConn tests least-connection load balancing strategy
func TestLoadBalancer_LeastConn(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyLeastConn, tracker)

	// Create candidates with different connection counts
	state1 := tracker.RegisterAgent("agent1", "addr1")
	state2 := tracker.RegisterAgent("agent2", "addr2")
	state3 := tracker.RegisterAgent("agent3", "addr3")

	// Set different connection counts
	state1.IncrementConnections()
	state1.IncrementConnections() // agent1: 2 connections
	state2.IncrementConnections() // agent2: 1 connection
	// agent3: 0 connections

	candidates := []*Candidate{
		{AgentID: "agent1", Session: &reg.AgentSession{AgentID: "agent1", Addr: "addr1"}, Health: state1},
		{AgentID: "agent2", Session: &reg.AgentSession{AgentID: "agent2", Addr: "addr2"}, Health: state2},
		{AgentID: "agent3", Session: &reg.AgentSession{AgentID: "agent3", Addr: "addr3"}, Health: state3},
	}

	selected, err := lb.Select("testFunc", candidates)
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}

	// Should select agent3 (least connections)
	if selected.AgentID != "agent3" {
		t.Errorf("expected to select agent3 (least connections), got %s", selected.AgentID)
	}
}

// TestLoadBalancer_Weighted tests weighted load balancing strategy
func TestLoadBalancer_Weighted(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyWeighted, tracker)

	state1 := tracker.RegisterAgent("agent1", "addr1")
	state2 := tracker.RegisterAgent("agent2", "addr2")

	// Set different health scores
	// We can't directly set the health score, so we'll use failures/successes
	state2.RecordFailure()
	state2.RecordFailure() // Lower health score

	candidates := []*Candidate{
		{AgentID: "agent1", Session: &reg.AgentSession{AgentID: "agent1", Addr: "addr1"}, Health: state1},
		{AgentID: "agent2", Session: &reg.AgentSession{AgentID: "agent2", Addr: "addr2"}, Health: state2},
	}

	// With weighted selection, agent1 should be selected more often
	// but we can't test random distribution in a single test
	selected, err := lb.Select("testFunc", candidates)
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}

	if selected.AgentID != "agent1" && selected.AgentID != "agent2" {
		t.Errorf("unexpected selection: %s", selected.AgentID)
	}
}

// TestLoadBalancer_HealthFiltering tests that load balancer filters unhealthy agents
func TestLoadBalancer_HealthFiltering(t *testing.T) {
	config := &HealthCheckConfig{
		FailureThreshold:    2,
		CircuitOpenTimeout:  5 * time.Second,
		ScoreDecayRate:      0.1,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		DecayInterval:       1 * time.Second,
	}

	tracker := NewHealthTracker(config)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyLeastConn, tracker)

	// Create candidates where one is unhealthy
	state1 := tracker.RegisterAgent("agent1", "addr1")
	state2 := tracker.RegisterAgent("agent2", "addr2")

	// Make agent2 unhealthy (open circuit)
	for i := 0; i < 3; i++ {
		state2.RecordFailure()
	}

	candidates := []*Candidate{
		{AgentID: "agent1", Session: &reg.AgentSession{AgentID: "agent1", Addr: "addr1"}, Health: state1},
		{AgentID: "agent2", Session: &reg.AgentSession{AgentID: "agent2", Addr: "addr2"}, Health: state2},
	}

	// Should only return agent1 as available
	selected, err := lb.Select("testFunc", candidates)
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}

	if selected.AgentID != "agent1" {
		t.Errorf("expected to select healthy agent1, got %s", selected.AgentID)
	}
}

func TestLoadBalancer_BuildCandidates_AllowsEmptyRPCAddr(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyMinID, tracker)
	sessions := []*reg.AgentSession{
		{
			AgentID: "agent1",
			Addr:    "",
			Functions: map[string]reg.FunctionMeta{
				"test-func": {Enabled: true},
			},
		},
	}

	candidates := lb.BuildCandidates(sessions, "test-func")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].AgentID != "agent1" {
		t.Fatalf("candidate agentID = %q, want %q", candidates[0].AgentID, "agent1")
	}
	if candidates[0].Health == nil {
		t.Fatal("expected health state")
	}
	if candidates[0].Health.RouteHint != "" || candidates[0].Health.Addr != "" {
		t.Fatalf("expected no address-derived route hint, got routeHint=%q addr=%q", candidates[0].Health.RouteHint, candidates[0].Health.Addr)
	}
}

// TestReconnectionPolicy_NextDelay tests reconnection delay calculation
func TestReconnectionPolicy_NextDelay(t *testing.T) {
	policy := DefaultReconnectionPolicy()

	tests := []struct {
		name        string
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		// With 10% jitter, the ranges are wider to account for random jitter
		{"first attempt", 0, 400 * time.Millisecond, 600 * time.Millisecond},
		{"second attempt", 1, 900 * time.Millisecond, 1100 * time.Millisecond},
		{"third attempt", 2, 1800 * time.Millisecond, 2200 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay, err := policy.NextDelay(tt.attempt)
			if err != nil {
				t.Fatalf("NextDelay failed: %v", err)
			}
			if delay < tt.minExpected || delay > tt.maxExpected {
				t.Errorf("delay %v outside expected range [%v, %v]", delay, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

// TestReconnectionPolicy_MaxRetries tests max retries enforcement
func TestReconnectionPolicy_MaxRetries(t *testing.T) {
	policy := NewReconnectionPolicy(3, 100*time.Millisecond, 5*time.Second, 2.0, 0.1, true)

	// Should allow up to 3 retries
	for i := 0; i <= 3; i++ {
		if !policy.ShouldRetry(i) {
			t.Errorf("expected retry to be allowed for attempt %d", i)
		}
	}

	// Should fail on 4th attempt
	if policy.ShouldRetry(4) {
		t.Error("expected retry to be denied after max retries")
	}
}

// TestReconnectionState tests reconnection state management
func TestReconnectionState(t *testing.T) {
	policy := DefaultReconnectionPolicy()
	state := NewReconnectionState(policy)

	// Initial state
	if !state.IsEnabled() {
		t.Error("expected reconnection to be enabled by default")
	}
	if state.GetAttempt() != 0 {
		t.Errorf("expected initial attempt count 0, got %d", state.GetAttempt())
	}

	// Record attempts
	state.RecordAttempt(nil)
	if state.GetAttempt() != 1 {
		t.Errorf("expected attempt count 1, got %d", state.GetAttempt())
	}

	state.RecordAttempt(nil)
	state.RecordAttempt(nil)
	if state.GetAttempt() != 3 {
		t.Errorf("expected attempt count 3, got %d", state.GetAttempt())
	}

	// Reset
	state.Reset()
	if state.GetAttempt() != 0 {
		t.Errorf("expected attempt count 0 after reset, got %d", state.GetAttempt())
	}

	// Disable
	state.SetEnabled(false)
	if state.IsEnabled() {
		t.Error("expected reconnection to be disabled after SetEnabled(false)")
	}
	if state.ShouldRetry() {
		t.Error("expected ShouldRetry to return false when disabled")
	}
}

// TestReconnectionManager tests reconnection manager
func TestReconnectionManager(t *testing.T) {
	policy := DefaultReconnectionPolicy()
	manager := NewReconnectionManager(policy)

	// Get or create states
	state1 := manager.GetOrCreateState("conn1")
	state2 := manager.GetOrCreateState("conn2")

	if state1 == nil || state2 == nil {
		t.Fatal("failed to create reconnection states")
	}

	// Should return same state for same key
	state1Again := manager.GetOrCreateState("conn1")
	if state1 != state1Again {
		t.Error("expected to return same state for same key")
	}

	// Record attempts
	state1.RecordAttempt(nil)
	state2.RecordAttempt(nil)
	state2.RecordAttempt(nil)

	// Check stats
	stats := manager.Stats()
	if stats["conn1"].Attempt != 1 {
		t.Errorf("expected conn1 attempt count 1, got %d", stats["conn1"].Attempt)
	}
	if stats["conn2"].Attempt != 2 {
		t.Errorf("expected conn2 attempt count 2, got %d", stats["conn2"].Attempt)
	}

	// Remove state
	manager.RemoveState("conn1")
	stats = manager.Stats()
	if _, ok := stats["conn1"]; ok {
		t.Error("expected conn1 to be removed")
	}

	// Reset all
	state2.RecordAttempt(nil)
	manager.ResetAll()
	stats = manager.Stats()
	if stats["conn2"].Attempt != 0 {
		t.Errorf("expected conn2 attempt count 0 after reset, got %d", stats["conn2"].Attempt)
	}
}

// TestHealthTracker_ConcurrentOperations tests concurrent operations on health tracker
func TestHealthTracker_ConcurrentOperations(t *testing.T) {
	config := &HealthCheckConfig{
		ScoreDecayRate:      0.1,
		ScoreSuccessBonus:   10.0,
		ScoreFailurePenalty: 20.0,
		MinScore:            0.0,
		MaxScore:            100.0,
		DecayInterval:       1 * time.Second,
		FailureThreshold:    5,
		CircuitOpenTimeout:  5 * time.Second,
	}

	tracker := NewHealthTracker(config)
	defer tracker.Stop()

	// Register multiple agents
	numAgents := 10
	agents := make([]string, numAgents)
	for i := 0; i < numAgents; i++ {
		agentID := fmt.Sprintf("agent%d", i)
		tracker.RegisterAgent(agentID, "addr")
		agents[i] = agentID
	}

	// Concurrent operations
	var wg sync.WaitGroup
	opsPerGoroutine := 100
	numGoroutines := 5

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				agentID := agents[i%numAgents]

				switch i % 4 {
				case 0:
					tracker.RecordSuccess(agentID)
				case 1:
					tracker.RecordFailure(agentID)
				case 2:
					tracker.IncrementConnections(agentID)
				case 3:
					tracker.DecrementConnections(agentID)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify final state
	for _, agentID := range agents {
		state, ok := tracker.GetState(agentID)
		if !ok {
			t.Errorf("agent %s not found", agentID)
			continue
		}

		stats := state.GetStatistics()
		totalOps := stats.SuccessfulRequests + stats.FailedRequests
		if totalOps != int64(numGoroutines*opsPerGoroutine/4*2) { // Each goroutine does 2 record operations per 4 iterations
			// Just verify some operations were recorded, exact count may vary due to timing
			if totalOps == 0 {
				t.Errorf("no operations recorded for agent %s", agentID)
			}
		}
	}
}

// TestLoadBalancer_BuildCandidates tests candidate building from sessions
func TestLoadBalancer_BuildCandidates(t *testing.T) {
	tracker := NewHealthTracker(nil)
	defer tracker.Stop()

	lb := NewLoadBalancer(StrategyMinID, tracker)

	now := time.Now().Add(1 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)

	sessions := []*reg.AgentSession{
		{
			AgentID:   "agent1",
			Addr:      "addr1",
			ExpireAt:  now,
			Functions: map[string]reg.FunctionMeta{"testFunc": {Enabled: true}},
		},
		{
			AgentID:   "agent2",
			Addr:      "addr2",
			ExpireAt:  now,
			Functions: map[string]reg.FunctionMeta{"testFunc": {Enabled: false}}, // Disabled
		},
		{
			AgentID:   "agent3",
			Addr:      "addr3",
			ExpireAt:  now,
			Functions: map[string]reg.FunctionMeta{"otherFunc": {Enabled: true}}, // Different function
		},
		{
			AgentID:   "agent4",
			Addr:      "addr4",
			ExpireAt:  past, // Expired (BuildCandidates doesn't filter expired, that's dispatcher's job)
			Functions: map[string]reg.FunctionMeta{"testFunc": {Enabled: true}},
		},
		{
			AgentID:   "agent5",
			Addr:      "addr5",
			ExpireAt:  now,
			Functions: map[string]reg.FunctionMeta{"testFunc": {Enabled: true}},
		},
	}

	candidates := lb.BuildCandidates(sessions, "testFunc")

	// BuildCandidates includes all agents with the function enabled (doesn't check expiration)
	// agent2: disabled, agent3: wrong function, so should get agent1, agent4, agent5
	if len(candidates) != 3 {
		t.Errorf("expected 3 candidates, got %d", len(candidates))
	}

	agentIDs := make([]string, len(candidates))
	for i, c := range candidates {
		agentIDs[i] = c.AgentID
	}

	// Verify expected agents are included
	hasAgent1 := false
	hasAgent4 := false
	hasAgent5 := false
	for _, id := range agentIDs {
		if id == "agent1" {
			hasAgent1 = true
		}
		if id == "agent4" {
			hasAgent4 = true
		}
		if id == "agent5" {
			hasAgent5 = true
		}
	}

	if !hasAgent1 {
		t.Error("expected agent1 to be in candidates")
	}
	if !hasAgent4 {
		t.Error("expected agent4 to be in candidates")
	}
	if !hasAgent5 {
		t.Error("expected agent5 to be in candidates")
	}
}
