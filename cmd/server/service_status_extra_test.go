package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kardianos/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureServerStdout 捕获函数执行期间的 stdout 输出。
func captureServerStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

// platformFamily 是 kardianos 平台串的归一函数：linux+systemd 返回
// "linux-systemd" 而非 "linux"，历史代码精确比较恒 false（真实缺陷，
// 2026-09-22 修复）。此表锁定归一语义本身。
func TestPlatformFamily(t *testing.T) {
	cases := []struct{ in, want string }{
		{"linux-systemd", "linux"},
		{"linux-upstart", "linux"},
		{"linux-openrc", "linux"},
		{"linux-SysV", "linux"},
		{"windows-service", "windows"},
		{"darwin-launchd", "darwin"},
		{"test", "test"},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, platformFamily(c.in), c.in)
	}
}

// createServerService 的 linux 分支：kardianos 在 linux+systemd 返回
// "linux-systemd"，修复前 == "linux" 恒 false，unit 的 network-online 依赖
// 与 croupier 运行用户从未写入（经 service.New 接缝捕获 svcConfig 断言）。
func TestCreateServerService_LinuxUnitOptions(t *testing.T) {
	saveServerServiceGlobals(t)
	oldDisplay := serviceDisplayName
	serviceConfigDir = t.TempDir()
	serviceName = "croupier-server-test"
	t.Cleanup(func() { serviceDisplayName = oldDisplay })

	var gotCfg *service.Config
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, cfg *service.Config) (service.Service, error) {
		gotCfg = cfg
		return newFakeServerSvc(), nil
	}
	t.Cleanup(func() { newKardianosService = restore })

	svc, err := createServerService()
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.NotNil(t, gotCfg)
	assert.Equal(t, "croupier-server-test", gotCfg.Name)
	assert.Equal(t, "croupier", gotCfg.UserName)
	assert.Contains(t, gotCfg.Dependencies, "After=network-online.target")
	assert.Contains(t, gotCfg.Dependencies, "Wants=network-online.target")
}

// runServerServiceStatus 三态与平台提示：经 service.New 接缝注入可控 fake，
// 不碰真实 systemd。
func TestRunServerServiceStatus_States(t *testing.T) {
	saveServerServiceGlobals(t)
	oldDisplay := serviceDisplayName
	serviceConfigDir = t.TempDir()
	serviceName = "croupier-server-test"
	serviceDisplayName = "Croupier Server Test"
	t.Cleanup(func() { serviceDisplayName = oldDisplay })

	cases := []struct {
		name     string
		status   service.Status
		platform string
		wantOut  []string
		notOut   []string
	}{
		{
			name:     "running-linux",
			status:   service.StatusRunning,
			platform: "linux-systemd",
			wantOut:  []string{"运行中 ✓", "平台: linux-systemd", "systemctl status croupier-server-test", "journalctl -u croupier-server-test -f"},
			notOut:   []string{"PowerShell"},
		},
		{
			name:     "stopped-linux",
			status:   service.StatusStopped,
			platform: "linux-systemd",
			wantOut:  []string{"已停止", "systemctl status croupier-server-test"},
		},
		{
			name:     "unknown",
			status:   service.StatusUnknown,
			platform: "linux-systemd",
			wantOut:  []string{"未安装", "请先执行: " + rootCmd.Name() + " service install"},
		},
		{
			name:     "running-windows",
			status:   service.StatusRunning,
			platform: "windows-service",
			wantOut:  []string{"运行中 ✓", "平台: windows-service", "PowerShell: Get-Service croupier-server-test", "services.msc"},
			notOut:   []string{"systemctl"},
		},
		{
			name:     "stopped-windows-no-hint",
			status:   service.StatusStopped,
			platform: "windows-service",
			wantOut:  []string{"已停止"},
			notOut:   []string{"PowerShell", "systemctl"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore := newKardianosService
			newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
				f := newFakeServerSvc()
				f.platform = tc.platform
				f.status = tc.status
				return f, nil
			}
			t.Cleanup(func() { newKardianosService = restore })

			var err error
			out := captureServerStdout(t, func() { err = runServerServiceStatus(nil, nil) })
			require.NoError(t, err)
			for _, want := range tc.wantOut {
				assert.Contains(t, out, want)
			}
			for _, ban := range tc.notOut {
				assert.NotContains(t, out, ban)
			}
		})
	}

	// Status 查询失败 → 统一报错
	t.Run("status-error", func(t *testing.T) {
		restore := newKardianosService
		newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
			f := newFakeServerSvc()
			f.statusErr = assert.AnError
			return f, nil
		}
		t.Cleanup(func() { newKardianosService = restore })

		err := runServerServiceStatus(nil, nil)
		assert.ErrorContains(t, err, "查询服务状态失败")
	})

	// createServerService 失败 → 入口守卫
	t.Run("create-failure", func(t *testing.T) {
		restore := newKardianosService
		newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
			return nil, assert.AnError
		}
		t.Cleanup(func() { newKardianosService = restore })

		err := runServerServiceStatus(nil, nil)
		assert.ErrorContains(t, err, "创建服务失败")
	})
}

// runServerServiceRun 的 createServerService 失败变体：配置文件存在、
// 服务名缺失 → service.New 拒绝 → "创建服务对象失败"。
func TestRunServerServiceRun_CreateFailure(t *testing.T) {
	saveServerServiceGlobals(t)
	oldName := serviceName
	dir := t.TempDir()
	serviceConfigDir = dir
	serviceName = ""
	t.Cleanup(func() { serviceName = oldName })
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.yaml"), []byte("log:\n  level: info\n"), 0o600))

	err := runServerServiceRun(nil, nil)
	assert.ErrorContains(t, err, "创建服务对象失败")
}

// createServerService 的 Abs 失败回退链：configPath 为相对路径且 Getwd 失败
// （删除 cwd 构造）→ 回退到「可执行文件同级目录」拼接，相对目录不得被
// 静默丢弃（Abs 的 err 返回会清零承接变量，历史实现曾在拼接时丢目录）。
func TestCreateServerService_AbsFailureFallback(t *testing.T) {
	saveServerServiceGlobals(t)
	oldArgs := os.Args
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Args = oldArgs
		require.NoError(t, os.Chdir(oldWd))
	})

	gone := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.MkdirAll(gone, 0o755))
	require.NoError(t, os.Chdir(gone))
	require.NoError(t, os.RemoveAll(gone)) // Getwd 此后失败

	serviceConfigDir = "relative-etc"
	os.Args = []string{"prog"}

	var gotCfg *service.Config
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, cfg *service.Config) (service.Service, error) {
		gotCfg = cfg
		return newFakeServerSvc(), nil
	}
	t.Cleanup(func() { newKardianosService = restore })

	_, err = createServerService()
	require.NoError(t, err)
	require.NotNil(t, gotCfg)
	assert.Equal(t,
		filepath.Join(filepath.Dir(gotCfg.Executable), "relative-etc", "server.yaml"),
		gotCfg.Arguments[1],
	)
}

// runServerServiceRun：configDir 为空 → defaultServerConfigDir 兜底（环境
// 变量注入保证确定性）→ 目录内无 server.yaml → 配置缺失报错。
func TestRunServerServiceRun_DefaultConfigDir(t *testing.T) {
	saveServerServiceGlobals(t)
	oldDir := serviceConfigDir
	serviceConfigDir = ""
	t.Cleanup(func() { serviceConfigDir = oldDir })
	t.Setenv("CROUPIER_CONFIG_DIR", t.TempDir()) // 目录内无 server.yaml

	err := runServerServiceRun(nil, nil)
	assert.ErrorContains(t, err, "配置文件不存在")
}

// runServerServiceRun 的启动成功路径：配置文件存在 → createServerService
// （接缝注入 fake）→ svcObj.Start 同步检查通过后立即返回 nil →
// runServerServiceRun 返回 nil；后台 runServerFunc 替身返回错误，触发
// fake 的 Stop（waitServerStopped 确认）。
func TestRunServerServiceRun_Starts(t *testing.T) {
	saveServerServiceGlobals(t)
	oldDir, oldName := serviceConfigDir, serviceName
	dir := t.TempDir()
	serviceConfigDir = dir
	serviceName = "croupier-server-test"
	t.Cleanup(func() { serviceConfigDir, serviceName = oldDir, oldName })
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.yaml"), []byte("log:\n  level: info\n"), 0o600))
	stubRunServerFunc(t, assert.AnError) // 后台启动失败 → svc.Stop

	fake := newFakeServerSvc()
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
		return fake, nil
	}
	t.Cleanup(func() { newKardianosService = restore })

	assert.NoError(t, runServerServiceRun(nil, nil))
	waitServerStopped(t, fake.stopped)
}
