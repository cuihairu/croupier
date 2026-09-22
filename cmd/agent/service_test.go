package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeService 是 service.Service 的最小假实现，只记录 Stop 调用。
// platform/status/statusErr 可注入，驱动 runServiceStatus 的三态与平台分支。
type fakeService struct {
	stopped   chan struct{}
	platform  string
	status    service.Status
	statusErr error
}

func newFakeService() *fakeService { return &fakeService{stopped: make(chan struct{}, 4)} }

func (f *fakeService) Run() error       { return nil }
func (f *fakeService) Start() error     { return nil }
func (f *fakeService) Stop() error      { f.stopped <- struct{}{}; return nil }
func (f *fakeService) Restart() error   { return nil }
func (f *fakeService) Install() error   { return nil }
func (f *fakeService) Uninstall() error { return nil }
func (f *fakeService) Logger(errs chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *fakeService) SystemLogger(errs chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *fakeService) String() string { return "fake" }
func (f *fakeService) Platform() string {
	if f.platform == "" {
		return "test"
	}
	return f.platform
}
func (f *fakeService) Status() (service.Status, error) {
	if f.statusErr != nil {
		return service.StatusUnknown, f.statusErr
	}
	if f.status == 0 {
		return service.StatusUnknown, nil
	}
	return f.status, nil
}

// saveServiceGlobals 还原测试触碰的全局状态。
func saveServiceGlobals(t *testing.T) {
	t.Helper()
	oldCfgFile := cfgFile
	t.Cleanup(func() { cfgFile = oldCfgFile })
}

func waitStopped(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for svc.Stop()")
	}
}

// Start 的 runAgent 失败分支：全局 cfgFile 为空 → 启动报错 → 假服务的 Stop 被调。
func TestAgentServiceStart_RunAgentFailure(t *testing.T) {
	saveServiceGlobals(t)
	cfgFile = ""

	svc := newAgentService("")
	fake := newFakeService()
	require.NoError(t, svc.Start(fake))
	waitStopped(t, fake.stopped)

	// Stop 方法：取消 ctx 并记录日志
	require.NoError(t, svc.Stop(fake))
}

// Start 的 ctx 取消分支：取消后 ctx.Done goroutine 触发 svc.Stop。
func TestAgentServiceStart_ContextCancel(t *testing.T) {
	saveServiceGlobals(t)
	cfgFile = ""

	svc := newAgentService("")
	fake := newFakeService()
	require.NoError(t, svc.Start(fake))
	svc.cancel()
	waitStopped(t, fake.stopped)
}

// writeAgentConfigForStart 播种一份能真正起监听的 agent 配置（空上游地址）。
func writeAgentConfigForStart(t *testing.T, dir, port string) string {
	t.Helper()
	path := filepath.Join(dir, "agent.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
server:
  addr: ""
  insecure: true
agent:
  gameId: demo-game
  env: development
  localAddr: "127.0.0.1:`+port+`"
`), 0o644))
	return path
}

// Start 的 runAgent 成功分支：网关监听就绪后打「服务已启动」，SIGTERM 收口。
func TestAgentServiceStart_RunAgentSuccess(t *testing.T) {
	saveServiceGlobals(t)
	cfgFile = writeAgentConfigForStart(t, t.TempDir(), "18932")

	svc := newAgentService(cfgFile)
	fake := newFakeService()
	require.NoError(t, svc.Start(fake))

	// 监听就绪 → runAgent 已进入信号等待（即成功分支）
	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:18932", 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("local gateway never came up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
	// 服务管理器语义：Stop 由管理器调用 → cancel ctx → ctx.Done goroutine 触发 svc.Stop
	require.NoError(t, svc.Stop(fake))
	waitStopped(t, fake.stopped)
}

func TestCreateService_ConfigPathVariants(t *testing.T) {
	oldDir := serviceConfigDir
	oldArgs := os.Args
	t.Cleanup(func() { serviceConfigDir = oldDir; os.Args = oldArgs })

	// 相对路径 + os.Args 带 --config → getFlagValue 命中覆盖
	serviceConfigDir = "relative-etc"
	os.Args = []string{"prog", "--config", "/abs/from/flag.yaml"}
	_, err := createService()
	require.NoError(t, err)

	// config-dir 为空 → defaultConfigDir 兜底
	serviceConfigDir = ""
	os.Args = []string{"prog"}
	_, err = createService()
	require.NoError(t, err)
}

func TestCreateService(t *testing.T) {
	svc, err := createService()
	require.NoError(t, err)
	// kardianos String() 语义：有 DisplayName 时返回 DisplayName
	assert.Equal(t, serviceDisplayName, svc.String())
	assert.NotEmpty(t, svc.Platform())

	// 空配置名 → service.New 拒绝
	oldName := serviceName
	serviceName = ""
	_, err = createService()
	serviceName = oldName
	assert.Error(t, err)
}

func TestGetFlagValue(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"长格式带值", []string{"prog", "--config", "/etc/agent.yaml"}, "/etc/agent.yaml"},
		{"长格式等号", []string{"prog", "--config=/x/agent.yaml"}, "/x/agent.yaml"},
		{"短格式带值", []string{"prog", "-c", "/short.yaml"}, "/short.yaml"},
		{"末尾无值", []string{"prog", "--config"}, ""},
		{"值是另一个旗标", []string{"prog", "--config", "--debug"}, ""},
		{"等号空值", []string{"prog", "--config="}, ""},
		{"不存在", []string{"prog", "--other", "v"}, ""},
		{"空参数表", []string{"prog"}, ""},
	}
	for _, tc := range cases {
		os.Args = tc.args
		assert.Equal(t, tc.want, getFlagValue("config"), tc.name)
	}
}

func TestRunServiceRun_MissingConfig(t *testing.T) {
	saveServiceGlobals(t)
	oldDir := serviceConfigDir
	serviceConfigDir = t.TempDir() // 目录内无 agent.yaml
	t.Cleanup(func() { serviceConfigDir = oldDir })

	err := runServiceRun(nil, nil)
	assert.ErrorContains(t, err, "配置文件不存在")
}

func TestDefaultConfigDir(t *testing.T) {
	// 1. 环境变量优先
	t.Setenv("CROUPIER_CONFIG_DIR", "/from/env")
	assert.Equal(t, "/from/env", defaultConfigDir())

	os.Unsetenv("CROUPIER_CONFIG_DIR")
	t.Cleanup(func() { _ = os.Setenv("CROUPIER_CONFIG_DIR", "") })

	// 2. 可执行文件同级的 etc 目录存在则用之
	execPath, err := os.Executable()
	require.NoError(t, err)
	etcDir := filepath.Join(filepath.Dir(execPath), "etc")
	require.NoError(t, os.MkdirAll(etcDir, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(etcDir) })
	assert.Equal(t, etcDir, defaultConfigDir())

	// 3. 无 etc 目录 → 回退系统配置目录（linux 分支）
	require.NoError(t, os.RemoveAll(etcDir))
	assert.Equal(t, "/etc/croupier", defaultConfigDir())
}

// runServiceStatus 只读：状态查询在本环境对未安装服务报错（或打印未安装提示）。
// 断言按实际返回分支收敛，避免依赖部署环境是否装了同名服务。
func TestRunServiceStatus(t *testing.T) {
	err := runServiceStatus(nil, nil)
	if err != nil {
		assert.ErrorContains(t, err, "查询服务状态失败")
		return
	}
	// 平台未报错 → 必然打印服务名（运行中/已停止/未安装 三态之一）
	// runServiceStatus 直接写 stdout，这里只验证无 panic 且返回 nil。
}

func TestPrintVersionInfo(t *testing.T) {
	oldVer, oldCommit, oldBuild := Version, GitCommit, BuildTime
	t.Cleanup(func() { Version, GitCommit, BuildTime = oldVer, oldCommit, oldBuild })

	// 仅版本行（GitCommit=unknown、BuildTime 为空时）
	Version, GitCommit, BuildTime = "vtest", "unknown", ""
	out := captureAgentStdout(t, PrintVersionInfo)
	assert.Contains(t, out, "Croupier Agent vtest")
	assert.NotContains(t, out, "Git commit")

	// 全量三行
	Version, GitCommit, BuildTime = "vtest", "abc123", "2026-01-01"
	out = captureAgentStdout(t, PrintVersionInfo)
	assert.True(t, strings.Contains(out, "Git commit: abc123") && strings.Contains(out, "Build time: 2026-01-01"))
}

// 五个 service 变更命令的入口守卫：服务名缺失 → createService 报错 →
// 统一 "创建服务失败"。真实的 install/start/stop/restart 会做系统级变更
// （systemd unit 安装、服务启停），单测不执行该部分（豁免入档）。
func TestRunServiceCommands_CreateFailure(t *testing.T) {
	saveServiceGlobals(t)
	serviceConfigDir = t.TempDir()
	serviceName = "" // createService 拒绝空服务名

	cmd := &cobra.Command{}
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"install":   runServiceInstall,
		"uninstall": runServiceUninstall,
		"start":     runServiceStart,
		"stop":      runServiceStop,
		"restart":   runServiceRestart,
	} {
		err := run(cmd, nil)
		assert.ErrorContains(t, err, "创建服务失败", name)
	}
}
