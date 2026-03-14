
package ops

import (
	"context"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsAgentMetricsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 指标
func NewOpsAgentMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentMetricsLogic {
	return &OpsAgentMetricsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentMetricsLogic) OpsAgentMetrics(req *OpsAgentMetricsRequest) (*OpsAgentMetricsResponse, error) {
	metricsStore := l.svcCtx.MetricsStore
	if metricsStore == nil {
		return &OpsAgentMetricsResponse{
			Code:    0,
			Message: "OK",
			Data:    []OpsMetricsData{},
		}, nil
	}

	// Parse since time
	var since time.Time
	if req.Since != "" {
		if t, err := time.Parse(time.RFC3339, req.Since); err == nil {
			since = t
		}
	}

	// Set limit
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	var entries []reg.MetricsEntry
	if req.AgentID != "" {
		// Get metrics for specific agent
		entries = metricsStore.GetAgentMetrics(req.AgentID, limit)
	} else {
		// Get all metrics
		entries = metricsStore.GetAllMetrics(since, limit)
	}

	// Convert to API format
	result := make([]OpsMetricsData, 0, len(entries))
	for _, entry := range entries {
		// Filter by since time
		if !since.IsZero() && entry.Received.Before(since) {
			continue
		}

		report := entry.Report
		if report == nil {
			continue
		}

		data := OpsMetricsData{
			AgentID:   entry.AgentID,
			Timestamp: entry.Received.Format(time.RFC3339),
			CPU: OpsCpuMetrics{
				Cores:        report.Cpu.Cores,
				UsagePercent: report.Cpu.UsagePercent,
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

		// Convert per-core usage
		if len(report.Cpu.PerCore) > 0 {
			data.CPU.PerCore = report.Cpu.PerCore
		}

		// Convert disk metrics
		for _, disk := range report.Disks {
			data.Disks = append(data.Disks, OpsDiskMetrics{
				MountPoint:     disk.MountPoint,
				Device:         disk.Device,
				FsType:         disk.FsType,
				TotalBytes:     disk.TotalBytes,
				UsedBytes:      disk.UsedBytes,
				AvailableBytes: disk.AvailableBytes,
				UsagePercent:   disk.UsagePercent,
			})
		}

		// Convert network metrics
		for _, net := range report.Networks {
			data.Networks = append(data.Networks, OpsNetworkMetrics{
				Interface:   net.Interface,
				BytesSent:   net.BytesSent,
				BytesRecv:   net.BytesRecv,
				PacketsSent: net.PacketsSent,
				PacketsRecv: net.PacketsRecv,
			})
		}

		result = append(result, data)
	}

	return &OpsAgentMetricsResponse{
		Code:    0,
		Message: "OK",
		Data:    result,
	}, nil
}
