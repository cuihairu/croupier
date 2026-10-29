// ARCH-017：TCPListener 停机原语——StopAccepting（不等待活跃会话）/
// WaitConnections（有界等待）/ Close（全加入）三者拆分的语义验证。
// 旧实现把「停止接收」与「等待活跃会话结束」耦合在 Close 的 wg.Wait 里，
// 活跃 Agent 连接（长生命）会让优雅停机死锁住 HTTP 停机与根取消。
package server

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// startTestListener boots a TCPListener on a random loopback port and waits
// until the accept loop is running.
func startTestListener(t *testing.T) *TCPListener {
	t.Helper()
	cfg := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	l, err := NewTCPListener(cfg, NewAgentSessionStore(), nil, slog.Default())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = l.Serve(ctx) }()
	t.Cleanup(cancel)
	<-l.serveStarted // 接受循环已运行
	return l
}

// dialAccepted opens a TCP conn to l and waits until the server accepted it
// (activeConns == 1), so the WaitGroup counter is stable before the wait
// paths are exercised.
func dialAccepted(t *testing.T, l *TCPListener) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", l.Addr())
	require.NoError(t, err)
	require.Eventually(t, func() bool { return l.ActiveConnections() == 1 },
		2*time.Second, 10*time.Millisecond)
	return conn
}

func TestTCPListener_StopAcceptingDoesNotWaitActiveConn(t *testing.T) {
	l := startTestListener(t)
	conn := dialAccepted(t, l)
	defer conn.Close()

	// StopAccepting 必须在活跃会话仍开着时立即返回（不等待 wg）。
	done := make(chan struct{})
	go func() { _ = l.StopAccepting(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopAccepting blocked on an active connection")
	}
	// 会话 goroutine 仍存活（连接未关）。
	require.EqualValues(t, 1, l.ActiveConnections())

	// 关闭连接后会话加入，WaitConnections 随即返回。
	_ = conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, l.WaitConnections(ctx))
}

func TestTCPListener_WaitConnectionsBounded(t *testing.T) {
	l := startTestListener(t)
	conn := dialAccepted(t, l)
	defer conn.Close()

	require.NoError(t, l.StopAccepting())

	// 会话仍开：有界等待必须超时返回，不得挂死。
	shortCtx, sc := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer sc()
	err := l.WaitConnections(shortCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// 连接关闭后，新等待返回 nil。
	_ = conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, l.WaitConnections(ctx))
}

func TestTCPListener_CloseStillJoins(t *testing.T) {
	l := startTestListener(t)
	conn := dialAccepted(t, l)

	// Close 必须阻塞到连接关闭（全加入语义保持）。
	closed := make(chan struct{})
	go func() { _ = l.Close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("Close returned while a connection was still open")
	case <-time.After(200 * time.Millisecond):
	}
	_ = conn.Close()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not join after the connection closed")
	}
	require.EqualValues(t, 0, l.ActiveConnections())
}

// 未启动 Serve 的 listener：StopAccepting 不得卡在 serveStarted 上。
func TestTCPListener_StopAcceptingBeforeServe(t *testing.T) {
	cfg := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	l, err := NewTCPListener(cfg, NewAgentSessionStore(), nil, slog.Default())
	require.NoError(t, err)
	require.NoError(t, l.StopAccepting())
	require.NoError(t, l.StopAccepting()) // 幂等
	require.True(t, l.IsClosed())
}
