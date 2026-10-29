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
// 领取方在派发完成/回写前崩溃时，领取行永远停在 status='claimed'。没有
// 租约恢复时：同槽重触发被唯一索引挡住 → 只推进不派发（触发丢失，直到
// 下一个 cron 周期），且历史行永远显示 claimed。租约语义：claimed 且
// created_at 早于租约窗口 = 孤儿领取（领取方已崩溃）→ 原子重领 + 重派发；
// 否则 = 活领取（另一实例持有）→ 跳过。

func staleClaimedLog(scheduleID uint, slot, now time.Time, age time.Duration) model.TaskScheduleRunLog {
	return model.TaskScheduleRunLog{
		ScheduleID: scheduleID, Slot: slot, Status: "claimed",
		CreatedAt: now.Add(-age),
	}
}

// 窗口一（领取后/派发前中断）：孤儿领取被重领并重新派发——触发不静默丢失。
func TestManager_StaleClaimRecoversTrigger(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] =
		staleClaimedLog(1, Slot(now), now, 2*time.Minute)

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-recovered"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(time.Minute) // 行已过期 2min > 租约 1min → 可重领
	m.tick(now)

	assert.Equal(t, int32(1), atomic.LoadInt32(&dispatches),
		"ARCH-023: 孤儿领取（租约过期）必须重领并重派发，而不是静默跳过")
	logs := store.runLogs
	require.Len(t, logs, 1)
	for _, l := range logs {
		assert.Equal(t, "dispatched", l.Status, "恢复派发后领取行落为终值")
		assert.Equal(t, "run-recovered", l.TaskRunID)
	}
}

// 窗口二（派发后/回写前中断）：领取行仍 claimed 但上一次派发可能已发出
// TaskRun。恢复无法区分两种窗口（领取行没有 taskID 可查），因此按重派发
// 处理——触发投递 at-least-once，业务副作用去重由 GAME-010 的游戏幂等承担。
// 本用例固化该决策：孤儿领取重派发（而非跳过），领取行回填新 TaskRunID。
func TestManager_Window2RedispatchIsAtLeastOnce(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	// 上一次派发可能已创建 TaskRun「run-orph」，但回写崩溃——领取行无 taskID。
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] =
		staleClaimedLog(1, Slot(now), now, 2*time.Minute)

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-second"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(time.Minute)
	m.tick(now)

	assert.Equal(t, int32(1), atomic.LoadInt32(&dispatches),
		"ARCH-023 窗口二边界：孤儿领取按 at-least-once 重派发（游戏侧去重）")
	for _, l := range store.runLogs {
		assert.Equal(t, "run-second", l.TaskRunID, "重派发结果回填领取行（旧孤儿 TaskRun 留在 task_runs，由 GAME-010 幂等去重）")
	}
}

// 活领取（租约未过）：另一实例正常持有中 → 不重领、不派发、不误伤。
func TestManager_FreshClaimNotRecovered(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] =
		staleClaimedLog(1, Slot(now), now, 30*time.Second)

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-x"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(time.Minute) // 行仅 30s < 租约 1min → 活领取
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches),
		"租约未过的领取行属于活实例，不得被重领（否则双派发）")
	for _, l := range store.runLogs {
		assert.Equal(t, "claimed", l.Status)
	}
}

// 已终态的领取行（dispatched/failed/skipped）永不重领——正常完成的槽位
// 不会被恢复逻辑二次派发。
func TestManager_TerminalClaimNeverRecovered(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	key := fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))
	term := staleClaimedLog(1, Slot(now), now, 2*time.Minute)
	term.Status = "dispatched"
	term.TaskRunID = "run-done"
	store.runLogs[key] = term

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-x"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(time.Minute)
	m.tick(now)

	assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches), "终态行不重领")
	assert.Equal(t, "run-done", store.runLogs[key].TaskRunID, "终态行保持原样")
}

// 并发恢复者（两个实例同时发现孤儿领取）：CAS 保证恰好一个重领并派发。
func TestManager_ConcurrentReclaimExactlyOneRedispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))] =
		staleClaimedLog(1, Slot(now), now, 2*time.Minute)

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-ok"}, nil
	})
	m := NewManager(store, d)
	m.SetClaimLease(time.Minute)

	const instances = 4
	ctx := context.Background()
	sched := store.schedules[0]
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
		"并发恢复者必须恰好一个命中（CAS 重领的原子性），其余跳过")
}
