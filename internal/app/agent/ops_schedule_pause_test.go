package agent

// 调度暂停用例（agent-supervisor-design §10.2 暂停与恢复，暂停对象=调度）：
// 维护窗口内 cron 冻结（进程运行态不动），暂停意图跨 agent 重启保留，恢复后
// 下一匹配槽照常触发（无补跑，crond 语义）；暂停/恢复各留一条事件，快照
// 携带暂停态供面板渲染。

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulePauseStorePersistsAndReloads(t *testing.T) {
	cfg := desiredTestDir(t)

	s1 := newSchedulePauseStore(cfg)
	_, ok := s1.Paused("sleeper")
	assert.False(t, ok, "no record yet: ok must be false (treated as not paused)")

	s1.SetPaused("sleeper", true)
	s1.SetPaused("worker", false)

	// 新实例（模拟 agent 重启）从盘上读回同一暂停态。
	s2 := newSchedulePauseStore(cfg)
	paused, ok := s2.Paused("sleeper")
	require.True(t, ok)
	assert.True(t, paused)
	paused, ok = s2.Paused("worker")
	require.True(t, ok)
	assert.False(t, paused)
	_, ok = s2.Paused("unknown")
	assert.False(t, ok)
}

func TestSchedulePauseStoreCorruptFileFailsOpen(t *testing.T) {
	cfg := desiredTestDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Dir, schedulePauseStateFile),
		[]byte(`{not-json`), 0o644))

	s := newSchedulePauseStore(cfg)
	_, ok := s.Paused("sleeper")
	assert.False(t, ok, "corrupted file must fail open to the empty map")

	// 后续写入覆盖损坏文件。
	s.SetPaused("sleeper", true)
	s2 := newSchedulePauseStore(cfg)
	paused, ok := s2.Paused("sleeper")
	require.True(t, ok)
	assert.True(t, paused)
}

func TestSetSchedulePauseGateRejectsWhenDisabled(t *testing.T) {
	newSrv := func(enabled, allowRestart bool) *OpsServer {
		return NewOpsServer(&OpsConfig{
			Enabled:       enabled,
			AllowRestart:  allowRestart,
			SupervisorLog: desiredTestDir(t),
			ManagedProcesses: map[string]ManagedProcessConfig{
				"demo-app": {Command: "sleep", Args: []string{"30"}},
			},
		}, "agent-a", "test", nil)
	}
	req := &opsv1.SchedulePauseRequest{ProcessName: "demo-app", Suspend: true}

	_, err := newSrv(false, true).SetSchedulePause(context.Background(), req)
	assert.Error(t, err, "ops.Enabled=false must refuse remote pause")

	_, err = newSrv(true, false).SetSchedulePause(context.Background(), req)
	assert.Error(t, err, "ops.AllowRestart=false must refuse remote pause")
}

func TestSetSchedulePauseUnknownProcessFails(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: desiredTestDir(t),
	}, "agent-a", "test", nil)

	_, err := s.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "ghost", Suspend: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	_, err = s.SetSchedulePause(context.Background(), &opsv1.SchedulePauseRequest{})
	assert.Error(t, err, "empty process name must be rejected")
}

func TestSchedulePauseFreezesCronUntilResume(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
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
	assert.Equal(t, int32(1), s.SampleSupervisedProcesses()[0].GetRestartCount())

	// 钉到分钟初+10s：后续 +1m/+2m/+3m 各落独立分钟槽（同 §10.6 用例口径）。
	now := time.Now().Truncate(time.Minute).Add(10 * time.Second)
	s.scheduleTick(now)
	assert.Equal(t, int32(2), s.SampleSupervisedProcesses()[0].GetRestartCount(), "unpaused slot fires")

	_, err := s.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "demo-app", Suspend: true})
	require.NoError(t, err)

	// 暂停期间：两个匹配槽都不触发（cron 冻结，进程运行态不动）。
	s.scheduleTick(now.Add(time.Minute))
	s.scheduleTick(now.Add(2 * time.Minute))
	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	assert.Equal(t, int32(2), snaps[0].GetRestartCount(), "paused slots must not fire")
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_RUNNING, snaps[0].GetState(),
		"pause must not touch the running process")

	// 恢复后：下一匹配槽恢复触发（无补跑——暂停期间的槽不补）。
	_, err = s.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "demo-app", Suspend: false})
	require.NoError(t, err)
	s.scheduleTick(now.Add(3 * time.Minute))
	assert.Equal(t, int32(3), s.SampleSupervisedProcesses()[0].GetRestartCount(),
		"resume must unfreeze the next slot")
}

func TestSchedulePauseSnapshotAndEvents(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
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

	assert.False(t, s.isSchedulePaused("scheduled-app"), "nothing paused before the call")

	resp, err := s.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "scheduled-app", Suspend: true})
	require.NoError(t, err)
	assert.True(t, resp.GetSuccess())
	assert.True(t, resp.GetSuspended())
	assert.Equal(t, "schedule firing suspended", resp.GetMessage())

	// 快照携带暂停态（仅声明了调度的进程渲染该字段）。
	byName := map[string]*opsv1.SupervisedProcessSnapshot{}
	for _, sp := range s.SampleSupervisedProcesses() {
		byName[sp.GetName()] = sp
	}
	require.Contains(t, byName, "scheduled-app")
	assert.True(t, byName["scheduled-app"].GetScheduleSuspended())
	// 无调度条目：暂停意图不产生面板字段（无 cron 可冻结）。
	_, err = s.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "plain-app", Suspend: true})
	require.NoError(t, err)
	assert.False(t, byName["plain-app"].GetScheduleSuspended())

	// 恢复：快照归位，事件闭集两型各一条。
	resp, err = s.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "scheduled-app", Suspend: false})
	require.NoError(t, err)
	assert.Equal(t, "schedule firing resumed", resp.GetMessage())
	assert.False(t, s.isSchedulePaused("scheduled-app"))
	for _, sp := range s.SampleSupervisedProcesses() {
		if sp.GetName() == "scheduled-app" {
			assert.False(t, sp.GetScheduleSuspended())
		}
	}

	// 事件闭集：scheduled-app 一次暂停 + 一次恢复，各留一条留痕
	// （plain-app 的暂停事件另计，不在本进程断言范围）。
	pausedCount, resumedCount := 0, 0
	for _, ev := range s.supLog.EventsSince(0) {
		if ev.GetProcess() != "scheduled-app" {
			continue
		}
		switch ev.GetEvent() {
		case supervisorEventSchedulePaused:
			pausedCount++
		case supervisorEventScheduleResumed:
			resumedCount++
		}
	}
	assert.Equal(t, 1, pausedCount)
	assert.Equal(t, 1, resumedCount)
}

func TestSchedulePauseSurvivesAgentRestart(t *testing.T) {
	cfg := desiredTestDir(t)
	mkCfg := func() *OpsConfig {
		return &OpsConfig{
			Enabled:       true,
			AllowRestart:  true,
			SupervisorLog: cfg,
			ManagedProcesses: map[string]ManagedProcessConfig{
				"demo-app": {
					Command:  "sleep",
					Args:     []string{"30"},
					Schedule: []ProcessScheduleAction{{Action: "restart", Cron: "*/5 * * * *"}},
				},
			},
		}
	}

	first := NewOpsServer(mkCfg(), "agent-a", "test", nil)
	defer first.Stop()
	_, err := first.SetSchedulePause(context.Background(),
		&opsv1.SchedulePauseRequest{ProcessName: "demo-app", Suspend: true})
	require.NoError(t, err)

	// 第二代 agent（同目录重启）：维护窗口意图跨 agent 重启保留。
	second := NewOpsServer(mkCfg(), "agent-a", "test", nil)
	assert.True(t, second.isSchedulePaused("demo-app"),
		"suspended schedule must persist across agent restarts")
}
