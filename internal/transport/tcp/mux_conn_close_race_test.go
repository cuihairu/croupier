package tcp

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// blockingWriteConn 在整个请求帧写完（MuxConn writeFrame 的收尾
// SetWriteDeadline(zero) 触发）后阻塞，直到对端已应答且 MuxConn 观测到
// 连接关闭——强制制造「响应已入队 + closed 已就绪」同时出现的 select 竞态窗口。
type blockingWriteConn struct {
	net.Conn
	afterWrite func()
	afterOnce  bool
}

func (c *blockingWriteConn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	if c.afterWrite != nil && !c.afterOnce && t.IsZero() {
		c.afterOnce = true
		c.afterWrite()
	}
	return err
}

// callRespondThenClose 跑一轮「对端写完响应立刻关闭连接」的 Call，返回错误说明。
func callRespondThenClose(t *testing.T) string {
	t.Helper()
	raw1, raw2 := net.Pipe()
	defer func() { _ = raw2.Close() }()

	respDone := make(chan struct{})
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, nil
	})

	var mc *MuxConn
	wrapped := &blockingWriteConn{Conn: raw1, afterWrite: func() {
		<-respDone
		deadline := time.Now().Add(2 * time.Second)
		for !mc.IsClosed() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
	}}
	mc = NewMuxConn(wrapped, &Config{SendTimeout: time.Second}, handler)
	defer func() { _ = mc.Close() }()

	runDone := make(chan error, 1)
	go func() { runDone <- mc.Run(context.Background()) }()

	go func() {
		defer close(respDone)
		payload, err := readFrame(raw2)
		if err != nil {
			return
		}
		_, msgID, reqID, _, err := protocol.ParseMessageFromBody(payload)
		if err != nil {
			return
		}
		_ = writeFrame(raw2, protocol.NewMessageBody(protocol.GetResponseMsgID(msgID), reqID, []byte("ok")))
		_ = raw2.Close()
	}()

	respID, body, err := mc.Call(context.Background(), protocol.MsgInvokeRequest, []byte(`{}`))
	if err != nil {
		return "Call error: " + err.Error()
	}
	if respID != protocol.MsgInvokeResponse {
		return "wrong respID"
	}
	if string(body) != "ok" {
		return "wrong body: " + string(body)
	}
	// Run 应因对端关闭而退出（经 Close 清理）。
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		return "Run did not observe peer close"
	}
	return ""
}

// TestMuxConn_Call_PrefersDeliveredResponseOverClosed 锁定 Call 的响应优先语义：
// 对端写完响应立刻关闭连接时，Run 先 fulfill 响应、随后因 EOF 触发 Close；
// 若 Call 的 select 在两者同时就绪时偏向 closed，已成功送达的响应会被误报为
// connection closed（e2e-agent-probe 的 register 用例曾按 ~15% 概率复现）。
func TestMuxConn_Call_PrefersDeliveredResponseOverClosed(t *testing.T) {
	for i := 0; i < 20; i++ {
		if msg := callRespondThenClose(t); msg != "" {
			t.Fatalf("round %d: %s", i, msg)
		}
	}
}

// silentWriteConn 吞掉所有写入并恒成功——模拟「连接已关但 writeFrame 仍返回
// 成功」的窗口（真实连接在该窗口写入会失败并提前返回）。
type silentWriteConn struct {
	net.Conn
}

func (c *silentWriteConn) Write(b []byte) (int, error) { return len(b), nil }

// TestMuxConn_Call_ClosedBeforeMountEmptyQueue 覆盖终 select 的 closed+空队列
// 分支：Close 经 callPrecheckDone 接缝完成于 closed 预检与 pending 挂载之间，
// failPending 因而看不到本请求的通道，终 select 只剩 closed 就绪。
func TestMuxConn_Call_ClosedBeforeMountEmptyQueue(t *testing.T) {
	raw, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	mc := NewMuxConn(&silentWriteConn{Conn: raw}, nil, nil)
	defer func() { _ = mc.Close() }()

	old := callPrecheckDone
	callPrecheckDone = func() { _ = mc.Close() }
	defer func() { callPrecheckDone = old }()

	_, _, err := mc.Call(context.Background(), protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil || err.Error() != "connection closed" {
		t.Fatalf("Call() err = %v, want connection closed", err)
	}
}
