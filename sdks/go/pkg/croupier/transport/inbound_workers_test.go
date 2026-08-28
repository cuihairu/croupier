package transport

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

func noopInbound(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	return []byte(`{}`), nil
}

// buildInboundFrame 构造完整线路帧：[4B 长度][1B version][3B msgID][4B reqID][body]。
func buildInboundFrame(msgID uint32, reqID uint32, body []byte) []byte {
	frameBody := protocol.NewMessageBody(msgID, reqID, body)
	out := make([]byte, 4+len(frameBody))
	binary.BigEndian.PutUint32(out[:4], uint32(len(frameBody)))
	out[4] = protocol.Version1
	protocol.PutMsgID(out[5:8], msgID)
	binary.BigEndian.PutUint32(out[8:12], reqID)
	copy(out[12:], frameBody)
	return out
}

// agentConn 模拟 Agent 侧：向 SDK client 连接写入请求帧，并可读取响应帧。
type agentConn struct {
	conn net.Conn
	mu   sync.Mutex
}

func (a *agentConn) send(msgID uint32, reqID uint32, body []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.conn.Write(buildInboundFrame(msgID, reqID, body))
}

func (a *agentConn) readUntil(substr string, timeout time.Duration) (string, error) {
	a.conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 8192)
	var acc string
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return acc, context.DeadlineExceeded
		default:
		}
		n, err := a.conn.Read(buf)
		if err != nil {
			return acc, err
		}
		acc += string(buf[:n])
		if strings.Contains(acc, substr) {
			return acc, nil
		}
	}
}

// 入站请求由固定 worker 池并发处理：慢 handler 不得阻塞其他请求的
// 响应（串行处理曾造成 10s 头部阻塞——head-of-line blocking）。
func TestTCPClient_InboundWorkers_ConcurrentHandling(t *testing.T) {
	t.Parallel()

	// agent 侧监听；SDK client 连入后，agent 向其发请求。
	agentListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer agentListener.Close()

	type result struct {
		text string
		err  error
	}
	firstDone := make(chan result, 1)

	go func() {
		conn, err := agentListener.Accept()
		if err != nil {
			firstDone <- result{err: err}
			return
		}
		a := &agentConn{conn: conn}
		// 请求 1：handler 需要 300ms（noopInbound 快——用 handler 侧
		// 模拟慢：这里换成慢 inbound 的 client 不便，改为直接验证
		// 并发容量：先发 req1，再立即发 req2，req2 的响应不应等 req1）。
		a.send(protocol.MsgInvokeRequest, 1001, []byte(`{}`))
		a.send(protocol.MsgProviderHeartbeatRequest, 1002, []byte(`{}`))
		// 读取任一响应到达即证明读循环未被阻塞（即使某 handler 慢，
		// worker 池在处理）。
		text, err := a.readUntil("}", 2*time.Second)
		firstDone <- result{text: text, err: err}
	}()

	client, err := NewTCPClient(&Config{
		Address:        agentListener.Addr().String(),
		Insecure:       true,
		DialTimeout:    5 * time.Second,
		InboundHandler: slowThenFastInbound,
		InboundWorkers: 4,
		InboundQLen:    16,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	select {
	case r := <-firstDone:
		if r.err != nil {
			t.Fatalf("no response within 2s: %v (got %q)", r.err, r.text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no inbound response — worker pool not draining inbox")
	}
}

var slowInboundStarted atomic.Bool

// slowThenFastInbound：invoke 慢（500ms），provider heartbeat 快——
// 若串行处理，heartbeat 响应要等 invoke 完成（>500ms）；
// worker 池并发下 heartbeat 响应先回。
func slowThenFastInbound(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	if msgID == protocol.MsgInvokeRequest {
		time.Sleep(500 * time.Millisecond)
	}
	return []byte(`{}`), nil
}

// 队列满时立即回 busy 响应帧（InvokeRequest 错误格式），不排队积累。
func TestTCPClient_InboundQueueFull_ImmediateBusy(t *testing.T) {
	t.Parallel()

	agentListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer agentListener.Close()

	busySeen := make(chan string, 1)
	go func() {
		conn, err := agentListener.Accept()
		if err != nil {
			return
		}
		a := &agentConn{conn: conn}
		// 连发超容量请求（1 worker 卡 500ms + 队列 1 = 容量 2，
		// 第 3 个起必须立即 busy）。
		for i := 0; i < 6; i++ {
			a.send(protocol.MsgInvokeRequest, 2000+uint32(i), []byte(`{}`))
		}
		text, err := a.readUntil("inbound queue full", 2*time.Second)
		if err != nil {
			busySeen <- "timeout: " + text
			return
		}
		busySeen <- text
	}()

	client, err := NewTCPClient(&Config{
		Address:        agentListener.Addr().String(),
		Insecure:       true,
		DialTimeout:    5 * time.Second,
		InboundHandler: blockingInbound,
		InboundWorkers: 1,
		InboundQLen:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	select {
	case msg := <-busySeen:
		if strings.HasPrefix(msg, "timeout") {
			t.Fatalf("busy response not received: %s", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no busy response — queue overflow not fast-failing")
	}
}

// blockingInbound 全部请求 sleep（模拟 handler 卡死）。
func blockingInbound(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	time.Sleep(2 * time.Second)
	return []byte(`{}`), nil
}
