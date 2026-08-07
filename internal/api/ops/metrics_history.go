package ops

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func agentMetricsHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *AgentMetricsHistoryRequest) (*AgentMetricsHistoryResponse, error) {
	if svcCtx == nil || svcCtx.MetricsStore == nil {
		return &AgentMetricsHistoryResponse{AgentID: req.AgentID}, nil
	}

	// Parse since time, default to 1 hour ago
	since := time.Now().Add(-1 * time.Hour)
	if req.Since != "" {
		if t, err := time.Parse(time.RFC3339, req.Since); err == nil {
			since = t
		}
	}

	// Default limit
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	entries := svcCtx.MetricsStore.GetHistory(req.AgentID, since, limit)

	resp := &AgentMetricsHistoryResponse{
		AgentID: req.AgentID,
		Entries: make([]MetricsHistoryEntry, len(entries)),
	}

	for i, entry := range entries {
		historyEntry := MetricsHistoryEntry{
			Timestamp: entry.Received.Format(time.RFC3339),
		}

		if entry.Report != nil {
			if entry.Report.GetCpu() != nil {
				historyEntry.CPU = convertCpuMetrics(entry.Report.GetCpu())
			}
			if entry.Report.GetMemory() != nil {
				historyEntry.Memory = convertMemoryMetrics(entry.Report.GetMemory())
			}
			if len(entry.Report.GetDisks()) > 0 {
				historyEntry.Disks = convertDiskMetrics(entry.Report.GetDisks())
			}
		}

		resp.Entries[i] = historyEntry
	}

	return resp, nil
}

func convertCpuMetrics(cpu *opsv1.CpuMetrics) *CpuMetrics {
	if cpu == nil {
		return nil
	}
	return &CpuMetrics{
		UsagePercent: cpu.GetUsagePercent(),
		Cores:        cpu.GetCores(),
		PerCore:      cpu.GetPerCore(),
		Load1M:       cpu.GetLoad_1M(),
		Load5M:       cpu.GetLoad_5M(),
		Load15M:      cpu.GetLoad_15M(),
	}
}

func convertMemoryMetrics(mem *opsv1.MemoryMetrics) *MemoryMetrics {
	if mem == nil {
		return nil
	}
	return &MemoryMetrics{
		TotalBytes:     mem.GetTotalBytes(),
		UsedBytes:      mem.GetUsedBytes(),
		AvailableBytes: mem.GetAvailableBytes(),
		UsagePercent:   mem.GetUsagePercent(),
		SwapTotal:      mem.GetSwapTotal(),
		SwapUsed:       mem.GetSwapUsed(),
	}
}

func convertDiskMetrics(disks []*opsv1.DiskMetrics) []DiskMetrics {
	result := make([]DiskMetrics, len(disks))
	for i, disk := range disks {
		result[i] = DiskMetrics{
			MountPoint:     disk.GetMountPoint(),
			Device:         disk.GetDevice(),
			FsType:         disk.GetFsType(),
			TotalBytes:     disk.GetTotalBytes(),
			UsedBytes:      disk.GetUsedBytes(),
			AvailableBytes: disk.GetAvailableBytes(),
			UsagePercent:   disk.GetUsagePercent(),
		}
	}
	return result
}
