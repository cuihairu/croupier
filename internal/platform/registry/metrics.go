package registry

import (
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/ops/v1"
)

// MetricsEntry stores a single metrics report from an agent.
type MetricsEntry struct {
	AgentID  string
	Report   *opsv1.MetricsReport
	Received time.Time
}

// MetricsStore stores metrics reports from agents with configurable retention.
type MetricsStore struct {
	mu          sync.RWMutex
	entries     []MetricsEntry   // Circular buffer for all entries
	byAgent     map[string][]int // agent_id -> indices in entries
	maxPerAgent int              // Maximum entries to keep per agent (default: 100)
	maxTotal    int              // Maximum total entries (default: 10000)
	head        int              // Circular buffer head
}

// NewMetricsStore creates a new metrics store.
func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		entries:     make([]MetricsEntry, 10000),
		byAgent:     make(map[string][]int),
		maxPerAgent: 100,
		maxTotal:    10000,
	}
}

// Add adds a metrics report to the store.
func (s *MetricsStore) Add(agentID string, report *opsv1.MetricsReport) {
	if agentID == "" || report == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current indices for this agent
	indices := s.byAgent[agentID]

	// Check if we need to evict old entries for this agent
	if len(indices) >= s.maxPerAgent {
		// Remove oldest entry for this agent
		oldIdx := indices[0]
		s.entries[oldIdx] = MetricsEntry{} // Clear
		s.byAgent[agentID] = append(indices[1:], s.head)
	} else {
		s.byAgent[agentID] = append(indices, s.head)
	}

	// Add new entry
	s.entries[s.head] = MetricsEntry{
		AgentID:  agentID,
		Report:   report,
		Received: time.Now(),
	}

	// Move head forward
	s.head = (s.head + 1) % s.maxTotal
}

// GetLatest returns the latest metrics report for an agent.
func (s *MetricsStore) GetLatest(agentID string) (*MetricsEntry, bool) {
	if agentID == "" {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	indices, ok := s.byAgent[agentID]
	if !ok || len(indices) == 0 {
		return nil, false
	}

	// Last index is the latest
	idx := indices[len(indices)-1]
	entry := s.entries[idx]
	if entry.AgentID == "" {
		return nil, false
	}

	return &entry, true
}

// GetAgentMetrics returns all metrics for an agent.
func (s *MetricsStore) GetAgentMetrics(agentID string, limit int) []MetricsEntry {
	if agentID == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	indices, ok := s.byAgent[agentID]
	if !ok || len(indices) == 0 {
		return nil
	}

	if limit <= 0 || limit > len(indices) {
		limit = len(indices)
	}

	result := make([]MetricsEntry, 0, limit)
	start := len(indices) - limit
	for i := start; i < len(indices); i++ {
		idx := indices[i]
		entry := s.entries[idx]
		if entry.AgentID != "" {
			result = append(result, entry)
		}
	}

	return result
}

// GetAllMetrics returns all metrics across all agents.
func (s *MetricsStore) GetAllMetrics(since time.Time, limit int) []MetricsEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MetricsEntry

	// Iterate through all entries (limited by limit)
	for i := 0; i < len(s.entries) && len(result) < limit; i++ {
		idx := (s.head - 1 - i + s.maxTotal) % s.maxTotal
		entry := s.entries[idx]
		if entry.AgentID == "" {
			continue
		}
		if !since.IsZero() && entry.Received.Before(since) {
			continue
		}
		result = append(result, entry)
	}

	return result
}

// ListAgents returns all agent IDs that have metrics.
func (s *MetricsStore) ListAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]string, 0, len(s.byAgent))
	for agentID := range s.byAgent {
		agents = append(agents, agentID)
	}
	return agents
}

// Clear removes all metrics for an agent.
func (s *MetricsStore) Clear(agentID string) {
	if agentID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	indices, ok := s.byAgent[agentID]
	if !ok {
		return
	}

	for _, idx := range indices {
		s.entries[idx] = MetricsEntry{}
	}

	delete(s.byAgent, agentID)
}

// Prune removes entries older than the given duration.
func (s *MetricsStore) Prune(olderThan time.Duration) {
	if olderThan <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)

	for agentID, indices := range s.byAgent {
		keep := make([]int, 0, len(indices))
		for _, idx := range indices {
			if s.entries[idx].Received.After(cutoff) {
				keep = append(keep, idx)
			} else {
				s.entries[idx] = MetricsEntry{}
			}
		}
		if len(keep) == 0 {
			delete(s.byAgent, agentID)
		} else {
			s.byAgent[agentID] = keep
		}
	}
}

// GetSystemInfo retrieves cached system info for an agent.
// System info is stored as part of metrics report's custom field or separate cache.
type SystemInfoCache struct {
	mu    sync.RWMutex
	infos map[string]*SystemInfoEntry
}

type SystemInfoEntry struct {
	Info      *opsv1.SystemInfo
	UpdatedAt time.Time
}

// NewSystemInfoCache creates a new system info cache.
func NewSystemInfoCache() *SystemInfoCache {
	return &SystemInfoCache{
		infos: make(map[string]*SystemInfoEntry),
	}
}

// Set stores system info for an agent.
func (c *SystemInfoCache) Set(agentID string, info *opsv1.SystemInfo) {
	if agentID == "" || info == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.infos[agentID] = &SystemInfoEntry{
		Info:      info,
		UpdatedAt: time.Now(),
	}
}

// Get retrieves system info for an agent.
func (c *SystemInfoCache) Get(agentID string) (*opsv1.SystemInfo, bool) {
	if agentID == "" {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.infos[agentID]
	if !ok {
		return nil, false
	}

	// Return a copy to avoid mutation
	info := &opsv1.SystemInfo{
		Hostname:      entry.Info.Hostname,
		Os:            entry.Info.Os,
		OsVersion:     entry.Info.OsVersion,
		KernelVersion: entry.Info.KernelVersion,
		Arch:          entry.Info.Arch,
		CpuCores:      entry.Info.CpuCores,
		TotalMemory:   entry.Info.TotalMemory,
		BootTime:      entry.Info.BootTime,
		AgentVersion:  entry.Info.AgentVersion,
		OpsStatus:     entry.Info.OpsStatus,
	}

	return info, true
}

// List returns all cached system info.
func (c *SystemInfoCache) List() map[string]*opsv1.SystemInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*opsv1.SystemInfo, len(c.infos))
	for agentID, entry := range c.infos {
		result[agentID] = entry.Info
	}
	return result
}

// Remove removes system info for an agent.
func (c *SystemInfoCache) Remove(agentID string) {
	if agentID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.infos, agentID)
}

// Prune removes entries older than the given duration.
func (c *SystemInfoCache) Prune(olderThan time.Duration) {
	if olderThan <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	for agentID, entry := range c.infos {
		if entry.UpdatedAt.Before(cutoff) {
			delete(c.infos, agentID)
		}
	}
}
