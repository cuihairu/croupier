package runbook

// 审核发现 896（白盒）：每次终态持久化失败都创建无限重试的独立 goroutine，
// 持续错误下运行越多越累积任务并周期性冲击数据库。改为单一共享有界恢复循环：
// 无论多少待收敛运行只启动一个 goroutine，串行写库、指数退避封顶、同 run 去重。
// 本测试直接驱动 recoveryLoop，断言并发写库数恒为 1（单 worker 串行），且全部收敛。

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// countingStore 统计并发 UpdateRunStatus 调用（maxConcurrent），用于证明
// 恢复循环以单 worker 串行写库，而非每运行一个 goroutine 风暴。
type countingStore struct {
	mu            sync.Mutex
	maxConcurrent int32
	curConcurrent int32
	calls         int32
	fail          bool
	done          map[uint]bool
}

func (s *countingStore) UpdateRunStatus(_ context.Context, id uint, _ string, _ int, _, _ string) error {
	s.mu.Lock()
	s.curConcurrent++
	if s.curConcurrent > s.maxConcurrent {
		s.maxConcurrent = s.curConcurrent
	}
	s.calls++
	s.mu.Unlock()

	// 模拟慢写，扩大并发窗口以便捕捉并发。
	time.Sleep(20 * time.Millisecond)

	s.mu.Lock()
	s.curConcurrent--
	if s.fail {
		s.mu.Unlock()
		return errors.New("db down")
	}
	if s.done == nil {
		s.done = map[uint]bool{}
	}
	s.done[id] = true
	s.mu.Unlock()
	return nil
}

func (s *countingStore) concurrent() int32 { s.mu.Lock(); defer s.mu.Unlock(); return s.maxConcurrent }
func (s *countingStore) totalCalls() int32 { s.mu.Lock(); defer s.mu.Unlock(); return s.calls }
func (s *countingStore) finished(id uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done != nil && s.done[id]
}

// Store 接口其余方法：恢复循环只用到 UpdateRunStatus，其余置空实现以满足接口。
func (s *countingStore) Create(context.Context, *model.Runbook) error { return nil }
func (s *countingStore) Find(context.Context, string, string, string) (*model.Runbook, error) {
	return nil, errors.New("not implemented")
}
func (s *countingStore) List(context.Context, string, string, int) ([]model.Runbook, error) {
	return nil, nil
}
func (s *countingStore) Delete(context.Context, string, string, string) error { return nil }
func (s *countingStore) CreateRun(context.Context, *model.RunbookRun) error   { return nil }
func (s *countingStore) FindRun(context.Context, string, string, string) (*model.RunbookRun, error) {
	return nil, errors.New("not implemented")
}
func (s *countingStore) ListRuns(context.Context, string, string, string, int) ([]model.RunbookRun, error) {
	return nil, nil
}
func (s *countingStore) ListRunning(context.Context, time.Time) ([]*model.RunbookRun, error) {
	return nil, nil
}

func TestRecoveryLoop_SingleBoundedWorkerSerializesWrites(t *testing.T) {
	store := &countingStore{}
	loop := newRecoveryLoop(store)

	// 一次入队 20 个运行终态。
	for i := 1; i <= 20; i++ {
		loop.enqueue(&pendingTerminal{runID: uint(i), status: "succeeded"})
	}

	// 等待全部收敛。
	require.Eventually(t, func() bool {
		for i := 1; i <= 20; i++ {
			if !store.finished(uint(i)) {
				return false
			}
		}
		return true
	}, 3*time.Second, 20*time.Millisecond, "全部 20 个运行必须被后台收敛")

	// 关键断言：并发写库恒为 1（单 worker 串行），证明非 goroutine 风暴。
	require.Equal(t, int32(1), store.concurrent(), "恢复循环必须单 worker 串行写库")
}

func TestRecoveryLoop_RetriesAndBacksOff(t *testing.T) {
	store := &countingStore{fail: true}
	loop := newRecoveryLoop(store)
	loop.enqueue(&pendingTerminal{runID: 1, status: "succeeded"})

	// 持续错误下循环不能忙循环耗尽资源：退避封顶（30s），一段时间内不应
	// 产生海量调用。
	time.Sleep(1200 * time.Millisecond)
	calls := store.totalCalls()
	// 1s 起步退避：1.2s 内最多 ~2 次写（首次立即 + 退避 1s 后 1 次）。
	require.LessOrEqual(t, calls, int32(3), "持续错误下必须以指数退避重试，不得忙循环")

	// 恢复后收敛。
	store.mu.Lock()
	store.fail = false
	store.mu.Unlock()
	require.Eventually(t, func() bool { return store.finished(1) }, 35*time.Second, 100*time.Millisecond,
		"DB 恢复后后台必须收敛")
}

func TestRecoveryLoop_DedupesSameRun(t *testing.T) {
	store := &countingStore{}
	loop := newRecoveryLoop(store)
	loop.enqueue(&pendingTerminal{runID: 1, status: "succeeded"})
	loop.enqueue(&pendingTerminal{runID: 1, status: "failed"}) // 同 run 去重，取最新
	require.Eventually(t, func() bool { return store.finished(1) }, 3*time.Second, 20*time.Millisecond,
		"同 run 去重后仍收敛")
	// 两次入队同 run：最终只写一次成功（去重后单条）。
	require.LessOrEqual(t, store.totalCalls(), int32(2), "同 run 重复入队不得重复写库")
}
