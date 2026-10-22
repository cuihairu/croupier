// Supervisor S1 采样链路测试：OpsServer.SampleSupervisedProcesses 对真实
// 子进程采样 RSS/CPU/uptime，超阈值打 flags；快照经 MetricsCollector 折入
// MetricsReport.SupervisedProcesses；ListProcesses 带出 uptime 与最新 flags。
package agent

import (
	"context"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func newSupervisorTestServer(t *testing.T, cfg ManagedProcessConfig) *OpsServer {
	t.Helper()
	return NewOpsServer(&OpsConfig{
		Enabled:          true,
		AllowRestart:     true,
		ManagedProcesses: map[string]ManagedProcessConfig{"sleeper": cfg},
	}, "agent-1", "test", nil)
}

func TestSampleSupervisedProcesses_Running(t *testing.T) {
	s := newSupervisorTestServer(t, ManagedProcessConfig{Command: "sleep", Args: []string{"30"}})
	defer s.Stop()

	_, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)

	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	snap := snaps[0]
	assert.Equal(t, "sleeper", snap.Name)
	assert.Positive(t, snap.Pid)
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_RUNNING, snap.State)
	assert.GreaterOrEqual(t, snap.UptimeSeconds, int64(0))
	assert.LessOrEqual(t, snap.UptimeSeconds, int64(30))
	assert.Positive(t, snap.RssBytes, "live sleep process must report RSS")
	assert.Empty(t, snap.Flags, "no thresholds configured, no flags expected")

	// ListProcesses 带出 uptime 与缓存采样（flags 空但字段非 nil 语义：
	// 未配置阈值时为空切片）。
	procs, err := s.ListProcesses(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, procs.Processes, 1)
	assert.GreaterOrEqual(t, procs.Processes[0].UptimeSeconds, int64(0))
	assert.Empty(t, procs.Processes[0].Flags)
}

func TestSampleSupervisedProcesses_MemThresholdFlag(t *testing.T) {
	// RSS 恒大于 0，阈值设 1 字节必然触发 mem_over_limit——不依赖环境负载。
	s := newSupervisorTestServer(t, ManagedProcessConfig{
		Command: "sleep", Args: []string{"30"},
		MemThresholdBytes: 1,
	})
	defer s.Stop()

	_, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)

	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	assert.Contains(t, snaps[0].Flags, flagMemOverLimit)

	// 缓存值回灌 ListProcesses
	procs, err := s.ListProcesses(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, procs.Processes, 1)
	assert.Contains(t, procs.Processes[0].Flags, flagMemOverLimit)
}

func TestSampleSupervisedProcesses_NotStarted(t *testing.T) {
	// 配置了但未启动：快照仍出现（面板能看到配置面），数值全零。
	s := newSupervisorTestServer(t, ManagedProcessConfig{Command: "sleep", Args: []string{"30"}})
	defer s.Stop()

	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	snap := snaps[0]
	assert.Equal(t, "sleeper", snap.Name)
	assert.Equal(t, int32(0), snap.Pid)
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_STOPPED, snap.State)
	assert.Equal(t, int64(0), snap.RssBytes)
	assert.Equal(t, int64(0), snap.UptimeSeconds)
	assert.Empty(t, snap.Flags)
}

func TestMetricsCollector_FoldsSupervisedSnapshots(t *testing.T) {
	s := newSupervisorTestServer(t, ManagedProcessConfig{Command: "sleep", Args: []string{"30"}})
	defer s.Stop()
	_, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)

	collector := NewMetricsCollector("agent-1").WithSampler(s)
	report := collector.Collect(context.Background())
	require.NotEmpty(t, report.SupervisedProcesses)
	assert.Equal(t, "sleeper", report.SupervisedProcesses[0].Name)

	// 无采样器时不产生空实例字段
	bare := NewMetricsCollector("agent-1").Collect(context.Background())
	assert.Empty(t, bare.SupervisedProcesses)
}

func TestSupervisorFlagsThresholds(t *testing.T) {
	cfg := ManagedProcessConfig{MemThresholdBytes: 100, CpuThresholdPercent: 50}
	assert.Empty(t, supervisorFlags(cfg, 99, 49.9))
	assert.Equal(t, []string{flagMemOverLimit}, supervisorFlags(cfg, 100, 0))
	assert.Equal(t, []string{flagCPUOverLimit}, supervisorFlags(cfg, 0, 50))
	assert.Len(t, supervisorFlags(cfg, 200, 80), 2)

	// 阈值为 0 = 关闭该检查
	off := ManagedProcessConfig{}
	assert.Empty(t, supervisorFlags(off, 1<<30, 100))
}

func TestSampleSupervisedProcesses_HandleRefreshAfterRestart(t *testing.T) {
	// 重启换 pid 后句柄必须重建，否则 CPUPercent 差分落在旧句柄上、
	// MemoryInfo 读到旧 pid 的资源。
	s := newSupervisorTestServer(t, ManagedProcessConfig{Command: "sleep", Args: []string{"30"}})
	defer s.Stop()

	_, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)

	s.mu.RLock()
	p := s.processes["sleeper"]
	s.mu.RUnlock()
	require.NotNil(t, p)

	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	oldPid := snaps[0].Pid
	require.Positive(t, oldPid)

	// 手动重启（走 RestartProcess：stop+start）
	_, err = s.RestartProcess(context.Background(), &opsv1.RestartProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	snaps = s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	newPid := snaps[0].Pid
	require.Positive(t, newPid)
	assert.NotEqual(t, oldPid, newPid, "pid must change across restart")

	p.mu.RLock()
	handlePid := int32(p.proc.Pid)
	p.mu.RUnlock()
	assert.Equal(t, newPid, handlePid, "gopsutil handle must follow the new pid")
}
