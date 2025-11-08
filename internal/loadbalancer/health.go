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
    // per-agent QPS window (last 60 seconds, per-second buckets)
    qps   map[string]*qpsWindow
}

// NewDefaultStatsCollector creates a new stats collector
func NewDefaultStatsCollector() *DefaultStatsCollector {
    return &DefaultStatsCollector{
        stats: make(map[string]*AgentStats),
        qps:   make(map[string]*qpsWindow),
    }
}

type qpsWindow struct {
    slots   [60]int64
    lastSec int64
}

func (w *qpsWindow) add(nowSec int64) {
    if w.lastSec == 0 {
        w.lastSec = nowSec
    }
    // advance and zero passed buckets if time jumped
    if nowSec > w.lastSec {
        diff := nowSec - w.lastSec
        if diff >= 60 {
            // too far, reset all
            for i := 0; i < 60; i++ { w.slots[i] = 0 }
        } else {
            for i := int64(1); i <= diff; i++ {
                w.slots[(w.lastSec+i)%60] = 0
            }
        }
        w.lastSec = nowSec
    }
    w.slots[nowSec%60]++
}

func (w *qpsWindow) qps(nowSec int64) float64 {
    if w.lastSec == 0 { return 0 }
    var sum int64
    var span int64
    // consider up to last 60 seconds actually elapsed
    if nowSec > w.lastSec { span = nowSec - w.lastSec + 1 } else { span = 1 }
    if span > 60 { span = 60 }
    // sum last span seconds ending at nowSec
    for i := int64(0); i < span; i++ {
        sum += w.slots[(nowSec-i)%60]
    }
    if span <= 0 { return 0 }
    return float64(sum) / float64(span)
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

    // update QPS window
    nowSec := time.Now().Unix()
    w := s.qps[agentID]
    if w == nil { w = &qpsWindow{}; s.qps[agentID] = w }
    w.add(nowSec)
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
    nowSec := time.Now().Unix()
    for id, stats := range s.stats {
        copy := *stats
        if w := s.qps[id]; w != nil {
            copy.QPS1m = w.qps(nowSec)
        }
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
