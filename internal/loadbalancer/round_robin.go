package loadbalancer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/cuihairu/croupier/internal/server/registry"
)

// RoundRobinBalancer implements round-robin load balancing
type RoundRobinBalancer struct {
	counter     uint64
	healthCheck HealthChecker
}

// NewRoundRobinBalancer creates a new round-robin load balancer
func NewRoundRobinBalancer(healthCheck HealthChecker) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		healthCheck: healthCheck,
	}
}

func (r *RoundRobinBalancer) Pick(ctx context.Context, candidates []*registry.AgentSession, key string) (*registry.AgentSession, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates available")
	}

	// Filter healthy candidates
	healthy := r.filterHealthy(candidates)
	if len(healthy) == 0 {
		return nil, errors.New("no healthy candidates available")
	}

	// Round-robin selection
	index := atomic.AddUint64(&r.counter, 1) % uint64(len(healthy))
	return healthy[index], nil
}

func (r *RoundRobinBalancer) Name() string {
	return "round_robin"
}

func (r *RoundRobinBalancer) UpdateHealth(agentID string, healthy bool) {
	if r.healthCheck != nil {
		r.healthCheck.SetHealthy(agentID, healthy)
	}
}

func (r *RoundRobinBalancer) filterHealthy(candidates []*registry.AgentSession) []*registry.AgentSession {
	if r.healthCheck == nil {
		return candidates
	}

	var healthy []*registry.AgentSession
	for _, agent := range candidates {
		if r.healthCheck.IsHealthy(agent.AgentID) {
			healthy = append(healthy, agent)
		}
	}
	return healthy
}

// WeightedRoundRobinBalancer implements weighted round-robin load balancing
type WeightedRoundRobinBalancer struct {
	mu          sync.RWMutex
	weights     map[string]int // agentID -> weight
	counters    map[string]int // agentID -> current counter
	healthCheck HealthChecker
}

// NewWeightedRoundRobinBalancer creates a new weighted round-robin load balancer
func NewWeightedRoundRobinBalancer(healthCheck HealthChecker) *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		weights:     make(map[string]int),
		counters:    make(map[string]int),
		healthCheck: healthCheck,
	}
}

func (w *WeightedRoundRobinBalancer) SetWeight(agentID string, weight int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.weights[agentID] = weight
}

func (w *WeightedRoundRobinBalancer) Pick(ctx context.Context, candidates []*registry.AgentSession, key string) (*registry.AgentSession, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates available")
	}

	healthy := w.filterHealthy(candidates)
	if len(healthy) == 0 {
		return nil, errors.New("no healthy candidates available")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Find agent with highest current weight
	var selected *registry.AgentSession
	maxWeight := -1

	for _, agent := range healthy {
		weight := w.weights[agent.AgentID]
		if weight <= 0 {
			weight = 1 // default weight
		}

		currentWeight := w.counters[agent.AgentID] + weight
		if currentWeight > maxWeight {
			maxWeight = currentWeight
			selected = agent
		}
	}

	if selected == nil {
		return healthy[0], nil
	}

	// Update counters
	w.counters[selected.AgentID] = maxWeight
	for _, agent := range healthy {
		if agent.AgentID != selected.AgentID {
			weight := w.weights[agent.AgentID]
			if weight <= 0 {
				weight = 1
			}
			w.counters[agent.AgentID] += weight
		}
	}

	// Normalize counters to prevent overflow
	minCounter := maxWeight
	for _, counter := range w.counters {
		if counter < minCounter {
			minCounter = counter
		}
	}
	for agentID := range w.counters {
		w.counters[agentID] -= minCounter
	}

	return selected, nil
}

func (w *WeightedRoundRobinBalancer) Name() string {
	return "weighted_round_robin"
}

func (w *WeightedRoundRobinBalancer) UpdateHealth(agentID string, healthy bool) {
	if w.healthCheck != nil {
		w.healthCheck.SetHealthy(agentID, healthy)
	}
}

func (w *WeightedRoundRobinBalancer) filterHealthy(candidates []*registry.AgentSession) []*registry.AgentSession {
	if w.healthCheck == nil {
		return candidates
	}

	var healthy []*registry.AgentSession
	for _, agent := range candidates {
		if w.healthCheck.IsHealthy(agent.AgentID) {
			healthy = append(healthy, agent)
		}
	}
	return healthy
}
