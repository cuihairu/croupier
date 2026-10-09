// Package agent provides Ops server implementation.
// This replaces the gRPC-based Ops server with a lightweight implementation.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OpsServer implements ops functionality for the agent.
// It provides system info, process management, and command execution capabilities.
type OpsServer struct {
	mu      sync.RWMutex
	config  *OpsConfig
	agentID string
	version string

	// Managed process tracking
	processes map[string]*managedProcess

	// supLog 是监管事件环+轮转文件（S2 双通道的 agent 侧 truth）。
	supLog *supervisorEventLog
}

// managedProcess represents a managed process state
type managedProcess struct {
	name      string
	cmd       *exec.Cmd
	state     opsv1.ProcessState
	pid       int32
	restarts  int32
	lastStart *timestamppb.Timestamp
	config    ManagedProcessConfig
	stopCh    chan struct{}
	// waitDone 由 monitorProcess 在 cmd.Wait() 返回后关闭——
	// Wait 的唯一属主是 monitorProcess；stopProcess 杀进程后等它收尸，
	// 消除并发双 Wait 的 DATA RACE。
	waitDone chan struct{}
	// supervisor 采样缓存与 gopsutil 句柄（CPUPercent 依赖同一句柄做
	// 区间差分）。均由 p.mu 保护。
	proc      *process.Process
	lastRSS   int64
	lastCPU   float64
	lastFlags []string
	// S2 监管状态机字段（p.mu 保护）：consecutiveFails 是连续失败计数
	// （存活超过退避封顶即清零）；nextRestartAt 是 BACKOFF 态的下次拉起
	// 时刻；lastEventUnix/lastSampleUnix 供快照的 last_event_unix 与
	// 事件上下文 last_heartbeat 使用。
	consecutiveFails int
	nextRestartAt    *time.Time
	lastEventUnix    int64
	lastSampleUnix   int64
	mu               sync.RWMutex
}

// NewOpsServer creates a new Ops server instance.
func NewOpsServer(config *OpsConfig, agentID, version string, _ interface{}) *OpsServer {
	if config == nil {
		config = DefaultOpsConfig()
	}
	return &OpsServer{
		config:    config,
		agentID:   agentID,
		version:   version,
		processes: make(map[string]*managedProcess),
		supLog:    newSupervisorEventLog(config.SupervisorLog),
	}
}

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

	p.mu.Lock()
	// Stop the process
	s.stopProcess(p)

	// Start it again
	if err := s.startProcess(p); err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("failed to restart process: %w", err)
	}

	oldPID := p.pid
	p.restarts++
	p.lastStart = timestamppb.Now()
	p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
	p.nextRestartAt = nil
	// 人工干预成功即复位熔断计数（BROKEN → RUNNING 的修复路径）。
	p.consecutiveFails = 0
	newPID := p.pid
	restarts := p.restarts
	p.mu.Unlock()

	manual := newSupervisorEvent(p.name, supervisorEventManualStart)
	manual.OldPid = oldPID
	manual.NewPid = newPID
	manual.RestartCount = restarts
	manual.Message = "manual restart"
	s.emitSupervisorEvent(p, manual)

	return &opsv1.RestartProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' restarted", req.ProcessName),
	}, nil
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

	p.mu.Lock()
	s.stopProcess(p)
	oldPID := p.pid
	p.pid = 0
	p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
	p.nextRestartAt = nil
	p.mu.Unlock()

	ev := newSupervisorEvent(p.name, supervisorEventManualStop)
	ev.OldPid = oldPID
	ev.Message = "manual stop"
	s.emitSupervisorEvent(p, ev)

	return &opsv1.StopProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' stopped", req.ProcessName),
	}, nil
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
		p = &managedProcess{
			name:   req.ProcessName,
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
			stopCh: make(chan struct{}),
		}

		s.mu.Lock()
		s.processes[req.ProcessName] = p
		s.mu.Unlock()
	}

	p.mu.Lock()

	if p.state == opsv1.ProcessState_PROCESS_STATE_RUNNING {
		p.mu.Unlock()
		return nil, fmt.Errorf("process '%s' is already running", req.ProcessName)
	}

	if err := s.startProcess(p); err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	p.restarts++
	p.lastStart = timestamppb.Now()
	p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
	p.nextRestartAt = nil
	// 手动 start 是 BROKEN 的复位路径：成功即清连续失败计数。
	p.consecutiveFails = 0
	newPID := p.pid
	restarts := p.restarts
	p.mu.Unlock()

	ev := newSupervisorEvent(p.name, supervisorEventManualStart)
	ev.NewPid = newPID
	ev.RestartCount = restarts
	ev.Message = "manual start"
	s.emitSupervisorEvent(p, ev)

	return &opsv1.StartProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' started", req.ProcessName),
		Pid:     p.pid,
	}, nil
}

// ExecuteCommand executes a command on the agent.
func (s *OpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	if !s.config.Enabled || !s.config.AllowExec {
		return nil, fmt.Errorf("ops exec is not enabled")
	}

	// Check if command is allowed
	if len(s.config.ExecAllowedCommands) > 0 {
		allowed := false
		for _, cmd := range s.config.ExecAllowedCommands {
			if cmd == req.Command {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("command '%s' is not allowed", req.Command)
		}
	}

	// Create command with timeout
	timeout := s.config.ExecTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Set environment variables if provided
	if len(req.Env) > 0 {
		env := os.Environ()
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	output, err := cmd.CombinedOutput()

	exitCode := int32(0)
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitError.ExitCode())
		} else {
			exitCode = -1
		}
	}

	return &opsv1.ExecuteCommandResponse{
		ExitCode: exitCode,
		StdOut:   string(output),
		StdErr:   "", // Combined in stdout
	}, nil
}

// startProcess starts a managed process
func (s *OpsServer) startProcess(p *managedProcess) error {
	cmd := exec.Command(p.config.Command, p.config.Args...)
	if p.config.WorkingDir != "" {
		cmd.Dir = p.config.WorkingDir
	}

	// Set environment variables
	env := os.Environ()
	for k, v := range p.config.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	p.cmd = cmd
	p.pid = int32(cmd.Process.Pid)
	p.waitDone = make(chan struct{})
	// 每个实例一个新 stopCh：手动 stop/restart 已把旧 stopCh close，
	// 复用会让新实例的退避等待与归属守卫误判为「主动停止」。
	p.stopCh = make(chan struct{})
	// 旧 gopsutil 句柄属于已死实例；pid 恰好复用时句柄校验不可靠，直接弃用。
	p.proc = nil

	// Start goroutine to monitor process
	go s.monitorProcess(p)

	return nil
}

// stopProcess stops a managed process
func (s *OpsServer) stopProcess(p *managedProcess) {
	if p.stopCh != nil {
		select {
		case <-p.stopCh:
			// Already closed
		default:
			close(p.stopCh)
		}
	}

	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		// 收尸交给 monitorProcess（Wait 唯一属主）；此处只等它完成，
		// 并发双 Wait 曾是 DATA RACE。
		if wd := p.waitDone; wd != nil {
			select {
			case <-wd:
			case <-time.After(stopProcessWaitTimeout):
			}
		}
	}
}

// stopProcessWaitTimeout 是 stopProcess 等待收尸完成的上限。提为包级
// 变量以便测试注入短超时，确定性覆盖等待超时分支（无人调用 close 的
// waitDone）。
var stopProcessWaitTimeout = 5 * time.Second

// monitorProcess waits for the watched instance and hands the exit to the
// supervisor state machine. Wait 的唯一属主仍是本 goroutine。
func (s *OpsServer) monitorProcess(p *managedProcess) {
	// 实例归属：本 monitor 只代表它 Wait 的这个 cmd/watchDone。Wait 期间
	// 外部 RestartProcess/StartProcess 可能换掉 p.cmd 开新实例——旧实例的
	// 死亡既不能翻当前状态，也不能再触发拉起（superviseExit 内以 p.cmd
	// 归属守卫兜底）。
	watched := p.cmd
	watchedWaitDone := p.waitDone
	if watched == nil || watched.Process == nil {
		return
	}

	err := watched.Wait()
	if watchedWaitDone != nil {
		close(watchedWaitDone)
	}

	s.superviseExit(p, watched, err)
}

// autoRestartAllowed reports whether automatic restart may proceed: the
// per-process switch AND the ops double gate (Enabled+AllowRestart), mirroring
// the manual start/stop gating. Caller must hold p.mu (reads p.config).
func (s *OpsServer) autoRestartAllowed(p *managedProcess) bool {
	return p.config.AutoRestart && s.config.Enabled && s.config.AllowRestart
}

// superviseExit runs the S2 restart loop after the watched instance exited:
// detect_down → (breaker | exponential backoff → auto restart) — restart
// failures loop back into the breaker count. 退避 sleep 在锁外且可被 stopCh
// 打断；每次状态迁移前都重查实例归属，手动 stop/restart 换掉 p.cmd 后旧
// monitor 直接退出。
func (s *OpsServer) superviseExit(p *managedProcess, watched *exec.Cmd, waitErr error) {
	// 主动停止：stopProcess 已 close stopCh（StopProcess 置 STOPPED；
	// RestartProcess 随后会换新实例）。此时不再触发 detect_down/拉起。
	if p.stopCh != nil {
		select {
		case <-p.stopCh:
			return
		default:
		}
	}

	exitCode, sigName := exitDetail(waitErr)

	p.mu.Lock()
	if p.cmd != watched || p.state == opsv1.ProcessState_PROCESS_STATE_STOPPED {
		p.mu.Unlock()
		return
	}
	oldPID := p.pid
	lastRSS := p.lastRSS
	lastSample := p.lastSampleUnix
	p.mu.Unlock()

	detectDown := newSupervisorEvent(p.name, supervisorEventDetectDown)
	detectDown.OldPid = oldPID
	detectDown.ExitCode = exitCode
	detectDown.Signal = sigName
	detectDown.OomSuspect = supervisorOomSuspect(sigName, lastRSS)
	detectDown.LastRssBytes = lastRSS
	detectDown.LastHeartbeatUnix = lastSample
	s.emitSupervisorEvent(p, detectDown)

	for {
		p.mu.Lock()
		if p.cmd != watched || p.state == opsv1.ProcessState_PROCESS_STATE_STOPPED {
			p.mu.Unlock()
			return
		}
		if !s.autoRestartAllowed(p) {
			p.state = opsv1.ProcessState_PROCESS_STATE_FAILED
			p.nextRestartAt = nil
			p.mu.Unlock()
			return
		}
		p.consecutiveFails++
		fails := p.consecutiveFails
		if breakerTripped(p.config, int32(fails)) {
			p.state = opsv1.ProcessState_PROCESS_STATE_BROKEN
			p.nextRestartAt = nil
			restarts := p.restarts
			msg := breakerMessage(p.config, int32(fails))
			p.mu.Unlock()
			tripped := newSupervisorEvent(p.name, supervisorEventBreakerTripped)
			tripped.OldPid = oldPID
			tripped.RestartCount = restarts
			tripped.Message = msg
			s.emitSupervisorEvent(p, tripped)
			return
		}
		delay := supervisorBackoffDelay(p.config, fails)
		nextAt := time.Now().Add(delay)
		p.state = opsv1.ProcessState_PROCESS_STATE_BACKOFF
		p.nextRestartAt = &nextAt
		stopCh := p.stopCh
		p.mu.Unlock()

		// 退避等待在锁外（持锁睡眠会堵满状态读，见 S1 修复注记），
		// 停机信号可打断。
		if stopCh != nil {
			select {
			case <-stopCh:
				p.mu.Lock()
				if p.cmd == watched {
					p.nextRestartAt = nil
				}
				p.mu.Unlock()
				return
			case <-time.After(delay):
			}
		} else {
			time.Sleep(delay)
		}

		p.mu.Lock()
		if p.cmd != watched || p.state == opsv1.ProcessState_PROCESS_STATE_STOPPED {
			p.mu.Unlock()
			return
		}
		p.nextRestartAt = nil
		p.state = opsv1.ProcessState_PROCESS_STATE_STARTING
		spawnErr := s.startProcess(p)
		if spawnErr == nil {
			newCmd := p.cmd
			newPID := p.pid
			p.restarts++
			p.lastStart = timestamppb.Now()
			p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
			restarts := p.restarts
			newStopCh := p.stopCh
			p.mu.Unlock()
			restarted := newSupervisorEvent(p.name, supervisorEventAutoRestart)
			restarted.OldPid = oldPID
			restarted.NewPid = newPID
			restarted.RestartCount = restarts
			s.emitSupervisorEvent(p, restarted)
			// 成功判定：新实例存活超过退避封顶才清连续失败计数，
			// 防止启动即崩的进程把退避序列刷穿（设计 §3.4）。
			s.armSuccessTimer(p, newCmd, newStopCh)
			return
		}
		p.state = opsv1.ProcessState_PROCESS_STATE_FAILED
		p.mu.Unlock()
		failed := newSupervisorEvent(p.name, supervisorEventRestartFailed)
		failed.OldPid = oldPID
		failed.Message = spawnErr.Error()
		s.emitSupervisorEvent(p, failed)
		// spawn 失败计入连续失败：循环回到顶部重查熔断/退避。
	}
}

// armSuccessTimer clears the consecutive-failure counter once the restarted
// instance (watched) has survived longer than the backoff cap while still
// being the current p.cmd in RUNNING state. Ownership guard同 monitor：期间
// 发生的手动操作/再次崩溃都会让本定时器失效。
func (s *OpsServer) armSuccessTimer(p *managedProcess, watched *exec.Cmd, stopCh chan struct{}) {
	delay := supervisorBackoffDelay(p.config, 2)
	go func() {
		if stopCh != nil {
			select {
			case <-stopCh:
				return
			case <-time.After(delay):
			}
		} else {
			time.Sleep(delay)
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.cmd == watched && p.state == opsv1.ProcessState_PROCESS_STATE_RUNNING {
			p.consecutiveFails = 0
		}
	}()
}

// Start starts all configured managed processes.
func (s *OpsServer) Start() error {
	if !s.config.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for name, cfg := range s.config.ManagedProcesses {
		if !cfg.AutoRestart {
			continue
		}

		p := &managedProcess{
			name:   name,
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
		}

		if err := s.startProcess(p); err != nil {
			// Log but don't fail
			continue
		}

		// monitor goroutine 已随 startProcess 启动：实例字段写入必须过
		// p.mu（s.mu 管不了 monitor 与 superviseExit 的读路径）。
		p.mu.Lock()
		p.restarts = 1
		p.lastStart = timestamppb.Now()
		p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
		p.mu.Unlock()
		s.processes[name] = p
	}

	return nil
}

// GetSupervisorLog returns a tail of the rotating supervisor event log for
// the download proxy (server pulls via the ops tunnel; size-capped).
func (s *OpsServer) GetSupervisorLog(ctx context.Context, req *opsv1.GetSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error) {
	content, fileName, truncated, err := s.supLog.ReadTail(int64(req.GetMaxBytes()))
	if err != nil {
		if os.IsNotExist(err) {
			// 尚未产生事件时文件不存在：返回空内容而非错误，面板可直接下载空日志。
			return &opsv1.GetSupervisorLogResponse{Content: []byte{}, FileName: fileName}, nil
		}
		return nil, err
	}
	return &opsv1.GetSupervisorLogResponse{
		Content:   content,
		FileName:  fileName,
		Truncated: truncated,
	}, nil
}

// Stop stops all managed processes.
func (s *OpsServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.processes {
		p.mu.Lock()
		s.stopProcess(p)
		p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
		// 此前误用 RUnlock：写锁配读解锁，processes 非空时 Go 运行时
		// 直接 fatal "sync: RUnlock of unlocked RWMutex" 崩溃整个 agent。
		p.mu.Unlock()
	}
}

// Close stops the Ops server.
func (s *OpsServer) Close() error {
	s.Stop()
	return nil
}

// ========== System Services (JSON methods to avoid circular dependency) ==========

// ListServicesJSON handles ListServicesRequest via JSON
func (s *OpsServer) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	var req ListServicesRequest
	if len(jsonReq) > 0 {
		if err := json.Unmarshal(jsonReq, &req); err != nil {
			return nil, err
		}
	}

	services, err := ListServices(req.State, req.NamePattern, int(req.Limit))
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := struct {
		Services []*ServiceInfo `json:"services"`
		Total    int32          `json:"total"`
	}{
		Services: make([]*ServiceInfo, len(services)),
		Total:    int32(len(services)),
	}
	for i := range services {
		resp.Services[i] = &services[i]
	}

	return json.Marshal(resp)
}

// GetServiceStatusJSON handles GetServiceStatusRequest via JSON
func (s *OpsServer) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	var req GetServiceStatusRequest
	if err := json.Unmarshal(jsonReq, &req); err != nil {
		return nil, err
	}

	status, err := GetServiceStatus(req.Name)
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := &ServiceStatusDetail{
		Name:        status.Name,
		DisplayName: status.DisplayName,
		Status:      status.Status,
		StartType:   status.StartType,
		ProcessID:   status.ProcessID,
		BinaryPath:  status.BinaryPath,
		Description: status.Description,
	}

	return json.Marshal(resp)
}

// ListCronJobsJSON handles ListCronJobsRequest via JSON
func (s *OpsServer) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	// 覆盖边界说明：linux 的 listCronJobsPlatform 恒返回 nil error（所有
	// 目录读取失败均静默跳过），此 err 分支仅 windows/stub 平台可达，
	// linux 测试构建不可覆盖。
	jobs, err := ListCronJobs()
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := struct {
		Jobs  []*CronJob `json:"jobs"`
		Total int32      `json:"total"`
	}{
		Jobs:  make([]*CronJob, len(jobs)),
		Total: int32(len(jobs)),
	}
	for i := range jobs {
		resp.Jobs[i] = &jobs[i]
	}

	return json.Marshal(resp)
}

// ========== System Services ==========

// ListServicesRequest requests a list of system services.
type ListServicesRequest struct {
	State       string `json:"state"`
	NamePattern string `json:"namePattern"`
	Limit       int32  `json:"limit"`
}

// ListServicesResponse contains the list of system services.
type ListServicesResponse struct {
	Services []*ServiceInfo `json:"services"`
	Total    int32          `json:"total"`
}

// GetServiceStatusRequest requests detailed status of a specific service.
type GetServiceStatusRequest struct {
	Name string `json:"name"`
}

// GetServiceStatusResponse contains detailed service status.
type GetServiceStatusResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	StartType   string `json:"startType"`
	ProcessID   uint32 `json:"processId"`
	BinaryPath  string `json:"binaryPath"`
	Description string `json:"description"`
}

// ListCronJobsResponse contains the list of cron jobs.
type ListCronJobsResponse struct {
	Jobs  []*CronJob `json:"jobs"`
	Total int32      `json:"total"`
}

// ListServices returns system services.
func (s *OpsServer) ListServices(ctx context.Context, req *ListServicesRequest) (*ListServicesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	services, err := ListServices(req.State, req.NamePattern, limit)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	servicePtrs := make([]*ServiceInfo, len(services))
	for i := range services {
		servicePtrs[i] = &services[i]
	}

	return &ListServicesResponse{
		Services: servicePtrs,
		Total:    int32(len(services)),
	}, nil
}

// GetServiceStatus returns detailed service status.
func (s *OpsServer) GetServiceStatus(ctx context.Context, req *GetServiceStatusRequest) (*GetServiceStatusResponse, error) {
	status, err := GetServiceStatus(req.Name)
	if err != nil {
		return nil, err
	}

	return &GetServiceStatusResponse{
		Name:        status.Name,
		DisplayName: status.DisplayName,
		Status:      status.Status,
		StartType:   status.StartType,
		ProcessID:   status.ProcessID,
		BinaryPath:  status.BinaryPath,
		Description: status.Description,
	}, nil
}

// ListCronJobs returns cron jobs on Linux systems.
func (s *OpsServer) ListCronJobs(ctx context.Context) (*ListCronJobsResponse, error) {
	// 覆盖边界说明：同 ListCronJobsJSON——linux 平台 ListCronJobs 恒返回
	// nil error，err 分支仅 windows/stub 平台可达。
	jobs, err := ListCronJobs()
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	jobPtrs := make([]*CronJob, len(jobs))
	for i := range jobs {
		jobPtrs[i] = &jobs[i]
	}

	return &ListCronJobsResponse{
		Jobs:  jobPtrs,
		Total: int32(len(jobs)),
	}, nil
}
