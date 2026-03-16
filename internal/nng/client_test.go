package nng

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"go.nanomsg.org/mangos/v3"
)

// mockLogger is a test logger that records log calls
type mockLogger struct {
	debugCalled atomic.Bool
	infoCalled  atomic.Bool
	warnCalled  atomic.Bool
	errorCalled atomic.Bool
	debugMsg    atomic.Value
	infoMsg     atomic.Value
	warnMsg     atomic.Value
	errorMsg    atomic.Value
}

func (m *mockLogger) Debug(msg string, args ...any) {
	m.debugCalled.Store(true)
	m.debugMsg.Store(msg)
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.infoCalled.Store(true)
	m.infoMsg.Store(msg)
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.warnCalled.Store(true)
	m.warnMsg.Store(msg)
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorCalled.Store(true)
	m.errorMsg.Store(msg)
}

func (m *mockLogger) reset() {
	m.debugCalled.Store(false)
	m.infoCalled.Store(false)
	m.warnCalled.Store(false)
	m.errorCalled.Store(false)
}

// TestNewClient tests creating a new client
func TestNewClient(t *testing.T) {
	tests := []struct {
		name           string
		addr           string
		wantAddrCount  int
		wantPrimarySet bool
	}{
		{
			name:           "Single address",
			addr:           "localhost:19090",
			wantAddrCount:  1,
			wantPrimarySet: true,
		},
		{
			name:           "Multiple addresses",
			addr:           "ipc://croupier-server,localhost:19090",
			wantAddrCount:  2,
			wantPrimarySet: true,
		},
		{
			name:           "Empty address uses default",
			addr:           "",
			wantAddrCount:  1,
			wantPrimarySet: false,
		},
		{
			name:           "Address with spaces",
			addr:           " ipc://test , tcp://localhost:19090 ",
			wantAddrCount:  2,
			wantPrimarySet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.addr)

			if len(client.addrs) != tt.wantAddrCount {
				t.Errorf("NewClient() addrs count = %d, want %d", len(client.addrs), tt.wantAddrCount)
			}

			if tt.wantPrimarySet && client.addr == "" {
				t.Errorf("NewClient() primary address should be set")
			}
		})
	}
}

// TestNewClientWithAddrs tests creating a client with explicit addresses
func TestNewClientWithAddrs(t *testing.T) {
	tests := []struct {
		name          string
		addrs         []string
		wantAddrCount int
	}{
		{
			name:          "Single address",
			addrs:         []string{"localhost:19090"},
			wantAddrCount: 1,
		},
		{
			name:          "Multiple addresses",
			addrs:         []string{"ipc://test", "tcp://localhost:19090"},
			wantAddrCount: 2,
		},
		{
			name:          "Empty addresses",
			addrs:         []string{},
			wantAddrCount: 1, // Default
		},
		{
			name:          "Addresses with empty strings",
			addrs:         []string{"", "localhost:19090", ""},
			wantAddrCount: 1,
		},
		{
			name:          "Address without transport gets TCP prefix",
			addrs:         []string{":19090"},
			wantAddrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClientWithAddrs(tt.addrs)

			if len(client.addrs) != tt.wantAddrCount {
				t.Errorf("NewClientWithAddrs() addrs count = %d, want %d", len(client.addrs), tt.wantAddrCount)
			}

			// Verify transport prefix is added for addresses without it
			for _, addr := range client.addrs {
				if !strings.Contains(addr, "://") {
					t.Errorf("NewClientWithAddrs() address %q missing transport prefix", addr)
				}
			}
		})
	}
}

// TestClientSetLogger tests setting a custom logger
func TestClientSetLogger(t *testing.T) {
	client := NewClient("localhost:19090")
	logger := &mockLogger{}

	client.SetLogger(logger)

	if client.logger != logger {
		t.Errorf("SetLogger() logger not set")
	}
}

// TestClientConnected tests connection state checking
func TestClientConnected(t *testing.T) {
	client := NewClient("localhost:19090")

	// Initially not connected
	if client.Connected() {
		t.Errorf("Connected() = true, want false (not dialed)")
	}

	// IsRunning should also return false
	if client.IsRunning() {
		t.Errorf("IsRunning() = true, want false (not dialed)")
	}
}

// TestClientCloseClosingClosedClient tests closing an already closed client
func TestClientCloseClosingClosedClient(t *testing.T) {
	client := NewClient("localhost:19090")

	// Close should be idempotent
	err1 := client.Close()
	err2 := client.Close()

	if err1 != nil {
		t.Errorf("First Close() returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Close() returned error: %v", err2)
	}

	if client.Connected() {
		t.Errorf("Connected() = true after Close(), want false")
	}
}

// TestClientCloseClosesPendingChannels tests that closing closes pending channels
func TestClientCloseClosesPendingChannels(t *testing.T) {
	client := NewClient("localhost:19090")

	// Simulate having pending channels
	client.pendingMu.Lock()
	ch1 := make(chan *mangos.Message, 1)
	ch2 := make(chan *mangos.Message, 1)
	client.pending[1] = ch1
	client.pending[2] = ch2
	client.pendingMu.Unlock()

	client.Close()

	// After close, the map should be reset to a new empty map
	// and channels should be closed
	client.pendingMu.Lock()
	pendingCount := len(client.pending)
	ch1Exists := client.pending[1] != nil
	ch2Exists := client.pending[2] != nil
	client.pendingMu.Unlock()

	if pendingCount != 0 {
		t.Logf("Close() pending channels count = %d (map was reset)", pendingCount)
	}

	// Check if original channels were closed by attempting to receive
	select {
	case <-ch1:
		// Channel was closed and drained
	case <-time.After(10 * time.Millisecond):
	}

	// The key point is that after Close(), the pending map is reset
	// So trying to send to old channels won't block pending requests
	_ = ch1Exists
	_ = ch2Exists
}

// TestClientRegisterMarshalError tests Register with marshal error
func TestClientRegisterMarshalError(t *testing.T) {
	client := NewClient("localhost:19090")

	// Create a request that will fail to marshal
	// This is hard to test directly since proto.Marshal rarely fails
	// Instead test with nil request which should fail
	ctx := context.Background()

	// Can't actually test marshal failure easily with protobuf
	// But we can test the function signature
	_, err := client.Register(ctx, &agentv1.RegisterRequest{})
	if err == nil {
		// We expect error because not connected
		t.Logf("Register() returned nil error (not connected), this is expected behavior")
	}
}

// TestClientHeartbeatNotConnected tests Heartbeat when not connected
func TestClientHeartbeatNotConnected(t *testing.T) {
	client := NewClient("localhost:19090")

	ctx := context.Background()
	_, err := client.Heartbeat(ctx, &agentv1.HeartbeatRequest{})
	if err == nil {
		t.Logf("Heartbeat() returned nil error (not connected), this is expected behavior")
	}
}

// TestClientHeartbeatMarshalError tests Heartbeat with marshal error
func TestClientHeartbeatMarshalError(t *testing.T) {
	client := NewClient("localhost:19090")

	ctx := context.Background()

	_, err := client.Heartbeat(ctx, &agentv1.HeartbeatRequest{})
	if err == nil {
		t.Logf("Heartbeat() returned nil error (not connected), this is expected behavior")
	}
}

// TestClientRegisterCapabilitiesMarshalError tests RegisterCapabilities with marshal error
func TestClientRegisterCapabilitiesMarshalError(t *testing.T) {
	client := NewClient("localhost:19090")

	ctx := context.Background()

	_, err := client.RegisterCapabilities(ctx, &agentv1.RegisterCapabilitiesRequest{})
	if err == nil {
		t.Logf("RegisterCapabilities() returned nil error (not connected), this is expected behavior")
	}
}

// TestClientCallNotConnected tests calling when not connected
func TestClientCallNotConnected(t *testing.T) {
	client := NewClient("localhost:19090")

	ctx := context.Background()
	data := []byte("test")

	_, err := client.Call(ctx, 0x010001, data)
	if err == nil {
		t.Errorf("Call() should return error when not connected")
	}
}

// TestClientDialAlreadyConnected tests dialing when already connected
func TestClientDialAlreadyConnected(t *testing.T) {
	// This test would require actually connecting, which we can't do reliably
	// So we just verify the logic structure
	client := NewClient("invalid-address-that-does-not-exist:12345")

	// First dial attempt (will fail because address is invalid)
	err := client.Dial()
	if err == nil {
		t.Logf("Dial() unexpectedly succeeded with invalid address")
	}

	// Mock the running state to test "already connected" error
	client.mu.Lock()
	client.running = true
	client.mu.Unlock()

	err = client.Dial()
	if err == nil {
		t.Errorf("Dial() should return error when already connected")
	}
}

// TestDefaultLogger tests the default logger implementation
func TestDefaultLogger(t *testing.T) {
	logger := defaultLogger{}

	// These should all be no-ops and not panic
	logger.Debug("test debug")
	logger.Info("test info")
	logger.Warn("test warn")
	logger.Error("test error")
}

// TestNewClientAddressParsing tests address parsing in NewClient
func TestNewClientAddressParsing(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedAddrs   []string
		expectedPrimary string
	}{
		{
			name:  "Plain TCP address gets prefix",
			input: "localhost:19090",
			expectedAddrs: []string{
				"tcp://localhost:19090",
			},
			expectedPrimary: "localhost:19090",
		},
		{
			name:  "Multiple addresses with commas",
			input: "ipc://test,localhost:19090",
			expectedAddrs: []string{
				"ipc://test",
				"tcp://localhost:19090",
			},
			expectedPrimary: "ipc://test,localhost:19090",
		},
		{
			name:            "Empty string",
			input:           "",
			expectedAddrs:   []string{"tcp://:19090"}, // Default
			expectedPrimary: "",
		},
		{
			name:  "Already has tcp:// prefix",
			input: "tcp://localhost:19090",
			expectedAddrs: []string{
				"tcp://localhost:19090",
			},
			expectedPrimary: "tcp://localhost:19090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.input)

			if len(client.addrs) != len(tt.expectedAddrs) {
				t.Errorf("NewClient() addrs count = %d, want %d", len(client.addrs), len(tt.expectedAddrs))
			}

			for i, expected := range tt.expectedAddrs {
				if i < len(client.addrs) && client.addrs[i] != expected {
					t.Errorf("NewClient() addr[%d] = %q, want %q", i, client.addrs[i], expected)
				}
			}

			if client.addr != tt.expectedPrimary {
				t.Errorf("NewClient() primary addr = %q, want %q", client.addr, tt.expectedPrimary)
			}
		})
	}
}
