package tcp

// 覆盖率补洞（group L）：mux_conn.go Wait（0%）、Close→failPending 对已
// 占用 pending 通道的 default 丢弃分支、server.go serveConn 的 s.closing
// 退出分支。只新增用例，不改产品代码与既有测试。

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// TestMuxConn_WaitReturnsAfterRun：Run 返回（deferred Close 已通知 worker
// 退出）后 Wait 必须返回——直接补测 0% 的 Wait。
func TestMuxConn_WaitReturnsAfterRun(t *testing.T) {
	c1, c2 := net.Pipe()
	mc := NewMuxConn(c1, &Config{RecvTimeout: 100 * time.Millisecond}, nil)

	c2.Close() // Run 的 readFrame 立即 EOF 返回
	_ = mc.Run(context.Background())

	done := make(chan struct{})
	go func() {
		mc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after Run terminated")
	}
}

// TestMuxConn_CloseFailsOccupiedPendingChannel：fulfillPending 已填满唯一
// 缓冲后，Close 的 failPending 对同一 reqID 的错误投递走 default 丢弃分支。
func TestMuxConn_CloseFailsOccupiedPendingChannel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, nil)

	respCh := make(chan muxResponse, 1)
	mc.pendingMu.Lock()
	mc.pending[4242] = respCh
	mc.pendingMu.Unlock()

	mc.fulfillPending(4242, muxResponse{msgID: protocol.MsgInvokeResponse, body: []byte("fulfilled")})

	if err := mc.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}

	// 缓冲内仍是先到的 fulfill 结果；failPending 的错误投递被 default 丢弃。
	select {
	case resp := <-respCh:
		if resp.msgID != protocol.MsgInvokeResponse || string(resp.body) != "fulfilled" {
			t.Fatalf("unexpected buffered response: msgID=%#x body=%q", resp.msgID, resp.body)
		}
	default:
		t.Fatal("expected the fulfilled response to remain buffered")
	}
}

// TestServeConn_StopsOnClosingSignal：应答写回后、读下一帧前，serveConn
// 的循环顶 select 必须观测到 s.closing 并优雅退出。用门控 handler 把连接
// 固定在「已收到请求、尚未应答」的位置再白盒置位 closing 信号，避免与
// 真实 Close 的强制断连（write error 路径）竞争。白盒 close(s.closing)
// 绕过了 Close 的 sync.Once，因此测试内不得再调用 srv.Close()。
func TestServeConn_StopsOnClosingSignal(t *testing.T) {
	handlerStarted := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	handler := transportcore.HandlerFunc(func(_ context.Context, _ uint32, _ uint32, body []byte) ([]byte, error) {
		startOnce.Do(func() { close(handlerStarted) })
		<-release
		return body, nil
	})

	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true, RecvTimeout: time.Second}, handler)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ctx) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// writeFrame 自带 4 字节长度前缀，这里只传消息体。
	payload := protocol.NewMessageBody(protocol.MsgInvokeRequest, 7, []byte("ping"))
	if err := writeFrame(conn, payload); err != nil {
		t.Fatalf("write request: %v", err)
	}

	<-handlerStarted
	close(srv.closing) // 置位停机信号
	close(release)     // handler 应答 → serveConn 回到循环顶 → 命中 closing 分支

	// 客户端读到应答即证明响应写回完成；此后 serveConn 在循环顶退出。
	respFrame, err := readFrame(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(respFrame)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if msgID != protocol.MsgInvokeResponse || reqID != 7 || string(body) != "ping" {
		t.Fatalf("unexpected response: msgID=%#x reqID=%d body=%q", msgID, reqID, body)
	}

	// Serve 在下一个 Accept 超时周期（≤1s）内因 closing 返回 nil。
	select {
	case serr := <-serveDone:
		if serr != nil {
			t.Fatalf("Serve returned error: %v", serr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not stop after closing signal")
	}
}
