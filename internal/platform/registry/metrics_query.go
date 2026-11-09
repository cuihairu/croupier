package registry

import (
	"encoding/json"

	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

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

	idx := indices[len(indices)-1]
	entry := s.entries[idx]
	if entry.AgentID == "" {
		return nil, false
	}

	return &entry, true
}

// GetHistory returns metrics history for an agent.
// First checks memory, then falls back to database.
func (s *MetricsStore) GetHistory(agentID string, since time.Time, limit int) []MetricsEntry {
	if agentID == "" {
		return nil
	}

	// First try memory
	memResult := s.getFromMemory(agentID, since, limit)
	if len(memResult) > 0 && memResult[len(memResult)-1].Received.After(since) {
		return memResult
	}

	// Fall back to database
	if s.db.Load() != nil {
		return s.getFromDB(agentID, since, limit)
	}

	return memResult
}

// getFromMemory retrieves metrics from memory.
func (s *MetricsStore) getFromMemory(agentID string, since time.Time, limit int) []MetricsEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indices, ok := s.byAgent[agentID]
	if !ok || len(indices) == 0 {
		return nil
	}

	if limit <= 0 || limit > len(indices) {
		limit = len(indices)
	}

	// Size the allocation from stored data (internal, bounded) instead of
	// the caller-provided limit, so the request cannot drive the allocation.
	const maxAllocation = 10000
	allocHint := len(indices)
	if allocHint > maxAllocation {
		allocHint = maxAllocation
	}

	result := make([]MetricsEntry, 0, allocHint)
	start := len(indices) - limit
	for i := start; i < len(indices); i++ {
		idx := indices[i]
		entry := s.entries[idx]
		if entry.AgentID != "" && entry.Received.After(since) {
			result = append(result, entry)
		}
	}

	return result
}

// getFromDB retrieves metrics from database.
func (s *MetricsStore) getFromDB(agentID string, since time.Time, limit int) []MetricsEntry {
	db := s.db.Load()
	var records []AgentMetricsHistory
	query := db.Where("agent_id = ? AND timestamp >= ?", agentID, since).
		Order("timestamp DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	query.Find(&records)

	result := make([]MetricsEntry, len(records))
	for i, record := range records {
		report := &opsv1.MetricsReport{
			AgentId: agentID,
		}
		if len(record.CPUJSON) > 0 {
			cpu := &opsv1.CpuMetrics{}
			_ = json.Unmarshal(record.CPUJSON, cpu)
			report.Cpu = cpu
		}
		if len(record.MemoryJSON) > 0 {
			mem := &opsv1.MemoryMetrics{}
			_ = json.Unmarshal(record.MemoryJSON, mem)
			report.Memory = mem
		}
		if len(record.DisksJSON) > 0 {
			disks := []*opsv1.DiskMetrics{}
			_ = json.Unmarshal(record.DisksJSON, &disks)
			report.Disks = disks
		}
		if len(record.ProbeJSON) > 0 {
			probes := []*opsv1.ProbeStats{}
			_ = json.Unmarshal(record.ProbeJSON, &probes)
			report.ProbeStats = probes
		}
		if len(record.NetworksJSON) > 0 {
			nets := []*opsv1.NetworkMetrics{}
			_ = json.Unmarshal(record.NetworksJSON, &nets)
			report.Networks = nets
		}
		if len(record.ProcessesJSON) > 0 {
			procs := []*opsv1.ProcessMetrics{}
			_ = json.Unmarshal(record.ProcessesJSON, &procs)
			report.Processes = procs
		}
		if len(record.PortsJSON) > 0 {
			ports := []*opsv1.PortMetrics{}
			_ = json.Unmarshal(record.PortsJSON, &ports)
			report.Ports = ports
		}
		result[i] = MetricsEntry{
			AgentID:  agentID,
			Report:   report,
			Received: record.Timestamp,
		}
	}

	return result
}

// GetAgentMetrics returns all metrics for an agent (alias for GetHistory).
func (s *MetricsStore) GetAgentMetrics(agentID string, limit int) []MetricsEntry {
	return s.GetHistory(agentID, time.Time{}, limit)
}

// GetAllMetrics returns all metrics across all agents.
func (s *MetricsStore) GetAllMetrics(since time.Time, limit int) []MetricsEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MetricsEntry
	for i := 0; i < len(s.entries) && len(result) < limit; i++ {
		idx := (s.head - 1 - i + s.config.MaxTotalEntries) % s.config.MaxTotalEntries
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
	delete(s.supEvents, agentID)
	delete(s.supLastSeq, agentID)
	delete(s.supLastTs, agentID)
}
