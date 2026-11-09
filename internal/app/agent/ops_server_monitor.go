package agent

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	processcore "github.com/cuihairu/croupier/core/process"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
	exit := processcore.ExitDetail(waitErr)

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
	detectDown.ExitCode = exit.Code
	detectDown.Signal = exit.Signal
	detectDown.OomSuspect = processcore.IsOOMSuspect(exit.Signal, lastRSS)
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
			p.supervisor.DecideExit(false, time.Now())
			p.mu.Unlock()
			return
		}
		decision := p.supervisor.DecideExit(true, time.Now())
		fails := decision.ConsecutiveFails
		p.consecutiveFails = fails
		if decision.Action == processcore.SupervisorBreak {
			p.state = opsv1.ProcessState_PROCESS_STATE_BROKEN
			p.nextRestartAt = nil
			restarts := p.restarts
			msg := breakerMessage(p.config, fails)
			p.mu.Unlock()
			tripped := newSupervisorEvent(p.name, supervisorEventBreakerTripped)
			tripped.OldPid = oldPID
			tripped.RestartCount = restarts
			tripped.Message = msg
			s.emitSupervisorEvent(p, tripped)
			return
		}
		delay := decision.Delay
		nextAt := p.supervisor.NextRestartAt()
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
		p.supervisor.MarkRestartAttempted()
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
	delay := p.supervisor.SuccessWindow()
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
			p.supervisor.RecordSuccess()
		}
	}()
}
