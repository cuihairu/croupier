package agent

// 定时动作调度（agent-supervisor-design §10.6 调度状态）：调度入口统一一份
// schedule 模型（与 server 侧 internal/tasks/scheduler 同一 ParseCron），但
// 定时触发与恢复环是两条执行路径——本文件只做「到点触发」，触发走显式动作
// 同一漏斗（doStart/doStop/doRestart，desired-state 与 supervisor 事件随流）；
// scheduled stop 与手动停同信号（stopCh），恢复环不会把定时停误判为崩溃拉起；
// 探测窗口不馈入调度（两路互不喂状态）。

import (
	"fmt"
	"sort"
	"time"

	processcore "github.com/cuihairu/croupier/core/process"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

type processScheduleEntry = processcore.ScheduleEntry

// compileProcessSchedules parses schedule declarations once at construction.
// Validate rejects bad entries at config load; compile skips defensively so a
// bad entry can never panic the loop.
func compileProcessSchedules(configs map[string]ManagedProcessConfig) map[string][]processScheduleEntry {
	out := make(map[string][]processScheduleEntry, len(configs))
	for name, cfg := range configs {
		if len(cfg.Schedule) == 0 {
			continue
		}
		entries := processcore.CompileSchedule(map[string][]processcore.ScheduleAction{name: cfg.Schedule})[name]
		if len(entries) > 0 {
			out[name] = entries
		}
	}
	return out
}

// nextScheduleEntry returns the soonest upcoming fire across a process's
// schedule entries (zero time when none matches within the cron horizon).
func nextScheduleEntry(entries []processScheduleEntry, now time.Time) (time.Time, string) {
	return processcore.NextSchedule(entries, now)
}

// startScheduleLoop launches the minute-slot scanner when any schedule is
// declared. Caller holds s.mu (Start).
func (s *OpsServer) startScheduleLoopLocked() {
	if len(s.scheds) == 0 || s.schedStop != nil {
		return
	}
	if s.schedTick <= 0 {
		s.schedTick = 30 * time.Second
	}
	s.schedFired = make(map[string]string, 8)
	schedStop := make(chan struct{})
	s.schedStop = schedStop
	go s.scheduleLoop(schedStop)
}

// scheduleLoop scans on a fixed tick; per-minute slot dedupe makes each
// matching minute fire exactly once regardless of tick phase.
func (s *OpsServer) scheduleLoop(schedStop <-chan struct{}) {
	ticker := time.NewTicker(s.schedTick)
	defer ticker.Stop()
	for {
		select {
		case <-schedStop:
			return
		case now := <-ticker.C:
			s.scheduleTick(now)
		}
	}
}

// scheduleTick fires every schedule action whose cron matches the current
// minute slot. Dedupe keys are (process, action, slot): the 30s scanner is
// idempotent, and slots missed while the agent was down never fire (no
// catch-up, crond semantics). Runs on the schedule loop goroutine only
// (schedFired is loop-owned); tests drive it directly with the real loop
// tick parked.
func (s *OpsServer) scheduleTick(now time.Time) {
	slot := fmt.Sprint(processcore.Slot(now).Unix())
	names := make([]string, 0, len(s.scheds))
	for name := range s.scheds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		// §10.2 调度暂停：维护窗口内 cron 冻结，进程运行态不动。暂停期间
		// 不写 schedFired——恢复后下一匹配槽照常触发（无补跑，crond 语义）。
		if s.isSchedulePaused(name) {
			continue
		}
		for _, entry := range s.scheds[name] {
			if !entry.Spec.Matches(now) {
				continue
			}
			key := name + "\x00" + entry.Action
			if s.schedFired[key] == slot {
				continue
			}
			s.schedFired[key] = slot
			s.fireScheduled(name, entry)
		}
	}
}

// fireScheduled executes one scheduled action through the same funnel as the
// panel's explicit actions (desired state + supervisor events ride along).
// Local schedule is declared intent from the agent's own config — same trust
// root as autoRestart/boot auto-start — so allowRestart (the remote-operation
// gate) does not apply (documented in config-agent.md).
func (s *OpsServer) fireScheduled(name string, entry processScheduleEntry) {
	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()

	if !ok {
		if entry.Action == ScheduleActionStop {
			// 幂等：实例不存在即无可停（未启动/已被停），无事件无副作用。
			return
		}
		// start/restart 面向未物化实例：与 StartProcess 同路物化。
		s.mu.RLock()
		cfg, hasCfg := s.config.ManagedProcesses[name]
		s.mu.RUnlock()
		if !hasCfg {
			return
		}
		p = &managedProcess{
			name:   name,
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
			stopCh: make(chan struct{}),
		}
		s.mu.Lock()
		s.processes[name] = p
		s.mu.Unlock()
	}

	switch entry.Action {
	case ScheduleActionStart:
		if _, err := s.doStartProcess(p, supervisorEventScheduleAction, "scheduled start ("+entry.Cron+")"); err != nil {
			s.emitScheduleFireError(p, entry, err)
		}
	case ScheduleActionStop:
		s.doStopProcess(p, supervisorEventScheduleAction, "scheduled stop ("+entry.Cron+")")
	case ScheduleActionRestart:
		if err := s.doRestartProcess(p, supervisorEventScheduleAction, "scheduled restart ("+entry.Cron+")"); err != nil {
			s.emitScheduleFireError(p, entry, err)
		}
	}
}

// emitScheduleFireError records a failed scheduled action (including gate /
// already-running rejections) — the event log is the full-truth audit trail.
func (s *OpsServer) emitScheduleFireError(p *managedProcess, entry processScheduleEntry, err error) {
	ev := newSupervisorEvent(p.name, supervisorEventScheduleAction)
	ev.Message = "scheduled " + entry.Action + " (" + entry.Cron + ") failed: " + err.Error()
	s.emitSupervisorEvent(p, ev)
}
