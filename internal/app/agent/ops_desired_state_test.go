package agent

// 管理状态持久化（§10.3 管理状态面首块）用例：运维手动 start/stop 的期望
// 跨 agent 重启保留，开机自启不再误启被主动停掉的进程；无记录时行为与
// 既有语义零漂移。

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func desiredTestDir(t *testing.T) SupervisorLogConfig {
	t.Helper()
	return SupervisorLogConfig{Dir: t.TempDir()}
}

func TestDesiredStateStorePersistsAndReloads(t *testing.T) {
	cfg := desiredTestDir(t)

	s1 := newDesiredStateStore(cfg)
	_, ok := s1.Running("sleeper")
	assert.False(t, ok, "no record yet: ok must be false (treated as running)")

	s1.SetRunning("sleeper", false)
	s1.SetRunning("worker", true)

	// 新实例（模拟 agent 重启）从盘上读回同一期望。
	s2 := newDesiredStateStore(cfg)
	running, ok := s2.Running("sleeper")
	require.True(t, ok)
	assert.False(t, running)
	running, ok = s2.Running("worker")
	require.True(t, ok)
	assert.True(t, running)
	_, ok = s2.Running("unknown")
	assert.False(t, ok)
}

func TestDesiredStateStoreCorruptFileFailsOpen(t *testing.T) {
	cfg := desiredTestDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Dir, desiredStateFile),
		[]byte(`{not-json`), 0o644))

	s := newDesiredStateStore(cfg)
	_, ok := s.Running("sleeper")
	assert.False(t, ok, "corrupted file must fail open to the empty map")

	// 后续写入覆盖损坏文件。
	s.SetRunning("sleeper", false)
	s2 := newDesiredStateStore(cfg)
	running, ok := s2.Running("sleeper")
	require.True(t, ok)
	assert.False(t, running)
}

func TestBootStartsAutoRestartProcessWithoutRecord(t *testing.T) {
	// 回归：无期望记录时 AutoRestart=true 仍开机自启（既有语义零漂移）。
	cfg := desiredTestDir(t)
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: cfg,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"30"}, AutoRestart: true},
		},
	}, "agent-desired-boot", "test", nil)
	defer s.Stop()

	require.NoError(t, s.Start())
	p := s.processes["sleeper"]
	require.NotNil(t, p)
	p.mu.RLock()
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_RUNNING, p.state)
	p.mu.RUnlock()
}

func TestBootSkipsOperatorStoppedProcess(t *testing.T) {
	cfg := desiredTestDir(t)

	// 第一代 agent：开机拉起 → 运维手动停（期望 stopped 落盘）。
	first := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: cfg,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"30"}, AutoRestart: true},
		},
	}, "agent-desired-1", "test", nil)
	require.NoError(t, first.Start())
	_, err := first.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)
	first.Stop()

	// 第二代 agent（同目录重启）：开机自启不得误启运维主动停掉的进程。
	second := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: cfg,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"30"}, AutoRestart: true},
		},
	}, "agent-desired-2", "test", nil)
	defer second.Stop()
	require.NoError(t, second.Start())
	p := second.processes["sleeper"]
	if p != nil {
		p.mu.RLock()
		state := p.state
		p.mu.RUnlock()
		assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_STOPPED, state,
			"operator-stopped process must stay stopped across agent restarts")
	}

	// 手动 start 恢复期望运行：第三代 agent 开机重新自启。
	_, err = second.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)
	second.Stop()

	third := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: cfg,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"30"}, AutoRestart: true},
		},
	}, "agent-desired-3", "test", nil)
	defer third.Stop()
	require.NoError(t, third.Start())
	p3 := third.processes["sleeper"]
	require.NotNil(t, p3, "desired-running process must boot-start again")
	p3.mu.RLock()
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_RUNNING, p3.state)
	p3.mu.RUnlock()
}

func TestRestartProcessRestoresDesiredRunning(t *testing.T) {
	cfg := desiredTestDir(t)
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: cfg,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"30"}, AutoRestart: true},
		},
	}, "agent-desired-restart", "test", nil)
	defer s.Stop()
	require.NoError(t, s.Start())

	// 停 → 期望 stopped；重启 → 期望恢复 running。
	_, err := s.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)
	_, err = s.RestartProcess(context.Background(), &opsv1.RestartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)

	running, ok := s.desired.Running("sleeper")
	require.True(t, ok)
	assert.True(t, running, "manual restart is an explicit running intent")
}

func TestBootSkipsAutoRestartFalseRegardlessOfDesired(t *testing.T) {
	// AutoRestart=false 永不开机自启——期望状态只作用于自启面，不扩大语义。
	cfg := desiredTestDir(t)
	newDesiredStateStore(cfg).SetRunning("sleeper", true)

	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: cfg,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"30"}},
		},
	}, "agent-desired-norestart", "test", nil)
	defer s.Stop()
	require.NoError(t, s.Start())
	p := s.processes["sleeper"]
	assert.Nil(t, p, "AutoRestart=false must never boot-start")
}
