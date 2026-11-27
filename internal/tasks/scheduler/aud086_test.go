// AUD-086：派发确定失败必须计入连续失败计数（dead_letter 阈值生效）。
//
// 缺陷形态：triggerSchedule 的派发错误分支只回填 run-log failed，不推进
// failures，且把 last_run_id 清空——下一轮触发的终态回查（LastRunID 为空）
// 不执行，consecutive_failures 永远停在旧值。阈值 2 的实测：派发 3 次、
// 计数恒 0、永不 dead_letter。
//
// 修复形态：err 非 nil（无 runID——结果已知、非未知）时 failures++；阈值
// 判定在下一到期触发时停止派发并 dead_letter；确定成功（上次 run 终态
// succeeded）的清零与未知结果（indeterminate / 终态未落）不计失败的既有
// 语义均不变。
package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud086Store 在 fakeStore 之上补齐真实 store 的推进回写：next_triggered_at
// 与 last_run_id 应用到内存计划。fakeStore 刻意不回写这两键（AUD-028 依赖
// 同一份到期快照），而本文件的多轮 tick 失败计数链路需要它——否则每轮 tick
// 领取的都是同一槽位，第二轮撞唯一冲突走不到派发分支。
type aud086Store struct{ *fakeStore }

func (a *aud086Store) UpdateSchedule(ctx context.Context, id uint, updates map[string]interface{}) error {
	if err := a.fakeStore.UpdateSchedule(ctx, id, updates); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.schedules {
		if a.schedules[i].ID != id {
			continue
		}
		if v, ok := updates["next_triggered_at"].(time.Time); ok {
			a.schedules[i].NextTriggeredAt = &v
		}
		if v, ok := updates["last_run_id"].(string); ok {
			a.schedules[i].LastRunID = v
		}
	}
	return nil
}

// 阈值 2、每分钟一次：连续两次派发确定失败后，第三次到期必须停派发并
// dead_letter；dead_letter 后不再被扫描——至多派发 2 次（而非缺陷下的 3 次）。
func TestManager_AUD086_DispatchFailureCountsTowardDeadLetter(t *testing.T) {
	base := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := &aud086Store{fakeStore: newFakeStore()}
	store.schedules = []model.TaskSchedule{dueSchedule(1, "* * * * *", "", 0, 2, Slot(base))}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return nil, fmt.Errorf("agent unreachable")
	})
	m := NewManager(store, d)

	// 第 1 轮：派发确定失败 → 计数推进到 1（缺陷下恒为旧值 0）。
	m.tick(base)
	require.Equal(t, int32(1), atomic.LoadInt32(&dispatches))
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"],
		"第一次派发确定失败必须推进连续失败计数")
	assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)

	// 第 2 轮：再失败 → 计数 2（阈值判定在派发前完成，本次越限下一轮生效）。
	m.tick(base.Add(time.Minute))
	require.Equal(t, int32(2), atomic.LoadInt32(&dispatches))
	updates = store.updates[1]
	assert.Equal(t, 2, updates[len(updates)-1]["consecutive_failures"])
	assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)

	// 第 3 轮：阈值已到——不再派发，直接 dead_letter。
	m.tick(base.Add(2 * time.Minute))
	assert.Equal(t, int32(2), atomic.LoadInt32(&dispatches),
		"AUD-086: 阈值 2 必须至多派发 2 次（缺陷下错误分支不计数，观察到 3 次派发）")
	assert.Equal(t, model.ScheduleStatusDeadLetter, store.schedules[0].Status)
	logged, err := store.HasRunLog(context.Background(), 1, Slot(base.Add(2*time.Minute)))
	require.NoError(t, err)
	require.True(t, logged)
	skipped := store.runLogs[fmt.Sprintf("1|%s", Slot(base.Add(2*time.Minute)).Format(time.RFC3339))]
	assert.Equal(t, model.RunLogSkipped, skipped.Status, "越限槽位记 skipped（既有 dead_letter 语义）")
	assert.Contains(t, skipped.Message, "dead_letter")

	// 第 4 轮：dead_letter 后不再被 ListDue 扫到——持续停派发。
	m.tick(base.Add(3 * time.Minute))
	assert.Equal(t, int32(2), atomic.LoadInt32(&dispatches), "dead_letter 后不得再派发")

	// 失败槽位均回填确定失败（failed，非 indeterminate——结果已知）。
	for i := 0; i <= 1; i++ {
		rec := store.runLogs[fmt.Sprintf("1|%s", Slot(base.Add(time.Duration(i)*time.Minute)).Format(time.RFC3339))]
		require.NotNil(t, rec)
		assert.Equal(t, model.RunLogFailed, rec.Status)
		assert.Contains(t, rec.Message, "agent unreachable")
	}
}

// 派发确定失败推进计数后，确定成功（上次 run 终态 succeeded）清零计数。
// 派发被接受（run 无终态）≠ 触发成功，不清零——既有终态回查语义不变。
func TestManager_AUD086_DefiniteSuccessResetsFailureCount(t *testing.T) {
	base := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := &aud086Store{fakeStore: newFakeStore()}
	store.schedules = []model.TaskSchedule{dueSchedule(1, "* * * * *", "", 0, 2, Slot(base))}

	var dispatches int32
	fail := true
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		if fail {
			return nil, fmt.Errorf("agent unreachable")
		}
		return &sdkv1.StartTaskResponse{TaskId: "run-ok"}, nil
	})
	m := NewManager(store, d)

	// 第 1 轮：派发确定失败 → 计数 1。
	m.tick(base)
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"])

	// 第 2 轮：派发成功（run-ok），run 尚无终态 → 计数保持 1，不清零。
	fail = false
	m.tick(base.Add(time.Minute))
	updates = store.updates[1]
	assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"],
		"派发被接受但 run 无终态，不得清零计数")
	require.Equal(t, "run-ok", store.schedules[0].LastRunID, "成功派发须回写 last_run_id（清零回查的依据）")

	// 第 3 轮：上一 run 终态 succeeded（确定成功）→ 计数清零。
	m.tick(base.Add(2 * time.Minute))
	updates = store.updates[1]
	assert.Equal(t, 0, updates[len(updates)-1]["consecutive_failures"],
		"确定成功（上次 run 终态 succeeded）必须清零连续失败计数")
	assert.Equal(t, int32(3), atomic.LoadInt32(&dispatches))
	assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)
}

// 未知派发结果的既有处置语义不变：不计失败、也不清零。
func TestManager_AUD086_UnknownOutcomeNotCountedAsFailure(t *testing.T) {
	t.Run("non-terminal last run neither counts nor resets", func(t *testing.T) {
		now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
		store := newFakeStore()
		// 上一 run 仍在运行（LastRunStatus 返回非终态 ""），计数 1。
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-pending", 1, 2, Slot(now))}
		NewManager(store, &fakeDispatcher{}).tick(now)

		updates := store.updates[1]
		require.NotEmpty(t, updates)
		assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"],
			"终态未知（运行中）不得计为失败，也不得清零")
		assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)
	})

	t.Run("stale claim indeterminate keeps count unchanged", func(t *testing.T) {
		now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
		lease := 60 * time.Second
		store := newFakeStore()
		// 计数 1（阈值 2）：上一轮派发失败已计入；当前槽残留崩溃领取者的
		// claimed——恢复为 indeterminate（派发结果未知），不得计为失败。
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 1, 2, Slot(now))}
		store.seedRunLog(model.TaskScheduleRunLog{
			ScheduleID: 1, Slot: Slot(now), Status: model.RunLogClaimed,
			CreatedAt: now.Add(-2 * lease),
		})

		var dispatches int32
		d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
			atomic.AddInt32(&dispatches, 1)
			return &sdkv1.StartTaskResponse{TaskId: "run-x"}, nil
		})
		m := NewManager(store, d)
		m.SetClaimLease(lease)
		m.tick(now)

		// 陈旧领取解析为 indeterminate，不重派发（既有 ARCH-023 语义）。
		assert.Equal(t, int32(0), atomic.LoadInt32(&dispatches))
		rec := store.runLogs[fmt.Sprintf("1|%s", Slot(now).Format(time.RFC3339))]
		assert.Equal(t, model.RunLogIndeterminate, rec.Status)
		// 未知结果不推进计数（保持 1），也不因此 dead_letter。
		assert.Equal(t, 1, store.schedules[0].ConsecutiveFailures,
			"indeterminate（派发结果未知）不得计为失败")
		assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)
	})
}
