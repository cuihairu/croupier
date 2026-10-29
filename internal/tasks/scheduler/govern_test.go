package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dueScheduleWithActor 是带创建者（ARCH-024 自动化主体）的到期调度。
func dueScheduleWithActor(id uint, actor string, now time.Time) model.TaskSchedule {
	s := dueSchedule(id, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))
	s.Actor = actor
	return s
}

func TestManager_GovernAllowDispatches(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueScheduleWithActor(1, "boss", now)}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	var sawActor string
	m.SetGovernHook(func(_ context.Context, s *model.TaskSchedule) (GovernDecision, error) {
		sawActor = s.Actor
		return GovernAllow, nil
	})

	m.tick(now)

	// 治理通过：以调度创建者身份重新授权后正常派发。
	require.Len(t, d.calls, 1)
	assert.Equal(t, "boss", sawActor)
	for _, l := range store.runLogs {
		assert.Equal(t, "dispatched", l.Status)
		assert.Equal(t, "run-ok", l.TaskRunID)
	}
}

func TestManager_GovernApprovalRequiredSkipsDispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueScheduleWithActor(1, "boss", now)}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	m.SetGovernHook(func(_ context.Context, _ *model.TaskSchedule) (GovernDecision, error) {
		return GovernApprovalRequired, nil
	})

	m.tick(now)

	// 政策要求审批：不派发、run-log 记 approval_required。
	assert.Empty(t, d.calls)
	for _, l := range store.runLogs {
		assert.Equal(t, "approval_required", l.Status)
	}
	// 推进下次触发时间（不重试同槽）。
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	_, has := updates[len(updates)-1]["next_triggered_at"]
	assert.True(t, has)
}

func TestManager_GovernDeniedSkipsDispatch(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueScheduleWithActor(1, "boss", now)}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	m.SetGovernHook(func(_ context.Context, _ *model.TaskSchedule) (GovernDecision, error) {
		return GovernDenied, errors.New("function_disabled")
	})

	m.tick(now)

	// 治理拒绝（禁用/撤权/权限不足）：不派发、run-log 记 failed + 原因。
	assert.Empty(t, d.calls)
	for _, l := range store.runLogs {
		assert.Equal(t, "failed", l.Status)
		assert.Contains(t, l.Message, "function_disabled")
	}
	updates := store.updates[1]
	require.NotEmpty(t, updates)
	_, has := updates[len(updates)-1]["next_triggered_at"]
	assert.True(t, has)
}

// 治理钩子不适用于 Kind=incident_report 进程内生成器：该路径不经函数
// 派发链，函数级治理（禁用/分配/政策）不适用，钩子不得触达。
func TestManager_GovernNotAppliedToLocalRunner(t *testing.T) {
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{localSchedule(1, now)}
	d := &fakeDispatcher{}
	lr := &localRunnerStub{}
	m := NewManager(store, d)
	m.SetLocalRunner(lr)
	// 钩子若被调用即拒绝——本地生成器路径不得触达治理。
	m.SetGovernHook(func(_ context.Context, _ *model.TaskSchedule) (GovernDecision, error) {
		return GovernDenied, errors.New("must not be called")
	})

	m.tick(now)

	require.Len(t, lr.calls, 1)
	assert.Empty(t, d.calls)
}
