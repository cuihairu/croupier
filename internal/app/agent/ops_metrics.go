package agent

import (
	"os"
	"runtime"
	"sync"

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
	// ProbeStatsSampler 探测统计供给接口（core/metrics.ProbeStatsSampler
	// 别名，ProbeManager 实现）。
	ProbeStatsSampler = metrics.ProbeStatsSampler
)

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(agentID string) *metrics.Collector {
	return metrics.NewCollector(agentID)
}

var (
	selfFingerprintOnce sync.Once
	selfFingerprintVal  string
)

// SelfBinaryFingerprint 返回 agent 自身二进制内容 sha256（hex 小写）。§10.5
// 指纹可先落项：随 SystemInfo 上行作失联排查锚点与自升级指纹面 wire 前置。
// 进程生命周期内缓存一次（运行实例指纹语义，同 §10.4——文件被换值不变，
// 直至重启换实例）；读不到返回空串，不伪造值。
func SelfBinaryFingerprint() string {
	selfFingerprintOnce.Do(func() {
		if exe, err := os.Executable(); err == nil {
			selfFingerprintVal = sha256HexFile(exe)
		}
	})
	return selfFingerprintVal
}

// GetSystemInfo collects detailed system information.
func GetSystemInfo(agentID, agentVersion string, opsConfig *OpsConfig) *opsv1.SystemInfo {
	info := &opsv1.SystemInfo{
		Arch:              runtime.GOARCH,
		CpuCores:          int32(runtime.NumCPU()),
		AgentVersion:      agentVersion,
		AgentBinarySha256: SelfBinaryFingerprint(),
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
