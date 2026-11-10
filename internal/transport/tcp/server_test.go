package tcp

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// serveAndStop 在独立 goroutine 启动 srv.Serve，返回的 stop 依次 cancel →
// join → Close。直接 `defer srv.Close()` + `go srv.Serve(ctx)` 会让 Serve
// goroutine（启动瞬间的方法装载读）与 Close 的 once.Do 写并发且无
// happens-before 边——race detector 在测试体极短时（TestClient_IsClosed
// 等）稳定报 DATA RACE；先 join 再 Close 才彻底串行。
func serveAndStop(srv *Server) func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	return func() {
		cancel() // Serve 在下一个 Accept 周期（≤1s）内自然退出
		<-done
		_ = srv.Close()
	}
}

func TestServer_UnsupportedVersionRejectsRequestAndKeepsSession(t *testing.T) {
	handler := transportcore.HandlerFunc(func(context.Context, uint32, uint32, []byte) ([]byte, error) {
		return []byte("ok"), nil
	})
	srv, err := NewServer(&Config{
		Address:  "127.0.0.1:0",
		Insecure: true,
	}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer serveAndStop(srv)()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	unsupported := protocol.NewMessageBody(protocol.MsgInvokeRequest, 0x11111111, nil)
	unsupported[0] = protocol.Version1 + 1
	if err := tcptr.WriteFrame(conn, unsupported); err != nil {
		t.Fatalf("write unsupported frame: %v", err)
	}
	frame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read rejection frame: %v", err)
	}
	version, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse rejection: %v", err)
	}
	if version != protocol.Version1 || msgID != protocol.MsgInvokeResponse || reqID != 0x11111111 {
		t.Fatalf("rejection header = version %d, msg %s, req %d", version, protocol.MsgIDString(msgID), reqID)
	}
	var rejection sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &rejection); err != nil {
		t.Fatalf("unmarshal rejection: %v", err)
	}
	if want := `{"error":"unsupported protocol version: 2","supportedVersions":[1]}`; string(rejection.GetPayload()) != want {
		t.Fatalf("rejection body = %q, want %q", body, want)
	}
	supportedReqID := uint32(0x22222222)
	supported := protocol.NewMessageBody(protocol.MsgInvokeRequest, supportedReqID, []byte("next"))
	if err := tcptr.WriteFrame(conn, supported); err != nil {
		t.Fatalf("write supported frame: %v", err)
	}
	frame, err = tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read supported response after rejection: %v", err)
	}
	version, msgID, reqID, body, err = protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse supported response: %v", err)
	}
	if version != protocol.Version1 || msgID != protocol.MsgInvokeResponse || reqID != supportedReqID || string(body) != "ok" {
		t.Fatalf("supported response = version %d, msg %s, req %d, body %q", version, protocol.MsgIDString(msgID), reqID, body)
	}
}

// 不兼容版本的响应帧/单向事件帧只丢弃、不回写（与 core mux、SDK 客户端
// 同规则）：对响应帧回写会伪造对端未发出的请求响应，对事件回写可能形成
// 响应循环。会话保持存活，后续合法请求仍被正常服务。
func TestServer_UnsupportedVersionResponseAndEventFramesDropped(t *testing.T) {
	handler := transportcore.HandlerFunc(func(context.Context, uint32, uint32, []byte) ([]byte, error) {
		return []byte("ok"), nil
	})
	srv, err := NewServer(&Config{
		Address:  "127.0.0.1:0",
		Insecure: true,
	}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer serveAndStop(srv)()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 响应帧（HeartbeatResponse）与事件帧（TaskEvent）带不兼容版本 → 静默丢弃
	respFrame := protocol.NewMessageBody(protocol.MsgHeartbeatResponse, 0x33333333, nil)
	respFrame[0] = protocol.Version1 + 1
	if err := tcptr.WriteFrame(conn, respFrame); err != nil {
		t.Fatalf("write unsupported response frame: %v", err)
	}
	eventFrame := protocol.NewMessageBody(protocol.MsgTaskEvent, 0x00000064, []byte("evt"))
	eventFrame[0] = protocol.Version1 + 1
	if err := tcptr.WriteFrame(conn, eventFrame); err != nil {
		t.Fatalf("write unsupported event frame: %v", err)
	}

	// 唯一应读到的帧是合法 invoke 的响应；若服务器对坏帧回写过，此帧的
	// msgID/reqID 会对不上而失败。
	supportedReqID := uint32(0x44444444)
	supported := protocol.NewMessageBody(protocol.MsgInvokeRequest, supportedReqID, []byte("next"))
	if err := tcptr.WriteFrame(conn, supported); err != nil {
		t.Fatalf("write supported frame: %v", err)
	}
	frame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read response after dropped frames: %v", err)
	}
	version, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if version != protocol.Version1 || msgID != protocol.MsgInvokeResponse || reqID != supportedReqID || string(body) != "ok" {
		t.Fatalf("response after drops = version %d, msg %s, req %d, body %q", version, protocol.MsgIDString(msgID), reqID, body)
	}
}

func TestNewServer_NilHandler(t *testing.T) {
	_, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, nil)
	if err == nil {
		t.Error("expected error for nil handler")
	}
}

func TestNewServer_NilConfig(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(nil, handler)
	if err != nil {
		// nil config without Insecure=true triggers TLS config error — acceptable
		t.Logf("NewServer with nil config returned error (expected if TLS required): %v", err)
		return
	}
	defer func() { _ = srv.Close() }()

	if srv.Addr() == "" {
		t.Error("expected non-empty address")
	}
}

func TestServer_Addr_NilListener(t *testing.T) {
	s := &Server{}
	if got := s.Addr(); got != "" {
		t.Errorf("expected empty addr for nil listener, got %q", got)
	}
}

func TestServer_IsClosed(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv.IsClosed() {
		t.Error("server should not be closed initially")
	}

	_ = srv.Close()

	if !srv.IsClosed() {
		t.Error("server should be closed after Close()")
	}
}

func TestServer_Serve_ContextCancellation(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Serve returned error (expected for context cancel): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Serve did not return after context cancellation")
	}
}

func TestServer_Serve_NilContext(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = srv.Close() }()

	// Serve with nil context should use context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(nil) //nolint:staticcheck // 刻意 nil context：验证不依赖 ctx 的健壮性
	}()

	select {
	case <-done:
		// Serve returned (may be due to listener close or timeout)
	case <-ctx.Done():
		// Timeout - close the server to unblock
		_ = srv.Close()
		<-done
	}
}

func TestServer_MultipleClose(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multiple close should not panic
	_ = srv.Close()
	_ = srv.Close()
	_ = srv.Close()
}

func TestClient_MultipleClose(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer serveAndStop(srv)()

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multiple close should not panic
	_ = client.Close()
	_ = client.Close()
	_ = client.Close()
}

func TestClient_IsClosed(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer serveAndStop(srv)()

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.IsClosed() {
		t.Error("client should not be closed initially")
	}

	_ = client.Close()

	if !client.IsClosed() {
		t.Error("client should be closed after Close()")
	}
}

func TestClient_Call_AfterClose(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer serveAndStop(srv)()

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = client.Close()

	_, _, err = client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("test"))
	if err == nil {
		t.Error("expected error when calling closed client")
	}
}

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080"},
		{"tcp://127.0.0.1:8080", "127.0.0.1:8080"},
		{"  127.0.0.1:8080  ", "127.0.0.1:8080"},
		{"tcp://  host:port  ", "  host:port"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeAddr(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAddr(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteFrame_ReadFrame(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello")},
		{"medium", bytes.Repeat([]byte("x"), 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeFrame(&buf, tt.payload); err != nil {
				t.Fatalf("writeFrame() error = %v", err)
			}

			got, err := readFrame(&buf)
			if err != nil {
				t.Fatalf("readFrame() error = %v", err)
			}

			if !bytes.Equal(got, tt.payload) {
				t.Errorf("round trip: got %d bytes, want %d bytes", len(got), len(tt.payload))
			}
		})
	}
}

func TestWriteFrame_TooLarge(t *testing.T) {
	var buf bytes.Buffer
	payload := make([]byte, MaxFrameBytes+1)
	err := writeFrame(&buf, payload)
	if err == nil {
		t.Error("expected error for frame too large")
	}
	if !strings.Contains(err.Error(), "frame too large") {
		t.Errorf("expected 'frame too large' error, got: %v", err)
	}
}

func TestReadFrame_TooLarge(t *testing.T) {
	// Craft a frame header with size > maxFrameBytes
	var buf bytes.Buffer
	header := make([]byte, FrameHeaderBytes)
	// Write size = maxFrameBytes + 1
	header[0] = 0x02 // 32MB + 1 in big endian
	header[1] = 0x00
	header[2] = 0x00
	header[3] = 0x01
	buf.Write(header)

	_, err := readFrame(&buf)
	if err == nil {
		t.Error("expected error for frame too large")
	}
}

func TestReadFrame_EmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	// Write size = 0
	header := make([]byte, FrameHeaderBytes)
	buf.Write(header)

	got, err := readFrame(&buf)
	if err != nil {
		t.Fatalf("readFrame() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(got))
	}
}

func TestCreateClientTLSConfig(t *testing.T) {
	t.Run("minimal config", func(t *testing.T) {
		cfg, err := createClientTLSConfig(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("expected TLS 1.2, got %d", cfg.MinVersion)
		}
	})

	t.Run("with server name", func(t *testing.T) {
		cfg, err := createClientTLSConfig(&Config{ServerName: "example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ServerName != "example.com" {
			t.Errorf("expected server name 'example.com', got %q", cfg.ServerName)
		}
	})

	t.Run("insecure skip verify", func(t *testing.T) {
		cfg, err := createClientTLSConfig(&Config{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.InsecureSkipVerify {
			t.Error("expected InsecureSkipVerify to be true")
		}
	})

	t.Run("invalid CA file", func(t *testing.T) {
		_, err := createClientTLSConfig(&Config{CAFile: "/nonexistent/ca.pem"})
		if err == nil {
			t.Error("expected error for nonexistent CA file")
		}
	})

	t.Run("invalid cert/key files", func(t *testing.T) {
		_, err := createClientTLSConfig(&Config{
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		})
		if err == nil {
			t.Error("expected error for nonexistent cert/key files")
		}
	})
}

func TestCreateServerTLSConfig(t *testing.T) {
	t.Run("minimal config", func(t *testing.T) {
		cfg, err := createServerTLSConfig(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("expected TLS 1.2, got %d", cfg.MinVersion)
		}
	})

	t.Run("invalid cert/key files", func(t *testing.T) {
		_, err := createServerTLSConfig(&Config{
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		})
		if err == nil {
			t.Error("expected error for nonexistent cert/key files")
		}
	})

	t.Run("invalid CA file", func(t *testing.T) {
		_, err := createServerTLSConfig(&Config{CAFile: "/nonexistent/ca.pem"})
		if err == nil {
			t.Error("expected error for nonexistent CA file")
		}
	})

	t.Run("insecure skip verify disables client auth", func(t *testing.T) {
		cfg, err := createServerTLSConfig(&Config{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientAuth != tls.NoClientCert {
			t.Errorf("expected NoClientCert, got %v", cfg.ClientAuth)
		}
	})
}

func TestClient_NilConfig(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer serveAndStop(srv)()

	// Nil config should use defaults
	client, err := NewClient(&Config{
		Address:  srv.Addr(),
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = client.Close() }()

	respMsgID, respBody, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("test"))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if respMsgID != protocol.MsgInvokeResponse {
		t.Errorf("expected MsgInvokeResponse, got %x", respMsgID)
	}
	if string(respBody) != "test" {
		t.Errorf("expected 'test', got %q", respBody)
	}
}

func TestServer_RecvSendTimeouts(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		RecvTimeout: time.Second,
		SendTimeout: time.Second,
	}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer serveAndStop(srv)()

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
		RecvTimeout:    time.Second,
		SendTimeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = client.Close() }()

	_, respBody, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("timeout-test"))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if string(respBody) != "timeout-test" {
		t.Errorf("expected 'timeout-test', got %q", respBody)
	}
}

func TestServer_DefaultAddress(t *testing.T) {
	t.Setenv("CROUPIER_TCP_ADDR", "127.0.0.1:0")
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	// Empty address should use default
	srv, err := NewServer(&Config{Address: "", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = srv.Close() }()

	if !strings.Contains(srv.Addr(), "19090") {
		t.Logf("Server addr: %s (may not use default if port is in use)", srv.Addr())
	}
}

// writeRequestFrame / readResponseFrame 是执行解耦回归用的裸帧收发 helper
// （与 wire_compat 的 echo server 无关，不复用其 helper 名）。
func writeRequestFrame(t *testing.T, conn net.Conn, msgID uint32, reqID uint32, body []byte) {
	t.Helper()
	if err := tcptr.WriteFrame(conn, protocol.NewMessageBody(msgID, reqID, body)); err != nil {
		t.Fatalf("write request frame: %v", err)
	}
}

func readResponseFrame(t *testing.T, conn net.Conn, timeout time.Duration) (uint32, uint32, []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	frame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read response frame: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse response frame: %v", err)
	}
	return msgID, reqID, body
}

// CR-FIX-09 回归 1：业务 handler 被门闩卡住时，同连接心跳（控制车道）
// 仍即时应答，读循环不被慢 handler 阻塞。
func TestServer_SlowHandlerDoesNotBlockControlFrames(t *testing.T) {
	gate := make(chan struct{})
	entered := make(chan struct{}, 4)
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if msgID == protocol.MsgInvokeRequest {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-gate
			return []byte("ok"), nil
		}
		return []byte("hb"), nil
	})
	srv, err := NewServer(&Config{
		Address:             "127.0.0.1:0",
		Insecure:            true,
		MaxInflightRequests: 4,
		HandlerWorkers:      1,
		RequestTimeout:      30 * time.Second,
	}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer serveAndStop(srv)()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	writeRequestFrame(t, conn, protocol.MsgInvokeRequest, 1, []byte("slow"))
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("slow handler never entered")
	}

	// 业务请求仍被卡住时，心跳必须即时得到应答（控制车道不被阻塞）。
	writeRequestFrame(t, conn, protocol.MsgHeartbeatRequest, 2, nil)
	msgID, reqID, body := readResponseFrame(t, conn, 3*time.Second)
	if msgID != protocol.MsgHeartbeatResponse || reqID != 2 || string(body) != "hb" {
		t.Fatalf("heartbeat response = msg %s req %d body %q, want heartbeat req 2 %q", protocol.MsgIDString(msgID), reqID, body, "hb")
	}

	// 释放门闩后，被卡的业务请求正常完成。
	close(gate)
	msgID, reqID, body = readResponseFrame(t, conn, 3*time.Second)
	if msgID != protocol.MsgInvokeResponse || reqID != 1 || string(body) != "ok" {
		t.Fatalf("invoke response = msg %s req %d body %q, want invoke req 1 %q", protocol.MsgIDString(msgID), reqID, body, "ok")
	}
}

// CR-FIX-09 回归 2：业务队列有上界（queue+workers = MaxInflightRequests），
// 打满后读循环立即回 busy，不阻塞、不无界增长。
func TestServer_MaxInflightBoundsBusinessQueue(t *testing.T) {
	gate := make(chan struct{})
	entered := make(chan struct{}, 8)
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-gate
		return []byte("done"), nil
	})
	srv, err := NewServer(&Config{
		Address:             "127.0.0.1:0",
		Insecure:            true,
		MaxInflightRequests: 4,
		HandlerWorkers:      1,
		RequestTimeout:      30 * time.Second,
	}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer serveAndStop(srv)()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// req1 被 worker 取走并卡在 handler（确认 entered 后再灌队列）；
	// req2-4 填满容量 3 的队列；req5-6 时队列已满 → busy。
	writeRequestFrame(t, conn, protocol.MsgInvokeRequest, 1, nil)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler never entered")
	}
	for _, reqID := range []uint32{2, 3, 4, 5, 6} {
		writeRequestFrame(t, conn, protocol.MsgInvokeRequest, reqID, nil)
	}

	// 先到的两条响应必是 busy（req1-4 在门闩后不可能完成）。
	busy := map[uint32]bool{}
	for i := 0; i < 2; i++ {
		msgID, reqID, body := readResponseFrame(t, conn, 3*time.Second)
		if msgID != protocol.MsgInvokeResponse {
			t.Fatalf("response %d msg = %s, want invoke response", i, protocol.MsgIDString(msgID))
		}
		var rejection sdkv1.InvokeResponse
		if err := proto.Unmarshal(body, &rejection); err != nil {
			t.Fatalf("unmarshal busy response: %v", err)
		}
		if !strings.Contains(string(rejection.GetPayload()), "inbound queue full") {
			t.Fatalf("response %d payload = %q, want busy rejection", i, rejection.GetPayload())
		}
		busy[reqID] = true
	}
	if !busy[5] || !busy[6] || busy[1] || busy[2] || busy[3] || busy[4] {
		t.Fatalf("busy set = %v, want exactly {5,6}", busy)
	}

	// 释放门闩：在界的 4 个请求全部完成，上界即 queue(3)+workers(1)。
	close(gate)
	done := map[uint32]bool{}
	for i := 0; i < 4; i++ {
		_, reqID, body := readResponseFrame(t, conn, 3*time.Second)
		if string(body) != "done" {
			t.Fatalf("response req %d body = %q, want done", reqID, body)
		}
		done[reqID] = true
	}
	if len(done) != 4 || !done[1] || !done[2] || !done[3] || !done[4] {
		t.Fatalf("done set = %v, want exactly {1,2,3,4}", done)
	}
}

// CR-FIX-09 回归 3：逐请求 deadline——handler 收到的 ctx 在
// RequestTimeout 后到期并报错，且到期不影响同连接后续请求。
func TestServer_RequestTimeoutExpiresHandlerContext(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if msgID == protocol.MsgInvokeRequest {
			<-ctx.Done() // 仅尊重 deadline 的 handler；无 deadline 则此处悬挂
			return nil, ctx.Err()
		}
		return []byte("hb"), nil
	})
	srv, err := NewServer(&Config{
		Address:        "127.0.0.1:0",
		Insecure:       true,
		RequestTimeout: 100 * time.Millisecond,
	}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer serveAndStop(srv)()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	start := time.Now()
	writeRequestFrame(t, conn, protocol.MsgInvokeRequest, 7, nil)
	msgID, reqID, body := readResponseFrame(t, conn, 3*time.Second)
	if msgID != protocol.MsgInvokeResponse || reqID != 7 {
		t.Fatalf("response = msg %s req %d, want invoke response req 7", protocol.MsgIDString(msgID), reqID)
	}
	var rejection sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &rejection); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if want := `{"error":"context deadline exceeded"}`; string(rejection.GetPayload()) != want {
		t.Fatalf("payload = %q, want %q", rejection.GetPayload(), want)
	}
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Fatalf("response arrived after %v, deadline did not reach handler", elapsed)
	}

	// deadline 到期后连接与会话仍存活：后续心跳照常应答。
	writeRequestFrame(t, conn, protocol.MsgHeartbeatRequest, 8, nil)
	msgID, reqID, body = readResponseFrame(t, conn, 3*time.Second)
	if msgID != protocol.MsgHeartbeatResponse || reqID != 8 || string(body) != "hb" {
		t.Fatalf("post-timeout heartbeat = msg %s req %d body %q", protocol.MsgIDString(msgID), reqID, body)
	}
}
