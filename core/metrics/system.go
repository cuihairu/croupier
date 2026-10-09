package metrics

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func (c *Collector) collectCPU(ctx context.Context) *opsv1.CpuMetrics {
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

func (c *Collector) collectMemory() *opsv1.MemoryMetrics {
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
