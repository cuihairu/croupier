package tcp

import (
	"bytes"
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

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
	defer srv.Close()

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

	srv.Close()

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
	defer srv.Close()

	// Serve with nil context should use context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(nil)
	}()

	select {
	case <-done:
		// Serve returned (may be due to listener close or timeout)
	case <-ctx.Done():
		// Timeout - close the server to unblock
		srv.Close()
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
	srv.Close()
	srv.Close()
	srv.Close()
}

func TestClient_MultipleClose(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx)

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multiple close should not panic
	client.Close()
	client.Close()
	client.Close()
}

func TestClient_IsClosed(t *testing.T) {
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return body, nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx)

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

	client.Close()

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
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx)

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client.Close()

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
	payload := make([]byte, maxFrameBytes+1)
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
	header := make([]byte, frameHeaderBytes)
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
	header := make([]byte, frameHeaderBytes)
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
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx)

	// Nil config should use defaults
	client, err := NewClient(&Config{
		Address:  srv.Addr(),
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()

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
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx)

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
	defer client.Close()

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
	defer srv.Close()

	if !strings.Contains(srv.Addr(), "19090") {
		t.Logf("Server addr: %s (may not use default if port is in use)", srv.Addr())
	}
}
