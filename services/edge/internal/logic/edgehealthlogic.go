package logic

import (
	"context"
	"runtime"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type EdgeHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEdgeHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EdgeHealthLogic {
	return &EdgeHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EdgeHealthLogic) EdgeHealth(req *types.EdgeHealthRequest) (resp *types.EdgeHealthResponse, err error) {
	logx.Infof("Edge health check request: requestId=%s", req.RequestId)

	// Calculate uptime (this would be tracked in production)
	uptime := time.Since(time.Now()).Seconds() // Placeholder

	// Get current system load
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryMB := m.Alloc / 1024 / 1024

	// Count active tunnels
	tunnels := l.svcCtx.TunnelMgr.ListTunnels("", "active", 1, 1000)
	activeTunnels := int64(len(tunnels))

	// Count connected agents (this would be tracked in production)
	connectedAgents := int64(0) // Placeholder

	// Calculate load metrics
	load := map[string]float64{
		"memory_usage_percent": float64(memoryMB) * 100.0 / 4096.0, // Assuming 4GB total memory
		"goroutines":           float64(runtime.NumGoroutine()),
		"cpu_usage":           0.0, // TODO: Implement CPU usage calculation
		"connection_usage":    float64(len(l.svcCtx.ProxyMgr.connections)) * 100.0 / float64(l.svcCtx.Config.Proxy.MaxConnections),
	}

	status := "healthy"
	// Check if any critical thresholds are exceeded
	if load["memory_usage_percent"] > 80.0 {
		status = "degraded"
	}
	if load["connection_usage"] > 90.0 {
		status = "degraded"
	}

	return &types.EdgeHealthResponse{
		Status:    status,
		Uptime:    int64(uptime),
		Version:   "1.0.0", // This should be set from build info
		Tunnels:   activeTunnels,
		Agents:    connectedAgents,
		Load:      load,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}