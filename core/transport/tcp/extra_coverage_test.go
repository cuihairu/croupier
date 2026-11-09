package tcp

// client / framing / mux_conn / tls_server 补测：Send 的 nil ctx 与已取消
// ctx 臂、Send 全路径（lock/未关闭 default/写 deadline 双分支/延迟清
// deadline/writeFrame）、SetWriteDeadline 失败臂、已关闭 closing 臂、
// IsClosed 存活臂、CreateClientTLSConfig 与 NormalizeAddr 包装、VerifyConnection
// 装配（client 与 server 两侧）、WriteFrame/ReadFrame 超限臂、readLoop 的
// 读超时 continue 臂、未支持版本响应写失败臂、事件无 handler continue 臂、
// 事件 handler 无错 continue 臂、请求无 handler 协议错误臂、handleInboundRequest
// 协议错误透传臂。
//
// 诚实跳过：无——全部可经 net.Pipe / 连接包装 / 直构注入。

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
)

// pipeClient 构造一个存活 client（net.Pipe + 对端排空 goroutine），供 Send
// 全路径使用；cleanup 关闭双端并等待排空退出。
func pipeClient(t *testing.T) (*Client, func()) {
	t.Helper()
	c1, c2 := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, c2)
	}()
	client := &Client{config: &Config{}, conn: c1, closing: make(chan struct{})}
	return client, func() {
		_ = client.Close()
		_ = c2.Close()
		<-done
	}
}

// deadlineFailConn 使 SetWriteDeadline 恒错（触发 Send 的写 deadline 失败臂）。
type deadlineFailConn struct{ net.Conn }

func (deadlineFailConn) SetWriteDeadline(time.Time) error { return errors.New("deadline boom") }

// timeoutReadConn 首次 Read 返回超时、其后返回 EOF：readLoop 的
// isTimeout → continue 臂命中后以 EOF 终止。
type timeoutReadConn struct {
	net.Conn
	reads int
}

func (c *timeoutReadConn) Read(p []byte) (int, error) {
	c.reads++
	if c.reads == 1 {
		return 0, timeoutErr{}
	}
	return 0, io.EOF
}

type timeoutErr struct{}

func (timeoutErr) Error() string { return "read timeout" }
func (timeoutErr) Timeout() bool { return true }

func TestClientSendNilAndCancelledCtx(t *testing.T) {
	// nil ctx → Background，且非事件 msgID 在触网前被拒
	require.ErrorContains(t,
		(&Client{}).Send(nil, protocol.MsgInvokeRequest, nil), "not an event")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t,
		(&Client{}).Send(ctx, protocol.MsgInvokeRequest, nil), context.Canceled)
}

func TestClientSendClosedClosing(t *testing.T) {
	c1, _ := net.Pipe()
	closed := make(chan struct{})
	close(closed)
	client := &Client{config: &Config{}, conn: c1, closing: closed}
	require.ErrorContains(t,
		client.Send(context.Background(), protocol.MsgTaskEvent, []byte(`{}`)),
		"client is closing")
}

func TestClientSendFullPath(t *testing.T) {
	client, cleanup := pipeClient(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// ctx 带 deadline → 150-152（SetWriteDeadline 成功）
	require.NoError(t, client.Send(ctx, protocol.MsgTaskEvent, []byte(`{}`)))

	client2, cleanup2 := pipeClient(t)
	defer cleanup2()
	client2.config = &Config{SendTimeout: time.Second}
	// ctx 无 deadline + SendTimeout>0 → 154-156
	require.NoError(t, client2.Send(context.Background(), protocol.MsgTaskEvent, []byte(`{}`)))
}

func TestClientSendDeadlineSetError(t *testing.T) {
	// ctx 带 deadline，SetWriteDeadline 失败 → 151-153
	c1, _ := net.Pipe()
	client := &Client{config: &Config{}, conn: deadlineFailConn{c1}, closing: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.ErrorContains(t,
		client.Send(ctx, protocol.MsgTaskEvent, []byte(`{}`)), "set write deadline")

	// SendTimeout 分支，SetWriteDeadline 失败 → 155-157
	c2, _ := net.Pipe()
	client2 := &Client{config: &Config{SendTimeout: time.Second}, conn: deadlineFailConn{c2}, closing: make(chan struct{})}
	require.ErrorContains(t,
		client2.Send(context.Background(), protocol.MsgTaskEvent, []byte(`{}`)), "set write deadline")
}

func TestClientIsClosedLiveAndWrappers(t *testing.T) {
	require.False(t, (&Client{closing: make(chan struct{})}).IsClosed())

	cfg, err := CreateClientTLSConfig(&Config{})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	cfg, err = CreateClientTLSConfig(&Config{VerifyConnection: func(tls.ConnectionState) error { return nil }})
	require.NoError(t, err)
	require.NotNil(t, cfg.VerifyConnection)

	require.Equal(t, "host:1", NormalizeAddr(" tcp://host:1 "))
}

func TestWriteFrameTooLarge(t *testing.T) {
	require.ErrorContains(t,
		WriteFrame(io.Discard, make([]byte, MaxFrameBytes+1)), "frame too large")
}

func TestReadFrameTooLarge(t *testing.T) {
	hdr := make([]byte, FrameHeaderBytes)
	binary.BigEndian.PutUint32(hdr, uint32(MaxFrameBytes+1))
	_, err := ReadFrame(bytes.NewReader(hdr))
	require.ErrorContains(t, err, "frame too large")
}

func TestServerTLSConfigVerifyConnection(t *testing.T) {
	cfg, err := ServerTLSConfig(&Config{VerifyConnection: func(tls.ConnectionState) error { return nil }})
	require.NoError(t, err)
	require.NotNil(t, cfg.VerifyConnection)
}

// runMuxAndTerminate 启动 Run 并等待其返回（限时），返回其结果。
func runMuxAndTerminate(t *testing.T, mc *MuxConn) error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()
	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
		return nil
	}
}

func TestMuxConnRun_ReadTimeoutContinues(t *testing.T) {
	c1, _ := net.Pipe()
	mc := NewMuxConn(&timeoutReadConn{Conn: c1}, &Config{}, nil)
	defer func() { _ = mc.Close() }()
	// 首次读超时 → continue（220），二次读 EOF → 返回错误
	require.Error(t, runMuxAndTerminate(t, mc))
}

func TestMuxConnRun_EventNilHandlerContinues(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, &Config{}, nil)
	defer func() { _ = mc.Close() }()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()
	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgTaskEvent, 0, []byte(`{}`))
	time.Sleep(50 * time.Millisecond)
	mc.Close()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

func TestMuxConnRun_EventHandlerNilReturnContinues(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, &Config{}, transportcore.HandlerFunc(
		func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			return nil, nil
		}))
	defer func() { _ = mc.Close() }()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()
	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgTaskEvent, 0, []byte(`{}`))
	time.Sleep(50 * time.Millisecond)
	mc.Close()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

func TestMuxConnRun_RequestNilHandlerProtocolError(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, &Config{}, nil)
	defer func() { _ = mc.Close() }()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()
	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	select {
	case err := <-errCh:
		require.Error(t, err)
		require.True(t, isProtocolError(err), "expected protocol error, got %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

func TestMuxConnRun_UnsupportedVersionWriteFails(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(writeFailConn{c1}, &Config{}, nil)
	defer func() { _ = mc.Close() }()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	// 手工构造未支持版本（99）请求帧：响应写回失败 → 238 返回错误
	body := []byte(`{}`)
	payload := make([]byte, protocol.HeaderSize+len(body))
	payload[0] = 99
	protocol.PutMsgID(payload[1:4], protocol.MsgInvokeRequest)
	binary.BigEndian.PutUint32(payload[4:8], 1)
	copy(payload[protocol.HeaderSize:], body)
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)
	_, err := c2.Write(frame)
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

func TestMuxConnHandleInboundRequestProtocolError(t *testing.T) {
	mc := &MuxConn{handler: transportcore.HandlerFunc(
		func(ctx context.Context, msgID, reqID uint32, body []byte) ([]byte, error) {
			return nil, NewProtocolError(errors.New("boom"))
		})}
	err := mc.handleInboundRequest(context.Background(), protocol.MsgInvokeRequest, 1, []byte(`{}`))
	require.True(t, isProtocolError(err))
}
