package registry

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"gorm.io/gorm"
)

// MetricsEntry stores a single metrics report from an agent.
type MetricsEntry struct {
	AgentID  string
	Report   *opsv1.MetricsReport
	Received time.Time
}

// AgentMetricsHistory is the database model for persisting metrics.
type AgentMetricsHistory struct {
	ID         uint      `gorm:"primaryKey"`
	AgentID    string    `gorm:"index:idx_agent_time,not null"`
	Timestamp  time.Time `gorm:"index:idx_agent_time,not null"`
	CPUJSON    []byte    `gorm:"type:json"`
	MemoryJSON []byte    `gorm:"type:json"`
	DisksJSON  []byte    `gorm:"type:json"`
}

func (AgentMetricsHistory) TableName() string {
	return "agent_metrics_history"
}

// MetricsStoreConfig configures the metrics store behavior.
type MetricsStoreConfig struct {
	// MaxMemoryEntries is the max entries in memory per agent.
	// Default: 10 (5 minutes @ 30s interval)
	MaxMemoryEntries int `json:"maxMemoryEntries" yaml:"maxMemoryEntries"`
	// MaxTotalEntries is the max total entries in memory across all agents.
	// Default: 2000
	MaxTotalEntries int `json:"maxTotalEntries" yaml:"maxTotalEntries"`
	// Retention is how long to keep metrics in the database.
	// Default: 7 days
	Retention time.Duration `json:"retention" yaml:"retention"`
	// CleanupInterval is how often to run the cleanup routine.
	// Default: 1 hour
	CleanupInterval time.Duration `json:"cleanupInterval" yaml:"cleanupInterval"`
}

// DefaultMetricsStoreConfig returns the default configuration.
func DefaultMetricsStoreConfig() MetricsStoreConfig {
	return MetricsStoreConfig{
		MaxMemoryEntries: 10, // 5 minutes @ 30s interval
		MaxTotalEntries:  2000,
		Retention:        7 * 24 * time.Hour, // 7 days
		CleanupInterval:  1 * time.Hour,
	}
}

// MetricsStore stores metrics with memory + database hybrid approach.
type MetricsStore struct {
	mu      sync.RWMutex
	db      *gorm.DB
	config  MetricsStoreConfig
	entries []MetricsEntry
	byAgent map[string][]int
	head    int
	// onReport 是上报后回调（告警规则评估等）。异步执行且有 recover
	// 防护，评估异常不影响存储链路。
	onReport func(ctx context.Context, agentID string, report *opsv1.MetricsReport)
}

// SetOnReport sets the post-report callback (e.g. alert rule evaluation).
func (s *MetricsStore) SetOnReport(fn func(ctx context.Context, agentID string, report *opsv1.MetricsReport)) {
	s.mu.Lock()
	s.onReport = fn
	s.mu.Unlock()
}

// NewMetricsStore creates a new metrics store with default config.
func NewMetricsStore() *MetricsStore {
	return NewMetricsStoreWithConfig(DefaultMetricsStoreConfig())
}

// NewMetricsStoreWithConfig creates a new metrics store with custom config.
func NewMetricsStoreWithConfig(config MetricsStoreConfig) *MetricsStore {
	// Apply defaults for zero values
	if config.MaxMemoryEntries <= 0 {
		config.MaxMemoryEntries = 10
	}
	if config.MaxTotalEntries <= 0 {
		config.MaxTotalEntries = 2000
	}
	if config.Retention <= 0 {
		config.Retention = 7 * 24 * time.Hour
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 1 * time.Hour
	}

	return &MetricsStore{
		config:  config,
		entries: make([]MetricsEntry, config.MaxTotalEntries),
		byAgent: make(map[string][]int),
	}
}

// SetDB sets the database connection for persistence.
func (s *MetricsStore) SetDB(db *gorm.DB) {
	s.db = db
}

// Add adds a metrics report to the store.
func (s *MetricsStore) Add(agentID string, report *opsv1.MetricsReport) {
	if agentID == "" || report == nil {
		return
	}

	s.mu.Lock()
	// Add to memory
	indices := s.byAgent[agentID]
	if len(indices) >= s.config.MaxMemoryEntries {
		oldIdx := indices[0]
		s.entries[oldIdx] = MetricsEntry{}
		s.byAgent[agentID] = append(indices[1:], s.head)
	} else {
		s.byAgent[agentID] = append(indices, s.head)
	}

	s.entries[s.head] = MetricsEntry{
		AgentID:  agentID,
		Report:   report,
		Received: time.Now(),
	}
	s.head = (s.head + 1) % s.config.MaxTotalEntries
	onReport := s.onReport
	s.mu.Unlock()

	// Persist to database asynchronously
	go s.persistToDB(agentID, report)

	// 上报回调（异步，评估失败不影响存储）。
	if onReport != nil {
		go func() {
			defer func() {
				_ = recover() // 回调异常绝不拖垮上报链路
			}()
			evalCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			onReport(evalCtx, agentID, report)
		}()
	}
}

// persistToDB saves metrics to database.
func (s *MetricsStore) persistToDB(agentID string, report *opsv1.MetricsReport) {
	if s.db == nil {
		return
	}

	cpuJSON, _ := json.Marshal(report.GetCpu())
	memoryJSON, _ := json.Marshal(report.GetMemory())
	disksJSON, _ := json.Marshal(report.GetDisks())

	entry := AgentMetricsHistory{
		AgentID:    agentID,
		Timestamp:  time.Now(),
		CPUJSON:    cpuJSON,
		MemoryJSON: memoryJSON,
		DisksJSON:  disksJSON,
	}

	s.db.Create(&entry)
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
	if s.db != nil {
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
	var records []AgentMetricsHistory
	query := s.db.Where("agent_id = ? AND timestamp >= ?", agentID, since).
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
			json.Unmarshal(record.CPUJSON, cpu)
			report.Cpu = cpu
		}
		if len(record.MemoryJSON) > 0 {
			mem := &opsv1.MemoryMetrics{}
			json.Unmarshal(record.MemoryJSON, mem)
			report.Memory = mem
		}
		if len(record.DisksJSON) > 0 {
			disks := []*opsv1.DiskMetrics{}
			json.Unmarshal(record.DisksJSON, &disks)
			report.Disks = disks
		}
		result[i] = MetricsEntry{
			AgentID:  agentID,
			Report:   report,
			Received: record.Timestamp,
		}
	}

	return result
}

// Prune removes old entries from database.
func (s *MetricsStore) Prune(olderThan time.Duration) {
	if olderThan <= 0 {
		return
	}

	// Prune memory
	s.mu.Lock()
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
	s.mu.Unlock()

	// Prune database
	if s.db != nil {
		s.db.Where("timestamp < ?", cutoff).Delete(&AgentMetricsHistory{})
	}
}

// StartCleanupRoutine starts a background routine to clean up old metrics.
// If interval is 0, uses the configured CleanupInterval.
func (s *MetricsStore) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = s.config.CleanupInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Prune(s.config.Retention)
			}
		}
	}()
}

// SystemInfoCache stores system info for agents.
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
}
