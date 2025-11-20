package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type EdgeMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEdgeMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EdgeMetricsLogic {
	return &EdgeMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EdgeMetricsLogic) EdgeMetrics(req *types.EdgeMetricsRequest) (resp *types.EdgeMetricsResponse, err error) {
	logx.Infof("Edge metrics request: start=%s, end=%s, type=%s", req.Start, req.End, req.Type)

	// Collect metrics based on type
	metrics := make(map[string]interface{})

	switch req.Type {
	case "system":
		metrics = l.collectSystemMetrics()
	case "tunnels":
		metrics = l.collectTunnelMetrics()
	case "agents":
		metrics = l.collectAgentMetrics()
	default:
		// Collect all metrics if no specific type
		metrics = l.collectAllMetrics()
	}

	return &types.EdgeMetricsResponse{
		Metrics: metrics,
	}, nil
}

func (l *EdgeMetricsLogic) collectSystemMetrics() map[string]interface{} {
	return map[string]interface{}{
		"timestamp":     time.Now().Unix(),
		"uptime":        time.Since(time.Now()).Seconds(), // TODO: Calculate real uptime
		"memory_total":  l.getMemoryStats().Sys,
		"memory_used":   l.getMemoryStats().Alloc,
		"goroutines":    l.getGoroutineCount(),
		"gc_stats":      l.getGCStats(),
		"load_average":  l.getLoadAverage(),
	}
}

func (l *EdgeMetricsLogic) collectTunnelMetrics() map[string]interface{} {
	tunnels := l.svcCtx.TunnelMgr.ListTunnels("", "", 1, 10000)

	var activeCount, closedCount, totalConnections int64
	var totalBytesIn, totalBytesOut int64
	protocolCounts := make(map[string]int64)

	for _, tunnel := range tunnels {
		switch tunnel.Status {
		case "active":
			activeCount++
		case "closed":
			closedCount++
		}

		totalConnections += tunnel.Connections
		totalBytesIn += tunnel.BytesIn
		totalBytesOut += tunnel.BytesOut
		protocolCounts[tunnel.Protocol]++
	}

	return map[string]interface{}{
		"timestamp":           time.Now().Unix(),
		"total_tunnels":       int64(len(tunnels)),
		"active_tunnels":      activeCount,
		"closed_tunnels":      closedCount,
		"total_connections":   totalConnections,
		"total_bytes_in":      totalBytesIn,
		"total_bytes_out":     totalBytesOut,
		"protocol_counts":     protocolCounts,
	}
}

func (l *EdgeMetricsLogic) collectAgentMetrics() map[string]interface{} {
	// In a real implementation, this would track connected agents
	return map[string]interface{}{
		"timestamp":          time.Now().Unix(),
		"connected_agents":   int64(0), // Placeholder
		"disconnected_agents": int64(0), // Placeholder
		"agent_regions":      map[string]int64{}, // Placeholder
	}
}

func (l *EdgeMetricsLogic) collectAllMetrics() map[string]interface{} {
	return map[string]interface{}{
		"system":  l.collectSystemMetrics(),
		"tunnels": l.collectTunnelMetrics(),
		"agents":  l.collectAgentMetrics(),
	}
}

func (l *EdgeMetricsLogic) getMemoryStats() interface{} {
	// Implementation would use runtime.MemStats
	return struct {
		Sys   uint64 `json:"sys"`
		Alloc uint64 `json:"alloc"`
	}{
		Sys:   1024 * 1024 * 1024, // 1GB placeholder
		Alloc: 512 * 1024 * 1024, // 512MB placeholder
	}
}

func (l *EdgeMetricsLogic) getGoroutineCount() int {
	// Implementation would use runtime.NumGoroutine()
	return 50 // placeholder
}

func (l *EdgeMetricsLogic) getGCStats() interface{} {
	return struct {
		NumGC        uint32 `json:"num_gc"`
		TotalPause   uint64 `json:"total_pause_ns"`
		GCCPUFraction float64 `json:"gc_cpu_fraction"`
	}{
		NumGC:        10,
		TotalPause:   1000000,
		GCCPUFraction: 0.01,
	}
}

func (l *EdgeMetricsLogic) getLoadAverage() interface{} {
	return struct {
		Load1  float64 `json:"load1"`
		Load5  float64 `json:"load5"`
		Load15 float64 `json:"load15"`
	}{
		Load1:  0.5,
		Load5:  0.3,
		Load15: 0.2,
	}
}