package dispatch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- 熔断器状态机：锁内二次校验（recheck）守卫 ----

// runTransitionUnderContendedLock 复现「读状态与拿到锁之间状态被并发改掉」
// 的窗口：持锁后起 N 个 goroutine 调用 transition（每个都会先原子读到
// pin 的状态、随后阻塞在 state.mu 上），settle 后原子改写状态再放锁。
// 放锁后 goroutine 逐个拿到锁，recheck 读到 flip 后的状态 ≠ pin → 提前
// return（覆盖 recheck-fail 分支）。flip 的值与守卫体的写入值刻意错开，
// 断言最终状态 == flip 即可证明守卫体没有执行。
func runTransitionUnderContendedLock(t *testing.T, pin, flip CircuitBreakerState, prepare func(s *AgentHealthState), transition func(s *AgentHealthState)) CircuitBreakerState {
	t.Helper()

	state := NewAgentHealthState("agent-guard", "", nil)
	state.circuitState.Store(int32(pin))
	if prepare != nil {
		prepare(state)
	}

	state.mu.Lock()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transition(state)
		}()
	}
	// settle：等待 goroutine 完成入口的原子读并阻塞在锁上（裕量远大于
	// goroutine 启动延迟，非精确时序依赖）。
	time.Sleep(50 * time.Millisecond)
	// 改写动作 happens-before Unlock，recheck 必然读到 flip。
	state.circuitState.Store(int32(flip))
	state.mu.Unlock()

	wg.Wait()
	return state.CircuitState()
}

// TestAgentHealthTransitionOnSuccessRecheckGuards 覆盖 transitionOnSuccess
// 两处锁内 recheck-fail 分支（HalfOpen→Closed 与 Open→HalfOpen）。
func TestAgentHealthTransitionOnSuccessRecheckGuards(t *testing.T) {
	t.Run("half-open stale read returns without closing", func(t *testing.T) {
		final := runTransitionUnderContendedLock(t,
			CircuitHalfOpen, CircuitOpen,
			nil,
			func(s *AgentHealthState) { s.transitionOnSuccess() },
		)
		// 守卫体若执行会把状态写成 Closed；保持 Open 证明走的是 recheck-return。
		assert.Equal(t, CircuitOpen, final)
	})

	t.Run("open stale read returns without half-opening", func(t *testing.T) {
		final := runTransitionUnderContendedLock(t,
			CircuitOpen, CircuitClosed,
			// openedAt 置为 1 小时前：time.Since >= CircuitOpenTimeout 成立，
			// 才会进入加锁路径，recheck 才可达。
			func(s *AgentHealthState) { s.circuitOpenedAt.Store(time.Now().Add(-time.Hour).UnixNano()) },
			func(s *AgentHealthState) { s.transitionOnSuccess() },
		)
		// 守卫体若执行会把状态写成 HalfOpen；保持 Closed 证明走的是 recheck-return。
		assert.Equal(t, CircuitClosed, final)
	})
}

// TestAgentHealthTransitionOnFailureRecheckGuards 覆盖 transitionOnFailure
// 两处锁内 recheck-fail 分支（Closed→Open 与 HalfOpen→Open）。
func TestAgentHealthTransitionOnFailureRecheckGuards(t *testing.T) {
	t.Run("closed stale read returns without opening", func(t *testing.T) {
		final := runTransitionUnderContendedLock(t,
			CircuitClosed, CircuitHalfOpen,
			// 连续失败数达到阈值：否则根本不会进入加锁路径。
			func(s *AgentHealthState) { atomic.StoreInt32(&s.consecutiveFailures, s.config.FailureThreshold) },
			func(s *AgentHealthState) { s.transitionOnFailure() },
		)
		// 守卫体若执行会把状态写成 Open；保持 HalfOpen 证明走的是 recheck-return。
		assert.Equal(t, CircuitHalfOpen, final)
	})

	t.Run("half-open stale read returns without opening", func(t *testing.T) {
		final := runTransitionUnderContendedLock(t,
			CircuitHalfOpen, CircuitClosed,
			nil,
			func(s *AgentHealthState) { s.transitionOnFailure() },
		)
		// 守卫体若执行会把状态写成 Open；保持 Closed 证明走的是 recheck-return。
		assert.Equal(t, CircuitClosed, final)
	})
}

// ---- StartTaskRequest：failover 轮次耗尽 ----

// TestDispatcherStartTaskRequestFailoverLoopExhausted：3 个候选、本地无
// session、转发全部失败 → 3 轮尝试耗尽后走循环尾（返回 lastErr，而非
// 下一轮 pick 失败的 failover-exhausted 503 包装）。
func TestDispatcherStartTaskRequestFailoverLoopExhausted(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")
	registerTestAgent(t, d, "agent-2")
	registerTestAgent(t, d, "agent-3")

	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	_, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.ErrorIs(t, err, errAgentUnreachable)
	assert.Len(t, fwd.calls, 3)
	// 返回的是最后一轮的原始错误，而不是 failover exhausted 包装。
	assert.NotContains(t, err.Error(), "failover exhausted")
	assert.Contains(t, err.Error(), "no route")
}

// ---- ListFunctionAgents：空/nil AgentID 条目跳过 ----

// TestDispatcherListFunctionAgentsSkipsMalformedEntries：registry 里混入
// 空 AgentID 与 nil 条目（防御式守卫）时不得 panic、不得产出空 ID。
func TestDispatcherListFunctionAgentsSkipsMalformedEntries(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-live")

	d.store.Mu().Lock()
	agents := d.store.AgentsUnsafe()
	agents[""] = &reg.AgentSession{
		AgentID:  "",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	}
	agents["ghost"] = nil
	d.store.Mu().Unlock()

	ids := d.ListFunctionAgents("test-func")
	require.Len(t, ids, 1)
	assert.Equal(t, "agent-live", ids[0])
}

// ---- callAgentRoutedCancel：转发失败 ----

// TestDispatcherCancelTaskForwardErrorSurfacesUnreachable：本地无 session、
// 转发 owner 也失败 → 取消错误带 forward 上下文与 errAgentUnreachable，
// 且本地 task routing 不被清理（清理只发生在取消成功后）。
func TestDispatcherCancelTaskForwardErrorSurfacesUnreachable(t *testing.T) {
	d := NewDispatcher(nil)
	d.RegisterTask("task-fwd-err", "agent-1")

	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	err := d.CancelTask(context.Background(), "task-fwd-err")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forward to owner of agent-1")
	assert.ErrorIs(t, err, errAgentUnreachable)
	assert.Len(t, fwd.calls, 1)

	agentID, ok := d.TaskAgentID("task-fwd-err")
	require.True(t, ok, "routing must survive a failed cancel")
	assert.Equal(t, "agent-1", agentID)
}

// ---- StartTaskOnAgent / CancelTaskOnAgent：定向投递路径 ----

func TestDispatcherStartTaskOnAgentValidationAndMarshal(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.StartTaskOnAgent(context.Background(), "agent-1", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent id is required")
	})

	t.Run("blank agent id", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.StartTaskOnAgent(context.Background(), "   ", &sdkv1.InvokeRequest{FunctionId: "test-func"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent id is required")
	})

	t.Run("nil metadata initialized and agentId injected before call error", func(t *testing.T) {
		d := NewDispatcher(nil)
		req := &sdkv1.InvokeRequest{FunctionId: "test-func"}
		_, err := d.StartTaskOnAgent(context.Background(), "agent-direct", req)
		require.Error(t, err)
		require.NotNil(t, req.Metadata, "metadata must be lazily initialized")
		assert.Equal(t, "agent-direct", req.Metadata["agentId"])
	})

	t.Run("marshal error", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.StartTaskOnAgent(context.Background(), "agent-direct", &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"bad": "\xff\xfe"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request")
	})
}

func TestDispatcherStartTaskOnAgentHAHealthTracking(t *testing.T) {
	t.Run("call error records failure", func(t *testing.T) {
		d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
		defer d.Close()
		d.GetHealthTracker().RegisterAgent("agent-direct", "")
		// 无 session resolver：callAgent 必失败。
		_, err := d.StartTaskOnAgent(context.Background(), "agent-direct", &sdkv1.InvokeRequest{FunctionId: "test-func"})
		require.Error(t, err)

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-direct")
		require.NoError(t, statErr)
		assert.Greater(t, stats.FailedRequests, int64(0))
		assert.Equal(t, int32(0), stats.ActiveConnections, "defer must decrement the connection count")
	})

	t.Run("success records success and returns raw bytes", func(t *testing.T) {
		d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
		defer d.Close()
		d.GetHealthTracker().RegisterAgent("agent-direct", "")

		respBody := mustMarshal(t, &sdkv1.StartTaskResponse{TaskId: "task-direct"})
		caller := &fakeSessionCaller{respBody: respBody}
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
			"agent-direct": caller,
		}})

		got, err := d.StartTaskOnAgent(context.Background(), "agent-direct", &sdkv1.InvokeRequest{FunctionId: "test-func"})
		require.NoError(t, err)
		assert.Equal(t, respBody, got)
		assert.Equal(t, 1, caller.callCount())

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-direct")
		require.NoError(t, statErr)
		assert.Greater(t, stats.SuccessfulRequests, int64(0))
		assert.Equal(t, int32(0), stats.ActiveConnections)
	})
}

func TestDispatcherCancelTaskOnAgentPaths(t *testing.T) {
	t.Run("blank agent id", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.CancelTaskOnAgent(context.Background(), "", "task-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent id and task id are required")
	})

	t.Run("blank task id", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.CancelTaskOnAgent(context.Background(), "  ", "task-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent id and task id are required")
	})

	t.Run("marshal error from invalid task id utf8", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.CancelTaskOnAgent(context.Background(), "agent-1", "\xff")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request")
	})

	t.Run("call error surfaces", func(t *testing.T) {
		d := NewDispatcher(nil)
		// 无 session resolver：callAgent 必失败。
		_, err := d.CancelTaskOnAgent(context.Background(), "agent-1", "task-1")
		require.Error(t, err)
	})

	t.Run("success returns raw response bytes", func(t *testing.T) {
		d := NewDispatcher(nil)
		caller := &fakeSessionCaller{respBody: []byte{0x01, 0x02}}
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
			"agent-1": caller,
		}})

		got, err := d.CancelTaskOnAgent(context.Background(), "agent-1", "task-1")
		require.NoError(t, err)
		assert.Equal(t, []byte{0x01, 0x02}, got)
		assert.Equal(t, 1, caller.callCount())
	})
}

// ---- TaskRunWriterAdapter.MarkRunFailed ----

// TestTaskRunWriterAdapterMarkRunFailed：nil 模型短路 + 真库路径把行标记
// failed（status/error_message/finished_at 三列）。
func TestTaskRunWriterAdapterMarkRunFailed(t *testing.T) {
	t.Parallel()

	t.Run("nil runs model is a no-op", func(t *testing.T) {
		w := NewTaskRunWriterAdapter(nil)
		require.NoError(t, w.MarkRunFailed(context.Background(), "task-x", "boom"))
	})

	t.Run("marks row failed with error and finish time", func(t *testing.T) {
		db := openDispatchTestDB(t)
		runs := model.NewTaskRunModel(db)
		require.NoError(t, runs.Create(context.Background(), &model.TaskRun{
			TaskID:     "task-mark",
			FunctionID: "test-func",
			AgentID:    "agent-1",
			Status:     "dispatching",
		}))

		w := NewTaskRunWriterAdapter(runs)
		require.NoError(t, w.MarkRunFailed(context.Background(), "task-mark", "agent unreachable"))

		run, err := runs.FindByTaskID(context.Background(), "task-mark")
		require.NoError(t, err)
		require.NotNil(t, run)
		assert.Equal(t, "failed", run.Status)
		assert.Equal(t, "agent unreachable", run.ErrorMessage)
		require.NotNil(t, run.FinishedAt, "finished_at must be stamped")
	})
}
