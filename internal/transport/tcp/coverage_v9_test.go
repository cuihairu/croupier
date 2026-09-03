package tcp

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// v9DeadlineErrConn 所有 Set*Deadline 均失败，用于触发 Client.Call 的
// deadline 设置错误分支。
type v9DeadlineErrConn struct{ net.Conn }

func (v9DeadlineErrConn) SetDeadline(time.Time) error      { return errors.New("deadline boom") }
func (v9DeadlineErrConn) SetWriteDeadline(time.Time) error { return errors.New("wdeadline boom") }
func (v9DeadlineErrConn) SetReadDeadline(time.Time) error  { return errors.New("rdeadline boom") }

func TestClientCall_SetDeadlineErrorV9(t *testing.T) {
	client := &Client{config: &Config{}, conn: v9DeadlineErrConn{}, closing: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte("x"))
	if err == nil || !containsStr(err.Error(), "set deadline") {
		t.Fatalf("expected set deadline error, got %v", err)
	}
}

func TestClientCall_WriteDeadlineErrorV9(t *testing.T) {
	client := &Client{
		config:  &Config{SendTimeout: time.Second},
		conn:    v9DeadlineErrConn{},
		closing: make(chan struct{}),
	}
	_, _, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil || !containsStr(err.Error(), "set write deadline") {
		t.Fatalf("expected write deadline error, got %v", err)
	}
}

func TestClientCall_ReadDeadlineErrorV9(t *testing.T) {
	client := &Client{
		config:  &Config{RecvTimeout: time.Second},
		conn:    v9DeadlineErrConn{},
		closing: make(chan struct{}),
	}
	_, _, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil || !containsStr(err.Error(), "set read deadline") {
		t.Fatalf("expected read deadline error, got %v", err)
	}
}

// TestClientCall_ReadFrameErrorV9 覆盖 client.go:111-113：对端读走请求后
// 立即断开，readFrame 失败。
func TestClientCall_ReadFrameErrorV9(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = readFrame(conn)
		_ = conn.Close()
	}()

	client, err := NewClient(&Config{Address: ln.Addr().String(), Insecure: true, ConnectTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	_, _, err = client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil {
		t.Fatal("expected read frame error after peer close")
	}
}

// TestReadFrame_TruncatedPayloadV9 覆盖 framing.go:46-48：header 声明的
// 长度超过实际数据。
func TestReadFrame_TruncatedPayloadV9(t *testing.T) {
	header := make([]byte, frameHeaderBytes)
	binary.BigEndian.PutUint32(header, 8)
	data := append(header, []byte("ab")...) // 只有 2 字节，声明 8 字节
	_, err := readFrame(io.Reader(bytesReader(data)))
	if err == nil {
		t.Fatal("expected unexpected EOF for truncated payload")
	}
}

// TestMuxConnDispatch_ControlLaneCtxDoneV9 覆盖 mux_conn.go:180-181：
// 控制车道投递阻塞时 ctx 取消。
func TestMuxConnDispatch_ControlLaneCtxDoneV9(t *testing.T) {
	mc := &MuxConn{
		ctrlInbox: make(chan muxTask),
		bizInbox:  make(chan muxTask),
		closed:    make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := mc.dispatchInbound(ctx, protocol.MsgHeartbeatRequest, 1, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestMuxConnDispatch_ControlLaneClosedV9 覆盖 mux_conn.go:178-179：
// 控制车道投递阻塞时连接关闭。
func TestMuxConnDispatch_ControlLaneClosedV9(t *testing.T) {
	mc := &MuxConn{
		ctrlInbox: make(chan muxTask),
		bizInbox:  make(chan muxTask),
		closed:    make(chan struct{}),
	}
	close(mc.closed)
	if err := mc.dispatchInbound(context.Background(), protocol.MsgHeartbeatRequest, 1, nil); err != nil {
		t.Fatalf("expected nil on closed conn, got %v", err)
	}
}

// TestMuxConnDispatch_BusyWriteErrorV9 覆盖 mux_conn.go:199-201：业务车道
// 饱和时内联 busy 帧写入失败。
func TestMuxConnDispatch_BusyWriteErrorV9(t *testing.T) {
	c1, _ := net.Pipe()
	requireNoError(t, c1.Close())
	mc := &MuxConn{
		ctrlInbox: make(chan muxTask),
		bizInbox:  make(chan muxTask, 1),
		closed:    make(chan struct{}),
		conn:      c1,
	}
	mc.bizInbox <- muxTask{} // 打满队列
	if err := mc.dispatchInbound(context.Background(), protocol.MsgInvokeRequest, 1, []byte(`{}`)); err == nil {
		t.Fatal("expected busy-frame write error on closed conn")
	}
}

// TestMuxConnRun_EventHandlerProtocolErrorV9 覆盖 mux_conn.go:281-284：
// 事件 handler 返回 ProtocolError 时 Run 终止。
func TestMuxConnRun_EventHandlerProtocolErrorV9(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, NewProtocolError(errors.New("event violation"))
	}))
	defer mc.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	if err := writeFrame(c2, protocol.NewMessageBody(protocol.MsgTaskEvent, 0, []byte(`{}`))); err != nil {
		t.Fatalf("write event: %v", err)
	}
	select {
	case err := <-errCh:
		if err == nil || !isProtocolError(err) {
			t.Fatalf("expected protocol error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate on event protocol error")
	}
}

// v9WriteFailConn 读透传、写失败，用于让 dispatchInbound 的 busy 写在
// Run 循环内返回非超时错误。
type v9WriteFailConn struct{ net.Conn }

func (v9WriteFailConn) Write([]byte) (int, error) { return 0, errors.New("write boom") }

// TestMuxConnRun_DispatchNonTimeoutErrorV9 覆盖 mux_conn.go:305：
// busy 帧写入失败（非超时）时 Run 直接返回错误。
func TestMuxConnRun_DispatchNonTimeoutErrorV9(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	release := make(chan struct{})
	defer close(release)
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		<-release
		return body, nil
	})
	mc := NewMuxConn(v9WriteFailConn{c1}, &Config{DispatchWorkers: 1, BusinessQLen: 1}, handler)
	defer mc.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgInvokeRequest, 1, []byte(`{}`)) // worker 占用
	time.Sleep(50 * time.Millisecond)
	peer.sendRequest(protocol.MsgInvokeRequest, 2, []byte(`{}`)) // 队列占满
	peer.sendRequest(protocol.MsgInvokeRequest, 3, []byte(`{}`)) // 饱和 → busy 写失败

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected dispatch write error to terminate Run")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

// TestMuxConnRun_DispatchTimeoutErrorV9 覆盖 mux_conn.go:302-304：控制车道
// 阻塞投递期间 ctx 超时，dispatchInbound 返回 DeadlineExceeded（属于
// timeout），Run 经 isTimeout 分支返回。
func TestMuxConnRun_DispatchTimeoutErrorV9(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	release := make(chan struct{})
	defer close(release)
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		<-release
		return body, nil
	})
	mc := NewMuxConn(c1, &Config{DispatchWorkers: 1, BusinessQLen: 1, ControlQLen: 1}, handler)
	defer mc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(ctx) }()

	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgHeartbeatRequest, 1, nil) // worker 占用
	time.Sleep(50 * time.Millisecond)
	peer.sendRequest(protocol.MsgHeartbeatRequest, 2, nil) // 控制队列占满
	peer.sendRequest(protocol.MsgHeartbeatRequest, 3, nil) // 投递阻塞至 ctx 超时

	select {
	case err := <-errCh:
		if err == nil || !isTimeout(err) {
			t.Fatalf("expected timeout error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate on control-lane ctx timeout")
	}
}

func TestMuxConnIsClosed_NilReceiverV9(t *testing.T) {
	var mc *MuxConn
	if !mc.IsClosed() {
		t.Fatal("nil MuxConn should report closed")
	}
}

func newV9EchoServer(t *testing.T, recv, send time.Duration) *Server {
	t.Helper()
	srv, err := NewServer(&Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		RecvTimeout: recv,
		SendTimeout: send,
	}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

// TestServerServe_AcceptTimeoutThenClosedV9 覆盖 server.go:85-87：
// accept 超时后 ctx 未取消 → continue；随后 listener 被关闭 → accept 错误返回。
func TestServerServe_AcceptTimeoutThenClosedV9(t *testing.T) {
	srv := newV9EchoServer(t, 0, 0)
	defer srv.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(context.Background()) }()

	// accept deadline 为 1s；等待一次超时 continue。
	time.Sleep(1100 * time.Millisecond)
	requireNoError(t, srv.listener.Close())

	select {
	case err := <-errCh:
		if err == nil || !containsStr(err.Error(), "accept") {
			t.Fatalf("expected accept error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after listener closed")
	}
}

// TestServerServeConn_ContextDoneV9 覆盖 server.go:105-106：ctx 取消后
// 读超时让循环回到顶部 select，serveConn 经 ctx.Done 退出。
func TestServerServeConn_ContextDoneV9(t *testing.T) {
	srv := newV9EchoServer(t, 150*time.Millisecond, 0)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 等待至少一次读超时，使读循环回到顶部 select 后再取消 ctx。
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
}

// TestServerServeConn_ServerCloseV9 覆盖 server.go:107-108：Server.Close
// 关闭活跃连接，serveConn 退出。
func TestServerServeConn_ServerCloseV9(t *testing.T) {
	srv := newV9EchoServer(t, 0, 0)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(context.Background()) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)
	requireNoError(t, srv.Close())

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve should return nil on Close, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after server close")
	}
}

// TestServerServeConn_ReadTimeoutContinueV9 覆盖 server.go:118-119：
// 空闲读超时后循环继续，后续请求仍可处理。
func TestServerServeConn_ReadTimeoutContinueV9(t *testing.T) {
	srv := newV9EchoServer(t, 150*time.Millisecond, time.Second)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 等待至少一次读超时 continue。
	time.Sleep(300 * time.Millisecond)

	req := protocol.NewMessageBody(protocol.MsgInvokeRequest, 1, []byte("alive"))
	if err := writeFrame(conn, req); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	frame, err := readFrame(conn)
	if err != nil {
		t.Fatalf("read after idle timeout: %v", err)
	}
	_, _, _, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if string(body) != "alive" {
		t.Fatalf("body = %q, want alive", body)
	}
}

// TestServerServeConn_WriteFrameErrorV9 覆盖 server.go:151-153：响应超大
// 且对端不读时写超时，serveConn 退出并关闭连接。
func TestServerServeConn_WriteFrameErrorV9(t *testing.T) {
	srv, err := NewServer(&Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		SendTimeout: 200 * time.Millisecond,
	}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return make([]byte, 8<<20), nil // 8MB，超过收发缓冲
	}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve(context.Background()) }()

	client, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	if tc, ok := client.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(2048)
	}

	req := protocol.NewMessageBody(protocol.MsgInvokeRequest, 1, []byte("big"))
	if err := writeFrame(client, req); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 从不主动读响应；服务端写满缓冲后写超时断开。初始若干字节会被
	// 内核接收缓冲收下，需持续读直到连接被服务端关闭。
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 64)
	gotErr := false
	for {
		if _, err := client.Read(buf); err != nil {
			gotErr = true
			break
		}
	}
	if !gotErr {
		t.Fatal("expected connection close after server write timeout")
	}
}

// TestCreateServerTLSConfig_WithCAV9 覆盖 server.go:246-247：合法 CA 文件
// 设置 ClientCAs 与 RequireAndVerifyClientCert。
func TestCreateServerTLSConfig_WithCAV9(t *testing.T) {
	certPath, keyPath := genSelfSignedCert(t, t.TempDir())
	cfg, err := createServerTLSConfig(&Config{CertFile: certPath, KeyFile: keyPath, CAFile: certPath})
	if err != nil {
		t.Fatalf("createServerTLSConfig: %v", err)
	}
	if cfg.ClientCAs == nil {
		t.Fatal("ClientCAs should be set from CAFile")
	}
	if cfg.ClientAuth != 4 { // tls.RequireAndVerifyClientCert
		t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert", cfg.ClientAuth)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOfSub(s, sub) >= 0)
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
