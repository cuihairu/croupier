package loadbalancer

import (
	"context"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/server/registry"
)

// DefaultHealthChecker implements basic health checking
type DefaultHealthChecker struct {
	mu      sync.RWMutex
	health  map[string]bool // agentID -> healthy
	scores  map[string]int  // agentID -> health score (0-100)
	timeout time.Duration
}

// NewDefaultHealthChecker creates a new health checker
func NewDefaultHealthChecker(timeout time.Duration) *DefaultHealthChecker {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &DefaultHealthChecker{
		health:  make(map[string]bool),
		scores:  make(map[string]int),
		timeout: timeout,
	}
}

func (h *DefaultHealthChecker) IsHealthy(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	healthy, exists := h.health[agentID]
	if !exists {
		return true // assume healthy if not tracked yet
	}
	return healthy
}

func (h *DefaultHealthChecker) SetHealthy(agentID string, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.health[agentID] = healthy
	if healthy {
		h.scores[agentID] = 100
	} else {
		h.scores[agentID] = 0
	}
}

func (h *DefaultHealthChecker) GetHealthScore(agentID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	score, exists := h.scores[agentID]
	if !exists {
		return 100 // assume fully healthy if not tracked
	}
	return score
}

func (h *DefaultHealthChecker) StartHealthCheck(ctx context.Context, agents []*registry.AgentSession) error {
	// Initialize all agents as healthy
	h.mu.Lock()
	for _, agent := range agents {
		h.health[agent.AgentID] = true
		h.scores[agent.AgentID] = 100
	}
	h.mu.Unlock()

	// Start periodic health checking
	ticker := time.NewTicker(h.timeout)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.performHealthCheck(agents)
			}
		}
	}()

	return nil
}

func (h *DefaultHealthChecker) performHealthCheck(agents []*registry.AgentSession) {
	// This is a basic implementation
	// In a real scenario, you would ping each agent or check metrics
	// For now, we'll assume all agents are healthy unless explicitly marked otherwise

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, agent := range agents {
		if _, exists := h.health[agent.AgentID]; !exists {
			h.health[agent.AgentID] = true
			h.scores[agent.AgentID] = 100
		}
	}
}

// DefaultStatsCollector implements basic statistics collection
type DefaultStatsCollector struct {
	mu    sync.RWMutex
	stats map[string]*AgentStats
}

// NewDefaultStatsCollector creates a new stats collector
func NewDefaultStatsCollector() *DefaultStatsCollector {
	return &DefaultStatsCollector{
		stats: make(map[string]*AgentStats),
	}
}

func (s *DefaultStatsCollector) RecordRequest(agentID string, duration time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := s.stats[agentID]
	if stats == nil {
		stats = &AgentStats{
			AgentID: agentID,
			Weight:  1,
			Healthy: true,
		}
		s.stats[agentID] = stats
	}

	stats.TotalRequests++
	if !success {
		stats.FailedRequests++
	}

	// Update average response time (simple moving average)
	if stats.TotalRequests == 1 {
		stats.AvgResponseTime = duration
	} else {
		stats.AvgResponseTime = time.Duration(
			(int64(stats.AvgResponseTime)*int64(stats.TotalRequests-1) + int64(duration)) / int64(stats.TotalRequests),
		)
	}

	stats.LastSeen = time.Now()
}

func (s *DefaultStatsCollector) GetStats(agentID string) *AgentStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := s.stats[agentID]
	if stats == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	copy := *stats
	return &copy
}

func (s *DefaultStatsCollector) GetAllStats() map[string]*AgentStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*AgentStats)
	for id, stats := range s.stats {
		copy := *stats
		result[id] = &copy
	}
	return result
}

func (s *DefaultStatsCollector) IncrementActiveConns(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.stats[agentID]
	if stats == nil {
		stats = &AgentStats{
			AgentID: agentID,
			Weight:  1,
			Healthy: true,
		}
		s.stats[agentID] = stats
	}
	stats.ActiveConns++
}

func (s *DefaultStatsCollector) DecrementActiveConns(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.stats[agentID]
	if stats != nil && stats.ActiveConns > 0 {
		stats.ActiveConns--
	}
}