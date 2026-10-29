// ARCH-017 回归：优雅停机不得被活跃 Agent 控制连接阻塞。
//
// 旧实现：停机 goroutine 先调用 TCPListener.Close()，其内部 wg.Wait() 等待
// 所有活跃会话结束——而会话只会在 rootCancel（排在 Close 之后）后结束，
// 形成互相等待：持有任一 Agent 连接时 SIGINT 永远走不完优雅停机。
//
// 新实现：StopAccepting（只停接受循环）→ HTTP drain → rootCancel（解除会话
// 读阻塞）→ 有界 WaitConnections → 冲刷/关池。本用例开一条「连接后不发任何
// 帧」的 Agent 连接占用 serveConn goroutine，断言 SIGINT 后 runServer 在有界
// 时间内返回 nil——旧代码在此挂死，新代码必须走完。
package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunServer_SigintWithActiveControlConn(t *testing.T) {
	// 预注册 SIGINT：任何时刻的 Kill 都不会落入默认处置（杀测试进程）。
	mine := make(chan os.Signal, 1)
	signal.Notify(mine, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(mine) })

	// SetupLoggerWithFile 替换进程级 slog 默认 logger（指向启动管道），
	// 结束后恢复，避免污染后续用例。
	oldLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	// 相对路径（data/uploads、日志）隔离。
	t.Chdir(t.TempDir())

	dbPath := filepath.Join(t.TempDir(), "shutdown.db")

	// 预选空闲控制端口（确定性拨号目标；控制服务器此时必然已监听）。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctrlPort := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	cfg := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 0\n"+
		"database:\n  driver: sqlite\n  dataSource: "+dbPath+"\n"+
		"auth:\n  jwtSecret: r26-shutdown-secret-0123456789\n"+
		"control:\n  addr: 127.0.0.1:"+fmt.Sprint(ctrlPort)+"\n"+
		"cluster:\n  enabled: false\n"+
		"telemetry:\n  enabled: false\n")

	oldCfg, oldMode, oldPort, oldDebug := cfgFile, mode, port, debug
	oldHost, oldLevel, oldBoot := host, logLevel, bootstrapDataDir
	t.Cleanup(func() {
		cfgFile, mode, port, debug = oldCfg, oldMode, oldPort, oldDebug
		host, logLevel, bootstrapDataDir = oldHost, oldLevel, oldBoot
	})
	cfgFile = cfg
	bootstrapDataDir = filepath.Join(t.TempDir(), "boot")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	readyCh := make(chan struct{}, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "Starting Croupier Server at ") {
				readyCh <- struct{}{}
				break
			}
		}
		// 持续排空：gin 调试路由表等大量输出写满管道会阻塞启动路径。
		io.Copy(io.Discard, r)
	}()

	done := make(chan error, 1)
	go func() { done <- runServer() }()

	select {
	case <-readyCh:
	case <-time.After(120 * time.Second):
		os.Stdout = oldStdout
		t.Fatal("runServer 未在期限内打出就绪行")
	}

	// 开一条 Agent 控制连接并保持打开（不发任何帧）。其 serveConn
	// goroutine 阻塞在 mux.Run 的读循环——旧 Close 的 wg.Wait 在此等待
	// 会话结束，而会话只会因 rootCancel 结束（排在 Close 之后）。
	agentConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", ctrlPort))
	require.NoError(t, err)
	defer agentConn.Close()
	time.Sleep(200 * time.Millisecond) // 让服务端 accept 并进入 serveConn

	// SIGINT → runServer 必须随优雅停机返回 nil，而非挂在活跃连接上。
	var shutdownErr error
	exited := false
	for i := 0; i < 20 && !exited; i++ {
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		select {
		case shutdownErr = <-done:
			exited = true
		case <-time.After(3 * time.Second):
		}
	}
	os.Stdout = oldStdout
	if !exited {
		t.Fatal("runServer 未随 SIGINT 退出：活跃 Agent 连接阻塞了优雅停机（ARCH-017）")
	}
	require.NoError(t, shutdownErr, "SIGINT 后 runServer 须走完优雅停机并返回 nil")
}
