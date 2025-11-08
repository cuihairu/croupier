package loadbalancer

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/server/registry"
)

// LoadBalancer defines the interface for different load balancing strategies
type LoadBalancer interface {
	// Pick selects an agent from the available candidates
	Pick(ctx context.Context, candidates []*registry.AgentSession, key string) (*registry.AgentSession, error)
	// Name returns the name of the load balancing strategy
	Name() string
	// UpdateHealth updates the health status of an agent
	UpdateHealth(agentID string, healthy bool)
}

// HealthChecker monitors agent health status
type HealthChecker interface {
	// IsHealthy returns true if the agent is healthy
	IsHealthy(agentID string) bool
	// SetHealthy marks an agent as healthy or unhealthy
	SetHealthy(agentID string, healthy bool)
	// GetHealthScore returns the health score (0-100)
	GetHealthScore(agentID string) int
	// StartHealthCheck begins periodic health checking
	StartHealthCheck(ctx context.Context, agents []*registry.AgentSession) error
}

// AgentStats holds statistics for an agent
type AgentStats struct {
	AgentID         string
	ActiveConns     int64
	TotalRequests   int64
	FailedRequests  int64
	AvgResponseTime time.Duration
	LastSeen        time.Time
	Weight          int
	Healthy         bool
	// Derived metrics (best-effort, sliding window)
	QPS1m float64
}

// StatsCollector collects and maintains agent statistics
type StatsCollector interface {
	// RecordRequest records a request to an agent
	RecordRequest(agentID string, duration time.Duration, success bool)
	// GetStats returns current stats for an agent
	GetStats(agentID string) *AgentStats
	// GetAllStats returns stats for all agents
	GetAllStats() map[string]*AgentStats
	// IncrementActiveConns increments active connection count
	IncrementActiveConns(agentID string)
	// DecrementActiveConns decrements active connection count
	DecrementActiveConns(agentID string)
}
