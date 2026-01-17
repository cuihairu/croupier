// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// cpuUsageTracker tracks CPU usage over time
var cpuUsageTracker = &cpuTracker{}

type cpuTracker struct {
	lastTotal     uint64
	lastIdle      uint64
	lastCPUTime   time.Time
	lastCPUUsage  float64
	lastGCPauseNs uint64
}

type AgentHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentHealthLogic {
	return &AgentHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentHealthLogic) AgentHealth(req *types.AgentHealthRequest) (*types.AgentHealthResponse, error) {
	if req == nil || strings.TrimSpace(req.AgentId) == "" {
		return nil, errors.New("agent_id 不能为空")
	}

	agentID := strings.TrimSpace(req.AgentId)
	configuredID := strings.TrimSpace(l.svcCtx.Config.Agent.ID)
	if configuredID != "" && agentID != configuredID {
		return nil, errors.New("agent_id mismatch")
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := int64(l.svcCtx.Uptime().Seconds())
	if uptime < 0 {
		uptime = 0
	}

	functionCount := int64(0)
	activeJobs := int64(0)
	if l.svcCtx.Core != nil {
		if l.svcCtx.Core.Store() != nil {
			functionCount = int64(len(l.svcCtx.Core.Store().List()))
		}
		// Get active job count from the core
		activeJobs = int64(l.svcCtx.Core.ActiveJobCount())
	}

	status := "running"
	if l.svcCtx.Core == nil {
		status = "stopped"
	}

	// Calculate CPU usage based on Go runtime metrics
	cpuUsage := calculateCPUUsage(&mem)

	return &types.AgentHealthResponse{
		Status:    status,
		Uptime:    uptime,
		Jobs:      activeJobs,
		Functions: functionCount,
		Memory:    int64(mem.Alloc),
		Cpu:       cpuUsage,
	}, nil
}

// calculateCPUUsage estimates CPU usage based on Go runtime metrics.
// This uses GC pause time and goroutine scheduling as proxies for CPU activity.
func calculateCPUUsage(mem *runtime.MemStats) float64 {
	now := time.Now()

	// Calculate time since last measurement
	elapsed := now.Sub(cpuUsageTracker.lastCPUTime)
	if elapsed < 100*time.Millisecond {
		// Return cached value if called too frequently
		return cpuUsageTracker.lastCPUUsage
	}

	// Use GC pause time as a proxy for CPU activity
	// This is an approximation - real CPU usage would require platform-specific code
	var cpuUsage float64

	if cpuUsageTracker.lastGCPauseNs > 0 && mem.PauseTotalNs > cpuUsageTracker.lastGCPauseNs {
		// Calculate GC CPU time percentage
		gcTime := float64(mem.PauseTotalNs - cpuUsageTracker.lastGCPauseNs)
		elapsedNs := float64(elapsed.Nanoseconds())
		if elapsedNs > 0 {
			// GC time as percentage, scaled by number of CPUs
			cpuUsage = (gcTime / elapsedNs) * 100 * float64(runtime.NumCPU())
		}
	}

	// Add a base load estimation based on goroutine count
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()
	if numGoroutines > numCPU {
		// Estimate some base CPU usage when goroutines exceed CPUs
		cpuUsage += float64(numGoroutines-numCPU) * 0.5
	}

	// Clamp to reasonable range
	if cpuUsage > 100 {
		cpuUsage = 100
	}
	if cpuUsage < 0 {
		cpuUsage = 0
	}

	// Update tracker
	cpuUsageTracker.lastCPUTime = now
	cpuUsageTracker.lastGCPauseNs = mem.PauseTotalNs
	cpuUsageTracker.lastCPUUsage = cpuUsage

	return cpuUsage
}
