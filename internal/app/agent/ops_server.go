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

	// desired 受管进程管理状态（§10.3 管理状态面）：运维手动 start/stop 的
	// 期望跨 agent 重启保留，开机自启不再误启被主动停掉的进程。
	desired *desiredStateStore

	// schedPause 调度暂停状态（§10.2 暂停与恢复，暂停对象=调度）：运维维护
	// 窗口的 cron 冻结意图，跨 agent 重启保留；scheduleTick 跳过暂停进程。
	schedPause *schedulePauseStore

	// 调度状态（§10.6）：scheds 构造期编译（config 构造后不变，快照与循环
	// 共读）；schedStop 由 Stop 关闭；schedFired 只属调度循环 goroutine；
	// schedTick 缺省 30s（测试钩子，直接驱动 scheduleTick 时拉长）。
	scheds     map[string][]processScheduleEntry
	schedStop  chan struct{}
	schedFired map[string]string
	schedTick  time.Duration

	// updateStabilityWindow 更新稳定性窗口（§10.4）：缺省 5s（零值走缺省常量，
	// 测试注入短窗口）。
	updateStabilityWindow time.Duration

	// hintOnce 保证 core_pattern 只读提示（S3 中档）只发一次。
	hintOnce sync.Once
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
	// snapshotDir 是本实例的崩溃快照目录（S3 轻档，startProcess 时建），
	// 建目录失败为空串（env 档开关仍注入，只是无落盘位置）。
	snapshotDir string
	// fingerprint 是本实例启动时二进制的 sha256（§10.4 指纹硬要求）。spawn 时
	// 算一次并缓存：运行中实例仍跑旧 inode，文件被换走后本字段仍反映进程实际
	// 运行的内容；空串=未运行或内容读不到（未运行实例的快照字段为空）。
	fingerprint string
	mu          sync.RWMutex
}

// NewOpsServer creates a new Ops server instance.
func NewOpsServer(config *OpsConfig, agentID, version string, _ interface{}) *OpsServer {
	if config == nil {
		config = DefaultOpsConfig()
	}
	return &OpsServer{
		config:     config,
		agentID:    agentID,
		version:    version,
		processes:  make(map[string]*managedProcess),
		supLog:     newSupervisorEventLog(config.SupervisorLog),
		desired:    newDesiredStateStore(config.SupervisorLog),
		scheds:     compileProcessSchedules(config.ManagedProcesses),
		schedPause: newSchedulePauseStore(config.SupervisorLog),
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
	newPID := p.pid
	restarts := p.restarts
	p.mu.Unlock()

	return newPID, restarts, nil
}

// ExecuteCommand executes a command on the agent.
func (s *OpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	if !s.config.Enabled || !s.config.AllowExec {
		return nil, fmt.Errorf("ops exec is not enabled")
	}

	// 白名单前置（§10.7 远程 Shell 边界）：命中才放行；空列表一律拒绝
	// （fail-closed）——Validate 已拦配置面，此处兜底程序内直构路径。
	allowed := false
	for _, cmd := range s.config.ExecAllowedCommands {
		if cmd == req.Command {
			allowed = true
			break
		}
	}
	if !allowed {
		if len(s.config.ExecAllowedCommands) == 0 {
			return nil, fmt.Errorf("exec whitelist is empty: allowExec requires execAllowedCommands")
		}
		return nil, fmt.Errorf("command '%s' is not allowed", req.Command)
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

	// S3 轻档：先建 per-process 快照目录（0700），再按 snapshotProfile 注入
	// 语言原生开关。目录失败不阻断启动（env 档仍生效）。
	p.snapshotDir = ensureProcessSnapshotDir(s.config.SnapshotDirOrDefault(), p.name)

	// Set environment variables
	env := os.Environ()
	for k, v := range p.config.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	// S3 轻档注入：快照档 env 追加在用户 env 之后（exec 对重复 key 取最后
	// 值——显式声明的档位优先），node 档走 argv 前置。
	applySnapshotProfile(cmd, p.config, p.snapshotDir)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	p.cmd = cmd
	p.pid = int32(cmd.Process.Pid)
	// 指纹硬要求（§10.4）：spawn 时对本实例二进制算 sha256 并缓存——快照随行
	// 上行，告警/探针 timeline 按 (target, 时间, 指纹) 与更新记录对齐。计算失败
	// 留空串（不阻断启动；指纹缺失本身是可见事实）。
	p.fingerprint = sha256HexFile(p.config.Command)
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
	exitCode, sigName := exitDetail(waitErr)

	// 主动停止判定收进锁内（stopProcess 的 close 与新实例的 stopCh 赋值都
	// 持 p.mu——锁外读曾与 RestartProcess 换 stopCh 竞态）：stopProcess 已
	// close stopCh（StopProcess 置 STOPPED；RestartProcess 随后会换新实例）
	// 时不再触发 detect_down/拉起。
	p.mu.Lock()
	stopped := false
	if p.stopCh != nil {
		select {
		case <-p.stopCh:
			stopped = true
		default:
		}
	}
	if stopped || p.cmd != watched || p.state == opsv1.ProcessState_PROCESS_STATE_STOPPED {
		p.mu.Unlock()
		return
	}
	oldPID := p.pid
	lastRSS := p.lastRSS
	lastSample := p.lastSampleUnix
	snapDir := p.snapshotDir
	p.mu.Unlock()

	detectDown := newSupervisorEvent(p.name, supervisorEventDetectDown)
	detectDown.OldPid = oldPID
	detectDown.ExitCode = exitCode
	detectDown.Signal = sigName
	detectDown.OomSuspect = supervisorOomSuspect(sigName, lastRSS)
	detectDown.LastRssBytes = lastRSS
	detectDown.LastHeartbeatUnix = lastSample
	// S3：崩溃时刻登记快照产物（best-effort——目录为空/不存在即空列表）。
	if snapshotProfileEnabled(p.config.SnapshotProfile) {
		detectDown.SnapshotDir = snapDir
		detectDown.SnapshotFiles = collectSnapshotArtifacts(snapDir, p.config.WorkingDir, p.config.SnapshotProfile)
	}
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
		// 管理状态门（§10.2 区分主动停止）：期望无记录沿用既有语义
		// （AutoRestart 即拉起）；运维手动停过的进程跨 agent 重启保持停。
		if !s.desired.bootShouldStart(name) {
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

	// S3 中档只读提示：有进程开轻档且宿主机 core_pattern 非管道模式时发
	// 一次建议命令（once 保证不刷屏；agent 不写宿主配置）。
	s.emitSnapshotHintOnce()

	// §10.6 调度状态：有 schedule 声明才起分钟槽扫描循环（零配置零漂移）。
	s.startScheduleLoopLocked()

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

	// 调度循环先停：Stop 之后的进程收尾不该再被定时动作打断。
	if s.schedStop != nil {
		close(s.schedStop)
		s.schedStop = nil
	}

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
