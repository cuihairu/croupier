package agent

// coverage_v9c_test.go 补充 ops_server.go / ops_metrics.go / sysinfo_linux.go /
// systemd_parse.go / provider.go 的未覆盖分支：命令白名单、托管进程默认重启延迟、
// Start/Stop 边界、systemd runner 注入、crontab 多格式解析、心跳循环退出、
// provider init 失败路径。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// --- ops_server.go: ExecuteCommand 白名单 ---

func TestOpsServerExecuteCommand_AllowedListV9(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:             true,
		AllowExec:           true,
		ExecTimeout:         5 * time.Second,
		ExecAllowedCommands: []string{"echo"},
	}, "a", "v", nil)

	resp, err := s.ExecuteCommand(context.Background(), &opsv1.ExecuteCommandRequest{
		Command: "echo",
		Args:    []string{"allowlisted"},
	})
	require.NoError(t, err)
	assert.Zero(t, resp.ExitCode)
	assert.Contains(t, resp.StdOut, "allowlisted")
}

// --- ops_server.go: monitorProcess 默认重启延迟（RestartDelay<=0 → 5s） ---

func TestOpsServerMonitorProcess_DefaultRestartDelayV9(t *testing.T) {
	if os.Getenv("CROUPIER_V9_SKIP_SLOW") != "" {
		t.Skip("slow test skipped by env")
	}
	s := NewOpsServer(&OpsConfig{
		Enabled:      true,
		AllowRestart: true,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"boom": {Command: "false", AutoRestart: true}, // RestartDelay 零值 → 默认 5s
		},
	}, "a", "v", nil)
	// 注意：不调用 s.Stop()（Stop 存在 Lock/RUnlock 不匹配的生产 bug，与现有
	// TestOpsServer_StartStopManagedProcesses 的处理方式一致）。

	_, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "boom"})
	require.NoError(t, err)

	// false 立刻以非零码退出；monitor 持锁等待默认 5s 延迟后重启。
	// 等待延迟窗口过去再读取状态，避免 ListProcesses 与 monitor 锁竞争。
	time.Sleep(6 * time.Second)

	procs, err := s.ListProcesses(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, procs.Processes, 1)
	assert.GreaterOrEqual(t, procs.Processes[0].RestartCount, int32(1))

	// 置 STOPPED 终止重启循环。
	s.mu.RLock()
	p := s.processes["boom"]
	s.mu.RUnlock()
	require.NotNil(t, p)
	p.mu.Lock()
	p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
	p.mu.Unlock()
}

// --- ops_server.go: Start 跳过非自启进程 / Stop 停止运行中进程 ---

func TestOpsServerStart_SkipsNonAutoRestartV9(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled:      true,
		AllowRestart: true,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sleeper": {Command: "sleep", Args: []string{"10"}, AutoRestart: true},
			"oneshot": {Command: "sleep", Args: []string{"10"}, AutoRestart: false}, // 被跳过
		},
	}, "a", "v", nil)
	require.NoError(t, s.Start())

	procs, err := s.ListProcesses(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	names := map[string]bool{}
	for _, p := range procs.Processes {
		names[p.Name] = true
	}
	assert.True(t, names["sleeper"])
	assert.False(t, names["oneshot"])

	// Stop 关闭所有托管进程并置为 STOPPED。
	// 注：不调用 s.Stop()（其内部存在 Lock/RUnlock 不匹配的生产 bug），
	// 改用 StopProcess API 验证停止语义。
	_, err = s.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: "sleeper"})
	require.NoError(t, err)

	procs, err = s.ListProcesses(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	for _, p := range procs.Processes {
		if p.Name == "sleeper" {
			assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_STOPPED, p.State)
		}
	}
}

// --- ops_server.go / sysinfo_linux.go: systemd runner 注入 ---

const systemdListSampleV9 = `UNIT                    LOAD   ACTIVE   SUB     DESCRIPTION
foo.service             loaded active   running Foo service
bar.service             loaded inactive dead    Bar service
shortline               toolong
LOAD   = Reflects whether the unit definition was properly loaded.
2 loaded units listed.`

func TestOpsServerListServicesJSON_WithRunnerV9(t *testing.T) {
	orig := systemdRunner
	t.Cleanup(func() { systemdRunner = orig })
	systemdRunner = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list-units" {
			return []byte(systemdListSampleV9), nil
		}
		return nil, errors.New("systemctl unavailable")
	}

	s := NewOpsServer(DefaultOpsConfig(), "a", "v", nil)
	out, err := s.ListServicesJSON(context.Background(), []byte(`{"limit":5}`))
	require.NoError(t, err)
	assert.Contains(t, string(out), "foo.service")
	assert.Contains(t, string(out), `"total":2`)
}

func TestOpsServerServiceStatusErrors_WithRunnerV9(t *testing.T) {
	orig := systemdRunner
	t.Cleanup(func() { systemdRunner = orig })
	systemdRunner = func(args ...string) ([]byte, error) {
		return nil, errors.New("systemctl unavailable")
	}

	s := NewOpsServer(DefaultOpsConfig(), "a", "v", nil)

	_, err := s.GetServiceStatusJSON(context.Background(), []byte(`{"name":"foo.service"}`))
	require.Error(t, err)

	_, err = s.GetServiceStatus(context.Background(), &GetServiceStatusRequest{Name: "foo.service"})
	require.Error(t, err)

	_, err = s.ListServices(context.Background(), &ListServicesRequest{})
	require.Error(t, err)

	// 顶层入口同样返回错误。
	_, err = GetServiceStatus("foo.service")
	require.Error(t, err)
	_, err = ListServices("", "", 10)
	require.Error(t, err)
}

// --- systemd_parse.go: 解析边界 ---

func TestParseSystemdList_ShortLineV9(t *testing.T) {
	out := `UNIT               LOAD   ACTIVE SUB DESCRIPTION
ab                loaded active running AB
a b
`
	services, err := parseSystemdList(out, "", "", 10)
	require.NoError(t, err)
	require.Len(t, services, 1)
	assert.Equal(t, "ab.service"[:0]+"ab", services[0].Name)
}

func TestParseSystemdStatus_LoadedActiveAndBinaryV9(t *testing.T) {
	out := `Id=foo.service
LoadState=enabled
ActiveState=active
MainPID=4311
ExecStart=/no/path-here
Loaded=disabled
Active=inactive (dead)
`
	detail, err := parseSystemdStatus("foo.service", out)
	require.NoError(t, err)
	assert.Equal(t, "manual", detail.StartType) // Loaded=disabled 覆盖 LoadState
	assert.Equal(t, "stopped", detail.Status)
	assert.Equal(t, uint32(4311), detail.ProcessID)
	assert.Empty(t, detail.BinaryPath) // ExecStart 无 path= → 空

	d2, err := parseSystemdStatus("bar.service", "Active=active (running)\nExecStart={path=/usr/bin/bar}")
	require.NoError(t, err)
	assert.Equal(t, "running", d2.Status)
	assert.Equal(t, "/usr/bin/bar", d2.BinaryPath) // 无空白结尾 → Trim("{}") 路径

	d3, err := parseSystemdStatus("baz.service", "Loaded: loaded (/lib/systemd/baz.service; enabled; vendor preset: enabled)")
	require.NoError(t, err)
	assert.Equal(t, "auto", d3.StartType)

	d4, err := parseSystemdStatus("qux.service", "Active: active (running)\nSubState=dead")
	require.NoError(t, err)
	assert.Equal(t, "stopped", d4.Status) // SubState 细化覆盖 Active:
}

// --- sysinfo_linux.go: parseCronFile 格式分支 ---

func TestParseCronFile_BranchesV9(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab")
	content := "# comment line\n\n" +
		"0 0 * * * root /usr/bin/backup\n" + // 系统格式（含 user 列）
		"* * * * *\n" + // 5 列：user 格式分支，command 为空 → 不产出
		"this is not a cron line\n" + // 字段不像 cron 调度 → 跳过
		"@0 * * * * root /opt/task.sh\n" // @ 前缀分支（fields[0] 含数字可通过调度校验）
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	jobs := parseCronFile(path, "root")
	require.Len(t, jobs, 2)

	byCmd := map[string]CronJob{}
	for _, j := range jobs {
		byCmd[j.Command] = j
	}
	backup, ok := byCmd["/usr/bin/backup"]
	require.True(t, ok, "system-format job missing")
	assert.Equal(t, "0 0 * * *", backup.Schedule)

	task, ok := byCmd["* * * * root /opt/task.sh"]
	require.True(t, ok, "@-schedule job missing")
	assert.Equal(t, "@0", task.Schedule)
	assert.Equal(t, "root", task.User)
}

// --- ops_metrics.go ---

func TestMetricsCollectorStartReporting_ZeroIntervalV9(t *testing.T) {
	c := NewMetricsCollector("agent-v9-report")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reported := make(chan struct{}, 1)
	go func() {
		c.StartReporting(ctx, 0, func(report *opsv1.MetricsReport) {
			select {
			case reported <- struct{}{}:
			default:
			}
		})
	}()

	select {
	case <-reported:
	case <-time.After(5 * time.Second):
		t.Fatal("immediate report on start missing")
	}
	cancel() // interval 归一为 30s；ctx 取消后循环退出
}

func TestGetSystemInfo_OpsConfigVariantsV9(t *testing.T) {
	withCfg := GetSystemInfo("agent-v9", "1.0", &OpsConfig{
		Enabled:          true,
		AllowRestart:     true,
		AllowExec:        false,
		ManagedProcesses: map[string]ManagedProcessConfig{"p": {Command: "x"}},
	})
	require.NotNil(t, withCfg.OpsStatus)
	assert.True(t, withCfg.OpsStatus.Enabled)
	assert.True(t, withCfg.OpsStatus.AllowRestart)
	assert.False(t, withCfg.OpsStatus.AllowExec)
	assert.Equal(t, []string{"p"}, withCfg.OpsStatus.ManagedProcesses)

	withoutCfg := GetSystemInfo("agent-v9", "1.0", nil)
	require.NotNil(t, withoutCfg.OpsStatus)
	assert.False(t, withoutCfg.OpsStatus.Enabled)
}

// --- provider.go ---

func TestProviderManagerStartHeartbeat_ContextCancelledV9(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(`
providers:
  hbv9:
    enabled: true
    type: openapi
    config:
      baseUrl: http://127.0.0.1:1
`), 0o600))

	t.Setenv("CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT", "10ms")
	m := NewProviderManager(agentlocal.NewLocalStore(), dir, nil)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, m.Load(ctx))
	time.Sleep(30 * time.Millisecond)
	cancel() // 心跳 goroutine 收到 Done 后退出
	time.Sleep(80 * time.Millisecond)
	require.NoError(t, m.Close())
}

func TestProviderManagerInitProvider_InitFailureV9(t *testing.T) {
	m := NewProviderManager(agentlocal.NewLocalStore(), t.TempDir(), nil)
	defer m.Close()

	// timeout 非法导致 openapi provider Init 失败；SyncExtensionProviders 记录错误并跳过。
	require.NoError(t, m.SyncExtensionProviders(context.Background(), map[string]ProviderEntry{
		"broken": {
			Enabled: true,
			Type:    "openapi",
			Config:  map[string]interface{}{"timeout": "not-a-duration"},
		},
	}))
	assert.False(t, m.IsPlatformFunction("broken.anything"))
}

// --- upstream.go: buildProviders 版本回填 ---

func TestBuildProviders_VersionBackfillV9(t *testing.T) {
	now := time.Now()
	localData := map[string][]agentlocal.Instance{
		"fn.v9": {
			{ProviderID: "svc-v9", Version: "", LastSeen: now},                       // 建立条目，版本空
			{ProviderID: "svc-v9", Version: "9.9.9", LastSeen: now.Add(time.Second)}, // 补齐版本
		},
	}
	providers := buildProviders(localData, nil)
	require.Len(t, providers, 1)
	assert.Equal(t, "9.9.9", providers[0].Version)
}
