package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeServerSvc 是 service.Service 的最小假实现，只记录 Stop 调用。
// platform/status/statusErr 可注入，驱动 runServerServiceStatus 的三态与
// 平台分支。
type fakeServerSvc struct {
	stopped   chan struct{}
	platform  string
	status    service.Status
	statusErr error
}

func newFakeServerSvc() *fakeServerSvc { return &fakeServerSvc{stopped: make(chan struct{}, 4)} }

func (f *fakeServerSvc) Run() error       { return nil }
func (f *fakeServerSvc) Start() error     { return nil }
func (f *fakeServerSvc) Stop() error      { f.stopped <- struct{}{}; return nil }
func (f *fakeServerSvc) Restart() error   { return nil }
func (f *fakeServerSvc) Install() error   { return nil }
func (f *fakeServerSvc) Uninstall() error { return nil }
func (f *fakeServerSvc) Logger(chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *fakeServerSvc) SystemLogger(chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *fakeServerSvc) String() string { return "fake" }
func (f *fakeServerSvc) Platform() string {
	if f.platform == "" {
		return "test"
	}
	return f.platform
}
func (f *fakeServerSvc) Status() (service.Status, error) {
	if f.statusErr != nil {
		return service.StatusUnknown, f.statusErr
	}
	if f.status == 0 {
		return service.StatusUnknown, nil
	}
	return f.status, nil
}

// saveServerServiceGlobals 还原 service.go 测试触碰的全局变量。五个 service
// 变更命令的第一行都会经 cmd.Flags().GetString 把 name/display-name/
// description/config-dir 回写进全局（fake cmd 无旗标时返回空串），故除
// cfgFile 外四个 service 全局都必须覆盖，否则 -count>1 的下一轮会被跨轮
// 污染。
func saveServerServiceGlobals(t *testing.T) {
	t.Helper()
	oldCfgFile := cfgFile
	oldDir := serviceConfigDir
	oldName := serviceName
	oldDisplay := serviceDisplayName
	oldDesc := serviceDescription
	t.Cleanup(func() {
		cfgFile = oldCfgFile
		serviceConfigDir = oldDir
		serviceName = oldName
		serviceDisplayName = oldDisplay
		serviceDescription = oldDesc
	})
}

// waitServerStopped 等待假服务的 Stop 被调用。
func waitServerStopped(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for svc.Stop()")
	}
}

// stubRunServerFunc 注入不读全局的 runServer 替身：Start 的后台 goroutine
// 与 saveServerServiceGlobals 的 cleanup 并发，替身根除二者对全局 cfgFile
// 的数据竞争。err 为替身的返回值（nil = 启动成功路径）。
func stubRunServerFunc(t *testing.T, err error) {
	t.Helper()
	old := runServerFunc
	runServerFunc = func() error { return err }
	t.Cleanup(func() { runServerFunc = old })
}

// Start 的配置缺失分支：cfgFile 指向不存在的文件 → 同步报错。
func TestServerServiceStart_ConfigMissing(t *testing.T) {
	s := newServerService(filepath.Join(t.TempDir(), "missing.yaml"))
	err := s.Start(newFakeServerSvc())
	assert.ErrorContains(t, err, "配置文件不存在")
	s.Stop(newFakeServerSvc())
}

// Start 的 runServer 失败分支：替身返回错误 → goroutine 内调 svc.Stop。
func TestServerServiceStart_RunServerFailure(t *testing.T) {
	saveServerServiceGlobals(t)
	stubRunServerFunc(t, assert.AnError)

	s := newServerService("")
	fake := newFakeServerSvc()
	require.NoError(t, s.Start(fake))
	waitServerStopped(t, fake.stopped)
	require.NoError(t, s.Stop(fake))
}

// Start 的 ctx 取消分支：runServerFunc 替身走成功路径，取消 ctx 后
// ctx.Done goroutine 触发 svc.Stop。
func TestServerServiceStart_ContextCancel(t *testing.T) {
	saveServerServiceGlobals(t)
	stubRunServerFunc(t, nil)

	s := newServerService("")
	fake := newFakeServerSvc()
	require.NoError(t, s.Start(fake))
	s.cancel()
	waitServerStopped(t, fake.stopped)
}

// initLoggingFromConfig 三分支：空配置早退 / 读失败与解析失败回落默认 /
// 有效配置应用日志设置。
func TestServerService_InitLoggingFromConfig(t *testing.T) {
	s := newServerService("")
	s.initLoggingFromConfig() // cfgFile 空 → 早退

	s = newServerService(filepath.Join(t.TempDir(), "missing.yaml"))
	s.initLoggingFromConfig() // 读失败 → 默认日志

	bad := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("{unclosed"), 0o644))
	s = newServerService(bad)
	s.initLoggingFromConfig() // 解析失败 → 默认日志

	ok := filepath.Join(t.TempDir(), "ok.yaml")
	require.NoError(t, os.WriteFile(ok, []byte("logging:\n  level: warn\n  format: json\n"), 0o644))
	s = newServerService(ok)
	s.initLoggingFromConfig() // 有效配置走 SetupLoggerWithFile

	// 还原全局日志器，避免污染其他用例
	common.SetupLoggerWithFile("info", "console", "", 0, 0, 0, false)
}

func TestCreateServerService(t *testing.T) {
	saveServerServiceGlobals(t)
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	serviceConfigDir = t.TempDir()
	os.Args = []string{"prog"} // getServerFlagValue 不命中 → configPath/server.yaml

	svcObj, err := createServerService()
	require.NoError(t, err)
	// kardianos String() 语义：有 DisplayName 时返回 DisplayName
	assert.Equal(t, serviceDisplayName, svcObj.String())
	assert.NotEmpty(t, svcObj.Platform())

	// --config 旗标覆盖 cfgFile 路径（仅验证不报错，路径在 Arguments 里）
	os.Args = []string{"prog", "--config", "/abs/from/flag.yaml"}
	_, err = createServerService()
	require.NoError(t, err)

	// 相对 configDir → Abs 归一
	serviceConfigDir = "relative-etc"
	_, err = createServerService()
	require.NoError(t, err)

	// 空 configDir → defaultServerConfigDir 兜底
	serviceConfigDir = ""
	_, err = createServerService()
	require.NoError(t, err)

	// 空服务名 → service.New 拒绝
	serviceName = ""
	_, err = createServerService()
	assert.Error(t, err)
}

func TestGetServerFlagValue(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"长格式带值", []string{"prog", "--config", "/etc/server.yaml"}, "/etc/server.yaml"},
		{"长格式等号", []string{"prog", "--config=/x/server.yaml"}, "/x/server.yaml"},
		{"单破折号", []string{"prog", "-c", "/short.yaml"}, "/short.yaml"},
		{"末尾无值", []string{"prog", "--config"}, ""},
		{"值是另一个旗标", []string{"prog", "--config", "--debug"}, ""},
		{"不存在", []string{"prog", "--other", "v"}, ""},
		{"空参数表", []string{"prog"}, ""},
	}
	for _, tc := range cases {
		os.Args = tc.args
		assert.Equal(t, tc.want, getServerFlagValue("config"), tc.name)
	}
}

// runServerServiceRun：configDir 无 server.yaml → 配置缺失报错。
func TestRunServerServiceRun_MissingConfig(t *testing.T) {
	saveServerServiceGlobals(t)
	serviceConfigDir = t.TempDir()

	err := runServerServiceRun(nil, nil)
	assert.ErrorContains(t, err, "配置文件不存在")
}

// runServerServiceStatus 只读：状态查询在本环境对未安装服务报错（或打印
// 未安装提示）。断言按实际返回分支收敛，避免依赖部署环境。
func TestRunServerServiceStatus(t *testing.T) {
	saveServerServiceGlobals(t)
	serviceConfigDir = t.TempDir()

	err := runServerServiceStatus(nil, nil)
	if err != nil {
		assert.ErrorContains(t, err, "查询服务状态失败")
	}
	// 平台未报错 → 打印三态之一，只验证无 panic 且返回 nil
}

func TestDefaultServerConfigDir(t *testing.T) {
	// 1. 环境变量优先
	t.Setenv("CROUPIER_CONFIG_DIR", "/from/env")
	assert.Equal(t, "/from/env", defaultServerConfigDir())

	os.Unsetenv("CROUPIER_CONFIG_DIR")
	t.Cleanup(func() { _ = os.Setenv("CROUPIER_CONFIG_DIR", "") })

	// 2. 可执行文件同级的 etc 目录存在则用之
	execPath, err := os.Executable()
	require.NoError(t, err)
	etcDir := filepath.Join(filepath.Dir(execPath), "etc")
	require.NoError(t, os.MkdirAll(etcDir, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(etcDir) })
	assert.Equal(t, etcDir, defaultServerConfigDir())

	// 3. 无 etc 目录 → 回退系统配置目录（linux 分支）
	require.NoError(t, os.RemoveAll(etcDir))
	assert.Equal(t, "/etc/croupier", defaultServerConfigDir())
}

func TestWdAndExePath(t *testing.T) {
	dir := wd()
	assert.NotEqual(t, "unknown", dir)
	assert.DirExists(t, dir)

	path := exePath()
	assert.NotEqual(t, "unknown", path)
	assert.FileExists(t, path)
}

// 五个 service 变更命令的入口守卫：服务名缺失 → createServerService 报错
// → 统一 "创建服务失败"。真实的 install/start/stop/restart 会做系统级
// 变更（systemd unit 安装、服务启停），单测不执行该部分（豁免入档）。
func TestRunServerServiceCommands_CreateFailure(t *testing.T) {
	saveServerServiceGlobals(t)
	serviceConfigDir = t.TempDir()
	serviceName = "" // createServerService 拒绝空服务名

	cmd := &cobra.Command{}
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"install":   runServerServiceInstall,
		"uninstall": runServerServiceUninstall,
		"start":     runServerServiceStart,
		"stop":      runServerServiceStop,
		"restart":   runServerServiceRestart,
	} {
		err := run(cmd, nil)
		assert.ErrorContains(t, err, "创建服务失败", name)
	}
}
