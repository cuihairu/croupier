package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 区间端点非法（非数字 / 缺失）→ parsePart 的"非法区间"分支。
func TestParseCronInvalidRangeEndpoints(t *testing.T) {
	for _, expr := range []string{
		"x-5 * * * *", // 下界非数字
		"5-x * * * *", // 上界非数字
		"1- * * * *",  // 上界缺失
		"-5 * * * *",  // 下界缺失
	} {
		_, err := ParseCron(expr)
		require.Error(t, err, expr)
		assert.Contains(t, err.Error(), "非法区间", expr)
	}
}

// MaxFailedRuns <= 0 时回退默认值 5：失败计数 4 仍应派发而非进 dead_letter。
func TestManager_TriggerDefaultsMaxFailedRuns(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	// 上次失败（failures 3+1=4），MaxFailedRuns=0 → 默认上限 5，4 < 5 继续派发。
	store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-failed", 3, 0, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	NewManager(store, d).tick(now)

	require.Len(t, d.calls, 1)
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	assert.Equal(t, 4, updates[len(updates)-1]["consecutive_failures"])
}

// 携带 Metadata 的计划：触发时解包进派发请求并补充 scope 字段。
func TestManager_TriggerUnpacksMetadata(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	schedule := dueSchedule(1, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))
	schedule.Metadata = model.JSON([]byte(`{"customKey":"customValue"}`))
	store.schedules = []model.TaskSchedule{schedule}

	var captured *sdkv1.InvokeRequest
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		captured = req
		return &sdkv1.StartTaskResponse{TaskId: "run-ok"}, nil
	})
	NewManager(store, d).tick(now)

	require.NotNil(t, captured)
	assert.Equal(t, "customValue", captured.Metadata["customKey"])
	assert.Equal(t, "demo", captured.Metadata["gameId"])
	assert.Equal(t, "prod", captured.Metadata["env"])
	assert.Equal(t, "1", captured.Metadata["scheduleId"])
}
