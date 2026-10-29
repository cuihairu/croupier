package scheduler

import (
	"context"
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
