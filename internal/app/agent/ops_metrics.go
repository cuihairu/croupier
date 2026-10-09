package agent

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MetricsCollector collects system metrics.
type MetricsCollector struct {
	agentID string
	// supervisor 供给托管进程快照（OpsServer 实现）；仅在上报启动前的
	// 装配阶段赋值，Collect 读取不加锁。
	supervisor SupervisorSampler
	// supervisor 事件增量游标与锁：Collect 由上报循环单 goroutine 调用，
	// 但 ReportMetricsOnce 可能并发触发一次即时上报，游标读写加锁。
	supEventSeq   int64
	supEventSeqMu sync.Mutex
}

// SupervisorSampler supplies snapshots of supervisor-managed processes for
// inclusion in metrics reports. The agent's OpsServer implements it.
type SupervisorSampler interface {
	SampleSupervisedProcesses() []*opsv1.SupervisedProcessSnapshot
	SupervisorEventsSince(lastSeq int64) []*opsv1.SupervisorEvent
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(agentID string) *MetricsCollector {
	return &MetricsCollector{agentID: agentID}
}

// WithSupervisor attaches a supervised-process sampler. Call before the
// reporting loop starts.
func (c *MetricsCollector) WithSupervisor(s SupervisorSampler) *MetricsCollector {
	if c != nil {
		c.supervisor = s
	}
	return c
}

func (c *MetricsCollector) lastSupervisorSeq() int64 {
	if c == nil {
		return 0
	}
	c.supEventSeqMu.Lock()
	defer c.supEventSeqMu.Unlock()
	return c.supEventSeq
}

func (c *MetricsCollector) setLastSupervisorSeq(seq int64) {
	if c == nil {
		return
	}
	c.supEventSeqMu.Lock()
	c.supEventSeq = seq
	c.supEventSeqMu.Unlock()
}

// Collect gathers current system metrics.
func (c *MetricsCollector) Collect(ctx context.Context) *opsv1.MetricsReport {
	report := &opsv1.MetricsReport{
		AgentId:   c.agentID,
		Timestamp: timestamppb.Now(),
		Cpu:       c.collectCPU(ctx),
		Memory:    c.collectMemory(),
		Disks:     c.collectDisks(),
		Networks:  c.collectNetworks(),
		Custom:    make(map[string]float64),
	}
	if c.supervisor != nil {
		report.SupervisedProcesses = c.supervisor.SampleSupervisedProcesses()
		// 事件捎带增量：只带自上次上报后新产生的事件（seq 游标），server
		// 端按 seq 去重后入内存环。
		if evs := c.supervisor.SupervisorEventsSince(c.lastSupervisorSeq()); len(evs) > 0 {
			report.SupervisorEvents = evs
			c.setLastSupervisorSeq(evs[len(evs)-1].Seq)
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

func (c *MetricsCollector) collectCPU(ctx context.Context) *opsv1.CpuMetrics {
	metrics := &opsv1.CpuMetrics{
		Cores: int32(runtime.NumCPU()),
	}

	// Overall CPU usage (with 1 second interval)
	percentages, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err == nil && len(percentages) > 0 {
		metrics.UsagePercent = percentages[0]
	}

	// Per-core CPU usage
	perCore, err := cpu.PercentWithContext(ctx, 0, true)
	if err == nil {
		metrics.PerCore = perCore
	}

	// Load average (Unix only, returns 0 on Windows)
	loadAvg, err := load.AvgWithContext(ctx)
	if err == nil {
		metrics.Load_1M = loadAvg.Load1
		metrics.Load_5M = loadAvg.Load5
		metrics.Load_15M = loadAvg.Load15
	}

	return metrics
}

func (c *MetricsCollector) collectMemory() *opsv1.MemoryMetrics {
	metrics := &opsv1.MemoryMetrics{}

	vmem, err := mem.VirtualMemory()
	if err == nil {
		metrics.TotalBytes = vmem.Total
		metrics.UsedBytes = vmem.Used
		metrics.AvailableBytes = vmem.Available
		metrics.UsagePercent = vmem.UsedPercent
	}

	swap, err := mem.SwapMemory()
	if err == nil {
		metrics.SwapTotal = swap.Total
		metrics.SwapUsed = swap.Used
	}

	return metrics
}

func (c *MetricsCollector) collectDisks() []*opsv1.DiskMetrics {
	var result []*opsv1.DiskMetrics

	partitions, err := disk.Partitions(false) // false = only physical disks
	if err != nil {
		return result
	}

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		result = append(result, &opsv1.DiskMetrics{
			MountPoint:     p.Mountpoint,
			Device:         p.Device,
			FsType:         p.Fstype,
			TotalBytes:     usage.Total,
			UsedBytes:      usage.Used,
			AvailableBytes: usage.Free,
			UsagePercent:   usage.UsedPercent,
			InodeTotal:     usage.InodesTotal,
			InodeUsed:      usage.InodesUsed,
		})
	}

	return result
}

func (c *MetricsCollector) collectNetworks() []*opsv1.NetworkMetrics {
	var result []*opsv1.NetworkMetrics

	counters, err := net.IOCounters(true) // true = per interface
	if err != nil {
		return result
	}

	for _, counter := range counters {
		// Skip loopback interface
		if counter.Name == "lo" || counter.Name == "lo0" {
			continue
		}

		result = append(result, &opsv1.NetworkMetrics{
			Interface:   counter.Name,
			BytesSent:   counter.BytesSent,
			BytesRecv:   counter.BytesRecv,
			PacketsSent: counter.PacketsSent,
			PacketsRecv: counter.PacketsRecv,
			ErrorsIn:    counter.Errin,
			ErrorsOut:   counter.Errout,
		})
	}

	return result
}

// StartReporting starts periodic metrics reporting.
func (c *MetricsCollector) StartReporting(ctx context.Context, interval time.Duration, handler func(*opsv1.MetricsReport)) {
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

// GetSystemInfo collects detailed system information.
func GetSystemInfo(agentID, agentVersion string, opsConfig *OpsConfig) *opsv1.SystemInfo {
	info := &opsv1.SystemInfo{
		Arch:         runtime.GOARCH,
		CpuCores:     int32(runtime.NumCPU()),
		AgentVersion: agentVersion,
	}

	// Hostname
	if hostInfo, err := host.Info(); err == nil {
		info.Hostname = hostInfo.Hostname
		info.Os = hostInfo.OS
		info.OsVersion = hostInfo.PlatformVersion
		info.KernelVersion = hostInfo.KernelVersion
		info.BootTime = timestamppb.New(time.Unix(int64(hostInfo.BootTime), 0))
	}

	// Total memory
	if vmem, err := mem.VirtualMemory(); err == nil {
		info.TotalMemory = vmem.Total
	}

	// Ops status
	if opsConfig != nil {
		managedNames := make([]string, 0, len(opsConfig.ManagedProcesses))
		for name := range opsConfig.ManagedProcesses {
			managedNames = append(managedNames, name)
		}
		info.OpsStatus = &opsv1.OpsStatus{
			Enabled:          opsConfig.Enabled,
			AllowRestart:     opsConfig.AllowRestart,
			AllowExec:        opsConfig.AllowExec,
			ManagedProcesses: managedNames,
		}
	} else {
		info.OpsStatus = &opsv1.OpsStatus{
			Enabled: false,
		}
	}

	return info
}
