package agent

// 定时动作调度（agent-supervisor-design §10.6 调度状态）：调度入口统一一份
// schedule 模型（与 server 侧 internal/tasks/scheduler 同一 ParseCron），但
// 定时触发与恢复环是两条执行路径——本文件只做「到点触发」，触发走显式动作
// 同一漏斗（doStart/doStop/doRestart，desired-state 与 supervisor 事件随流）；
// scheduled stop 与手动停同信号（stopCh），恢复环不会把定时停误判为崩溃拉起；
// 探测窗口不馈入调度（两路互不喂状态）。

import (
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/tasks/scheduler"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// processScheduleEntry is one compiled schedule action.
type processScheduleEntry struct {
	action string
	cron   string
	spec   *scheduler.CronSpec
}

// compileProcessSchedules parses schedule declarations once at construction.
// Validate rejects bad entries at config load; compile skips defensively so a
// bad entry can never panic the loop.
func compileProcessSchedules(configs map[string]ManagedProcessConfig) map[string][]processScheduleEntry {
	out := make(map[string][]processScheduleEntry, len(configs))
	for name, cfg := range configs {
		if len(cfg.Schedule) == 0 {
			continue
		}
		entries := make([]processScheduleEntry, 0, len(cfg.Schedule))
		for _, sc := range cfg.Schedule {
			spec, err := scheduler.ParseCron(sc.Cron)
			if err != nil {
				continue
			}
			entries = append(entries, processScheduleEntry{action: sc.Action, cron: sc.Cron, spec: spec})
		}
		if len(entries) > 0 {
			out[name] = entries
		}
	}
	return out
}

// nextScheduleEntry returns the soonest upcoming fire across a process's
// schedule entries (zero time when none matches within the cron horizon).
func nextScheduleEntry(entries []processScheduleEntry, now time.Time) (time.Time, string) {
	var best time.Time
	action := ""
	for _, e := range entries {
		nx := e.spec.Next(now)
		if nx.IsZero() {
			continue
		}
		if best.IsZero() || nx.Before(best) {
			best, action = nx, e.action
		}
	}
	return best, action
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
	s.schedStop = make(chan struct{})
	go s.scheduleLoop()
}

// scheduleLoop scans on a fixed tick; per-minute slot dedupe makes each
// matching minute fire exactly once regardless of tick phase.
func (s *OpsServer) scheduleLoop() {
	ticker := time.NewTicker(s.schedTick)
	defer ticker.Stop()
	for {
		select {
		case <-s.schedStop:
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
	slot := strconv.FormatInt(scheduler.Slot(now).Unix(), 10)
	names := make([]string, 0, len(s.scheds))
	for name := range s.scheds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, entry := range s.scheds[name] {
			if !entry.spec.Matches(now) {
				continue
			}
			key := name + "\x00" + entry.action
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
		if entry.action == ScheduleActionStop {
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

	switch entry.action {
	case ScheduleActionStart:
		if _, err := s.doStartProcess(p, supervisorEventScheduleAction, "scheduled start ("+entry.cron+")"); err != nil {
			s.emitScheduleFireError(p, entry, err)
		}
	case ScheduleActionStop:
		s.doStopProcess(p, supervisorEventScheduleAction, "scheduled stop ("+entry.cron+")")
	case ScheduleActionRestart:
		if err := s.doRestartProcess(p, supervisorEventScheduleAction, "scheduled restart ("+entry.cron+")"); err != nil {
			s.emitScheduleFireError(p, entry, err)
		}
	}
}

// emitScheduleFireError records a failed scheduled action (including gate /
// already-running rejections) — the event log is the full-truth audit trail.
func (s *OpsServer) emitScheduleFireError(p *managedProcess, entry processScheduleEntry, err error) {
	ev := newSupervisorEvent(p.name, supervisorEventScheduleAction)
	ev.Message = "scheduled " + entry.action + " (" + entry.cron + ") failed: " + err.Error()
	s.emitSupervisorEvent(p, ev)
}
