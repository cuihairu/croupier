package registry

import (
	"context"

	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/common/dbtype"

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
	AgentID   string    `gorm:"size:64;uniqueIndex:uidx_agent_hour,not null"`
	HourStart time.Time `gorm:"uniqueIndex:uidx_agent_hour,not null"`
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
	db      atomic.Pointer[gorm.DB]
	mu      sync.RWMutex
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
	s.db.Store(db)
}
