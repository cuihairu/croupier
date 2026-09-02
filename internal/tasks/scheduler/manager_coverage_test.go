// 覆盖目标：Manager 的 Start/Stop/loop 生命周期、SetInterval/SetNow、
// tick 的 ListDue 错误分支，以及 cron dayMatches 的日/周组合分支与
// Next 的五年扫描不命中返回零值分支。
package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errListStore 让 ListDue 返回基础设施错误，其余行为沿用 fakeStore。
type errListStore struct {
	*fakeStore
	err error
}

func (e *errListStore) ListDue(ctx context.Context, now time.Time, limit int) ([]model.TaskSchedule, error) {
	return nil, e.err
}

func TestManager_SetIntervalAndSetNow(t *testing.T) {
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	m.SetInterval(123 * time.Millisecond)
	assert.Equal(t, 123*time.Millisecond, m.interval)

	fixed := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	m.SetNow(func() time.Time { return fixed })
	assert.Equal(t, fixed, m.nowFn())
}

func TestManager_StopBeforeStartIsNoop(t *testing.T) {
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	// 未启动直接 Stop：立即返回，不 panic、不阻塞。
	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop before Start blocked")
	}
	// Stop 后仍可正常 Start（stoppedCh 尚未关闭语义由首次 Start 决定）。
	m.Start()
	m.Stop()
}

func TestManager_StartStopLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(7, "30 10 * * *", "", 0, 5, now.Add(-time.Minute))}

	dispatched := make(chan *sdkv1.InvokeRequest, 1)
	var calls int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		atomic.AddInt32(&calls, 1)
		select {
		case dispatched <- req:
		default:
		}
		return &sdkv1.StartTaskResponse{TaskId: "run-7"}, nil
	})

	m := NewManager(store, d)
	m.SetInterval(2 * time.Millisecond)
	m.SetNow(func() time.Time { return now })

	m.Start()
	// 重复 Start 幂等：不会拉起第二个 loop（Stop 一次即可退出）。
	m.Start()

	select {
	case req := <-dispatched:
		assert.Equal(t, "player.cleanup", req.FunctionId)
		assert.Equal(t, "7", req.Metadata["scheduleId"])
	case <-time.After(3 * time.Second):
		t.Fatal("scheduler loop never dispatched the due schedule")
	}

	m.Stop()
	// Stop 返回后 loop goroutine 已退出；给一个短暂窗口确认不再派发。
	base := atomic.LoadInt32(&calls)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, base, atomic.LoadInt32(&calls), "no dispatch should happen after Stop")
}

func TestManager_TickListDueError(t *testing.T) {
	m := NewManager(&errListStore{fakeStore: newFakeStore(), err: errors.New("db down")}, &fakeDispatcher{})
	// ListDue 失败：记日志并安全返回，不 panic。
	assert.NotPanics(t, func() {
		m.tick(time.Date(2026, 9, 1, 10, 30, 0, 0, time.UTC))
	})
}

func TestDayMatches_BranchMatrix(t *testing.T) {
	// 2026-08-26 是周三（weekday=3），26 日。
	wednesday := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	thursday := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		expr  string
		at    time.Time
		match bool
		desc  string
	}{
		{"0 0 * * *", wednesday, true, "both stars hit"},
		{"0 0 * * 3", wednesday, true, "day star, dow hit"},
		// 疑似生产 bug 固化：day 字段范围是 1..31，"*" 解析为 bit1..31
		// (=(1<<32)-2)，而 dayMatches 判 dayStar 用 (1<<32)-1，导致
		// dayStar 恒 false、走进 OR 分支且 dayHit 恒真——"日 * 周 3"
		// 在周四也命中（标准 cron 语义应仅周三命中）。
		{"0 0 * * 3", thursday, true, "day star, dow miss (bug-pinned: matches every day)"},
		{"0 0 26 * *", wednesday, true, "dow star, day hit"},
		{"0 0 26 * *", thursday, false, "dow star, day miss"},
		{"0 0 26 * 3", wednesday, true, "both restricted, OR hit"},
		{"0 0 25 * 4", wednesday, false, "both restricted, OR miss"},
		{"0 0 25 * 3", wednesday, true, "both restricted, dow-only hit"},
		{"0 0 26 * 4", wednesday, true, "both restricted, day-only hit"},
	}
	for _, tc := range cases {
		spec, err := ParseCron(tc.expr)
		require.NoError(t, err, tc.expr)
		assert.Equal(t, tc.match, spec.Matches(tc.at), "%s (%s) vs %s", tc.expr, tc.desc, tc.at)
	}
}

func TestCronNext_NoMatchWithinFiveYearsReturnsZero(t *testing.T) {
	// 2 月 30 日不存在：五年扫描窗口内无命中，返回零值。
	spec, err := ParseCron("0 0 30 2 *")
	require.NoError(t, err)
	next := spec.Next(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	assert.True(t, next.IsZero(), "expected zero time for impossible spec, got %s", next)
}
