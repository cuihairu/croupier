package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kardianos/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// createService 的 linux 分支：kardianos 在 linux+systemd 返回
// "linux-systemd"，修复前 == "linux" 恒 false，unit 的 network-online 依赖
// 与 croupier 运行用户从未写入（经 service.New 接缝捕获 svcConfig 断言）。
func TestCreateService_LinuxUnitOptions(t *testing.T) {
	saveServiceGlobals(t)
	oldDir, oldName := serviceConfigDir, serviceName
	serviceConfigDir = t.TempDir()
	serviceName = "croupier-agent-test"
	t.Cleanup(func() { serviceConfigDir, serviceName = oldDir, oldName })

	var gotCfg *service.Config
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, cfg *service.Config) (service.Service, error) {
		gotCfg = cfg
		return newFakeService(), nil
	}
	t.Cleanup(func() { newKardianosService = restore })

	svc, err := createService()
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.NotNil(t, gotCfg)
	assert.Equal(t, "croupier-agent-test", gotCfg.Name)
	assert.Equal(t, "croupier", gotCfg.UserName)
	assert.Contains(t, gotCfg.Dependencies, "After=network-online.target")
	assert.Contains(t, gotCfg.Dependencies, "Wants=network-online.target")
}

// runServiceStatus 三态与平台提示：经 service.New 接缝注入可控 fake，
// 不碰真实 systemd。
func TestRunServiceStatus_States(t *testing.T) {
	saveServiceGlobals(t)
	oldDir, oldName, oldDisplay := serviceConfigDir, serviceName, serviceDisplayName
	serviceConfigDir = t.TempDir()
	serviceName = "croupier-agent-test"
	serviceDisplayName = "Croupier Agent Test"
	t.Cleanup(func() { serviceConfigDir, serviceName, serviceDisplayName = oldDir, oldName, oldDisplay })

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
			wantOut:  []string{"运行中 ✓", "平台: linux-systemd", "systemctl status croupier-agent-test", "journalctl -u croupier-agent-test -f"},
			notOut:   []string{"PowerShell"},
		},
		{
			name:     "stopped-linux",
			status:   service.StatusStopped,
			platform: "linux-systemd",
			wantOut:  []string{"已停止", "systemctl status croupier-agent-test"},
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
			wantOut:  []string{"运行中 ✓", "平台: windows-service", "PowerShell: Get-Service croupier-agent-test", "services.msc"},
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
				f := newFakeService()
				f.platform = tc.platform
				f.status = tc.status
				return f, nil
			}
			t.Cleanup(func() { newKardianosService = restore })

			var err error
			out := captureAgentStdout(t, func() { err = runServiceStatus(nil, nil) })
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
			f := newFakeService()
			f.statusErr = assert.AnError
			return f, nil
		}
		t.Cleanup(func() { newKardianosService = restore })

		err := runServiceStatus(nil, nil)
		assert.ErrorContains(t, err, "查询服务状态失败")
	})

	// createService 失败 → 入口守卫
	t.Run("create-failure", func(t *testing.T) {
		restore := newKardianosService
		newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
			return nil, assert.AnError
		}
		t.Cleanup(func() { newKardianosService = restore })

		err := runServiceStatus(nil, nil)
		assert.ErrorContains(t, err, "创建服务失败")
	})
}

// runServiceRun 的 createService 失败变体：配置文件存在、服务名缺失 →
// service.New 拒绝 → "创建服务对象失败"。
func TestRunServiceRun_CreateFailure(t *testing.T) {
	saveServiceGlobals(t)
	oldDir, oldName := serviceConfigDir, serviceName
	dir := t.TempDir()
	serviceConfigDir = dir
	serviceName = "" // createService → service.New 拒绝空服务名
	t.Cleanup(func() { serviceConfigDir, serviceName = oldDir, oldName })
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("log:\n  level: info\n"), 0o600))

	err := runServiceRun(nil, nil)
	assert.ErrorContains(t, err, "创建服务对象失败")
}

// createService 的 Abs 失败回退链：configPath 为相对路径且 Getwd 失败
// （删除 cwd 构造）→ filepath.Abs(execPath) 对绝对输入不查 Getwd 恒成功
// → 回退到「可执行文件同级目录」拼接。
func TestCreateService_AbsFailureFallback(t *testing.T) {
	saveServiceGlobals(t)
	oldDir, oldArgs := serviceConfigDir, os.Args
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		serviceConfigDir, os.Args = oldDir, oldArgs
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
		return newFakeService(), nil
	}
	t.Cleanup(func() { newKardianosService = restore })

	_, err = createService()
	require.NoError(t, err)
	require.NotNil(t, gotCfg)
	// 回退：相对路径挂到可执行文件目录下
	assert.Equal(t,
		filepath.Join(filepath.Dir(gotCfg.Executable), "relative-etc", "agent.yaml"),
		gotCfg.Arguments[1],
	)
}

// runServiceRun：configDir 为空 → defaultConfigDir 兜底（环境变量注入保证
// 确定性）→ 目录内无 agent.yaml → 配置缺失报错。
func TestRunServiceRun_DefaultConfigDir(t *testing.T) {
	saveServiceGlobals(t)
	oldDir := serviceConfigDir
	serviceConfigDir = ""
	t.Cleanup(func() { serviceConfigDir = oldDir })
	t.Setenv("CROUPIER_CONFIG_DIR", t.TempDir()) // 目录内无 agent.yaml

	err := runServiceRun(nil, nil)
	assert.ErrorContains(t, err, "配置文件不存在")
}

// runServiceRun 的启动成功路径：配置文件存在 → createService（接缝注入
// fake）→ svcObj.Start 立即返回 nil → runServiceRun 返回 nil；随后后台
// runAgent 因无效配置失败，触发 fake 的 Stop（waitStopped 确认）。
func TestRunServiceRun_Starts(t *testing.T) {
	saveServiceGlobals(t)
	oldDir, oldName := serviceConfigDir, serviceName
	dir := t.TempDir()
	serviceConfigDir = dir
	serviceName = "croupier-agent-test"
	t.Cleanup(func() { serviceConfigDir, serviceName = oldDir, oldName })
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("[]\n"), 0o600)) // 无效配置 → runAgent 快速失败

	fake := newFakeService()
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
		return fake, nil
	}
	t.Cleanup(func() { newKardianosService = restore })

	assert.NoError(t, runServiceRun(nil, nil))
	waitStopped(t, fake.stopped) // Start 内 runAgent 失败 → svc.Stop()
}
