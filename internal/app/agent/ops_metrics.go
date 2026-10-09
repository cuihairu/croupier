package agent

import (
	"runtime"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"

	"github.com/cuihairu/croupier/core/metrics"
)

// 指标采集已上收 core/metrics（agent-core K3）：sidecar 保留类型别名、构造
// 转发与 GetSystemInfo（OpsConfig/监管状态属 sidecar 面，不上收）；采集本体
// （gopsutil CPU/内存/磁盘/网络 + 受管进程快照折入 + 事件增量游标）见
// core/metrics，capture/devops 等 core 系 agent 可独立复用。

type (
	// MetricsCollector 系统指标采集器（core/metrics.Collector 别名）。
	MetricsCollector = metrics.Collector
	// SupervisorSampler 受管进程快照供给接口（core/metrics.Sampler 别名，
	// OpsServer 实现）。
	SupervisorSampler = metrics.Sampler
)

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(agentID string) *metrics.Collector {
	return metrics.NewCollector(agentID)
}

// GetSystemInfo collects detailed system information.
func GetSystemInfo(agentID, agentVersion string, opsConfig *OpsConfig) *opsv1.SystemInfo {
	info := &opsv1.SystemInfo{
		Arch:         runtime.GOARCH,
		CpuCores:     int32(runtime.NumCPU()),
		AgentVersion: agentVersion,
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
