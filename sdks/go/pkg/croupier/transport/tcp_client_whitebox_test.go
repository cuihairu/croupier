package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// errorConn 是只写失败的 net.Conn 桩：Read 阻塞到关闭，Write 一律报错。
type errorConn struct {
	closed chan struct{}
}

func newErrorConn() *errorConn { return &errorConn{closed: make(chan struct{})} }

func (c *errorConn) Read(b []byte) (int, error) {
	<-c.closed
	return 0, io.EOF
}

func (c *errorConn) Write(b []byte) (int, error) {
	return 0, errors.New("connection is write-broken")
}

func (c *errorConn) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}

func (c *errorConn) LocalAddr() net.Addr                { return nil }
func (c *errorConn) RemoteAddr() net.Addr               { return nil }
func (c *errorConn) SetDeadline(t time.Time) error      { return nil }
func (c *errorConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *errorConn) SetWriteDeadline(t time.Time) error { return nil }

// 手工构造的 TCPClient（不启动读循环）验证 Call 的发送失败分支。
func TestCallWriteFailureReturnsSendError(t *testing.T) {
	c := &TCPClient{
		conn:      newErrorConn(),
		config:    &Config{},
		pending:   make(map[uint32]chan responseTuple),
		nextReqID: 1,
		closing:   make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := c.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil {
		t.Fatal("expected send error")
	}
	if want := "send:"; len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Fatalf("error should wrap send failure, got %v", err)
	}
}

// C-013：writeResponseFrame 写失败不再静默丢弃——回传错误并 failAllPending
// （dead=true，在途 Call 以 errConnectionClosed 失败）。
func TestWriteResponseFrameFailureFailsPending(t *testing.T) {
	respCh := make(chan responseTuple, 1)
	c := &TCPClient{
		conn:      newErrorConn(),
		config:    &Config{},
		pending:   map[uint32]chan responseTuple{42: respCh},
		nextReqID: 1,
		closing:   make(chan struct{}),
	}

	err := c.writeResponseFrame(protocol.MsgInvokeRequest, 42, []byte(`{}`))
	if err == nil {
		t.Fatal("expected write response frame error")
	}
	if want := "write response frame:"; len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Fatalf("error should wrap write failure, got %v", err)
	}

	c.mu.RLock()
	dead := c.dead
	c.mu.RUnlock()
	if !dead {
		t.Fatal("write failure must mark connection dead (failAllPending)")
	}

	select {
	case resp := <-respCh:
		if !errors.Is(resp.err, errConnectionClosed) {
			t.Fatalf("pending call should fail with errConnectionClosed, got %v", resp.err)
		}
	default:
		t.Fatal("pending call was not failed after write failure")
	}
}

// 响应无处投递（pending 通道满且无读者——等待方已取消/超时）时，
// receiveLoop 必须丢弃该响应并继续处理后续帧，不得阻塞投递把整条
// 连接的接收循环卡死（否则同连接后续请求全部无响应）。
func TestReceiveLoopDropsUndeliverableResponseAndContinues(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer func() { _ = serverSide.Close() }()
	defer func() { _ = clientSide.Close() }()

	respMsgID := protocol.GetResponseMsgID(protocol.MsgInvokeRequest)
	makeNormalFrame := func(reqID uint32) []byte {
		body := protocol.NewMessageBody(respMsgID, reqID, []byte(`{}`))
		frame := make([]byte, 4+len(body))
		binary.BigEndian.PutUint32(frame[:4], uint32(len(body)))
		copy(frame[4:], body)
		return frame
	}
	// 不兼容版本（0x7F）的响应帧：手写 payload 头（8 字节 header + 1 字节数据）。
	makeBadVersionFrame := func(reqID uint32) []byte {
		body := make([]byte, protocol.HeaderSize+1)
		body[0] = 0x7F
		protocol.PutMsgID(body[1:4], respMsgID)
		binary.BigEndian.PutUint32(body[4:8], reqID)
		body[protocol.HeaderSize] = '{'
		frame := make([]byte, 4+len(body))
		binary.BigEndian.PutUint32(frame[:4], uint32(len(body)))
		copy(frame[4:], body)
		return frame
	}
	writeFrame := func(frame []byte, label string) {
		t.Helper()
		writeDone := make(chan error, 1)
		go func() {
			_, err := serverSide.Write(frame)
			writeDone <- err
		}()
		select {
		case err := <-writeDone:
			if err != nil {
				t.Fatalf("write %s frame: %v", label, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("write of %s frame timed out: receiveLoop stalled on delivery", label)
		}
	}

	// reqID=7：等待方已取消，容量为 1 的 pending 通道被陈旧响应占满
	// 且无读者——复现取消与重复响应交错的真实形态。
	stale := make(chan responseTuple, 1)
	stale <- responseTuple{msgID: respMsgID, body: []byte(`stale`)}
	// reqID=8：存活中的等待方。
	live := make(chan responseTuple, 1)
	c := &TCPClient{
		conn:      clientSide,
		config:    &Config{},
		pending:   map[uint32]chan responseTuple{7: stale, 8: live},
		nextReqID: 9,
		closing:   make(chan struct{}),
	}
	c.readLoopWg.Add(1)
	go c.receiveLoop()

	// 两条无处投递的响应（不兼容版本 + 正常版本重复帧）都必须被丢弃。
	writeFrame(makeBadVersionFrame(7), "bad-version reqID=7")
	writeFrame(makeNormalFrame(7), "duplicate reqID=7")
	// 随后存活请求的响应必须照常送达——证明接收循环未被前两条卡死。
	writeFrame(makeNormalFrame(8), "live reqID=8")

	select {
	case resp := <-live:
		if resp.msgID != respMsgID {
			t.Fatalf("live call got unexpected msgID %d, want %d", resp.msgID, respMsgID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receiveLoop stalled: live call never received response")
	}

	// 两条废弃响应均被丢弃而非入队：reqID=7 通道仍只有陈旧那条。
	if len(stale) != 1 {
		t.Fatalf("stale channel should hold exactly 1 tuple after drops, got %d", len(stale))
	}

	// 收尾：连接断开后读循环退出、不悬挂。
	_ = clientSide.Close()
	done := make(chan struct{})
	go func() {
		c.readLoopWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("receiveLoop did not exit after conn close")
	}
}
