package tcp

import (
	"context"
	"net"
	"testing"
	"time"

	coretcp "github.com/cuihairu/croupier/core/transport/tcp"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// 混合版本兼容回归（agent-core：传输底座上收 core/transport 后，既有 wire
// 不得漂移）。internal/transport/tcp 是 core 的 shim，生产两侧各走一条：
// server 面（cluster/互联监听）与 client 面（agent 注册通道）。本文件用
// 跨栈配对锁死字节兼容：任一侧实现漂移帧格式/请求关联应答语义即红。

// startEchoServer 起一个 internal shim server（echo handler），返回拨号地址。
func startEchoServer(t *testing.T) string {
	t.Helper()
	handler := transportcore.HandlerFunc(func(_ context.Context, _ uint32, _ uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = srv.Close()
	})
	return srv.Addr()
}

// TestWireCompat_FrameConstantsParity shim 与 core 的帧头/上限常量必须一致，
// 混布新旧二进制时同一条 TCP 流才能互认。
func TestWireCompat_FrameConstantsParity(t *testing.T) {
	if FrameHeaderBytes != coretcp.FrameHeaderBytes {
		t.Fatalf("FrameHeaderBytes drift: shim=%d core=%d", FrameHeaderBytes, coretcp.FrameHeaderBytes)
	}
	if MaxFrameBytes != coretcp.MaxFrameBytes {
		t.Fatalf("MaxFrameBytes drift: shim=%d core=%d", MaxFrameBytes, coretcp.MaxFrameBytes)
	}
}

// TestWireCompat_CoreClientShimServer 新底座 client（agents/* 注册通道实测
// 路径）↔ 生产 server（shim，cluster/互联监听同款）往返：帧、请求关联、
// 应答 msgID 映射全链一致。
func TestWireCompat_CoreClientShimServer(t *testing.T) {
	addr := startEchoServer(t)
	client, err := coretcp.NewClient(&coretcp.Config{
		Address:     addr,
		Insecure:    true,
		RecvTimeout: 5 * time.Second,
		SendTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("core NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	respMsgID, respBody, err := client.Call(context.Background(), protocol.MsgHeartbeatRequest, []byte("ping-core"))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if respMsgID != protocol.MsgHeartbeatResponse {
		t.Fatalf("resp msgID = %d, want %d", respMsgID, protocol.MsgHeartbeatResponse)
	}
	if string(respBody) != "ping-core" {
		t.Fatalf("payload echo mismatch: %q", respBody)
	}
}

// TestWireCompat_ShimClientCoreMuxConn 旧路径 client（shim）↔ 上收后的
// MuxConn（server 面实测路径，internal/server/tcp_listener 同款装配）往返：
// 双向多路复用应答与既有单帧请求方互认。
func TestWireCompat_ShimClientCoreMuxConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	handler := transportcore.HandlerFunc(func(_ context.Context, _ uint32, _ uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		mux := coretcp.NewMuxConn(conn, &coretcp.Config{RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second}, handler)
		ctx, cancel := context.WithCancel(context.Background())
		_ = mux.Run(ctx)
		cancel()
		_ = mux.Close()
		mux.Wait()
	}()

	client, err := NewClient(&Config{
		Address:     ln.Addr().String(),
		Insecure:    true,
		RecvTimeout: 5 * time.Second,
		SendTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("shim NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	respMsgID, respBody, err := client.Call(context.Background(), protocol.MsgRegisterRequest, []byte("hello-mux"))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if respMsgID != protocol.MsgRegisterResponse {
		t.Fatalf("resp msgID = %d, want %d", respMsgID, protocol.MsgRegisterResponse)
	}
	if string(respBody) != "hello-mux" {
		t.Fatalf("payload echo mismatch: %q", respBody)
	}
}
