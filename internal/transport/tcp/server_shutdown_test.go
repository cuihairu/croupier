package tcp

// AUD-081 回归：Serve 生命周期在明文或 TLS、空闲或在途连接时均可取消，
// 监听与 worker 有界收尾。此前 accept deadline 只设在直接断言到的
// *net.TCPListener 上——crypto/tls 的包装 listener 不实现 SetDeadline，
// TLS 空闲时 Accept 永久阻塞，ctx 取消无人理会；且 ctx 取消不关闭在途
// 连接，无 RecvTimeout 的空闲连接会把 serveConn + per-conn worker goroutine
// 挂到进程退出。

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"runtime"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// newTLSServer 用自签证书起一个 TLS echo server（Insecure=false 走
// tls.Listen 路径），返回尚未启动 Serve 的 server。
func newTLSServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	certPath, keyPath := genSelfSignedCert(t, dir)
	handler := transportcore.HandlerFunc(func(_ context.Context, _ uint32, _ uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{
		Address:  "127.0.0.1:0",
		CertFile: certPath,
		KeyFile:  keyPath,
	}, handler)
	if err != nil {
		t.Fatalf("NewServer(tls) error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

// startServe 在后台起 Serve 并返回 cancel 与结果通道。与 serveAndStop
// 不同：本组用例要在测试中段主动 cancel 并观测返回时延，不预置收尾。
func startServe(srv *Server) (context.CancelFunc, <-chan error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	return cancel, done
}

// tlsEchoRoundtrip 完成一次 TLS 握手 + 帧往返，返回保持打开的连接
// （证明该连接已建立、已进入服务路径）。
func tlsEchoRoundtrip(t *testing.T, srv *Server, reqID uint32) net.Conn {
	t.Helper()
	conn, err := tls.Dial("tcp", srv.Addr(), &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 测试自签证书
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	writeRequestFrame(t, conn, protocol.MsgHeartbeatRequest, reqID, []byte("ping"))
	msgID, respID, body := readResponseFrame(t, conn, 3*time.Second)
	if msgID != protocol.MsgHeartbeatResponse || respID != reqID || string(body) != "ping" {
		t.Fatalf("echo response = msg %s req %d body %q, want heartbeat req %d %q",
			protocol.MsgIDString(msgID), respID, body, reqID, "ping")
	}
	return conn
}

// awaitGoroutineSettle 轮询等待 goroutine 数回落到基线（+2 容忍测试框架
// 自身抖动）；超时则 dump 栈辅助定位泄漏路径。
func awaitGoroutineSettle(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if runtime.NumGoroutine() <= baseline+2 {
			return
		}
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			stacks := string(buf[:n])
			if len(stacks) > 8000 {
				stacks = stacks[:8000] + "\n...truncated..."
			}
			t.Fatalf("goroutines did not settle after ctx cancel: baseline %d, now %d\n%s",
				baseline, runtime.NumGoroutine(), stacks)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// assertConnClosedByServer 断言服务端已主动关闭连接：读到的是关闭类错误
// （EOF/reset），而不是读 deadline 到期（后者说明连接仍开着）。
func assertConnClosedByServer(t *testing.T, conn net.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err := tcptr.ReadFrame(conn)
	if err == nil {
		t.Fatal("server did not close the in-flight connection after ctx cancel")
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatalf("read deadline hit before server close: connection still open after ctx cancel (%v)", err)
	}
}

// (a) TLS 空闲：ctx 取消后 Serve 须在 accept deadline 周期（≤1s）内返回，
// 而不是永久阻塞在包装 listener 的 Accept 上。
func TestServer_Serve_TLSIdleCancelReturnsPromptly(t *testing.T) {
	srv := newTLSServer(t)
	cancel, done := startServe(srv)

	// 先完成一次往返，证明 Accept 已在工作（排除启动竞态假绿）。
	probe := tlsEchoRoundtrip(t, srv, 1)
	_ = probe.Close()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2s of ctx cancel on idle TLS listener")
	}
}

// (b) TLS 在途连接：cancel 后 Serve 有界返回，服务端主动关闭在途连接，
// serveConn 与 per-conn worker 有界收尾（无 goroutine 泄漏）。
func TestServer_Serve_TLSInFlightCancelBoundedShutdown(t *testing.T) {
	srv := newTLSServer(t)
	baseline := runtime.NumGoroutine()
	cancel, done := startServe(srv)

	conn := tlsEchoRoundtrip(t, srv, 2)
	defer func() { _ = conn.Close() }()

	cancel()
	start := time.Now()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return in bounded time after cancel with in-flight TLS connection")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("Serve took %v to return after cancel", elapsed)
	}

	assertConnClosedByServer(t, conn)
	awaitGoroutineSettle(t, baseline)
}

// (c) 明文回归：既有 ≤1s 周期取消语义不受影响，且在途连接同样有界收尾。
func TestServer_Serve_PlaintextCancelStillBounded(t *testing.T) {
	handler := transportcore.HandlerFunc(func(_ context.Context, _ uint32, _ uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("NewServer(plaintext) error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	baseline := runtime.NumGoroutine()
	cancel, done := startServe(srv)

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer func() { _ = conn.Close() }()
	writeRequestFrame(t, conn, protocol.MsgHeartbeatRequest, 3, []byte("ping"))
	msgID, respID, body := readResponseFrame(t, conn, 3*time.Second)
	if msgID != protocol.MsgHeartbeatResponse || respID != 3 || string(body) != "ping" {
		t.Fatalf("echo response = msg %s req %d body %q", protocol.MsgIDString(msgID), respID, body)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2s of ctx cancel on idle plaintext listener")
	}

	assertConnClosedByServer(t, conn)
	awaitGoroutineSettle(t, baseline)
}

// listen 结构回归：TLS 包装后仍能取到底层 *net.TCPListener——包装层不透传
// SetDeadline，Serve 的 accept deadline 必须设在 unwrap 结果上（AUD-081
// 修复前提），明文路径则两者是同一对象。
func TestListen_UnderlyingTCPListenerWired(t *testing.T) {
	t.Run("tls wrapped", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath := genSelfSignedCert(t, dir)
		ln, tcpLn, err := listen(&Config{Address: "127.0.0.1:0", CertFile: certPath, KeyFile: keyPath})
		if err != nil {
			t.Fatalf("listen(tls) error = %v", err)
		}
		defer func() { _ = ln.Close() }()
		if tcpLn == nil {
			t.Fatal("TLS path must return the underlying *net.TCPListener for accept deadlines")
		}
		if _, ok := ln.(*net.TCPListener); ok {
			t.Fatal("TLS path must wrap the raw listener, not return it directly")
		}
	})

	t.Run("plaintext unwrapped", func(t *testing.T) {
		ln, tcpLn, err := listen(&Config{Address: "127.0.0.1:0", Insecure: true})
		if err != nil {
			t.Fatalf("listen(plaintext) error = %v", err)
		}
		defer func() { _ = ln.Close() }()
		if tcpLn == nil {
			t.Fatal("plaintext path must expose the *net.TCPListener")
		}
		if raw, ok := ln.(*net.TCPListener); !ok || raw != tcpLn {
			t.Fatal("plaintext path must return the raw *net.TCPListener itself")
		}
	})
}
