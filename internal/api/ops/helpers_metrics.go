package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

func opsMetrics(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMetricsRequest) (*OpsMetricsResponse, error) {
	if svcCtx == nil {
		return &OpsMetricsResponse{
			Metrics: []OpsMetricsData{},
		}, nil
	}

	// Aggregate latest metrics from MetricsStore across registered agents
	if svcCtx.MetricsStore == nil {
		return &OpsMetricsResponse{
			Metrics: []OpsMetricsData{},
		}, nil
	}

	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsMetricsResponse{
			Metrics: []OpsMetricsData{},
		}, nil
	}

	store.Mu().RLock()
	agents := make([]string, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if req != nil {
			if req.GameId != "" && sess.GameID != req.GameId {
				continue
			}
			if req.Env != "" && sess.Env != req.Env {
				continue
			}
		}
		agents = append(agents, sess.AgentID)
	}
	store.Mu().RUnlock()

	result := make([]OpsMetricsData, 0, len(agents))
	for _, agentID := range agents {
		entry, ok := svcCtx.MetricsStore.GetLatest(agentID)
		if !ok || entry == nil || entry.Report == nil {
			continue
		}
		report := entry.Report
		data := OpsMetricsData{
			AgentID:   agentID,
			Timestamp: utils.FormatTimestamp(entry.Received),
			CPU: OpsCpuMetrics{
				UsagePercent: report.Cpu.UsagePercent,
				Cores:        report.Cpu.Cores,
				PerCore:      report.Cpu.PerCore,
				Load1M:       report.Cpu.Load_1M,
				Load5M:       report.Cpu.Load_5M,
				Load15M:      report.Cpu.Load_15M,
			},
			Memory: OpsMemoryMetrics{
				TotalBytes:     report.Memory.TotalBytes,
				UsedBytes:      report.Memory.UsedBytes,
				AvailableBytes: report.Memory.AvailableBytes,
				UsagePercent:   report.Memory.UsagePercent,
				SwapTotal:      report.Memory.SwapTotal,
				SwapUsed:       report.Memory.SwapUsed,
			},
		}
		for _, d := range report.Disks {
			data.Disks = append(data.Disks, OpsDiskMetrics{
				MountPoint:     d.MountPoint,
				Device:         d.Device,
				FsType:         d.FsType,
				TotalBytes:     d.TotalBytes,
				UsedBytes:      d.UsedBytes,
				AvailableBytes: d.AvailableBytes,
				UsagePercent:   d.UsagePercent,
			})
		}
		for _, n := range report.Networks {
			data.Networks = append(data.Networks, OpsNetworkMetrics{
				Interface:   n.Interface,
				BytesSent:   n.BytesSent,
				BytesRecv:   n.BytesRecv,
				PacketsSent: n.PacketsSent,
				PacketsRecv: n.PacketsRecv,
			})
		}
		result = append(result, data)
	}

	return &OpsMetricsResponse{
		Metrics: result,
	}, nil
}
