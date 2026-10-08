package agent

import (
	"time"

	"github.com/shirou/gopsutil/v4/process"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// 超限标记（SupervisedProcessSnapshot.flags 闭集）。
const (
	flagMemOverLimit = "mem_over_limit"
	flagCPUOverLimit = "cpu_over_limit"
)

// SampleSupervisedProcesses samples every managed process and returns a
// snapshot per process (RUNNING ones carry live RSS/CPU). It is called once
// per metrics interval; results are also cached on each managedProcess so
// ListProcesses can expose the latest resource values without re-sampling.
//
// 与 ListProcesses 的口径差：快照覆盖「配置面 ∪ 实例面」——配置了但从未
// 启动的进程也以 STOPPED 出现（面板能看到完整配置面），ListProcesses 仍
// 只列已物化的实例（既有语义，见 TestOpsServerStart_SkipsNonAutoRestartV9）。
func (s *OpsServer) SampleSupervisedProcesses() []*opsv1.SupervisedProcessSnapshot {
	s.mu.RLock()
	procs := make([]*managedProcess, 0, len(s.processes)+len(s.config.ManagedProcesses))
	seen := make(map[string]bool, len(s.processes))
	for _, p := range s.processes {
		procs = append(procs, p)
		seen[p.name] = true
	}
	for name, cfg := range s.config.ManagedProcesses {
		if !seen[name] {
			// 只读合成条目：不写回 s.processes（物化仍由 StartProcess/Start 负责）。
			procs = append(procs, &managedProcess{
				name:   name,
				config: cfg,
				state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
			})
		}
	}
	s.mu.RUnlock()

	now := time.Now()
	out := make([]*opsv1.SupervisedProcessSnapshot, 0, len(procs))
	for _, p := range procs {
		p.mu.Lock()
		snap := &opsv1.SupervisedProcessSnapshot{
			Name:         p.name,
			Pid:          p.pid,
			State:        p.state,
			RestartCount: p.restarts,
		}
		if p.state == opsv1.ProcessState_PROCESS_STATE_RUNNING {
			if p.lastStart != nil {
				if up := int64(now.Sub(p.lastStart.AsTime()).Seconds()); up > 0 {
					snap.UptimeSeconds = up
				}
			}
			if p.pid > 0 {
				sampleProcessResources(p, snap)
			}
			snap.Flags = supervisorFlags(p.config, snap.RssBytes, snap.CpuPercent)
		}
		// 缓存最新采样，供 ListProcesses 展示（RPC 路径不再触发 /proc 读取）。
		p.lastRSS = snap.RssBytes
		p.lastCPU = snap.CpuPercent
		p.lastFlags = snap.Flags
		p.mu.Unlock()
		out = append(out, snap)
	}
	return out
}

// sampleProcessResources reads RSS and CPU% for a running process. The
// gopsutil handle is cached on the managedProcess so CPUPercent reports the
// delta since the previous sample; the first sample covers the span since
// process start. The handle is refreshed when the pid changed (restart).
// Caller must hold p.mu.
func sampleProcessResources(p *managedProcess, snap *opsv1.SupervisedProcessSnapshot) {
	if p.proc == nil || p.proc.Pid != p.pid {
		proc, err := process.NewProcess(p.pid)
		if err != nil {
			return
		}
		p.proc = proc
	}
	if mi, err := p.proc.MemoryInfo(); err == nil && mi != nil {
		snap.RssBytes = int64(mi.RSS)
	}
	// v4 的 CPUPercent 无 interval 参数：首次调用报进程启动以来的均值，
	// 之后按同一句柄的上次调用做区间差分。
	if cp, err := p.proc.CPUPercent(); err == nil {
		snap.CpuPercent = cp
	}
}

// supervisorFlags evaluates threshold markers. A threshold of 0 disables the
// corresponding check. Markers only expose the anomaly; killing or restarting
// the process stays a human decision.
func supervisorFlags(cfg ManagedProcessConfig, rssBytes int64, cpuPercent float64) []string {
	var flags []string
	if cfg.MemThresholdBytes > 0 && rssBytes >= cfg.MemThresholdBytes {
		flags = append(flags, flagMemOverLimit)
	}
	if cfg.CpuThresholdPercent > 0 && cpuPercent >= cfg.CpuThresholdPercent {
		flags = append(flags, flagCPUOverLimit)
	}
	return flags
}
