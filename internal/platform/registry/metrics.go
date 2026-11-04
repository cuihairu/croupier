package registry

import (
	"context"
	"encoding/json"
	"github.com/cuihairu/croupier/internal/common/dbtype"
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
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
	ID        uint      `gorm:"primaryKey"`
	AgentID   string    `gorm:"index:idx_agent_time,not null"`
	Timestamp time.Time `gorm:"index:idx_agent_time,not null"`
	CPUJSON   dbtype.JSON

	MemoryJSON dbtype.JSON

	DisksJSON dbtype.JSON

	// ProbeJSON 探测统计快照（probe_stats，评审简档 §13.1 统计面；
	// 迁移 0044 加列）。空 = 该份上报无探测统计（如 capture/devops agent）。
	ProbeJSON dbtype.JSON

	// NetworksJSON 网络收发速率快照（network_rate，OPS-010；迁移 0051 加列）。
	// 空 = 该份上报无网络指标或未生成（老版本 agent / 关闭采集）。
	NetworksJSON dbtype.JSON

	// ProcessesJSON 进程清单快照（processes，OPS-010；迁移 0051 加列）。
	// 空 = 无进程面数据（关闭采集、不可枚举）。
	ProcessesJSON dbtype.JSON

	// PortsJSON 监听端口清单快照（ports，OPS-010；迁移 0051 加列）。
	// 空 = 无端口面数据。
	PortsJSON dbtype.JSON
}

// AgentMetricsHourly is the 90-day hourly rollup row for an agent
// (OPS-010 服务端聚合：原始 7 天 + 聚合 90 天）。One row per (agent, hour)
// carries the window-averaged resource metrics; the raw agent_metrics_history
// rows are pruned at 7 days while this table survives to 90.
type AgentMetricsHourly struct {
	ID        uint      `gorm:"primaryKey"`
	AgentID   string    `gorm:"index:idx_agent_hour,not null"`
	HourStart time.Time `gorm:"index:idx_agent_hour,not null"`
	// CPUUsageAvg 窗口平均 CPU 使用率（0-100）。
	CPUUsageAvg float64
	// MemUsageAvg 窗口平均内存使用率（0-100）。
	MemUsageAvg float64
	// DisksJSON 各挂载点平均使用率（[ {mount_point, usage_percent} ]）。
	DisksJSON dbtype.JSON
	// NetRatesJSON 各接口平均收发速率（[ {interface, bytes_sent_rate,
	// bytes_recv_rate} ]）。
	NetRatesJSON dbtype.JSON
	// ProcessCountAvg 窗口平均进程数（top-N 面）。
	ProcessCountAvg float64
	// PortCountAvg 窗口平均监听端口数。
	PortCountAvg float64
	// SampleCount 折入本小时的原始上报条数。
	SampleCount int
}

func (AgentMetricsHistory) TableName() string {
	return "agent_metrics_history"
}

func (AgentMetricsHourly) TableName() string {
	return "agent_metrics_hourly"
}

// MetricsRetention 原始指标保留时长（OPS-010：原始 7 天）。
const MetricsRetention = 7 * 24 * time.Hour

// MetricsHourlyRetention 小时聚合保留时长（OPS-010：聚合 90 天）。
const MetricsHourlyRetention = 90 * 24 * time.Hour

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
		MaxMemoryEntries: 10, // 5 minutes @ 15s interval
		MaxTotalEntries:  2000,
		Retention:        MetricsRetention, // 7 days
		CleanupInterval:  1 * time.Hour,
	}
}

// supervisorEventRingSize 每个 agent 的 supervisor 事件内存环上限（与 agent
// 侧 supervisorEventLog 环深一致）。
const supervisorEventRingSize = 500

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
	// onSupervisorEvent 是新监管事件入库后回调（herald 告警出口等，仅
	// 去重后的新事件触发）。异步执行且有 recover 防护，回调异常不影响
	// 存储链路。
	onSupervisorEvent func(ctx context.Context, agentID string, ev *opsv1.SupervisorEvent)
	// supervisor 事件环（agent 经 metrics 上报捎带，server 内存缓存）。
	// lastSeq/lastTs 用于去重：seq 不新且 ts 不新视为重传丢弃；seq 回退但
	// ts 更新说明 agent 重启后游标归零，按新事件重新入库。
	supEvents  map[string][]*opsv1.SupervisorEvent
	supLastSeq map[string]int64
	supLastTs  map[string]int64
}

// SetOnReport sets the post-report callback (e.g. alert rule evaluation).
func (s *MetricsStore) SetOnReport(fn func(ctx context.Context, agentID string, report *opsv1.MetricsReport)) {
	s.mu.Lock()
	s.onReport = fn
	s.mu.Unlock()
}

// SetOnSupervisorEvent sets the post-ingest callback fired for each NEW
// supervisor event (重传去重后；herald 告警出口消费).
func (s *MetricsStore) SetOnSupervisorEvent(fn func(ctx context.Context, agentID string, ev *opsv1.SupervisorEvent)) {
	s.mu.Lock()
	s.onSupervisorEvent = fn
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
		config.Retention = MetricsRetention
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
	newSupEvents := s.ingestSupervisorEventsLocked(agentID, report.GetSupervisorEvents())
	onReport := s.onReport
	onSupEvent := s.onSupervisorEvent
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

	// 新监管事件回调（异步逐事件，回调失败不影响存储与上报链路）。
	for _, ev := range newSupEvents {
		if onSupEvent == nil {
			break
		}
		go func(ev *opsv1.SupervisorEvent) {
			defer func() {
				_ = recover()
			}()
			evCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			onSupEvent(evCtx, agentID, ev)
		}(ev)
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

	// 探测统计只有配探针面的 agent 才有；空切片不落列（NULL 而非 "[]"），
	// 读侧按列非空重建。
	var probeJSON dbtype.JSON
	if probes := report.GetProbeStats(); len(probes) > 0 {
		if raw, err := json.Marshal(probes); err == nil {
			probeJSON = dbtype.JSON(raw)
		}
	}
	// OPS-010 三面（网络速率/进程/端口）同规则：有数据才落列，老版本
	// agent 或关闭采集时保持 NULL，读侧按列非空重建。
	var networksJSON, processesJSON, portsJSON dbtype.JSON
	if nets := report.GetNetworks(); len(nets) > 0 {
		if raw, err := json.Marshal(nets); err == nil {
			networksJSON = dbtype.JSON(raw)
		}
	}
	if procs := report.GetProcesses(); len(procs) > 0 {
		if raw, err := json.Marshal(procs); err == nil {
			processesJSON = dbtype.JSON(raw)
		}
	}
	if ports := report.GetPorts(); len(ports) > 0 {
		if raw, err := json.Marshal(ports); err == nil {
			portsJSON = dbtype.JSON(raw)
		}
	}

	entry := AgentMetricsHistory{
		AgentID:       agentID,
		Timestamp:     time.Now(),
		CPUJSON:       cpuJSON,
		MemoryJSON:    memoryJSON,
		DisksJSON:     disksJSON,
		ProbeJSON:     probeJSON,
		NetworksJSON:  networksJSON,
		ProcessesJSON: processesJSON,
		PortsJSON:     portsJSON,
	}

	s.db.Create(&entry)
}

// ingestSupervisorEventsLocked merges piggybacked supervisor events into the
// per-agent ring and returns the newly ingested events (重传去重后) for the
// post-ingest callback. Caller must hold s.mu.
func (s *MetricsStore) ingestSupervisorEventsLocked(agentID string, evs []*opsv1.SupervisorEvent) []*opsv1.SupervisorEvent {
	if len(evs) == 0 {
		return nil
	}
	if s.supEvents == nil {
		s.supEvents = make(map[string][]*opsv1.SupervisorEvent)
		s.supLastSeq = make(map[string]int64)
		s.supLastTs = make(map[string]int64)
	}
	lastSeq, lastTs := s.supLastSeq[agentID], s.supLastTs[agentID]
	ring := s.supEvents[agentID]
	var fresh []*opsv1.SupervisorEvent
	for _, ev := range evs {
		if ev == nil {
			continue
		}
		// 重传去重：seq 与 ts 都不比已入库基线新才丢弃；seq 回退但 ts 更新
		// 说明 agent 重启后游标归零，按新事件重新入库。
		if ev.Seq <= lastSeq && ev.TsUnix <= lastTs {
			continue
		}
		ring = append(ring, ev)
		fresh = append(fresh, ev)
		if len(ring) > supervisorEventRingSize {
			ring = ring[len(ring)-supervisorEventRingSize:]
		}
		lastSeq, lastTs = ev.Seq, ev.TsUnix
	}
	s.supEvents[agentID] = ring
	s.supLastSeq[agentID] = lastSeq
	s.supLastTs[agentID] = lastTs
	return fresh
}

// GetSupervisorEvents returns buffered supervisor events for an agent, oldest
// first. sinceSeq > 0 keeps only events with a higher seq (incremental pull);
// limit > 0 caps the result to the newest events.
func (s *MetricsStore) GetSupervisorEvents(agentID string, sinceSeq int64, limit int) []*opsv1.SupervisorEvent {
	if agentID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*opsv1.SupervisorEvent
	for _, ev := range s.supEvents[agentID] {
		if ev == nil || (sinceSeq > 0 && ev.Seq <= sinceSeq) {
			continue
		}
		out = append(out, ev)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// SupervisorLatestSeq returns the newest buffered event seq for an agent
// (0 when none), so panels can bootstrap their incremental cursor.
func (s *MetricsStore) SupervisorLatestSeq(agentID string) int64 {
	if agentID == "" {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.supLastSeq[agentID]
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

// StartCleanupRoutine starts a background routine to clean up old metrics
// and roll up the previous hour into the 90-day aggregate (OPS-010). If
// interval is 0, uses the configured CleanupInterval.
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
				if s.db != nil {
					s.RollupHourly(ctx)
					s.PruneHourly(MetricsHourlyRetention)
				}
				s.Prune(s.config.Retention)
			}
		}
	}()
}

// RollupHourly aggregates the previous completed hour's raw rows into
// agent_metrics_hourly (OPS-010 聚合 90 天）。Idempotent per (agent, hour):
// re-rolling the same hour overwrites the row. Raw rows older than
// MetricsRetention are left to Prune.
func (s *MetricsStore) RollupHourly(ctx context.Context) {
	if s.db == nil {
		return
	}
	now := time.Now()
	// Roll up the hour that just completed: [now-1h truncated to hour, +1h).
	hourStart := now.Add(-time.Hour).Truncate(time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	var rows []AgentMetricsHistory
	if err := s.db.WithContext(ctx).
		Where("timestamp >= ? AND timestamp < ?", hourStart, hourEnd).
		Find(&rows).Error; err != nil {
		return
	}

	byAgent := make(map[string][]AgentMetricsHistory)
	for _, r := range rows {
		if r.AgentID == "" {
			continue
		}
		byAgent[r.AgentID] = append(byAgent[r.AgentID], r)
	}

	for agentID, agentRows := range byAgent {
		if len(agentRows) == 0 {
			continue
		}
		agg := aggregateHourly(agentRows)
		if agg.SampleCount == 0 {
			continue
		}
		agg.AgentID = agentID
		agg.HourStart = hourStart
		// Upsert: one row per (agent, hour). Conflict on a compound unique
		// key would be ideal; use find-or-create to stay portable across
		// sqlite/pg/mysql (no dialect-specific ON CONFLICT).
		var existing AgentMetricsHourly
		err := s.db.WithContext(ctx).
			Where("agent_id = ? AND hour_start = ?", agentID, hourStart).
			First(&existing).Error
		if err != nil {
			// No row yet — create.
			if e := s.db.WithContext(ctx).Create(&agg).Error; e != nil {
				continue
			}
			continue
		}
		agg.ID = existing.ID
		s.db.WithContext(ctx).Save(&agg)
	}
}

// aggregateHourly averages one agent's raw rows into a single hourly row.
func aggregateHourly(rows []AgentMetricsHistory) AgentMetricsHourly {
	var cpuSum, memSum, procSum, portSum float64
	var cpuN, memN, procN, portN int
	diskByMount := make(map[string]*diskAccum)
	netByIface := make(map[string]*netRateAccum)

	for _, r := range rows {
		if len(r.CPUJSON) > 0 {
			var cpu opsv1.CpuMetrics
			if json.Unmarshal(r.CPUJSON, &cpu) == nil {
				cpuSum += cpu.GetUsagePercent()
				cpuN++
			}
		}
		if len(r.MemoryJSON) > 0 {
			var mem opsv1.MemoryMetrics
			if json.Unmarshal(r.MemoryJSON, &mem) == nil {
				memSum += mem.GetUsagePercent()
				memN++
			}
		}
		if len(r.DisksJSON) > 0 {
			var disks []*opsv1.DiskMetrics
			if json.Unmarshal(r.DisksJSON, &disks) == nil {
				for _, d := range disks {
					acc := diskByMount[d.GetMountPoint()]
					if acc == nil {
						acc = &diskAccum{}
						diskByMount[d.GetMountPoint()] = acc
					}
					acc.usage += d.GetUsagePercent()
					acc.n++
				}
			}
		}
		if len(r.NetworksJSON) > 0 {
			var nets []*opsv1.NetworkMetrics
			if json.Unmarshal(r.NetworksJSON, &nets) == nil {
				for _, n := range nets {
					acc := netByIface[n.GetInterface()]
					if acc == nil {
						acc = &netRateAccum{}
						netByIface[n.GetInterface()] = acc
					}
					acc.sent += n.GetBytesSentRate()
					acc.recv += n.GetBytesRecvRate()
					acc.n++
				}
			}
		}
		if len(r.ProcessesJSON) > 0 {
			var procs []*opsv1.ProcessMetrics
			if json.Unmarshal(r.ProcessesJSON, &procs) == nil {
				procSum += float64(len(procs))
				procN++
			}
		}
		if len(r.PortsJSON) > 0 {
			var ports []*opsv1.PortMetrics
			if json.Unmarshal(r.PortsJSON, &ports) == nil {
				portSum += float64(len(ports))
				portN++
			}
		}
	}

	agg := AgentMetricsHourly{SampleCount: len(rows)}
	if cpuN > 0 {
		agg.CPUUsageAvg = cpuSum / float64(cpuN)
	}
	if memN > 0 {
		agg.MemUsageAvg = memSum / float64(memN)
	}
	if procN > 0 {
		agg.ProcessCountAvg = procSum / float64(procN)
	}
	if portN > 0 {
		agg.PortCountAvg = portSum / float64(portN)
	}

	if len(diskByMount) > 0 {
		type diskAvg struct {
			MountPoint   string  `json:"mount_point"`
			UsagePercent float64 `json:"usage_percent"`
		}
		out := make([]diskAvg, 0, len(diskByMount))
		for mount, acc := range diskByMount {
			avg := 0.0
			if acc.n > 0 {
				avg = acc.usage / float64(acc.n)
			}
			out = append(out, diskAvg{MountPoint: mount, UsagePercent: round2(avg)})
		}
		if raw, err := json.Marshal(out); err == nil {
			agg.DisksJSON = dbtype.JSON(raw)
		}
	}
	if len(netByIface) > 0 {
		type netAvg struct {
			Interface     string  `json:"interface"`
			BytesSentRate float64 `json:"bytes_sent_rate"`
			BytesRecvRate float64 `json:"bytes_recv_rate"`
		}
		out := make([]netAvg, 0, len(netByIface))
		for iface, acc := range netByIface {
			out = append(out, netAvg{
				Interface:     iface,
				BytesSentRate: round2(acc.sent / float64(acc.n)),
				BytesRecvRate: round2(acc.recv / float64(acc.n)),
			})
		}
		if raw, err := json.Marshal(out); err == nil {
			agg.NetRatesJSON = dbtype.JSON(raw)
		}
	}

	return agg
}

// netRateAccum accumulates per-interface rate samples within an hour.
type netRateAccum struct {
	sent float64
	recv float64
	n    int
}

type diskAccum struct {
	usage float64
	n     int
}

// round2 rounds to two decimals (percent/rate display precision).
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// PruneHourly removes hourly aggregate rows older than the given duration
// (OPS-010 聚合 90 天）。
func (s *MetricsStore) PruneHourly(olderThan time.Duration) {
	if olderThan <= 0 || s.db == nil {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	s.db.Where("hour_start < ?", cutoff).Delete(&AgentMetricsHourly{})
}

// GetHourlyHistory returns hourly aggregate history for an agent, oldest
// first, newest ≤ limit. Rows are rebuilt into synthetic MetricsEntry
// reports (CPU/Memory averages + a Disks/Networks projection) so the panel
// can render 90-day curves through the same entry shape.
func (s *MetricsStore) GetHourlyHistory(agentID string, since time.Time, limit int) []MetricsEntry {
	if agentID == "" || s.db == nil {
		return nil
	}
	var rows []AgentMetricsHourly
	q := s.db.Where("agent_id = ?", agentID)
	if !since.IsZero() {
		q = q.Where("hour_start >= ?", since)
	}
	q = q.Order("hour_start DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	q.Find(&rows)

	// Reverse to oldest-first to match GetHistory ordering.
	result := make([]MetricsEntry, len(rows))
	for i := range rows {
		result[i] = metricsEntryFromHourly(rows[len(rows)-1-i])
	}
	return result
}

func metricsEntryFromHourly(row AgentMetricsHourly) MetricsEntry {
	report := &opsv1.MetricsReport{
		AgentId: row.AgentID,
		Cpu:     &opsv1.CpuMetrics{UsagePercent: row.CPUUsageAvg},
		Memory:  &opsv1.MemoryMetrics{UsagePercent: row.MemUsageAvg},
	}
	if len(row.DisksJSON) > 0 {
		var disks []struct {
			MountPoint   string  `json:"mount_point"`
			UsagePercent float64 `json:"usage_percent"`
		}
		if json.Unmarshal(row.DisksJSON, &disks) == nil {
			for _, d := range disks {
				report.Disks = append(report.Disks, &opsv1.DiskMetrics{
					MountPoint:   d.MountPoint,
					UsagePercent: d.UsagePercent,
				})
			}
		}
	}
	if len(row.NetRatesJSON) > 0 {
		var nets []struct {
			Interface     string  `json:"interface"`
			BytesSentRate float64 `json:"bytes_sent_rate"`
			BytesRecvRate float64 `json:"bytes_recv_rate"`
		}
		if json.Unmarshal(row.NetRatesJSON, &nets) == nil {
			for _, n := range nets {
				report.Networks = append(report.Networks, &opsv1.NetworkMetrics{
					Interface:     n.Interface,
					BytesSentRate: n.BytesSentRate,
					BytesRecvRate: n.BytesRecvRate,
				})
			}
		}
	}
	return MetricsEntry{
		AgentID:  row.AgentID,
		Report:   report,
		Received: row.HourStart,
	}
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

	// Return a copy to avoid mutation. proto.Clone 逐字段拷贝——手写列清单会随
	// proto 加字段静默漂移（agent_binary_sha256 落地时即踩中），Clone 根治。
	if cloned, ok := proto.Clone(entry.Info).(*opsv1.SystemInfo); ok && cloned != nil {
		return cloned, true
	}
	return entry.Info, true
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
	delete(s.supEvents, agentID)
	delete(s.supLastSeq, agentID)
	delete(s.supLastTs, agentID)
}
