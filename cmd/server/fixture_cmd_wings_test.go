// 覆盖率巡检第二十三轮（wt-api）：dev-fixture 命令 RunE + provider
// 两翼——
// ① fixtureCmd.RunE 启动失败翼：BaseDir 指向已存在文件 → MkdirAll
//
//	失败 → StartDashboardFixture 报错上抛；
//
// ② fixtureCmd.RunE 全链：真实 server+agent+SDK+provider 起全栈，
//
//	FIXTURE_READY 后 SIGINT 触发清理与退出（测试自身预注册 SIGINT
//	Notify 消除「信号早于命令内 Notify 注册」的进程死亡竞态窗口）；
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
	"os/signal"
	"path/filepath"
	"strings"
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

func TestFixtureCmd_RunE_FullBootAndSignalShutdown(t *testing.T) {
	// 预注册 SIGINT：任何时刻的 Kill 都不会落入默认处置（测试进程被杀）
	mine := make(chan os.Signal, 1)
	signal.Notify(mine, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(mine) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpAddr := ln.Addr().String()
	require.NoError(t, ln.Close())

	oldBase, oldHTTP := fixtureBaseDir, fixtureHTTPAddr
	fixtureBaseDir, fixtureHTTPAddr = t.TempDir(), httpAddr
	t.Cleanup(func() { fixtureBaseDir, fixtureHTTPAddr = oldBase, oldHTTP })

	// 就绪信号必须锚定 FIXTURE_READY 行（全组件起完后打印，紧邻命令内
	// signal.Notify 注册）——healthz 只代表 HTTP 监听先起，此时 SIGINT
	// 还到不了命令的等待通道（首版实证：60s 超时未退出）。
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	readyCh := make(chan struct{}, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "FIXTURE_READY ") {
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
	time.Sleep(200 * time.Millisecond) // 打印与 signal.Notify 之间的窗口

	// 幂等重发：覆盖 Notify 注册时刻的残余竞态
	for i := 0; i < 20; i++ {
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		select {
		case err := <-done:
			require.NoError(t, err, "SIGINT 后 RunE 须干净退出")
			_ = w.Close()
			return
		case <-time.After(3 * time.Second):
		}
	}
	t.Fatal("RunE 未随 SIGINT 退出")
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
