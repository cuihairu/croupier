// 覆盖率巡检第二十三轮（wt-api）：dev-fixture 命令 RunE + provider
// 两翼——
// ① fixtureCmd.RunE 启动失败翼：BaseDir 指向已存在文件 → MkdirAll
//
//	失败 → StartDashboardFixture 报错上抛；
//
// ② fixtureCmd.RunE 全链：真实 server+agent+SDK+provider 起全栈，
//
//	FIXTURE_READY 后 SIGINT 触发清理与退出（命令内 signal.Notify 已
//	提前到启动之前注册——与 FIXTURE_READY 行零窗口——就绪后首发
//	SIGINT 必落入命令等待通道）；
//	CleanupScope 失败的 Fprintf 翼登记不可达（需运行中破坏 fixture
//	库写路径，无确定性注入面，破坏失败即用例白跑）；
//
// ③ players provider 更新的 name-only 翼（192）：PUT 只带 name（既有
//
//	CRUD 用例只动 level，本翼从未触达）；
//
// ④ readBody 的 nil Body 翼（237）：http.Server 恒给非 nil Body，
//
//	零值 http.Request 直测 helper 契约。
package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixtureCmd_RunE_StartError(t *testing.T) {
	occupied := filepath.Join(t.TempDir(), "occupied-file")
	require.NoError(t, os.WriteFile(occupied, nil, 0o600))

	oldBase := fixtureBaseDir
	fixtureBaseDir = occupied
	t.Cleanup(func() { fixtureBaseDir = oldBase })

	require.Error(t, fixtureCmd.RunE(fixtureCmd, nil),
		"BaseDir 被文件占位 → MkdirAll 失败 → RunE 报错")
}

// TestFixtureCmd_RunE_SignalNotifyBeforeStart 钉死注册程序序（零墙钟、
// 零竞态窗口）：signal 注册必须先于 StartDashboardFixture 调用——启动
// 失败翼（BaseDir 被文件占位）里启动根本不会成功、FIXTURE_READY 永不
// 打印，修复后的程序序下桩仍必然已被调用；旧实现（Printf 之后才注册）
// 在此路径从不注册，事件缺 notify，断言必红。
func TestFixtureCmd_RunE_SignalNotifyBeforeStart(t *testing.T) {
	occupied := filepath.Join(t.TempDir(), "occupied-file")
	require.NoError(t, os.WriteFile(occupied, nil, 0o600))

	notifyCalled := false
	origNotify := fixtureSignalNotify
	fixtureSignalNotify = func(sig chan os.Signal) { notifyCalled = true }
	t.Cleanup(func() { fixtureSignalNotify = origNotify })

	oldBase := fixtureBaseDir
	fixtureBaseDir = occupied
	t.Cleanup(func() { fixtureBaseDir = oldBase })

	require.Error(t, fixtureCmd.RunE(fixtureCmd, nil))
	require.True(t, notifyCalled,
		"signal.Notify must be registered before StartDashboardFixture is called")
}

func TestFixtureCmd_RunE_FullBootAndSignalShutdown(t *testing.T) {
	// 顺序钉死（零墙钟依赖）：fixtureSignalNotify 桩在真实注册后记
	// "notify"，stdoutRecorder 在 FIXTURE_READY 落盘时记 "ready"——程序序
	// 必为 notify 先于 ready。旧实现（Printf 之后才注册）该断言必红：
	// 桩根本不被调用，事件序列缺 notify。
	events := []string{}
	var evMu sync.Mutex
	origNotify := fixtureSignalNotify
	fixtureSignalNotify = func(sig chan os.Signal) {
		origNotify(sig)
		evMu.Lock()
		events = append(events, "notify")
		evMu.Unlock()
	}
	t.Cleanup(func() { fixtureSignalNotify = origNotify })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpAddr := ln.Addr().String()
	require.NoError(t, ln.Close())

	oldBase, oldHTTP := fixtureBaseDir, fixtureHTTPAddr
	fixtureBaseDir, fixtureHTTPAddr = t.TempDir(), httpAddr
	t.Cleanup(func() { fixtureBaseDir, fixtureHTTPAddr = oldBase, oldHTTP })

	// 就绪信号锚定 FIXTURE_READY 行（全组件起完后打印）：命令内
	// signal.Notify 已提前到启动之前（76f057be9 同型修补），就绪行与
	// 监听注册之间零窗口，就绪后首发 SIGINT 即触发清理退出——旧实现的
	// 200ms 等待 + 幂等重发工作窗已随根因消失，不再需要。
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	readyCh := make(chan struct{}, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "FIXTURE_READY ") {
				evMu.Lock()
				events = append(events, "ready")
				evMu.Unlock()
				readyCh <- struct{}{}
				break
			}
		}
		// 持续排空：子进程（e2eprovider）继承同一管道，不排空会写满阻塞
		io.Copy(io.Discard, r)
	}()

	done := make(chan error, 1)
	go func() { done <- fixtureCmd.RunE(fixtureCmd, nil) }()

	select {
	case <-readyCh:
	case <-time.After(120 * time.Second):
		os.Stdout = oldStdout
		t.Fatal("fixture 未在期限内打出 FIXTURE_READY")
	}
	os.Stdout = oldStdout

	// 程序序断言：注册必须先于就绪行（window-free，不依赖墙钟时序）。
	evMu.Lock()
	order := append([]string(nil), events...)
	evMu.Unlock()
	require.NotEmpty(t, order, "no order events recorded")
	require.Equal(t, "notify", order[0],
		"signal.Notify must register before the FIXTURE_READY line is printed")

	// 首发 SIGINT：监听注册先于启动，就绪行打印时命令等待通道必然在位。
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case err := <-done:
		require.NoError(t, err, "SIGINT 后 RunE 须干净退出")
		_ = w.Close()
		return
	case <-time.After(10 * time.Second):
		t.Fatal("RunE 未随 SIGINT 退出")
	}
}

// players provider 更新路径的 name-only 翼（req.Level == nil）。
func TestPlayersProvider_UpdateNameOnlyWing(t *testing.T) {
	h := newPlayersProvider().handler()

	code, updated := doJSON(t, h, http.MethodPut, "/players/p-002", `{"name":"Renamed"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Renamed", updated["name"])
	assert.Equal(t, float64(20), updated["level"], "level 未随 name-only 更新变动（种子值保持）")
}

// readBody 的 nil Body 契约（http.Server 请求恒非 nil Body，直测 helper）。
func TestReadBody_NilBodyWing(t *testing.T) {
	raw, err := readBody(&http.Request{})
	require.NoError(t, err)
	assert.Nil(t, raw)
}
