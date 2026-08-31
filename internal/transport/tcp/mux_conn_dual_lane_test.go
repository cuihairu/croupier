package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// 双车道派发测试：业务车道饱和时（fail-fast busy），控制车道（心跳等）
// 仍然可达——会话存活不受业务过载影响。

// muxTestPeer 驱动 MuxConn 的对端：直接读写帧（net.Pipe 同步语义）。
type muxTestPeer struct {
	conn net.Conn
	t    *testing.T
}

func (p *muxTestPeer) sendRequest(msgID uint32, reqID uint32, body []byte) {
	payload := protocol.NewMessageBody(msgID, reqID, body)
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)
	if _, err := p.conn.Write(frame); err != nil {
		p.t.Fatalf("send request: %v", err)
	}
}

func (p *muxTestPeer) readFrame() (msgID uint32, reqID uint32, body []byte) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(p.conn, hdr); err != nil {
		p.t.Fatalf("read frame header: %v", err)
	}
	n := binary.BigEndian.Uint32(hdr)
	buf := make([]byte, n)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		p.t.Fatalf("read frame body: %v", err)
	}
	_, msgID, reqID, data, err := protocol.ParseMessageFromBody(buf)
	if err != nil {
		p.t.Fatalf("parse frame: %v", err)
	}
	return msgID, reqID, data
}

func TestMuxConn_DualLane_BusinessSaturation(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	// 业务 handler 慢（阻塞直到释放）；控制 handler 计数即时返回。
	var ctrlHandled atomic.Int64
	release := make(chan struct{})
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if protocol.IsControlRequest(msgID) {
			ctrlHandled.Add(1)
			return body, nil
		}
		<-release // 业务请求挂起
		return body, nil
	})

	cfg := &Config{DispatchWorkers: 1, BusinessQLen: 1, ControlQLen: 8}
	mc := NewMuxConn(c1, cfg, handler)
	go func() { _ = mc.Run(context.Background()) }()
	defer mc.Close()

	peer := &muxTestPeer{conn: c2, t: t}

	// 打满业务车道：1 worker 挂起 + 1 队列占位 = 2 个在途；第 3 个必饱和。
	peer.sendRequest(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	time.Sleep(50 * time.Millisecond)                            // 等 worker 取走第 1 个
	peer.sendRequest(protocol.MsgInvokeRequest, 2, []byte(`{}`)) // 占队列

	// 第 3 个业务请求：期望立即收到 busy 错误帧（fail-fast）。
	peer.sendRequest(protocol.MsgInvokeRequest, 3, []byte(`{}`))
	done := make(chan struct{})
	var busyMsgID, busyReqID uint32
	var busyBody []byte
	go func() {
		defer close(done)
		busyMsgID, busyReqID, busyBody = peer.readFrame()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("business saturation did not fail fast within 2s")
	}
	if busyReqID != 3 {
		t.Fatalf("busy frame reqID = %d, want 3", busyReqID)
	}
	if busyMsgID != protocol.MsgInvokeResponse {
		t.Fatalf("busy frame msgID = %#x, want InvokeResponse", busyMsgID)
	}
	if !bytes.Contains(busyBody, []byte("inbound queue full")) {
		t.Fatalf("busy frame body = %q, want queue-full error", busyBody)
	}

	// 业务车道已饱和的情况下：心跳（控制车道）仍然被处理。
	peer.sendRequest(protocol.MsgHeartbeatRequest, 4, []byte(`{}`))
	hbMsgID, hbReqID, _ := peer.readFrame()
	// 注意：挂起的业务响应尚未写（handler 阻塞），此处读到的心跳响应证明
	// 控制请求没有排在业务之后。
	if hbReqID != 4 || hbMsgID != protocol.MsgHeartbeatResponse {
		t.Fatalf("heartbeat frame = (%#x, %d), want HeartbeatResponse/4", hbMsgID, hbReqID)
	}
	if ctrlHandled.Load() != 1 {
		t.Fatalf("control handled = %d, want 1", ctrlHandled.Load())
	}

	// 释放业务 handler，收尾（drain 挂起响应，避免 pipe 泄漏干扰）。
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for i := 0; i < 2; i++ {
		_, _, _ = peer.readFrame()
		if time.Now().After(deadline) {
			t.Fatal("timed out draining business responses")
		}
	}
}

func TestMuxConn_DualLane_ControlNeverRejected(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	// 控制车道打满（ControlQLen=1 + 1 worker 占用）后，再投控制请求：
	// 读循环应阻塞等待（自然背压）而非回 reject；释放后全部完成。
	release := make(chan struct{})
	var handled atomic.Int64
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		<-release
		handled.Add(1)
		return body, nil
	})

	mc := NewMuxConn(c1, &Config{DispatchWorkers: 1, BusinessQLen: 1, ControlQLen: 1}, handler)
	go func() { _ = mc.Run(context.Background()) }()
	defer mc.Close()

	peer := &muxTestPeer{conn: c2, t: t}

	// worker 占用 + 队列占满。
	peer.sendRequest(protocol.MsgHeartbeatRequest, 1, nil)
	time.Sleep(50 * time.Millisecond)
	peer.sendRequest(protocol.MsgHeartbeatRequest, 2, nil)
	// 第 3 个控制请求：读循环阻塞（不 reject、不断连）。
	peer.sendRequest(protocol.MsgHeartbeatRequest, 3, nil)
	time.Sleep(100 * time.Millisecond)

	// 释放：三个心跳全部应答（1 从 worker、1 从队列、1 从阻塞读循环）。
	close(release)
	for i := 0; i < 3; i++ {
		if _, reqID, _ := func() (uint32, uint32, []byte) {
			m, r, b := peer.readFrame()
			return m, r, b
		}(); reqID < 1 || reqID > 3 {
			t.Fatalf("unexpected reqID %d", reqID)
		}
	}
	if handled.Load() != 3 {
		t.Fatalf("handled = %d, want 3", handled.Load())
	}
}

func TestMuxConn_DualLane_ConcurrentWritesSafe(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	var wg sync.WaitGroup
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		wg.Done()
		return body, nil
	})

	mc := NewMuxConn(c1, &Config{DispatchWorkers: 4, BusinessQLen: 64, ControlQLen: 8}, handler)
	go func() { _ = mc.Run(context.Background()) }()
	defer mc.Close()

	peer := &muxTestPeer{conn: c2, t: t}

	const total = 32
	wg.Add(total)
	for i := 1; i <= total; i++ {
		peer.sendRequest(protocol.MsgInvokeRequest, uint32(i), []byte(`{}`))
	}

	// 全部收到响应且 reqID 无损（写锁保证帧不交错）。
	got := make(map[uint32]bool, total)
	for i := 0; i < total; i++ {
		_, reqID, _ := peer.readFrame()
		got[reqID] = true
	}
	wg.Wait()
	for i := 1; i <= total; i++ {
		if !got[uint32(i)] {
			t.Fatalf("missing response for reqID %d", i)
		}
	}
}
