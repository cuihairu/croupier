package dispatch

import (
	"fmt"
	"math"
	"sort"
	"sync"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// LoadBalanceStrategy defines the load balancing strategy
type LoadBalanceStrategy string

const (
	// StrategyMinID selects agent with minimum ID (original behavior)
	StrategyMinID LoadBalanceStrategy = "min_id"
	// StrategyRoundRobin selects agents in round-robin fashion
	StrategyRoundRobin LoadBalanceStrategy = "round_robin"
	// StrategyLeastConn selects agent with least active connections
	StrategyLeastConn LoadBalanceStrategy = "least_conn"
	// StrategyWeighted selects agent based on health score weights
	StrategyWeighted LoadBalanceStrategy = "weighted"
)

// Candidate represents an agent that can be selected for routing
type Candidate struct {
	AgentID   string
	Session   *reg.AgentSession
	Health    *AgentHealthState
	Available bool
}

// LoadBalancer selects the best agent for routing based on strategy
type LoadBalancer struct {
	strategy LoadBalanceStrategy
	tracker  *HealthTracker

	// For round-robin tracking
	mu              sync.Mutex
	roundRobinIndex map[string]int32 // functionID -> index
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy LoadBalanceStrategy, tracker *HealthTracker) *LoadBalancer {
	if strategy == "" {
		strategy = StrategyMinID // Default to original behavior
	}

	return &LoadBalancer{
		strategy:        strategy,
		tracker:         tracker,
		roundRobinIndex: make(map[string]int32),
	}
}

// SetStrategy updates the load balancing strategy
func (lb *LoadBalancer) SetStrategy(strategy LoadBalanceStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
}

// GetStrategy returns the current load balancing strategy
func (lb *LoadBalancer) GetStrategy() LoadBalanceStrategy {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.strategy
}

// Select selects the best agent for the given function
func (lb *LoadBalancer) Select(functionID string, candidates []*Candidate) (*Candidate, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available agents for function %s", functionID)
	}

	// Filter to only available agents if health tracking is enabled
	var available []*Candidate
	if lb.tracker != nil {
		for _, c := range candidates {
			if c.Health != nil && c.Health.IsAvailable() {
				available = append(available, c)
			} else if c.Health == nil {
				// No health state yet, consider available
				available = append(available, c)
			}
		}
	} else {
		available = candidates
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("no healthy agents available for function %s", functionID)
	}

	// Select based on strategy
	lb.mu.Lock()
	strategy := lb.strategy
	lb.mu.Unlock()

	switch strategy {
	case StrategyMinID:
		return lb.selectMinID(available)
	case StrategyRoundRobin:
		return lb.selectRoundRobin(functionID, available)
	case StrategyLeastConn:
		return lb.selectLeastConn(available)
	case StrategyWeighted:
		return lb.selectWeighted(available)
	default:
		return lb.selectMinID(available)
	}
}

// selectMinID selects the agent with minimum ID (original behavior)
func (lb *LoadBalancer) selectMinID(candidates []*Candidate) (*Candidate, error) {
	var chosen *Candidate
	for _, c := range candidates {
		if chosen == nil || c.AgentID < chosen.AgentID {
			chosen = c
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no candidates available")
	}
	return chosen, nil
}

// selectRoundRobin selects agents in round-robin fashion
func (lb *LoadBalancer) selectRoundRobin(functionID string, candidates []*Candidate) (*Candidate, error) {
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	// Sort by AgentID for stable ordering
	sorted := make([]*Candidate, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AgentID < sorted[j].AgentID
	})

	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Get or initialize index for this function
	idx, ok := lb.roundRobinIndex[functionID]
	if !ok {
		idx = 0
	}

	// Select candidate and update index
	selected := sorted[idx%int32(len(sorted))]
	lb.roundRobinIndex[functionID] = (idx + 1) % int32(len(sorted))

	return selected, nil
}

// selectLeastConn selects the agent with least active connections
func (lb *LoadBalancer) selectLeastConn(candidates []*Candidate) (*Candidate, error) {
	var chosen *Candidate
	minConns := int32(math.MaxInt32)

	for _, c := range candidates {
		var conns int32
		if c.Health != nil {
			conns = c.Health.ActiveConnections()
		}
		if conns < minConns {
			minConns = conns
			chosen = c
		} else if conns == minConns && chosen != nil {
			// Tie-breaker: prefer lower agent ID
			if c.AgentID < chosen.AgentID {
				chosen = c
			}
		}
	}

	if chosen == nil {
		return nil, fmt.Errorf("no candidates available")
	}

	return chosen, nil
}

// selectWeighted selects agent based on health score weights
func (lb *LoadBalancer) selectWeighted(candidates []*Candidate) (*Candidate, error) {
	// Calculate total weight
	totalWeight := float64(0)
	weights := make([]float64, len(candidates))

	for i, c := range candidates {
		var weight float64
		if c.Health != nil {
			// Use health score as weight, with minimum weight of 1
			weight = math.Max(1.0, c.Health.HealthScore())
		} else {
			// No health info, use default weight
			weight = 50.0
		}
		weights[i] = weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		// Fallback to least connections
		return lb.selectLeastConn(candidates)
	}

	// Weighted random selection
	target := totalWeight * 0.5 // Use midpoint for stability (could use random)
	accumulated := float64(0)

	for i, weight := range weights {
		accumulated += weight
		if accumulated >= target {
			return candidates[i], nil
		}
	}

	// Fallback to last candidate
	return candidates[len(candidates)-1], nil
}

// BuildCandidates builds candidate list from agent sessions
func (lb *LoadBalancer) BuildCandidates(sessions []*reg.AgentSession, functionID string) []*Candidate {
	candidates := make([]*Candidate, 0, len(sessions))

	for _, session := range sessions {
		if session == nil || session.AgentID == "" {
			continue
		}

		// Check if function is enabled
		meta, ok := session.Functions[functionID]
		if !ok || !meta.Enabled {
			continue
		}

		var health *AgentHealthState
		if lb.tracker != nil {
			if h, ok := lb.tracker.GetState(session.AgentID); ok {
				health = h
			} else {
				health = lb.tracker.RegisterAgent(session.AgentID, "")
			}
		}

		candidates = append(candidates, &Candidate{
			AgentID:   session.AgentID,
			Session:   session,
			Health:    health,
			Available: health == nil || health.IsAvailable(),
		})
	}

	return candidates
}

// GetStatistics returns load balancer statistics
func (lb *LoadBalancer) GetStatistics() LoadBalancerStats {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	return LoadBalancerStats{
		Strategy:        lb.strategy,
		RoundRobinState: lb.roundRobinIndex,
	}
}

// LoadBalancerStats contains load balancer statistics
type LoadBalancerStats struct {
	Strategy        LoadBalanceStrategy       `json:"strategy"`
	RoundRobinState map[string]int32          `json:"roundRobinState,omitempty"`
	AgentStats      map[string]CandidateStats `json:"agentStats,omitempty"`
}

// CandidateStats contains statistics for a single candidate
type CandidateStats struct {
	AgentID             string  `json:"agentId"`
	ActiveConnections   int32   `json:"activeConnections"`
	ConsecutiveFailures int32   `json:"consecutiveFailures"`
	HealthScore         float64 `json:"healthScore"`
	CircuitState        string  `json:"circuitState"`
	Available           bool    `json:"available"`
}
