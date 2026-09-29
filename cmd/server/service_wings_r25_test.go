// 覆盖率巡检第二十五轮（wt-api）：cmd/server service 变更命令主体——
// runServerServiceInstall/Uninstall/Start/Stop/Restart 五体经
// newKardianosService 接缝注入可控 fake（status/statusErr + 各方法错误 +
// 调用计数）分支矩阵全数直测，不碰真实 systemd（与 cmd/agent 侧
// service_wings_r25_test.go 同批；cmd-2 豁免随两包收窄）。调用形态沿用
// service_status_extra_test.go 先例：直接调 run* 函数体（install 传空
// cobra.Command——其首行经 Flags().GetString 回写全局，
// saveServerServiceGlobals 兜底还原）。
package main

import (
	"errors"
	"os"
	"testing"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serverSvcCmdFake：五个变更命令的可控假服务（与 agent 侧 svcCmdFake
// 同构；两包测试各自独立编译，无共享）。计数器供「未触达该调用即早退」
// 断言（区别于只看错误串）。
type serverSvcCmdFake struct {
	status                                      service.Status
	statusErr                                   error
	installErr, uninstallErr, startErr, stopErr error
	installs, uninstalls, starts, stops         int
}

func (f *serverSvcCmdFake) Run() error     { return nil }
func (f *serverSvcCmdFake) Restart() error { return nil }
func (f *serverSvcCmdFake) Start() error {
	if f.startErr != nil {
		return f.startErr
	}
	f.starts++
	return nil
}
func (f *serverSvcCmdFake) Stop() error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.stops++
	return nil
}
func (f *serverSvcCmdFake) Install() error {
	if f.installErr != nil {
		return f.installErr
	}
	f.installs++
	return nil
}
func (f *serverSvcCmdFake) Uninstall() error {
	if f.uninstallErr != nil {
		return f.uninstallErr
	}
	f.uninstalls++
	return nil
}
func (f *serverSvcCmdFake) Logger(chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *serverSvcCmdFake) SystemLogger(chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}
func (f *serverSvcCmdFake) String() string   { return "server-svc-cmd-fake" }
func (f *serverSvcCmdFake) Platform() string { return "linux-systemd" }
func (f *serverSvcCmdFake) Status() (service.Status, error) {
	if f.statusErr != nil {
		return service.StatusUnknown, f.statusErr
	}
	return f.status, nil
}

// injectServerSvcCmdFake：newKardianosService 接缝注入，返回 fake 供断言。
func injectServerSvcCmdFake(t *testing.T) *serverSvcCmdFake {
	t.Helper()
	fake := &serverSvcCmdFake{}
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
		return fake, nil
	}
	t.Cleanup(func() { newKardianosService = restore })
	return fake
}

// injectServerSvcCreateError：createServerService 失败翼。
func injectServerSvcCreateError(t *testing.T) {
	t.Helper()
	restore := newKardianosService
	newKardianosService = func(_ service.Interface, _ *service.Config) (service.Service, error) {
		return nil, errors.New("svc construct denied")
	}
	t.Cleanup(func() { newKardianosService = restore })
}

func TestRunServerServiceInstall_Branches(t *testing.T) {
	saveServerServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectServerSvcCreateError(t)
		require.ErrorContains(t, runServerServiceInstall(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("already-exists", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		require.ErrorContains(t, runServerServiceInstall(&cobra.Command{}, nil), "已存在")
		assert.Zero(t, fake.installs, "已存在即早退，Install 不得触达")
	})

	t.Run("install-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.installErr = errors.New("unit write denied")
		require.ErrorContains(t, runServerServiceInstall(&cobra.Command{}, nil), "安装服务失败")
	})

	t.Run("success", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceInstall(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.installs)
		// server 侧打印完整平台串（agent 侧只打 [:1]，两包契约各自锁定）
		assert.Contains(t, out, "平台: linux-systemd")
		assert.Contains(t, out, "安装成功")
	})
}

func TestRunServerServiceUninstall_Branches(t *testing.T) {
	saveServerServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectServerSvcCreateError(t)
		require.ErrorContains(t, runServerServiceUninstall(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServerServiceUninstall(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		require.ErrorContains(t, runServerServiceUninstall(&cobra.Command{}, nil), "不存在")
		assert.Zero(t, fake.uninstalls)
	})

	t.Run("running-stop-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.stopErr = errors.New("stop timed out")
		require.ErrorContains(t, runServerServiceUninstall(&cobra.Command{}, nil), "停止服务失败")
		assert.Zero(t, fake.uninstalls)
	})

	t.Run("stopped-uninstall-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusStopped
		fake.uninstallErr = errors.New("unit busy")
		require.ErrorContains(t, runServerServiceUninstall(&cobra.Command{}, nil), "卸载服务失败")
	})

	// running + 干净 Stop：走 2s 等待后卸载成功（sleep 翼随成功路径覆盖）
	t.Run("running-success", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceUninstall(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.stops)
		assert.Equal(t, 1, fake.uninstalls)
		assert.Contains(t, out, "已卸载")
	})
}

func TestRunServerServiceStart_Branches(t *testing.T) {
	saveServerServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectServerSvcCreateError(t)
		require.ErrorContains(t, runServerServiceStart(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServerServiceStart(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown-hint-install", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		err := runServerServiceStart(&cobra.Command{}, nil)
		require.ErrorContains(t, err, "不存在")
		assert.Contains(t, err.Error(), "service install")
		assert.Zero(t, fake.starts)
	})

	t.Run("already-running", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceStart(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Contains(t, out, "已在运行中")
		assert.Zero(t, fake.starts, "已在运行即早退，Start 不得触达")
	})

	t.Run("start-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusStopped
		fake.startErr = errors.New("start denied")
		require.ErrorContains(t, runServerServiceStart(&cobra.Command{}, nil), "启动服务失败")
	})

	t.Run("success", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusStopped
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceStart(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.starts)
		assert.Contains(t, out, "已启动")
	})
}

func TestRunServerServiceStop_Branches(t *testing.T) {
	saveServerServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectServerSvcCreateError(t)
		require.ErrorContains(t, runServerServiceStop(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServerServiceStop(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		require.ErrorContains(t, runServerServiceStop(&cobra.Command{}, nil), "不存在")
		assert.Zero(t, fake.stops)
	})

	t.Run("already-stopped", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusStopped
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceStop(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Contains(t, out, "已停止")
		assert.NotContains(t, out, "✅", "已停止早退翼不打成功标")
		assert.Zero(t, fake.stops, "已停止即早退，Stop 不得触达")
	})

	t.Run("stop-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.stopErr = errors.New("stop denied")
		require.ErrorContains(t, runServerServiceStop(&cobra.Command{}, nil), "停止服务失败")
	})

	t.Run("success", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceStop(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Equal(t, 1, fake.stops)
		assert.Contains(t, out, "✅")
	})
}

func TestRunServerServiceRestart_Branches(t *testing.T) {
	saveServerServiceGlobals(t)

	t.Run("create-fail", func(t *testing.T) {
		injectServerSvcCreateError(t)
		require.ErrorContains(t, runServerServiceRestart(&cobra.Command{}, nil), "创建服务失败")
	})

	t.Run("status-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.statusErr = errors.New("systemd bus down")
		require.ErrorContains(t, runServerServiceRestart(&cobra.Command{}, nil), "查询服务状态失败")
	})

	t.Run("unknown", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		require.ErrorContains(t, runServerServiceRestart(&cobra.Command{}, nil), "不存在")
		assert.Zero(t, fake.starts)
	})

	t.Run("running-stop-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.stopErr = errors.New("stop timed out")
		require.ErrorContains(t, runServerServiceRestart(&cobra.Command{}, nil), "停止服务失败")
		assert.Zero(t, fake.starts)
	})

	// running + 干净 Stop 后 Start 失败：覆盖 Stop→2s 等待→Start 错误翼
	t.Run("running-start-error", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusRunning
		fake.startErr = errors.New("start denied")
		require.ErrorContains(t, runServerServiceRestart(&cobra.Command{}, nil), "启动服务失败")
		assert.Equal(t, 1, fake.stops)
	})

	// stopped：跳过 Stop 块直接 Start（status != Running 分支）
	t.Run("stopped-success", func(t *testing.T) {
		fake := injectServerSvcCmdFake(t)
		fake.status = service.StatusStopped
		var err error
		out := captureServerStdout(t, func() { err = runServerServiceRestart(&cobra.Command{}, nil) })
		require.NoError(t, err)
		assert.Zero(t, fake.stops, "stopped 重启不触达 Stop")
		assert.Equal(t, 1, fake.starts)
		assert.Contains(t, out, "已重启")
	})
}

// Start 后台 goroutine 的 panic 恢复翼（154-156）：runAgentFunc 同款的
// runServerFunc 替身改为 panic → recover 捕获 → svc.Stop。既有用例只盖了
// 错误返回与成功两翼，panic 翼从未触达。
func TestServerServiceStart_RunServerPanic(t *testing.T) {
	saveServerServiceGlobals(t)
	old := runServerFunc
	runServerFunc = func() error { panic("r25 panic wing") }
	t.Cleanup(func() { runServerFunc = old })

	s := newServerService("")
	fake := newFakeServerSvc()
	require.NoError(t, s.Start(fake))
	waitServerStopped(t, fake.stopped)
	require.NoError(t, s.Stop(fake))
}

// wd 的 Getwd 失败翼（595）：chdir 进已删除目录构造 ENOENT。全包无
// t.Parallel（顺序执行），进程级 cwd 操纵安全，结束恢复原目录。
func TestWd_GetwdFailureWing(t *testing.T) {
	home, err := os.Getwd()
	require.NoError(t, err)
	doomed := t.TempDir()
	require.NoError(t, os.Chdir(doomed))
	require.NoError(t, os.RemoveAll(doomed))
	t.Cleanup(func() { _ = os.Chdir(home) })
	assert.Equal(t, "unknown", wd())
}
