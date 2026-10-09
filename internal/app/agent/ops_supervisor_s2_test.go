// S2 supervisor 测试：退避序列/封顶、熔断闭集、退避等待可打断、事件环与
// 轮转文件、ReadTail 截断对齐、oom 启发式边界、状态机端到端（检测→退避→
// 自动拉起→熔断）。
package agent

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

func newS2TestServer(t *testing.T, cfg *OpsConfig) *OpsServer {
	t.Helper()
	cfg.Enabled = true
	cfg.AllowRestart = true
	cfg.SupervisorLog = SupervisorLogConfig{Dir: t.TempDir()}
	s := NewOpsServer(cfg, "a", "v", nil)
	t.Cleanup(func() {
		for _, p := range s.processes {
			_, _ = s.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: p.name})
		}
	})
	return s
}

func waitSupervisorEvent(t *testing.T, s *OpsServer, event string, timeout time.Duration) *opsv1.SupervisorEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, ev := range s.SupervisorEventsSince(0) {
			if ev.Event == event {
				return ev
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func waitProcessState(t *testing.T, s *OpsServer, name string, state opsv1.ProcessState, timeout time.Duration) *opsv1.ManagedProcess {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		procs, err := s.ListProcesses(context.Background(), nil)
		require.NoError(t, err)
		for _, p := range procs.Processes {
			if p.Name == name && p.State == state {
				return p
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process %q did not reach state %v within %v", name, state, timeout)
	return nil
}

func stopManaged(t *testing.T, s *OpsServer, name string) {
	t.Helper()
	_, err := s.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: name, Force: true})
	require.NoError(t, err)
}

func TestSupervisorBackoffDelay(t *testing.T) {
	cfg := ManagedProcessConfig{RestartBackoffInitial: time.Second, RestartBackoffMax: 60 * time.Second}
	assert.Equal(t, time.Second, supervisorBackoffDelay(cfg, 1))
	assert.Equal(t, 2*time.Second, supervisorBackoffDelay(cfg, 2))
	assert.Equal(t, 4*time.Second, supervisorBackoffDelay(cfg, 3))
	assert.Equal(t, 32*time.Second, supervisorBackoffDelay(cfg, 6))
	// 封顶：2^7=128s > 60s
	assert.Equal(t, 60*time.Second, supervisorBackoffDelay(cfg, 7))
	assert.Equal(t, 60*time.Second, supervisorBackoffDelay(cfg, 100))

	// 零值回退默认 1s 起步、60s 封顶。
	zero := ManagedProcessConfig{}
	assert.Equal(t, time.Second, supervisorBackoffDelay(zero, 1))
	assert.Equal(t, 60*time.Second, supervisorBackoffDelay(zero, 50))

	// max < initial 时以 initial 为准（不出现倒退）。
	odd := ManagedProcessConfig{RestartBackoffInitial: 10 * time.Second, RestartBackoffMax: 3 * time.Second}
	assert.Equal(t, 10*time.Second, supervisorBackoffDelay(odd, 1))
	assert.Equal(t, 10*time.Second, supervisorBackoffDelay(odd, 5))
}

func TestBreakerTripped(t *testing.T) {
	cfg := ManagedProcessConfig{RestartBreakerLimit: 5}
	assert.False(t, breakerTripped(cfg, 4))
	assert.True(t, breakerTripped(cfg, 5))
	assert.True(t, breakerTripped(cfg, 9))

	// limit=0 → 永不熔断
	assert.False(t, breakerTripped(ManagedProcessConfig{}, 1000))

	msg := breakerMessage(cfg, 5)
	assert.Contains(t, msg, "5")
	assert.Contains(t, msg, "熔断")
}

func TestSupervisorOomSuspect(t *testing.T) {
	// 非 kill 信号 / 无 RSS → 必 false（与系统内存无关的短路分支）。
	assert.False(t, supervisorOomSuspect("", 1<<40))
	assert.False(t, supervisorOomSuspect("terminated", 1<<40))
	assert.False(t, supervisorOomSuspect("SIGKILL", 0))
	// kill 类信号但 RSS 极小 → false（低于 90% available）。
	assert.False(t, supervisorOomSuspect("SIGKILL", 1))
}

func TestExitDetailClosedSet(t *testing.T) {
	code, sig := exitDetail(nil)
	assert.Equal(t, int32(0), code)
	assert.Equal(t, "", sig)
}

func TestSupervisorEventLogEmitAndRing(t *testing.T) {
	dir := t.TempDir()
	log := newSupervisorEventLog(SupervisorLogConfig{Dir: dir, MaxSizeMB: 1, MaxBackups: 1, MaxAgeDays: 1})
	require.NotNil(t, log)

	a := newSupervisorEvent("app", supervisorEventDetectDown)
	a.ExitCode = 137
	log.emit(a)
	b := newSupervisorEvent("app", supervisorEventManualStop)
	log.emit(b)

	assert.Equal(t, int64(2), log.LatestSeq())
	assert.Equal(t, int64(1), a.Seq)
	assert.Equal(t, int64(2), b.Seq)
	assert.NotZero(t, a.TsUnix)

	// EventsSince 增量语义（严格大于）。
	assert.Len(t, log.EventsSince(0), 2)
	assert.Len(t, log.EventsSince(1), 1)
	assert.Equal(t, int64(2), log.EventsSince(1)[0].Seq)
	assert.Empty(t, log.EventsSince(2))

	// 轮转文件落盘为 JSON lines。
	content, name, truncated, err := log.ReadTail(1 << 20)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, "supervisor.log", name)
	assert.Contains(t, string(content), `"event":"detect_down"`)
	assert.Contains(t, string(content), `"event":"manual_stop"`)
}

func TestSupervisorEventLogRingCap(t *testing.T) {
	log := newSupervisorEventLog(SupervisorLogConfig{Dir: t.TempDir()})
	for i := 0; i < supervisorEventRingSize+10; i++ {
		log.emit(newSupervisorEvent("app", supervisorEventAutoRestart))
	}
	evs := log.EventsSince(0)
	assert.Len(t, evs, supervisorEventRingSize)
	// 环保留最新：首条 seq 应是 11。
	assert.Equal(t, int64(11), evs[0].Seq)
	assert.Equal(t, int64(supervisorEventRingSize+10), evs[len(evs)-1].Seq)
}

func TestSupervisorLogReadTailTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "supervisor.log")
	// 手写多行 JSON，验证尾部截断对齐到行首。
	lines := ""
	for i := 0; i < 50; i++ {
		lines += `{"seq":` + string(rune('0'+i%10)) + `,"event":"x"}` + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(lines), 0o644))

	log := newSupervisorEventLog(SupervisorLogConfig{Dir: dir})
	content, name, truncated, err := log.ReadTail(60)
	require.NoError(t, err)
	assert.True(t, truncated)
	assert.Equal(t, "supervisor.log", name)
	// 截断后每行必须是完整 JSON 行（行首对齐）。
	require.NotEmpty(t, content)
	assert.Equal(t, byte('{'), content[0])
	assert.Equal(t, byte('\n'), content[len(content)-1])
	for _, ln := range splitJSONLines(content) {
		assert.True(t, len(ln) > 0 && ln[0] == '{' && ln[len(ln)-1] == '}')
	}
	// 小文件不截断。
	full, _, tr, err := log.ReadTail(1 << 20)
	require.NoError(t, err)
	assert.False(t, tr)
	assert.Equal(t, len(lines), len(full))
}

func TestSupervisorLogReadTailMissingFile(t *testing.T) {
	log := newSupervisorEventLog(SupervisorLogConfig{Dir: t.TempDir()})
	_, name, _, err := log.ReadTail(1024)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, "supervisor.log", name)
}

func TestSupervisorLogReadTailEmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "supervisor.log"), nil, 0o644))
	log := newSupervisorEventLog(SupervisorLogConfig{Dir: dir})
	content, _, truncated, err := log.ReadTail(1024)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Empty(t, content)
}

// TestSuperviseExitBreakerTrips 走完整状态机：命令立刻以非零码退出、
// autoRestart 开、熔断阈值 2、退避极小——两轮退避拉起后熔断为 BROKEN。
func TestSuperviseExitBreakerTrips(t *testing.T) {
	s := newS2TestServer(t, &OpsConfig{
		ManagedProcesses: map[string]ManagedProcessConfig{
			"boom": {
				Command:               "sh",
				Args:                  []string{"-c", "exit 3"},
				AutoRestart:           true,
				RestartBackoffInitial: 10 * time.Millisecond,
				RestartBackoffMax:     20 * time.Millisecond,
				RestartBreakerLimit:   2,
			},
		},
	})
	require.NoError(t, s.Start())

	broken := waitProcessState(t, s, "boom", opsv1.ProcessState_PROCESS_STATE_BROKEN, 5*time.Second)
	assert.GreaterOrEqual(t, broken.RestartCount, int32(1))

	// BROKEN 快照 flags 恒带 breaker_tripped。
	snap := s.SampleSupervisedProcesses()
	require.Len(t, snap, 1)
	assert.Contains(t, snap[0].Flags, flagBreakerTripped)
	assert.Equal(t, int64(0), snap[0].NextRestartAtUnix)

	// 事件序列：detect_down 首条、breaker_tripped 末条、seq 单调连续。
	evs := s.SupervisorEventsSince(0)
	require.NotEmpty(t, evs)
	assert.Equal(t, supervisorEventDetectDown, evs[0].Event)
	assert.Equal(t, int32(3), evs[0].ExitCode)
	assert.Equal(t, supervisorEventBreakerTripped, evs[len(evs)-1].Event)
	assert.NotEmpty(t, evs[len(evs)-1].Message)
	for i := 1; i < len(evs); i++ {
		assert.Equal(t, evs[i-1].Seq+1, evs[i].Seq)
	}
	assert.Contains(t, evs[1].Event, supervisorEventAutoRestart)
}

// TestSuperviseExitAutoRestartSequence 命令启动即退、不熔断：断言
// detect_down → auto_restart 事件成对出现、restarts 递增。
func TestSuperviseExitAutoRestartSequence(t *testing.T) {
	s := newS2TestServer(t, &OpsConfig{
		ManagedProcesses: map[string]ManagedProcessConfig{
			"flaky": {
				Command:               "sh",
				Args:                  []string{"-c", "exit 1"},
				AutoRestart:           true,
				RestartBackoffInitial: 5 * time.Millisecond,
				RestartBackoffMax:     10 * time.Millisecond,
				RestartBreakerLimit:   0,
			},
		},
	})
	require.NoError(t, s.Start())

	require.NotNil(t, waitSupervisorEvent(t, s, supervisorEventAutoRestart, 5*time.Second))
	stopManaged(t, s, "flaky")

	evs := s.SupervisorEventsSince(0)
	require.NotEmpty(t, evs)
	assert.Equal(t, supervisorEventDetectDown, evs[0].Event)
	sawAuto := false
	for _, ev := range evs {
		if ev.Event == supervisorEventAutoRestart {
			sawAuto = true
			assert.Greater(t, ev.NewPid, int32(0))
		}
	}
	assert.True(t, sawAuto)
}

// TestSuperviseExitBackoffOutsideLock 断言退避窗口内状态为 BACKOFF 且
// next_restart_at 已排程——等待发生在锁外，读路径不被阻塞。
func TestSuperviseExitBackoffOutsideLock(t *testing.T) {
	s := newS2TestServer(t, &OpsConfig{
		ManagedProcesses: map[string]ManagedProcessConfig{
			"slow": {
				Command:               "sh",
				Args:                  []string{"-c", "exit 1"},
				AutoRestart:           true,
				RestartBackoffInitial: 2 * time.Second,
				RestartBackoffMax:     2 * time.Second,
			},
		},
	})
	require.NoError(t, s.Start())

	// 退避窗口 2s，1.5s 内应观测到 BACKOFF + NextRestartAtUnix>0。
	p := waitProcessState(t, s, "slow", opsv1.ProcessState_PROCESS_STATE_BACKOFF, 1500*time.Millisecond)
	assert.Greater(t, p.NextRestartAtUnix, int64(0))
	stopManaged(t, s, "slow")
}

// TestSuperviseExitManualStopSuppressesRestart 手动 stop 后实例退出不再拉起：
// stopCh 短路在 detect_down 之前生效。
func TestSuperviseExitManualStopSuppressesRestart(t *testing.T) {
	s := newS2TestServer(t, &OpsConfig{
		ManagedProcesses: map[string]ManagedProcessConfig{
			"victim": {
				Command:               "sh",
				Args:                  []string{"-c", "sleep 30"},
				AutoRestart:           true,
				RestartBackoffInitial: 10 * time.Millisecond,
			},
		},
	})
	require.NoError(t, s.Start())
	waitProcessState(t, s, "victim", opsv1.ProcessState_PROCESS_STATE_RUNNING, 5*time.Second)
	stopManaged(t, s, "victim")

	procs, err := s.ListProcesses(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_STOPPED, procs.Processes[0].State)

	// 给状态机留出误拉起的窗口，再确认没有 auto_restart 且停在 STOPPED。
	time.Sleep(300 * time.Millisecond)
	for _, ev := range s.SupervisorEventsSince(0) {
		assert.NotEqual(t, supervisorEventAutoRestart, ev.Event)
	}
	procs, err = s.ListProcesses(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_STOPPED, procs.Processes[0].State)
}

// TestManualStartResetsBreaker BROKEN 态人工 Start 复位成功判定路径：换长驻
// 命令后进程回到 RUNNING，manual_start 事件落环。
func TestManualStartResetsBreaker(t *testing.T) {
	dir := t.TempDir()
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: SupervisorLogConfig{Dir: dir},
		ManagedProcesses: map[string]ManagedProcessConfig{
			"rebuilt": {
				Command:               "sh",
				Args:                  []string{"-c", "exit 1"},
				AutoRestart:           true,
				RestartBackoffInitial: 5 * time.Millisecond,
				RestartBackoffMax:     5 * time.Millisecond,
				RestartBreakerLimit:   1,
			},
		},
	}, "a", "v", nil)
	t.Cleanup(func() { stopManaged(t, s, "rebuilt") })
	require.NoError(t, s.Start())
	waitProcessState(t, s, "rebuilt", opsv1.ProcessState_PROCESS_STATE_BROKEN, 5*time.Second)

	// 人工拉起换成不退出的命令。
	s.mu.RLock()
	p := s.processes["rebuilt"]
	s.mu.RUnlock()
	require.NotNil(t, p)
	p.mu.Lock()
	p.config.Args = []string{"-c", "sleep 30"}
	p.mu.Unlock()

	resp, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "rebuilt"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	waitProcessState(t, s, "rebuilt", opsv1.ProcessState_PROCESS_STATE_RUNNING, 3*time.Second)

	var sawManual bool
	for _, ev := range s.SupervisorEventsSince(0) {
		if ev.Event == supervisorEventManualStart {
			sawManual = true
		}
	}
	assert.True(t, sawManual)
}

func splitJSONLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}
