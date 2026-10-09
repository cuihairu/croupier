// Package metrics gopsutil v4 系统指标采集器（矩阵拍板库选型，上收自
// sidecar-agent 的 ops_metrics.go，agent-core K3）。可独立运行：capture/
// devops 等 core 系 agent 无函数通道也能采集并上报；受管进程快照经 Sampler
// 注入（sidecar 的 OpsServer 实现），不注入即采集纯系统指标。
package metrics

import (
	"context"
	"runtime"
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Sampler 供给受管进程快照与事件增量，折入每次指标上报（supervisor 事件
// 捎带增量游标，server 端按 seq 去重后入内存环）。
type Sampler interface {
	SampleSupervisedProcesses() []*opsv1.SupervisedProcessSnapshot
	SupervisorEventsSince(lastSeq int64) []*opsv1.SupervisorEvent
}

// Collector collects system metrics.
type Collector struct {
	agentID string
	// sampler 供给受管进程快照；仅在 Collect 前的装配阶段赋值，Collect 读取
	// 不加锁（指针赋值原子性由调用方装配时序保证，与 sidecar 上收前一致）。
	sampler Sampler
	// sampler 事件增量游标与锁：Collect 由上报循环单 goroutine 调用，但
	// 即时上报可能并发触发一次，游标读写加锁。
	supEventSeq   int64
	supEventSeqMu sync.Mutex
}

// NewCollector creates a new metrics collector.
func NewCollector(agentID string) *Collector {
	return &Collector{agentID: agentID}
}

// WithSampler attaches a supervised-process sampler. Call before the
// reporting loop starts.
func (c *Collector) WithSampler(s Sampler) *Collector {
	if c != nil {
		c.sampler = s
	}
	return c
}

func (c *Collector) lastSamplerSeq() int64 {
	if c == nil {
		return 0
	}
	c.supEventSeqMu.Lock()
	defer c.supEventSeqMu.Unlock()
	return c.supEventSeq
}

func (c *Collector) setLastSamplerSeq(seq int64) {
	if c == nil {
		return
	}
	c.supEventSeqMu.Lock()
	c.supEventSeq = seq
	c.supEventSeqMu.Unlock()
}

// Collect gathers current system metrics.
func (c *Collector) Collect(ctx context.Context) *opsv1.MetricsReport {
	report := &opsv1.MetricsReport{
		AgentId:   c.agentID,
		Timestamp: timestamppb.Now(),
		Cpu:       c.collectCPU(ctx),
		Memory:    c.collectMemory(),
		Disks:     c.collectDisks(),
		Networks:  c.collectNetworks(),
		Custom:    make(map[string]float64),
	}
	if c.sampler != nil {
		report.SupervisedProcesses = c.sampler.SampleSupervisedProcesses()
		if evs := c.sampler.SupervisorEventsSince(c.lastSamplerSeq()); len(evs) > 0 {
			report.SupervisorEvents = evs
			c.setLastSamplerSeq(evs[len(evs)-1].Seq)
		}
	}

	// Add Go runtime metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	report.Custom["go_alloc_bytes"] = float64(m.Alloc)
	report.Custom["go_sys_bytes"] = float64(m.Sys)
	report.Custom["go_num_goroutines"] = float64(runtime.NumGoroutine())

	return report
}

// StartReporting starts periodic metrics reporting.
func (c *Collector) StartReporting(ctx context.Context, interval time.Duration, handler func(*opsv1.MetricsReport)) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Report immediately on start
	report := c.Collect(ctx)
	handler(report)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report := c.Collect(ctx)
			handler(report)
		}
	}
}
