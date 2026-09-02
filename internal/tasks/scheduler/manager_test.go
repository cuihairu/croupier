package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeStore 是 ScheduleStore 的内存替身。
type fakeStore struct {
	schedules []model.TaskSchedule
	runLogs   map[string]model.TaskScheduleRunLog // key: scheduleID|slot
	updates   map[uint][]map[string]interface{}
}

func newFakeStore(schedules ...model.TaskSchedule) *fakeStore {
	return &fakeStore{runLogs: map[string]model.TaskScheduleRunLog{}, updates: map[uint][]map[string]interface{}{}}
}

func (f *fakeStore) ListDue(ctx context.Context, now time.Time, limit int) ([]model.TaskSchedule, error) {
	var out []model.TaskSchedule
	for i := range f.schedules {
		s := &f.schedules[i]
		if s.Status != model.ScheduleStatusActive || s.NextTriggeredAt == nil || s.NextTriggeredAt.After(now) {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func (f *fakeStore) UpdateSchedule(ctx context.Context, id uint, updates map[string]interface{}) error {
	f.updates[id] = append(f.updates[id], updates)
	for i := range f.schedules {
		if f.schedules[i].ID == id {
			if v, ok := updates["status"].(string); ok {
				f.schedules[i].Status = v
			}
			if v, ok := updates["consecutive_failures"].(int); ok {
				f.schedules[i].ConsecutiveFailures = v
			}
		}
	}
	return nil
}

func (f *fakeStore) HasRunLog(ctx context.Context, scheduleID uint, slot time.Time) (bool, error) {
	_, ok := f.runLogs[fmt.Sprintf("%d|%s", scheduleID, slot.Format(time.RFC3339))]
	return ok, nil
}

func (f *fakeStore) CreateRunLog(ctx context.Context, log *model.TaskScheduleRunLog) (bool, error) {
	key := fmt.Sprintf("%d|%s", log.ScheduleID, log.Slot.Format(time.RFC3339))
	if _, ok := f.runLogs[key]; ok {
		return false, nil
	}
	f.runLogs[key] = *log
	return true, nil
}

func (f *fakeStore) LastRunStatus(ctx context.Context, taskRunID string) (string, error) {
	if taskRunID == "" {
		return "", nil
	}
	if taskRunID == "run-failed" {
		return "failed", nil
	}
	if taskRunID == "run-ok" {
		return "succeeded", nil
	}
	return "", nil
}

// fakeDispatcher 记录派发请求。
type fakeDispatcher struct {
	calls []string
	err   error
}

func (f *fakeDispatcher) StartTask(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	f.calls = append(f.calls, req.FunctionId)
	if f.err != nil {
		return nil, f.err
	}
	return &sdkv1.StartTaskResponse{TaskId: "run-ok"}, nil
}

func dueSchedule(id uint, cron string, lastRunID string, failures, maxFailed int, now time.Time) model.TaskSchedule {
	return model.TaskSchedule{
		Model:               gormModelID(id),
		CronExpr:            cron,
		GameID:              "demo",
		Env:                 "prod",
		FunctionID:          "player.cleanup",
		Status:              model.ScheduleStatusActive,
		MaxFailedRuns:       maxFailed,
		ConsecutiveFailures: failures,
		LastRunID:           lastRunID,
		NextTriggeredAt:     &now,
	}
}

func TestManager_TriggerHappyPath(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-ok", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	m := NewManager(store, d)

	m.tick(now)

	require.Len(t, d.calls, 1)
	// 下次触发时间推进到明天 10:30。
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	last := updates[len(updates)-1]
	next, ok := last["next_triggered_at"].(time.Time)
	require.True(t, ok)
	assert.Equal(t, now.Add(24*time.Hour), next)
	// 成功清零失败计数。
	assert.Equal(t, 0, last["consecutive_failures"])
	// 触发记录已写。
	logged, _ := store.HasRunLog(context.Background(), 1, Slot(now))
	assert.True(t, logged)
}

func TestManager_IdempotentSlot(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "* * * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	m := NewManager(store, d)

	_, _ = store.CreateRunLog(context.Background(), &model.TaskScheduleRunLog{ScheduleID: 1, Slot: Slot(now)})
	m.tick(now)
	// 同槽已记录：不派发，只推进时间。
	assert.Empty(t, d.calls)
}

func TestManager_FailureCountingAndDeadLetter(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)

	t.Run("failure increments", func(t *testing.T) {
		store := newFakeStore()
		// 上次执行失败（LastRunID=run-failed），已失败 1 次，上限 3。
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-failed", 1, 3, now.Add(-time.Minute))}
		d := &fakeDispatcher{}
		NewManager(store, d).tick(now)
		updates := store.updates[1]
		require.NotEmpty(t, updates)
		assert.Equal(t, 2, updates[len(updates)-1]["consecutive_failures"])
		assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)
	})

	t.Run("reaches limit enters dead_letter", func(t *testing.T) {
		store := newFakeStore()
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-failed", 2, 3, now.Add(-time.Minute))}
		d := &fakeDispatcher{}
		NewManager(store, d).tick(now)
		// 达上限：不再派发，直接 dead_letter。
		assert.Empty(t, d.calls)
		assert.Equal(t, model.ScheduleStatusDeadLetter, store.schedules[0].Status)
	})
}

func TestManager_InvalidCronDeadLetters(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "not-a-cron", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	NewManager(store, d).tick(now)
	assert.Empty(t, d.calls)
	assert.Equal(t, model.ScheduleStatusDeadLetter, store.schedules[0].Status)
}

func TestManager_DispatchErrorStillAdvances(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{err: fmt.Errorf("agent unreachable")}
	NewManager(store, d).tick(now)

	require.Len(t, d.calls, 1)
	// 派发失败也推进下次时间（下个周期再试，不阻塞调度循环）。
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	_, has := updates[len(updates)-1]["next_triggered_at"]
	assert.True(t, has)
	// run log 记为 failed。
	for _, l := range store.runLogs {
		assert.Equal(t, "failed", l.Status)
	}
}

func TestManager_MetadataCarriesScope(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}
	var captured *sdkv1.InvokeRequest
	d2 := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		captured = req
		return &sdkv1.StartTaskResponse{TaskId: "x"}, nil
	})
	NewManager(store, d2).tick(now)

	require.NotNil(t, captured)
	assert.Equal(t, "player.cleanup", captured.FunctionId)
	assert.Equal(t, "demo", captured.Metadata["gameId"])
	assert.Equal(t, "prod", captured.Metadata["env"])
	assert.Equal(t, "1", captured.Metadata["scheduleId"])
}

// gormModelID 构造带 ID 的 gorm.Model（测试便利）。
func gormModelID(id uint) gorm.Model { return gorm.Model{ID: id} }

// dispatcherFunc 适配函数签名。
type dispatcherFunc func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error)

func (f dispatcherFunc) StartTask(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	return f(ctx, req)
}
