package loadbalancer

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/server/registry"
)

// LeastConnectionsBalancer implements least connections load balancing
type LeastConnectionsBalancer struct {
	stats       StatsCollector
	healthCheck HealthChecker
}

// NewLeastConnectionsBalancer creates a new least connections load balancer
func NewLeastConnectionsBalancer(stats StatsCollector, healthCheck HealthChecker) *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		stats:       stats,
		healthCheck: healthCheck,
	}
}

func (l *LeastConnectionsBalancer) Pick(ctx context.Context, candidates []*registry.AgentSession, key string) (*registry.AgentSession, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates available")
	}

	healthy := l.filterHealthy(candidates)
	if len(healthy) == 0 {
		return nil, errors.New("no healthy candidates available")
	}

	if l.stats == nil {
		// Fallback to round-robin if no stats available
		return healthy[0], nil
	}

	// Find agent with least active connections
	var selected *registry.AgentSession
	minConnections := int64(-1)

	for _, agent := range healthy {
		stats := l.stats.GetStats(agent.AgentID)
		if stats == nil {
			// No stats yet, prefer this agent
			return agent, nil
		}

		if minConnections == -1 || stats.ActiveConns < minConnections {
			minConnections = stats.ActiveConns
			selected = agent
		}
	}

	if selected == nil {
		return healthy[0], nil
	}

	return selected, nil
}

func (l *LeastConnectionsBalancer) Name() string {
	return "least_connections"
}

func (l *LeastConnectionsBalancer) UpdateHealth(agentID string, healthy bool) {
	if l.healthCheck != nil {
		l.healthCheck.SetHealthy(agentID, healthy)
	}
}

func (l *LeastConnectionsBalancer) filterHealthy(candidates []*registry.AgentSession) []*registry.AgentSession {
	if l.healthCheck == nil {
		return candidates
	}

	var healthy []*registry.AgentSession
	for _, agent := range candidates {
		if l.healthCheck.IsHealthy(agent.AgentID) {
			healthy = append(healthy, agent)
		}
	}
	return healthy
}