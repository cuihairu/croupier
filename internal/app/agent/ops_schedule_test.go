package agent

// 定时动作调度用例（agent-supervisor-design §10.6 调度状态）：调度入口统一
// 一份 schedule 模型（与 server 侧 internal/tasks/scheduler 同一 ParseCron），
// 触发走显式动作同一漏斗（desired-state 与 supervisor 事件随流）。关键属性：
// scheduled stop 与手动停同信号（stopCh），恢复环不把定时停误拉起；分钟槽
// 去重保证 30s 扫描幂等；无补跑（agent 停机错过槽位不补触发，crond 语义）。

import (
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateScheduleClosedSetAndCron(t *testing.T) {
	// 动作闭集：start/stop/restart 之外的值配置加载即失败。
	badAction := ManagedProcessConfig{Schedule: []ProcessScheduleAction{{Action: "pause", Cron: "* * * * *"}}}
	err := badAction.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action")

	// cron 与 server 侧同一解析器：字段数/取值域不对即失败（一份 schedule 模型）。
	badCrons := []string{"0 0 * *", "* * * * * *", "61 * * * *", "0 25 * * *", ""}
	for _, expr := range badCrons {
		cfg := ManagedProcessConfig{Schedule: []ProcessScheduleAction{{Action: "stop", Cron: expr}}}
		err := cfg.Validate()
		require.Error(t, err, "cron %q must be rejected", expr)
	}

	good := ManagedProcessConfig{Schedule: []ProcessScheduleAction{
		{Action: "stop", Cron: "0 3 * * *"},
		{Action: "start", Cron: "30 7 * * 1-5"},
		{Action: "restart", Cron: "*/10 * * * *"},
	}}
	assert.NoError(t, good.Validate())

	// 空 schedule 合法（存量配置零漂移）。
	var empty ManagedProcessConfig
	assert.NoError(t, empty.Validate())
}

func TestScheduledStopRecordsDesiredStateAndBlocksAutoRestart(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {
				Command:     "sleep",
				Args:        []string{"30"},
				AutoRestart: true,
				Schedule:    []ProcessScheduleAction{{Action: "stop", Cron: "* * * * *"}},
			},
		},
	}, "agent-a", "test", nil)
	s.schedTick = 20 * time.Millisecond
	require.NoError(t, s.Start())
	defer s.Stop()

	// 开机自启先起来。
	require.Eventually(t, func() bool {
		snaps := s.SampleSupervisedProcesses()
		return len(snaps) == 1 && snaps[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond, "boot should start the process")

	// 定时 stop 到点：走与手动停同一漏斗 → STOPPED + 期望停落盘。
	require.Eventually(t, func() bool {
		snaps := s.SampleSupervisedProcesses()
		return len(snaps) == 1 && snaps[0].GetState() == opsv1.ProcessState_PROCESS_STATE_STOPPED
	}, 3*time.Second, 20*time.Millisecond, "scheduled stop should fire")

	running, ok := s.desired.Running("demo-app")
	assert.True(t, ok, "scheduled stop must record desired state")
	assert.False(t, running)

	// §10.6 关键属性：定时停=主动停止（同 stopCh 信号），恢复环不误拉起。
	time.Sleep(300 * time.Millisecond)
	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	assert.Equal(t, int32(0), snaps[0].GetPid(), "process must stay stopped after scheduled stop")

	// 事件留痕：schedule_fire（区别于 manual_stop，§10.3 四类状态可分辨）。
	fired := false
	for _, ev := range s.supLog.EventsSince(0) {
		if ev.GetEvent() == "schedule_fire" {
			fired = true
		}
	}
	assert.True(t, fired, "scheduled stop must emit a schedule_fire event")
}

func TestScheduledStartMaterializesAndRuns(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {
				Command: "sleep",
				Args:    []string{"30"},
				// 非 AutoRestart：开机自启不拉起，仅定时 start 到点物化并启动。
				Schedule: []ProcessScheduleAction{{Action: "start", Cron: "* * * * *"}},
			},
		},
	}, "agent-a", "test", nil)
	s.schedTick = 20 * time.Millisecond
	require.NoError(t, s.Start())
	defer s.Stop()

	require.Eventually(t, func() bool {
		snaps := s.SampleSupervisedProcesses()
		return len(snaps) == 1 && snaps[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond, "scheduled start should materialize and run the process")

	running, ok := s.desired.Running("demo-app")
	assert.True(t, ok)
	assert.True(t, running)
}

func TestScheduledRestartAndMinuteSlotDedup(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {
				Command:     "sleep",
				Args:        []string{"30"},
				AutoRestart: true,
				Schedule:    []ProcessScheduleAction{{Action: "restart", Cron: "* * * * *"}},
			},
		},
	}, "agent-a", "test", nil)
	// 直接驱动 scheduleTick：真实循环 tick 拉长到 1h 避免并发触发。
	s.schedTick = time.Hour
	require.NoError(t, s.Start())
	defer s.Stop()

	require.Eventually(t, func() bool {
		snaps := s.SampleSupervisedProcesses()
		return len(snaps) == 1 && snaps[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond)
	// boot 计一次 restart。
	require.Equal(t, int32(1), s.SampleSupervisedProcesses()[0].GetRestartCount())

	now := time.Now()
	s.scheduleTick(now)
	assert.Equal(t, int32(2), s.SampleSupervisedProcesses()[0].GetRestartCount(), "matching slot fires once")

	// 同一分钟槽内重复扫描不重复触发（30s 扫描器的幂等窗口）。
	s.scheduleTick(now)
	s.scheduleTick(now.Add(30 * time.Second))
	assert.Equal(t, int32(2), s.SampleSupervisedProcesses()[0].GetRestartCount(), "same slot must not re-fire")

	// 下一分钟槽到点再触发（每个匹配分钟恰一次）。
	s.scheduleTick(now.Add(time.Minute))
	assert.Equal(t, int32(3), s.SampleSupervisedProcesses()[0].GetRestartCount(), "next slot fires again")
}

func TestScheduledStopOnMissingInstanceIsIdempotent(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {
				Command: "sleep",
				Args:    []string{"30"},
				// 非 AutoRestart：开机不启动 → 实例不存在。
				Schedule: []ProcessScheduleAction{{Action: "stop", Cron: "* * * * *"}},
			},
		},
	}, "agent-a", "test", nil)
	s.schedTick = time.Hour
	require.NoError(t, s.Start())
	defer s.Stop()

	// 未运行的进程无可停：幂等空操作，不物化实例、不留事件。
	s.scheduleTick(time.Now())

	s.mu.RLock()
	count := len(s.processes)
	s.mu.RUnlock()
	assert.Equal(t, 0, count)
	assert.Empty(t, s.supLog.EventsSince(0))
}

func TestScheduleSnapshotNextFields(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"scheduled-app": {
				Command:  "sleep",
				Args:     []string{"30"},
				Schedule: []ProcessScheduleAction{{Action: "stop", Cron: "30 7 * * *"}},
			},
			"plain-app": {Command: "sleep", Args: []string{"30"}},
		},
	}, "agent-a", "test", nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 2)
	byName := map[string]*struct {
		at    int64
		event string
	}{}
	for _, sp := range snaps {
		byName[sp.GetName()] = &struct {
			at    int64
			event string
		}{sp.GetNextScheduleAtUnix(), sp.GetNextScheduleAction()}
	}

	// 调度状态事前可见：下一次触发时间与动作随快照上行。
	now := time.Now()
	nextAt := time.Unix(byName["scheduled-app"].at, 0)
	assert.Equal(t, "stop", byName["scheduled-app"].event)
	assert.False(t, byName["scheduled-app"].at == 0)
	assert.True(t, nextAt.After(now), "next fire must be in the future")
	assert.True(t, nextAt.Sub(now) < 24*time.Hour, "daily cron fires within a day")

	// 无调度条目零字段（零漂移）。
	assert.Equal(t, int64(0), byName["plain-app"].at)
	assert.Empty(t, byName["plain-app"].event)
}

func TestScheduleLoopFiresEndToEnd(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {
				Command:  "sleep",
				Args:     []string{"30"},
				Schedule: []ProcessScheduleAction{{Action: "start", Cron: "* * * * *"}},
			},
		},
	}, "agent-a", "test", nil)
	s.schedTick = 25 * time.Millisecond
	require.NoError(t, s.Start())
	defer s.Stop()

	// 真实循环（ticker + 分钟槽去重 + 漏斗）端到端拉起进程。
	require.Eventually(t, func() bool {
		snaps := s.SampleSupervisedProcesses()
		return len(snaps) == 1 && snaps[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond, "schedule loop should fire without manual ticks")
}
