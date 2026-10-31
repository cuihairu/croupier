package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── ARCH-023：定时触发先领取再派发 ────────────────────────────────────
// 旧顺序（HasRunLog 判重 → StartTask → 补写 CreateRunLog 且忽略冲突）在双
// Manager 下双方都通过判重、双双派发、仅 1 条唯一记录。修复后领取
// （CreateRunLog 唯一索引）在派发之前，唯一冲突即跳过派发。

// 领取必须发生在派发之前：dispatcher 在 StartTask 内回查 run-log 已存在。
func TestManager_ClaimsSlotBeforeDispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}

	var claimVisible int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		if logged, _ := store.HasRunLog(ctx, 1, Slot(now)); logged {
			atomic.AddInt32(&claimVisible, 1)
		}
		return &sdkv1.StartTaskResponse{TaskId: "run-ok"}, nil
	})
	NewManager(store, d).tick(now)

	assert.Equal(t, int32(1), atomic.LoadInt32(&claimVisible),
		"ARCH-023: run-log 必须在派发前领取（claim），而非派发后补写")
	// 回填终值：dispatched + TaskRunID。
	for _, l := range store.runLogs {
		assert.Equal(t, "dispatched", l.Status)
		assert.Equal(t, "run-ok", l.TaskRunID)
	}
}

// 派发失败也回填终值：领取后 StartTask 失败 → status=failed（不留 claimed）。
func TestManager_DispatchFailureFinalizesClaim(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{err: assert.AnError}
	NewManager(store, d).tick(now)

	for _, l := range store.runLogs {
		assert.Equal(t, "failed", l.Status)
		assert.NotEmpty(t, l.Message)
	}
}

// 双实例并发触发同一触发槽：恰好一次派发，其余全部因领取冲突被跳过。
func TestManager_ConcurrentTriggerExactlyOneDispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-ok"}, nil
	})
	m := NewManager(store, d)
	ctx := context.Background()
	sched := store.schedules[0]

	const instances = 4
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	for i := 0; i < instances; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start.Wait()
			m.triggerSchedule(ctx, &sched, now)
		}()
	}
	start.Done()
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&dispatches),
		"ARCH-023: 并发触发同一槽位必须恰好派发一次（领取令牌唯一索引兜底）")
	// 唯一一条领取记录被回填为 dispatched（不是残留 claimed）。
	logged, err := store.HasRunLog(context.Background(), 1, Slot(now))
	require.NoError(t, err)
	assert.True(t, logged)
	for _, l := range store.runLogs {
		assert.Equal(t, "dispatched", l.Status)
	}
}

// ── ARCH-023 崩溃恢复：领取租约 ────────────────────────────────────────

// 领取后/派发前中断：领取者崩溃，槽位残留 claimed。租约过期后被下一实例
// 解析为 indeterminate（不重派发——无法区分是否已派发，副作用去重归
// GAME-010），调度推进、审计链不再永久停在 claimed。
func TestManager_CrashAfterClaimBeforeDispatch_RecoversWithoutRedispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	lease := 60 * time.Second
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	// 模拟崩溃领取者：claimed 且已过租约（CreatedAt = now - 2×lease）。
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] = model.TaskScheduleRunLog{
		ScheduleID: 1, Slot: Slot(now), Status: model.RunLogClaimed,
		CreatedAt: now.Add(-2 * lease),
	}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-again"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(lease)
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches),
		"ARCH-023: 陈旧领取恢复不重派发（重发=对派发后中断赌二次执行）")
	require.Len(t, store.runLogs, 1)
	rec := store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))]
	assert.Equal(t, model.RunLogIndeterminate, rec.Status,
		"租约过期的 claimed 必须解析为 indeterminate（派发结果未知，不得臆断失败）")
	assert.Contains(t, rec.Message, "lease expired")
	assert.Empty(t, rec.TaskRunID, "恢复不得臆造 TaskRunID（回写未落，结果未知）")
	// 调度推进：next_triggered_at 被更新（下一槽照常触发）。
	assert.NotEmpty(t, store.updates[1], "崩溃恢复必须推进调度，不能卡死")
}

// 派发后/回写前中断：任务可能已派发（真正运行中）但回写未落，库面同样
// 残留 claimed。恢复动作与上一类一致：终态 indeterminate + 推进 +
// 绝不二次派发（R30-04：两类中断无法区分，indeterminate 是唯一诚实终态）。
func TestManager_CrashAfterDispatchBeforeWriteback_MarksIndeterminateNoRedispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	lease := 60 * time.Second
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] = model.TaskScheduleRunLog{
		ScheduleID: 1, Slot: Slot(now), Status: model.RunLogClaimed,
		CreatedAt: now.Add(-10 * lease),
	}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-again"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(lease)
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches),
		"ARCH-023: 回写前中断的恢复不得重复派发（TaskRun 可能仍在运行）")
	rec := store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))]
	assert.Equal(t, model.RunLogIndeterminate, rec.Status)
	assert.Empty(t, rec.TaskRunID, "回写未落，恢复不得臆造 TaskRunID")
}

// 活跃领取不误判：他实例刚领取（租约内）正在派发，观察者不得把其标记为
// failed——否则活领取被误杀、审计链被污染。
func TestManager_ActiveClaimWithinLease_NotEvicted(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	lease := 60 * time.Second
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] = model.TaskScheduleRunLog{
		ScheduleID: 1, Slot: Slot(now), Status: "claimed",
		CreatedAt: now.Add(-lease / 2),
	}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-x"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(lease)
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches))
	rec := store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))]
	assert.Equal(t, "claimed", rec.Status, "租约内的活跃领取不得被观察者改写")
}

// R30-04 跨槽恢复缺口：领取者崩溃后遗留的旧槽位不在当前触发槽（triggerSchedule
// 的 resolveStaleClaim 只读 Slot(now)），此前从未被扫描——run-log 永久停在
// claimed。修复后 tick 的统一扫描（reapStaleClaims）按租约窗口列出全部陈旧
// 领取并解析为 indeterminate；当前槽照常派发一次，旧槽绝不重派发。
func TestManager_CrossSlotStaleClaim_ReapedIndeterminate(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	lease := 60 * time.Second
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	// 旧槽位（08:30 ≠ 当前槽 10:30）的崩溃领取者遗留：claimed 且过租约。
	oldSlot := Slot(now.Add(-2 * time.Hour))
	require.NotEqual(t, Slot(now), oldSlot, "旧槽必须与当前触发槽不同，才模拟跨槽缺口")
	store.runLogs[fmt.Sprintf("1|%s", oldSlot.Format(time.RFC3339))] = model.TaskScheduleRunLog{
		ScheduleID: 1, Slot: oldSlot, Status: model.RunLogClaimed,
		CreatedAt: now.Add(-2 * lease),
	}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-now"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(lease)
	m.tick(now)

	// 当前槽照常恰好派发一次。
	assert.Equal(t, int32(1), atomic.LoadInt32(&dispatches),
		"当前槽必须照常派发（旧槽遗留不阻塞新触发）")
	// 旧槽遗留被跨槽扫描解析为 indeterminate，且不重派发（dispatches 仍为 1）。
	oldRec := store.runLogs[fmt.Sprintf("1|%s", oldSlot.Format(time.RFC3339))]
	assert.Equal(t, model.RunLogIndeterminate, oldRec.Status,
		"跨槽陈旧领取必须解析为 indeterminate，run-log 不得永久停在 claimed")
	assert.Empty(t, oldRec.TaskRunID, "旧槽恢复不得臆造 TaskRunID")
	// 当前槽回填 dispatched。
	curRec := store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))]
	assert.Equal(t, model.RunLogDispatched, curRec.Status)
	assert.Equal(t, "run-now", curRec.TaskRunID)
}

// 跨槽扫描独立于到期循环：计划尚未到期（next_triggered_at 在未来）时，
// reapStaleClaims 仍按租约窗口列出并解析其陈旧领取——不得因 ListDue 为空
// 而漏扫（R30-04 缺口本质是「只有到期路径才看得到 claimed」）。
func TestManager_ReapStaleClaims_NotDueSchedule(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	lease := 60 * time.Second
	store := newFakeStore()
	// 计划 11:30 才到期（now=10:30 时不触发）。
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(time.Hour))}
	oldSlot := Slot(now.Add(-2 * time.Hour))
	store.runLogs[fmt.Sprintf("1|%s", oldSlot.Format(time.RFC3339))] = model.TaskScheduleRunLog{
		ScheduleID: 1, Slot: oldSlot, Status: model.RunLogClaimed,
		CreatedAt: now.Add(-2 * lease),
	}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-x"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(lease)
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches), "未到期计划不得派发")
	rec := store.runLogs[fmt.Sprintf("1|%s", oldSlot.Format(time.RFC3339))]
	assert.Equal(t, model.RunLogIndeterminate, rec.Status,
		"未到期计划的陈旧领取同样被跨槽扫描解析")
}

// 已终态记录不解析：回写完成但推进崩溃的槽位（dispatched + next_triggered_at
// 未推进），下一实例只推进、不改写终值、不重派发。
func TestManager_TerminalRunLog_NotRewritten(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	lease := 60 * time.Second
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] = model.TaskScheduleRunLog{
		ScheduleID: 1, Slot: Slot(now), Status: "dispatched", TaskRunID: "run-done",
		CreatedAt: now.Add(-10 * lease),
	}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-again"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(lease)
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches))
	rec := store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))]
	assert.Equal(t, "dispatched", rec.Status, "终态记录不被租约解析改写")
	assert.Equal(t, "run-done", rec.TaskRunID)
}
