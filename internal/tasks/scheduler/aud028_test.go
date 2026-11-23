package scheduler

// AUD-028 回归：定时任务按计划发生时间领取而不是按当前分钟。
//
// 缺陷形态：triggerSchedule 用 Slot(now)（处理时刻）生成领取键。两个实例读到
// 同一条到期记录（相同的 next_triggered_at），即使跨分钟继续处理，也算出不同
// 的槽位（:59 → 10:30、:00 → 10:31），(schedule_id, slot) 唯一索引无法仲裁，
// 双双领取、双双派发。
//
// 修复形态：领取槽 = Slot(NextTriggeredAt)（计划发生时间），跨分钟处理仍领取
// 同一槽位，唯一索引把第二个实例的领取判为冲突——只派发一次。

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

// TestManager_CrossMinuteClaimOnce 覆盖 AUD-028 的核心场景：两个实例已读取同一条
// 到期记录（fakeStore 不回写 next_triggered_at，故第二轮 tick 读到的是同一份
// 到期快照），即使在 10:30:59 与 10:31:00 两个不同分钟处理，也只能派发一次。
func TestManager_CrossMinuteClaimOnce(t *testing.T) {
	occurrence := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	// 计划发生时间正是 10:30 这一槽（生产上 next_triggered_at 即槽对齐的发生时刻）。
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, occurrence)}

	var dispatches int32
	d := dispatcherFunc(func(context.Context, *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&dispatches, 1)
		return &sdkv1.StartTaskResponse{TaskId: "run-aud028"}, nil
	})
	m := NewManager(store, d)
	ctx := context.Background()

	// 实例 A：10:30:59 处理本轮到期。
	m.tick(occurrence.Add(59 * time.Second))
	require.Equal(t, int32(1), atomic.LoadInt32(&dispatches), "第一分钟内必须派发一次")

	// 实例 B：10:31:00（跨分钟）处理同一份到期记录。
	m.tick(occurrence.Add(time.Minute))
	require.Equal(t, int32(1), atomic.LoadInt32(&dispatches),
		"AUD-028: 跨分钟处理同一到期记录必须只派发一次（按当前分钟领取会导致双派发）")

	// 两轮领取的是同一槽位（计划发生时间 10:30）——只有一条领取记录。
	key := fmt.Sprintf("1|%s", Slot(occurrence).Format(time.RFC3339))
	require.Len(t, store.runLogs, 1, "跨分钟处理不得产生第二条领取记录")
	rec := store.runLogs[key]
	assert.Equal(t, model.RunLogDispatched, rec.Status)
	assert.Equal(t, "run-aud028", rec.TaskRunID)

	logged, err := store.HasRunLog(ctx, 1, Slot(occurrence))
	require.NoError(t, err)
	assert.True(t, logged)
}

// TestManager_NilNextTriggeredAtFallsBackToProcessingSlot 覆盖触发槽派生的
// 防御分支：NextTriggeredAt 为 nil（ListDue 已保证非空，此处验证回退不 panic）。
func TestManager_NilNextTriggeredAtFallsBackToProcessingSlot(t *testing.T) {
	assert.Equal(t, Slot(time.Now()), triggerSlot(&model.TaskSchedule{}, time.Now()))
	assert.Equal(t, Slot(time.Now()), triggerSlot(nil, time.Now()))
}
