// 覆盖率巡检第二十五轮（wt-api）：cmd/agent service 变更命令主体——
// install/uninstall/start/stop/restart 五体经 newKardianosService 接缝注入
// 可控 fake（status/statusErr + 各方法错误 + 调用计数）分支矩阵全数直测，
// 不碰真实 systemd。cmd-2 豁免随之收窄：变更面只在「未注入接缝」的真实
// 路径上属系统级变更；接缝之下五体的每个错误/早退/成功分支均可构造。
// 调用形态沿用 service_status_extra_test.go 先例：直接调 run* 函数体
// （install 传空 cobra.Command——其首行经 Flags().GetString 回写全局，
// saveServiceGlobals 兜底还原）。
package main

import (
	"errors"
	"testing"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// svcCmdFake：五个变更命令的可控假服务。status/statusErr 驱动前置状态
// 查询分支，per-method 错误驱动各系统调用错误翼，计数器供「未触达该
// 调用即早退」断言（区别于只看错误串）。
type svcCmdFake struct {
	status                                      service.Status
	statusErr                                   error
	installErr, uninstallErr, startErr, stopErr error
	installs, uninstalls, starts, stops         int
}

func (f *svcCmdFake) Run() error     { return nil }
func (f *svcCmdFake) Restart() error { return nil }
func (f *svcCmdFake) Start() error {
	if f.startErr != nil {
		return f.startErr
	}
	f.starts++
	return nil
}
func (f *svcCmdFake) Stop() error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.stops++
	return nil
}
func (f *svcCmdFake) Install() error {
	if f.installErr != nil {
		return f.installErr
	}
	f.installs++
	return nil
}
func (f *svcCmdFake) Uninstall() error {
	if f.uninstallErr != nil {
		return f.uninstallErr
	}
	f.uninstalls++
	return nil
}
func (f *svcCmdFake) Logger(chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *svcCmdFake) SystemLogger(chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *svcCmdFake) String() string   { return "svc-cmd-fake" }
func (f *svcCmdFake) Platform() string { return "linux-systemd" }
func (f *svcCmdFake) Status() (service.Status, error) {
	if f.statusErr != nil {
		return service.StatusUnknown, f.statusErr
	}
	return f.status, nil
}

// injectSvcCmdFake：newKardianosService 接缝注入（service_status_extra_
// test.go 同款模式），返回 fake 供断言。
func injectSvcCmdFake(t *testing.T) *svcCmdFake {
	t.Helper()
	fake := &svcCmdFake{}
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
		return fake, nil
	}
	t.Cleanup(func() { newKardianosService = restore })
	return fake
}

// injectSvcCreateError：createService 失败翼（newKardianosService 返回错误）。
func injectSvcCreateError(t *testing.T) {
	t.Helper()
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
		return nil, errors.New("svc construct denied")
	}
	t.Cleanup(func() { newKardianosService = restore })
}

func TestRunServiceInstall_Branches(t *testing.T) {
	saveServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectSvcCreateError(t)
		require.ErrorContains(t, runServiceInstall(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("already-exists", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		require.ErrorContains(t, runServiceInstall(&cobra.Command{}, nil), "已存在")
		assert.Zero(t, fake.installs, "已存在即早退，Install 不得触达")
	})

	t.Run("install-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.installErr = errors.New("unit write denied")
		require.ErrorContains(t, runServiceInstall(&cobra.Command{}, nil), "安装服务失败")
	})

	t.Run("success", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		var err error
		out := captureAgentStdout(t, func() { err = runServiceInstall(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.installs)
		assert.Contains(t, out, "安装成功")
	})
}

func TestRunServiceUninstall_Branches(t *testing.T) {
	saveServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectSvcCreateError(t)
		require.ErrorContains(t, runServiceUninstall(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServiceUninstall(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		require.ErrorContains(t, runServiceUninstall(&cobra.Command{}, nil), "不存在")
		assert.Zero(t, fake.uninstalls)
	})

	t.Run("running-stop-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.stopErr = errors.New("stop timed out")
		require.ErrorContains(t, runServiceUninstall(&cobra.Command{}, nil), "停止服务失败")
		assert.Zero(t, fake.uninstalls)
	})

	t.Run("stopped-uninstall-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusStopped
		fake.uninstallErr = errors.New("unit busy")
		require.ErrorContains(t, runServiceUninstall(&cobra.Command{}, nil), "卸载服务失败")
	})

	// running + 干净 Stop：走 2s 等待后卸载成功（sleep 翼随成功路径覆盖）
	t.Run("running-success", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		var err error
		out := captureAgentStdout(t, func() { err = runServiceUninstall(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.stops)
		assert.Equal(t, 1, fake.uninstalls)
		assert.Contains(t, out, "已卸载")
	})
}

func TestRunServiceStart_Branches(t *testing.T) {
	saveServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectSvcCreateError(t)
		require.ErrorContains(t, runServiceStart(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServiceStart(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown-hint-install", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		err := runServiceStart(&cobra.Command{}, nil)
		require.ErrorContains(t, err, "不存在")
		assert.Contains(t, err.Error(), "service install")
		assert.Zero(t, fake.starts)
	})

	t.Run("already-running", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		var err error
		out := captureAgentStdout(t, func() { err = runServiceStart(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Contains(t, out, "已在运行中")
		assert.Zero(t, fake.starts, "已在运行即早退，Start 不得触达")
	})

	t.Run("start-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusStopped
		fake.startErr = errors.New("start denied")
		require.ErrorContains(t, runServiceStart(&cobra.Command{}, nil), "启动服务失败")
	})

	t.Run("success", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusStopped
		var err error
		out := captureAgentStdout(t, func() { err = runServiceStart(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.starts)
		assert.Contains(t, out, "已启动")
	})
}

func TestRunServiceStop_Branches(t *testing.T) {
	saveServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectSvcCreateError(t)
		require.ErrorContains(t, runServiceStop(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServiceStop(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		require.ErrorContains(t, runServiceStop(&cobra.Command{}, nil), "不存在")
		assert.Zero(t, fake.stops)
	})

	t.Run("already-stopped", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusStopped
		var err error
		out := captureAgentStdout(t, func() { err = runServiceStop(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Contains(t, out, "已停止")
		assert.NotContains(t, out, "✅", "已停止早退翼不打成功标")
		assert.Zero(t, fake.stops, "已停止即早退，Stop 不得触达")
	})

	t.Run("stop-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.stopErr = errors.New("stop denied")
		require.ErrorContains(t, runServiceStop(&cobra.Command{}, nil), "停止服务失败")
	})

	t.Run("success", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		var err error
		out := captureAgentStdout(t, func() { err = runServiceStop(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.stops)
		assert.Contains(t, out, "✅")
	})
}

func TestRunServiceRestart_Branches(t *testing.T) {
	saveServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectSvcCreateError(t)
		require.ErrorContains(t, runServiceRestart(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServiceRestart(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		require.ErrorContains(t, runServiceRestart(&cobra.Command{}, nil), "不存在")
		assert.Zero(t, fake.starts)
	})

	t.Run("running-stop-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.stopErr = errors.New("stop timed out")
		require.ErrorContains(t, runServiceRestart(&cobra.Command{}, nil), "停止服务失败")
		assert.Zero(t, fake.starts)
	})

	// running + 干净 Stop 后 Start 失败：覆盖 Stop→2s 等待→Start 错误翼
	t.Run("running-start-error", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.startErr = errors.New("start denied")
		require.ErrorContains(t, runServiceRestart(&cobra.Command{}, nil), "启动服务失败")
		assert.Equal(t, 1, fake.stops)
	})

	// stopped：跳过 Stop 块直接 Start（status != Running 分支）
	t.Run("stopped-success", func(t *testing.T) {
		fake := injectSvcCmdFake(t)
		fake.status = service.StatusStopped
		var err error
		out := captureAgentStdout(t, func() { err = runServiceRestart(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Zero(t, fake.stops, "stopped 重启不触达 Stop")
		assert.Equal(t, 1, fake.starts)
		assert.Contains(t, out, "已重启")
	})
}
