package server

import (
	"context"
	"fmt"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/tasks"
)

// pruneMetricsOnce 执行一轮指标/缓存修剪（供循环调用与直接测试）。
func (s *ControlService) pruneMetricsOnce() {
	s.metricsStore.Prune(time.Hour)
	s.systemInfoCache.Prune(time.Hour)
}

// backgroundLoopInterval 是指标修剪/会话清理/摘除宽限清扫三个后台循环的
// 定时间隔；抽为包级变量仅为测试注入短间隔覆盖 ticker 分支，默认 5 分钟
// 与历史行为一致。
var backgroundLoopInterval = 5 * time.Minute

// watchdogIntervalMin/Max 是 D3 看门狗收割 tick 间隔的钳位边界；Min 抽为
// 包级变量仅为测试注入短间隔（生产默认 1s 起）。
var (
	watchdogIntervalMin = time.Second
	watchdogIntervalMax = 30 * time.Second
)

// watchdogIntervalFor 由无事件预算推导看门狗收割 tick 间隔：budget/2，
// 钳位到 [watchdogIntervalMin, watchdogIntervalMax]。短预算（测试）得短间隔。
func watchdogIntervalFor(budget time.Duration) time.Duration {
	if budget <= 0 {
		return watchdogIntervalMax
	}
	half := budget / 2
	if half < watchdogIntervalMin {
		half = watchdogIntervalMin
	}
	if half > watchdogIntervalMax {
		half = watchdogIntervalMax
	}
	return half
}

func (s *ControlService) pruneOldMetrics() {
	ticker := time.NewTicker(backgroundLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.pruneMetricsOnce()
		}
	}
}

// runSessionCleanup 执行一轮过期会话清理（供循环调用与直接测试）。
func (s *ControlService) runSessionCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deleted, err := s.agentSessionLoader.DeleteExpired(ctx)
	if err != nil {
		s.logger.Error("failed to delete expired sessions", "error", err)
	} else if deleted > 0 {
		s.logger.Info("deleted expired sessions from database", "count", deleted)
	}
}

func (s *ControlService) cleanupLoop() {
	ticker := time.NewTicker(backgroundLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runSessionCleanup()
		}
	}
}

// runContractRemovalSweep 执行一轮摘除宽限到期契约的清扫（供循环调用与直接测试）。
// 未接线契约服务（内存注册面）时 SweepExpiredContractRemovals 为 no-op。
func (s *ControlService) runContractRemovalSweep() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	finalized, err := s.registry.SweepExpiredContractRemovals(ctx)
	if err != nil {
		s.logger.Error("failed to sweep expired contract removals", "error", err, "finalized", finalized)
	} else if finalized > 0 {
		s.logger.Info("finalized expired contract removals", "count", finalized)
	}
}

func (s *ControlService) contractRemovalSweepLoop() {
	ticker := time.NewTicker(backgroundLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runContractRemovalSweep()
		}
	}
}

// runTaskWatchdog 执行一轮无事件任务收割（供循环调用与直接测试）。
// ARCH-018 D3 看门狗：活动行 updated_at 早于预算 → timed_out（终态，
// 含义是「无进一步证据」而非「游戏侧失败」）。
func (s *ControlService) runTaskWatchdog() {
	s.mu.RLock()
	reaper := s.taskReaper
	budget := s.watchdogBudget
	s.mu.RUnlock()
	if reaper == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reaped, err := reaper.ReapStale(ctx, budget, tasks.StatusTimedOut,
		"no event past watchdog budget; execution state unknown")
	if err != nil {
		s.logger.Error("failed to reap stale tasks", "error", err)
	} else if reaped > 0 {
		s.logger.Info("reaped stale tasks to timed_out", "count", reaped)
	}
}

func (s *ControlService) taskWatchdogLoop() {
	s.mu.RLock()
	interval := s.watchdogInterval
	s.mu.RUnlock()
	if interval <= 0 {
		interval = watchdogIntervalMax
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runTaskWatchdog()
		}
	}
}

// LoadAgentSessions loads active agent sessions from the database into memory.
// 多实例时按共享归属表活跃全集过滤——断连 agent 的残留快照行不再作为
// 僵尸复活（归属表无行 = 无任何实例持有连接）。
func (s *ControlService) LoadAgentSessions() error {
	if s.agentSessionLoader == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var keep func(*reg.AgentSession) bool
	s.mu.RLock()
	directory := s.activeAgentDirectory
	s.mu.RUnlock()
	if directory != nil {
		ids, err := directory.ActiveAgentIDs(ctx)
		if err != nil {
			// 查询失败降级全量恢复：启动路径不放大归属表故障。
			s.logger.Warn("active agent directory unavailable, restoring all snapshots", "error", err)
		} else {
			alive := make(map[string]bool, len(ids))
			for _, id := range ids {
				alive[id] = true
			}
			keep = func(sess *reg.AgentSession) bool { return alive[sess.AgentID] }
		}
	}
	if err := s.registry.LoadFromDBFiltered(ctx, s.agentSessionLoader, keep); err != nil {
		return fmt.Errorf("failed to load agent sessions: %w", err)
	}
	return nil
}

// ---- validation helpers ----
