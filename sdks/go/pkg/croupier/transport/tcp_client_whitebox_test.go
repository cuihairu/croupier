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

// receiveLoop 在响应无处投递（pending 通道无接收方）时应让位于 closing 并退出。
func TestReceiveLoopBlockedDeliveryStopsOnClosing(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer func() { _ = serverSide.Close() }()
	defer func() { _ = clientSide.Close() }()

	blocked := make(chan responseTuple) // 无缓冲、无接收方
	c := &TCPClient{
		conn:      clientSide,
		config:    &Config{},
		pending:   map[uint32]chan responseTuple{7: blocked},
		nextReqID: 8,
		closing:   make(chan struct{}),
	}
	c.readLoopWg.Add(1)
	go c.receiveLoop()

	// 写入一条 reqID=7 的响应帧；receiveLoop 解析后阻塞在通道投递上。
	respMsgID := protocol.GetResponseMsgID(protocol.MsgInvokeRequest)
	body := protocol.NewMessageBody(respMsgID, 7, []byte(`{}`))
	frame := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(body)))
	copy(frame[4:], body)
	writeDone := make(chan error, 1)
	go func() {
		_, err := serverSide.Write(frame)
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("write frame: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("frame write timed out")
	}

	// 等待投递阻塞生效，再关闭 closing 让 receiveLoop 退出。
	time.Sleep(100 * time.Millisecond)
	close(c.closing)

	done := make(chan struct{})
	go func() {
		c.readLoopWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("receiveLoop did not exit after closing")
	}
}
