package ops

import (
	"context"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func agentMetricsHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *AgentMetricsHistoryRequest) (*AgentMetricsHistoryResponse, error) {
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, req.AgentID); err != nil {
		return nil, err
	}
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

	// OPS-010 聚合面（90 天小时聚合）：window=hourly 走 agent_metrics_hourly，
	// 否则原始 7 天面 agent_metrics_history。
	var entries []reg.MetricsEntry
	if req.Window == "hourly" {
		entries = svcCtx.MetricsStore.GetHourlyHistory(req.AgentID, since, limit)
	} else {
		entries = svcCtx.MetricsStore.GetHistory(req.AgentID, since, limit)
	}

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
			if len(entry.Report.GetNetworks()) > 0 {
				historyEntry.Networks = convertNetworkMetrics(entry.Report.GetNetworks())
			}
			if len(entry.Report.GetProcesses()) > 0 {
				historyEntry.Processes = convertProcessMetrics(entry.Report.GetProcesses())
			}
			if len(entry.Report.GetPorts()) > 0 {
				historyEntry.Ports = convertPortMetrics(entry.Report.GetPorts())
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

func convertNetworkMetrics(networks []*opsv1.NetworkMetrics) []NetworkHistoryMetrics {
	result := make([]NetworkHistoryMetrics, len(networks))
	for i, n := range networks {
		result[i] = NetworkHistoryMetrics{
			Interface:     n.GetInterface(),
			BytesSent:     n.GetBytesSent(),
			BytesRecv:     n.GetBytesRecv(),
			BytesSentRate: n.GetBytesSentRate(),
			BytesRecvRate: n.GetBytesRecvRate(),
		}
	}
	return result
}

func convertProcessMetrics(processes []*opsv1.ProcessMetrics) []ProcessHistoryMetrics {
	result := make([]ProcessHistoryMetrics, len(processes))
	for i, p := range processes {
		entry := ProcessHistoryMetrics{
			Pid:            p.GetPid(),
			Name:           p.GetName(),
			Status:         p.GetStatus(),
			CpuPercent:     p.GetCpuPercent(),
			MemoryBytes:    p.GetMemoryBytes(),
			Threads:        p.GetThreads(),
			ListeningPorts: p.GetListeningPorts(),
		}
		if st := p.GetStartTime(); st != nil {
			entry.StartTime = time.Unix(st.Seconds, int64(st.Nanos)).UTC().Format(time.RFC3339)
		}
		result[i] = entry
	}
	return result
}

func convertPortMetrics(ports []*opsv1.PortMetrics) []PortHistoryMetrics {
	result := make([]PortHistoryMetrics, len(ports))
	for i, p := range ports {
		result[i] = PortHistoryMetrics{
			Port:        p.GetPort(),
			Protocol:    p.GetProtocol(),
			Pid:         p.GetPid(),
			ProcessName: p.GetProcessName(),
			State:       p.GetState(),
		}
	}
	return result
}
