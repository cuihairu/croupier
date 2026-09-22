package server

// 覆盖目标：pruneOldMetrics/cleanupLoop 的 ticker.C 调度分支（此前仅覆盖
// ctx.Done 退出分支）。手法：backgroundLoopInterval 是包级变量，测试注入
// 5ms 短间隔；cleanupLoop 用 mock loader 的 DeleteExpired 原子计数观察触发；
// pruneOldMetrics 用挂 sqlite 的 MetricsStore 观察一条 2 小时前的
// AgentMetricsHistory 行被 Prune(time.Hour) 的 DB 分支删除。
// 轮询窗口 2s 远大于 5ms 间隔，不构成时序敏感断言。

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func withShortLoopInterval(t *testing.T) {
	t.Helper()
	orig := backgroundLoopInterval
	backgroundLoopInterval = 5 * time.Millisecond
	t.Cleanup(func() { backgroundLoopInterval = orig })
}

func waitLoopCond(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("loop tick condition not met within 2s")
}

// cleanupLoop：ticker 触发后 runSessionCleanup 调 DeleteExpired。
func TestControlService_CleanupLoop_TickFires(t *testing.T) {
	withShortLoopInterval(t)
	loader := &mockAgentSessionLoader{}
	svc := newTestControlServiceWithLoader(loader)

	done := make(chan struct{})
	go func() {
		svc.cleanupLoop()
		close(done)
	}()

	waitLoopCond(t, func() bool { return loader.deleteCalled >= 1 })

	svc.cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupLoop did not exit after cancel")
	}
}

// pruneOldMetrics：ticker 触发后 pruneMetricsOnce → MetricsStore.Prune(1h)
// 删除 sqlite 中 2 小时前的历史行。
func TestControlService_PruneOldMetrics_TickFires(t *testing.T) {
	withShortLoopInterval(t)
	svc := newTestControlService()

	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentMetricsHistory{}))
	require.NoError(t, db.Create(&registry.AgentMetricsHistory{
		AgentID:   "agent-loop",
		Timestamp: time.Now().Add(-2 * time.Hour),
	}).Error)
	svc.MetricsStore().SetDB(db)

	done := make(chan struct{})
	go func() {
		svc.pruneOldMetrics()
		close(done)
	}()

	waitLoopCond(t, func() bool {
		var count int64
		if err := db.Model(&registry.AgentMetricsHistory{}).
			Where("agent_id = ?", "agent-loop").Count(&count).Error; err != nil {
			return false
		}
		return count == 0
	})

	svc.cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pruneOldMetrics did not exit after cancel")
	}
}

// contractRemovalSweepLoop：ticker 触发后 runContractRemovalSweep →
// registry.SweepExpiredContractRemovals 转发到契约服务的清扫入口。
// sweepContractService 是 registry.Store 契约物化接口的桩，原子计数
// 观察触发（循环 goroutine 并发读写，与 mockAgentSessionLoader 同手法）。
type sweepContractService struct {
	called int32
}

func (s *sweepContractService) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error {
	return nil
}
func (s *sweepContractService) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	return "", nil
}
func (s *sweepContractService) MarkContractRemovalPending(ctx context.Context, gameID, env, functionID string) error {
	return nil
}
func (s *sweepContractService) FinalizeExpiredContractRemovals(ctx context.Context, grace time.Duration) (int, error) {
	atomic.AddInt32(&s.called, 1)
	return 0, nil
}
func (s *sweepContractService) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	return nil
}
func (s *sweepContractService) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	return nil
}
func (s *sweepContractService) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	return nil
}
func (s *sweepContractService) RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	return nil
}

func TestControlService_ContractRemovalSweepLoop_TickFires(t *testing.T) {
	withShortLoopInterval(t)
	svc := newTestControlService()
	fake := &sweepContractService{}
	svc.registry.SetContractService(fake)

	done := make(chan struct{})
	go func() {
		svc.contractRemovalSweepLoop()
		close(done)
	}()

	waitLoopCond(t, func() bool { return atomic.LoadInt32(&fake.called) >= 1 })

	svc.cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("contractRemovalSweepLoop did not exit after cancel")
	}
}
