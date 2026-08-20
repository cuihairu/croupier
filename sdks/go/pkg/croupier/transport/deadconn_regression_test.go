package transport

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// TestCallFailsFastWhenConnectionDies 回归：连接断开后 receiveLoop 退出，
// 在途 Call 必须立刻收到错误而不是阻塞到 ctx deadline（后台 goroutine 用
// context.Background() 的调用方会永久挂起）。
func TestCallFailsFastWhenConnectionDies(t *testing.T) {
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	// 每个连接进来都立即断开（支持 -count 重复运行）
	go func() {
		for {
			conn, err := server.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	client, err := NewTCPClient(&Config{Address: server.Addr().String(), Insecure: true})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	done := make(chan error, 1)
	go func() {
		_, _, err := client.Call(context.Background(), 0x0101, []byte("req"))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after connection death")
		}
		if !errors.Is(err, errConnectionClosed) && err.Error() != "client is closing" {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Call blocked after connection died; pending requests must fail fast")
	}
}
