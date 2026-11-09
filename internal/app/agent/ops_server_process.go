package agent

import (
	"context"
	"fmt"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetSystemInfo returns system information.
func (s *OpsServer) GetSystemInfo(ctx context.Context, _ *emptypb.Empty) (*opsv1.SystemInfo, error) {
	return GetSystemInfo(s.agentID, s.version, s.config), nil
}

// ListProcesses returns the list of managed processes.
func (s *OpsServer) ListProcesses(ctx context.Context, _ *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &opsv1.ListProcessesResponse{
		Processes: make([]*opsv1.ManagedProcess, 0, len(s.processes)),
	}

	for _, p := range s.processes {
		p.mu.RLock()
		mp := &opsv1.ManagedProcess{
			Name:         p.name,
			Command:      p.config.Command,
			WorkingDir:   p.config.WorkingDir,
			State:        p.state,
			Pid:          p.pid,
			RestartCount: p.restarts,
			LastStart:    p.lastStart,
			Flags:        p.lastFlags,
		}
		if p.state == opsv1.ProcessState_PROCESS_STATE_RUNNING && p.lastStart != nil {
			if up := int64(time.Since(p.lastStart.AsTime()).Seconds()); up > 0 {
				mp.UptimeSeconds = up
			}
		}
		if p.state == opsv1.ProcessState_PROCESS_STATE_BACKOFF && p.nextRestartAt != nil {
			mp.NextRestartAtUnix = p.nextRestartAt.Unix()
		}
		p.mu.RUnlock()
		resp.Processes = append(resp.Processes, mp)
	}

	return resp, nil
}

// ReportMetrics handles metrics reporting (just acknowledges).
func (s *OpsServer) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	// Metrics are handled by the MetricsCollector in upstream.go
	// This is just an acknowledgment for the protocol
	return &emptypb.Empty{}, nil
}

// RestartProcess restarts a managed process.
func (s *OpsServer) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}

	s.mu.Lock()
	p, ok := s.processes[req.ProcessName]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("process '%s' not found", req.ProcessName)
	}

	if err := s.doRestartProcess(p, supervisorEventManualStart, "manual restart"); err != nil {
		return nil, err
	}

	return &opsv1.RestartProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' restarted", req.ProcessName),
	}, nil
}

// doRestartProcess is the shared restart funnel (manual ops and scheduled
// actions §10.6): stop → start → state updates → desired-state → event.
func (s *OpsServer) doRestartProcess(p *managedProcess, event, message string) error {
	p.mu.Lock()
	// Stop the process（旧 pid 先于 stop 捕获：startProcess 会立刻写入新 pid）
	oldPID := p.pid
	s.stopProcess(p)

	// Start it again
	if err := s.startProcess(p); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to restart process: %w", err)
	}

	// 重启成功即改回期望运行（此前若处于手动停态，重启是显式运行意图）。
	p.restarts++
	p.lastStart = timestamppb.Now()
	p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
	p.nextRestartAt = nil
	// 人工干预成功即复位熔断计数（BROKEN → RUNNING 的修复路径）。
	p.consecutiveFails = 0
	p.supervisor.RecordStart()
	newPID := p.pid
	restarts := p.restarts
	p.mu.Unlock()
	s.desired.SetRunning(p.name, true)

	ev := newSupervisorEvent(p.name, event)
	ev.OldPid = oldPID
	ev.NewPid = newPID
	ev.RestartCount = restarts
	ev.Message = message
	s.emitSupervisorEvent(p, ev)

	return nil
}

// StopProcess stops a managed process.
func (s *OpsServer) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}

	s.mu.Lock()
	p, ok := s.processes[req.ProcessName]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("process '%s' not found", req.ProcessName)
	}

	s.doStopProcess(p, supervisorEventManualStop, "manual stop")

	return &opsv1.StopProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' stopped", req.ProcessName),
	}, nil
}

// doStopProcess is the shared stop funnel (manual ops and scheduled actions
// §10.6)：与手动停同信号（stopCh close），恢复环据此判定主动停止、不误拉起。
func (s *OpsServer) doStopProcess(p *managedProcess, event, message string) {
	oldPID := s.coreStopProcess(p)
	// 停止是期望状态变更：跨 agent 重启不再被开机自启误启。
	s.desired.SetRunning(p.name, false)

	ev := newSupervisorEvent(p.name, event)
	ev.OldPid = oldPID
	ev.Message = message
	s.emitSupervisorEvent(p, ev)
}

// coreStopProcess is the silent stop core（状态迁移，不写事件、不改期望状态）——
// stop 漏斗与更新换包（§10.4 更新用独立事件分型，不留中间 manual_stop 噪音）共用。
// oldPID 在锁内捕获后返回：锁外读 p.pid 会与 monitor 写路径竞态。
func (s *OpsServer) coreStopProcess(p *managedProcess) int32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	s.stopProcess(p)
	oldPID := p.pid
	p.pid = 0
	p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
	p.nextRestartAt = nil
	p.supervisor.CancelBackoff()
	return oldPID
}

// StartProcess starts a managed process.
func (s *OpsServer) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}

	s.mu.Lock()
	p, ok := s.processes[req.ProcessName]
	s.mu.Unlock()

	if !ok {
		// Check if we have a config for this process
		s.mu.RLock()
		cfg, hasCfg := s.config.ManagedProcesses[req.ProcessName]
		s.mu.RUnlock()

		if !hasCfg {
			return nil, fmt.Errorf("process '%s' not configured", req.ProcessName)
		}

		// Create new managed process
		p = newManagedProcess(req.ProcessName, cfg)

		s.mu.Lock()
		s.processes[req.ProcessName] = p
		s.mu.Unlock()
	}

	newPID, err := s.doStartProcess(p, supervisorEventManualStart, "manual start")
	if err != nil {
		return nil, err
	}

	return &opsv1.StartProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' started", req.ProcessName),
		// 锁内已捕获 newPID：此处重读 p.pid 会与 monitor 的自动拉起写路径
		// 竞态（DATA RACE，覆盖批次实测复现）。
		Pid: newPID,
	}, nil
}

// doStartProcess is the shared start funnel (manual ops and scheduled actions
// §10.6)。返回锁内捕获的新 pid（-1 表示未知，调用方自行处理）。
func (s *OpsServer) doStartProcess(p *managedProcess, event, message string) (int32, error) {
	newPID, restarts, err := s.coreStartProcess(p)
	if err != nil {
		return newPID, err
	}
	// 启动是期望状态变更：期望 running 落盘（清掉此前的停意图）。
	s.desired.SetRunning(p.name, true)

	ev := newSupervisorEvent(p.name, event)
	ev.NewPid = newPID
	ev.RestartCount = restarts
	ev.Message = message
	s.emitSupervisorEvent(p, ev)
	// 面板手工首启也可能第一次真正开轻档——提示兜底覆盖此路径。
	s.emitSnapshotHintOnce()

	return newPID, nil
}

// coreStartProcess is the silent start core（拉起+状态迁移，不写事件、不改期望
// 状态）—— start 漏斗与更新换包（§10.4）共用。返回锁内捕获的新 pid 与重启计数。
func (s *OpsServer) coreStartProcess(p *managedProcess) (int32, int32, error) {
	p.mu.Lock()

	if p.state == opsv1.ProcessState_PROCESS_STATE_RUNNING {
		p.mu.Unlock()
		return -1, 0, fmt.Errorf("process '%s' is already running", p.name)
	}

	if err := s.startProcess(p); err != nil {
		p.mu.Unlock()
		return -1, 0, fmt.Errorf("failed to start process: %w", err)
	}

	p.restarts++
	p.lastStart = timestamppb.Now()
	p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
	p.nextRestartAt = nil
	// start 是 BROKEN 的复位路径：成功即清连续失败计数。
	p.consecutiveFails = 0
	p.supervisor.RecordStart()
	newPID := p.pid
	restarts := p.restarts
	p.mu.Unlock()

	return newPID, restarts, nil
}
