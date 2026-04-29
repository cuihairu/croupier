// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// TestNewTCPClient_Success tests successful TCP client creation
func TestNewTCPClient_Success(t *testing.T) {
	t.Parallel()

	// Start a mock server
	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}
	defer client.Close()

	if client.IsClosed() {
		t.Error("Client should not be closed after creation")
	}
}

// TestNewTCPClient_InvalidAddress tests TCP client creation with invalid address
func TestNewTCPClient_InvalidAddress(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		address     string
		expectError bool
	}{
		{
			name:        "invalid host:port",
			address:     "invalid:address:format",
			expectError: true,
		},
		{
			name:        "invalid port",
			address:     "localhost:abc",
			expectError: true,
		},
		{
			name:        "empty address",
			address:     "",
			expectError: true,
		},
		{
			name:        "non-existent host",
			address:     "nonexistent.example.com:19090",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{
				Address:     tc.address,
				Insecure:    true,
				DialTimeout: 1 * time.Second,
			}

			client, err := NewTCPClient(config)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if client != nil {
					client.Close()
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

// TestNewTCPClient_WithTimeout tests TCP client creation with timeout
func TestNewTCPClient_WithTimeout(t *testing.T) {
	t.Parallel()

	// Use an unreachable IP to trigger timeout
	config := &Config{
		Address:     "192.0.2.1:19090", // TEST-NET-1, guaranteed unreachable
		Insecure:    true,
		DialTimeout: 100 * time.Millisecond,
	}

	start := time.Now()
	_, err := NewTCPClient(config)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error for unreachable address")
	}

	// Should timeout within DialTimeout + some margin
	if elapsed < config.DialTimeout {
		t.Errorf("Connection returned too quickly: %v < %v", elapsed, config.DialTimeout)
	}
	if elapsed > config.DialTimeout+500*time.Millisecond {
		t.Errorf("Connection took too long to timeout: %v > %v", elapsed, config.DialTimeout+500*time.Millisecond)
	}
}

// TestTCPClient_Call_Success tests successful RPC call
func TestTCPClient_Call_Success(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte("test request")

	respMsgID, respBody, err := client.Call(ctx, msgID, reqBody)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if respMsgID != 0x010201 {
		t.Errorf("Expected response msg ID 0x010201, got 0x%06X", respMsgID)
	}

	if string(respBody) != string(reqBody) {
		t.Errorf("Expected response body %q, got %q", string(reqBody), string(respBody))
	}
}

// TestTCPClient_Call_EmptyBody tests call with empty body
func TestTCPClient_Call_EmptyBody(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte{}

	respMsgID, respBody, err := client.Call(ctx, msgID, reqBody)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if respMsgID != 0x010201 {
		t.Errorf("Expected response msg ID 0x010201, got 0x%06X", respMsgID)
	}

	if len(respBody) != 0 {
		t.Errorf("Expected empty response body, got %q", string(respBody))
	}
}

// TestTCPClient_Call_ContextCancellation tests call with context cancellation
func TestTCPClient_Call_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := startSlowMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	msgID := uint32(0x010101)
	reqBody := []byte("test request")

	_, _, err = client.Call(ctx, msgID, reqBody)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestTCPClient_Call_ContextTimeout tests call with context timeout
func TestTCPClient_Call_ContextTimeout(t *testing.T) {
	t.Parallel()

	server := startSlowMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	msgID := uint32(0x010101)
	reqBody := []byte("test request")

	_, _, err = client.Call(ctx, msgID, reqBody)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
}

// TestTCPClient_Call_AfterClose tests call after client is closed
func TestTCPClient_Call_AfterClose(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	client.Close()

	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte("test request")

	_, _, err = client.Call(ctx, msgID, reqBody)
	if err == nil {
		t.Error("Expected error when calling on closed client")
	}
}

// TestTCPClient_ConcurrentCalls tests multiple concurrent calls
func TestTCPClient_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	numCalls := 10
	var wg sync.WaitGroup
	errChan := make(chan error, numCalls)
	successes := make(chan struct{}, numCalls)

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			ctx := context.Background()
			msgID := uint32(0x010101)
			reqBody := []byte(string(rune('0' + index)))

			_, respBody, err := client.Call(ctx, msgID, reqBody)
			if err != nil {
				errChan <- err
				return
			}

			if string(respBody) != string(reqBody) {
				errChan <- fmt.Errorf("response mismatch")
				return
			}

			select {
			case successes <- struct{}{}:
			default:
			}
		}(i)
	}

	wg.Wait()
	close(errChan)
	close(successes)

	successCount := len(successes)
	errorCount := len(errChan)

	if successCount < numCalls-2 {
		t.Errorf("Too many failures: %d successes out of %d calls", successCount, numCalls)
	}

	t.Logf("Concurrent calls: %d successes, %d errors", successCount, errorCount)
}

// TestTCPClient_Close_Idempotent tests that Close is idempotent
func TestTCPClient_Close_Idempotent(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close multiple times - should not panic
	err1 := client.Close()
	err2 := client.Close()
	err3 := client.Close()

	if err1 != nil {
		t.Errorf("First Close failed: %v", err1)
	}
	// Subsequent closes should return nil or not error
	if err2 != nil && err2 != io.EOF {
		t.Errorf("Second Close returned error: %v", err2)
	}
	if err3 != nil && err3 != io.EOF {
		t.Errorf("Third Close returned error: %v", err3)
	}

	if !client.IsClosed() {
		t.Error("Client should be closed after Close()")
	}
}

// TestTCPClient_Close_CancelsPendingCalls tests that pending calls are cancelled on close
func TestTCPClient_Close_CancelsPendingCalls(t *testing.T) {
	t.Parallel()

	server := startSlowMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start a call that will block
	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte("test request")

	errChan := make(chan error, 1)
	go func() {
		_, _, err := client.Call(ctx, msgID, reqBody)
		errChan <- err
	}()

	// Give the call time to start
	time.Sleep(50 * time.Millisecond)

	// Close the client
	client.Close()

	// The call should return an error
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error from call after client close")
		}
	case <-time.After(time.Second):
		t.Error("Call did not return after client close")
	}
}

// TestTCPClient_LargePayload tests sending large payloads
func TestTCPClient_LargePayload(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test with 1MB payload
	largePayload := make([]byte, 1024*1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	ctx := context.Background()
	msgID := uint32(0x010101)

	_, respBody, err := client.Call(ctx, msgID, largePayload)
	if err != nil {
		t.Fatalf("Call with large payload failed: %v", err)
	}

	if len(respBody) != len(largePayload) {
		t.Errorf("Response length mismatch: got %d, want %d", len(respBody), len(largePayload))
	}
}

// TestTCPClient_OversizedPayload tests error handling for oversized payload
func TestTCPClient_OversizedPayload(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create a payload that exceeds maxFrameBytes
	oversizedPayload := make([]byte, 33*1024*1024) // 33 MB

	ctx := context.Background()
	msgID := uint32(0x010101)

	_, _, err = client.Call(ctx, msgID, oversizedPayload)
	if err == nil {
		t.Error("Expected error for oversized payload")
	}
}

// TestTCPClient_RequestIDIncrement tests that request IDs increment
func TestTCPClient_RequestIDIncrement(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte("test")

	// Make multiple calls
	for i := 0; i < 5; i++ {
		_, _, err := client.Call(ctx, msgID, reqBody)
		if err != nil {
			t.Errorf("Call %d failed: %v", i, err)
		}
	}

	// Server should have received all requests with incrementing IDs
	server.mu.RLock()
	requestCount := server.requestCount
	server.mu.RUnlock()

	if requestCount < 5 {
		t.Errorf("Expected at least 5 requests, got %d", requestCount)
	}
}

// TestTCPClient_NetworkError tests handling of network errors
func TestTCPClient_NetworkError(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte("test request")

	// Close the server to simulate network error
	server.Close()

	// Call should fail
	_, _, err = client.Call(ctx, msgID, reqBody)
	if err == nil {
		t.Error("Expected error when server is closed")
	}

	// Client should still be closable without panic
	client.Close()
}

// TestTCPClient_MultipleMessageTypes tests sending different message types
func TestTCPClient_MultipleMessageTypes(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	reqBody := []byte("test")

	// Test different message types
	messageTypes := []struct {
		msgID uint32
		name  string
	}{
		{protocol.MsgRegisterRequest, "Register"},
		{protocol.MsgInvokeRequest, "Invoke"},
		{protocol.MsgHeartbeatRequest, "Heartbeat"},
	}

	for _, mt := range messageTypes {
		t.Run(mt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := client.Call(ctx, mt.msgID, reqBody)
			if err != nil {
				t.Errorf("%s request failed: %v", mt.name, err)
			}
		})
	}
}

// TestTCPClient_ProtocolVersion tests protocol version in frame
func TestTCPClient_ProtocolVersion(t *testing.T) {
	t.Parallel()

	server := startValidatingMockServer(t, true) // validate protocol version
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	msgID := uint32(0x010101)
	reqBody := []byte("test")

	_, _, err = client.Call(ctx, msgID, reqBody)
	if err != nil {
		t.Errorf("Call failed: %v", err)
	}

	if !server.versionValid {
		t.Error("Protocol version was not valid")
	}
}

// TestTCPClient_FrameParsing tests frame parsing edge cases
func TestTCPClient_FrameParsing(t *testing.T) {
	t.Parallel()

	t.Run("zero frame size", func(t *testing.T) {
		t.Parallel()

		server := startMockServerWithFrameSize(t, 0)
		defer server.Close()

		config := &Config{
			Address:     server.Addr(),
			Insecure:    true,
			DialTimeout: 5 * time.Second,
		}

		client, err := NewTCPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		msgID := uint32(0x010101)
		reqBody := []byte("test")

		// Should handle zero frame size gracefully
		client.Call(ctx, msgID, reqBody)
	})

	t.Run("minimum frame size", func(t *testing.T) {
		t.Parallel()

		// Minimum frame is protocol header (8 bytes)
		server := startMockServerWithFrameSize(t, 8)
		defer server.Close()

		config := &Config{
			Address:     server.Addr(),
			Insecure:    true,
			DialTimeout: 5 * time.Second,
		}

		client, err := NewTCPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		msgID := uint32(0x010101)
		reqBody := []byte("test")

		client.Call(ctx, msgID, reqBody)
	})
}

// TestParseHostPort tests host:port parsing
func TestParseHostPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		addr        string
		expectHost  string
		expectPort  int
		expectError bool
	}{
		{
			name:        "standard host:port",
			addr:        "localhost:19090",
			expectHost:  "localhost",
			expectPort:  19090,
			expectError: false,
		},
		{
			name:        "IPv4 address",
			addr:        "127.0.0.1:19090",
			expectHost:  "127.0.0.1",
			expectPort:  19090,
			expectError: false,
		},
		{
			name:        "IPv6 with brackets",
			addr:        "[::1]:19090",
			expectHost:  "::1",
			expectPort:  19090,
			expectError: false,
		},
		{
			name:        "IPv6 without brackets",
			addr:        "::1:19090",
			expectHost:  "::",
			expectPort:  19090,
			expectError: false,
		},
		{
			name:        "invalid port",
			addr:        "localhost:abc",
			expectHost:  "",
			expectPort:  0,
			expectError: true,
		},
		{
			name:        "missing port",
			addr:        "localhost",
			expectHost:  "localhost",
			expectPort:  19090, // default
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, port, err := parseHostPort(tc.addr)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if host != tc.expectHost {
					t.Errorf("Expected host %q, got %q", tc.expectHost, host)
				}
				if port != tc.expectPort {
					t.Errorf("Expected port %d, got %d", tc.expectPort, port)
				}
			}
		})
	}
}

// TestBuildDialAddrs tests building dial addresses
func TestBuildDialAddrs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      *Config
		expectAddrs []string
	}{
		{
			name: "single address",
			config: &Config{
				Address:  "localhost:19090",
				Insecure: true,
			},
			expectAddrs: []string{"tcp://localhost:19090"},
		},
		{
			name: "multiple addresses comma-separated",
			config: &Config{
				Address:  "primary:19090,backup:19090",
				Insecure: true,
			},
			expectAddrs: []string{"tcp://primary:19090", "tcp://backup:19090"},
		},
		{
			name: "IPC address with TCP fallback",
			config: &Config{
				IPCAddress: "ipc://croupier-agent",
				Address:    "localhost:19090",
				Insecure:   true,
			},
			expectAddrs: []string{"ipc://croupier-agent", "tcp://localhost:19090"},
		},
		{
			name: "explicit addresses list",
			config: &Config{
				Addresses: []string{"addr1:19090", "addr2:19090"},
				Insecure:  true,
			},
			expectAddrs: []string{"tcp://addr1:19090", "tcp://addr2:19090"},
		},
		{
			name: "empty config uses default",
			config: &Config{
				Insecure: true,
			},
			expectAddrs: []string{"tcp://127.0.0.1:19090"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			addrs := buildDialAddrs(tc.config)

			if len(addrs) != len(tc.expectAddrs) {
				t.Errorf("Expected %d addresses, got %d", len(tc.expectAddrs), len(addrs))
			}

			for i, expect := range tc.expectAddrs {
				if i < len(addrs) && addrs[i] != expect {
					t.Errorf("Address %d: expected %q, got %q", i, expect, addrs[i])
				}
			}
		})
	}
}

// TestNormalizeAddress tests address normalization
func TestNormalizeAddress(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		addr     string
		insecure bool
		expected string
	}{
		{
			name:     "already has tcp://",
			addr:     "tcp://localhost:19090",
			insecure: true,
			expected: "tcp://localhost:19090",
		},
		{
			name:     "add tcp:// for insecure",
			addr:     "localhost:19090",
			insecure: true,
			expected: "tcp://localhost:19090",
		},
		{
			name:     "add tls+tcp:// for secure",
			addr:     "localhost:19090",
			insecure: false,
			expected: "tls+tcp://localhost:19090",
		},
		{
			name:     "preserve ipc://",
			addr:     "ipc://croupier-agent",
			insecure: true,
			expected: "ipc://croupier-agent",
		},
		{
			name:     "preserve inproc://",
			addr:     "inproc://server",
			insecure: true,
			expected: "inproc://server",
		},
		{
			name:     "preserve ws://",
			addr:     "ws://localhost:19090",
			insecure: true,
			expected: "ws://localhost:19090",
		},
		{
			name:     "preserve wss://",
			addr:     "wss://localhost:19090",
			insecure: false,
			expected: "wss://localhost:19090",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := normalizeAddress(tc.addr, tc.insecure)

			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestIsIPCSupported tests IPC support detection
func TestIsIPCSupported(t *testing.T) {
	t.Parallel()

	// All major platforms should support IPC
	supported := isIPCSupported()

	if !supported {
		t.Error("IPC should be supported on this platform")
	}
}

// TestDefaultConfig tests default configuration
func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()

	if config.Address == "" {
		t.Error("Address should not be empty")
	}
	if config.Host == "" {
		t.Error("Host should not be empty")
	}
	if config.Port == 0 {
		t.Error("Port should not be 0")
	}
	if config.DialTimeout == 0 {
		t.Error("DialTimeout should not be 0")
	}
	if config.RecvTimeout == 0 {
		t.Error("RecvTimeout should not be 0")
	}
	if config.SendTimeout == 0 {
		t.Error("SendTimeout should not be 0")
	}
	if config.ReadQLen == 0 {
		t.Error("ReadQLen should not be 0")
	}
	if config.WriteQLen == 0 {
		t.Error("WriteQLen should not be 0")
	}
}

// TestTCPClient_Reconnect tests client reconnection
func TestTCPClient_Reconnect(t *testing.T) {
	t.Parallel()

	server1 := startMockServer(t)
	addr1 := server1.Addr()

	config := &Config{
		Address:     addr1,
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Make a successful call
	ctx := context.Background()
	_, _, err = client.Call(ctx, 0x010101, []byte("test"))
	if err != nil {
		t.Errorf("First call failed: %v", err)
	}

	// Close the client
	client.Close()

	// Start a new server on the same port (simulate restart)
	server1.Close()
	time.Sleep(100 * time.Millisecond)

	server2 := startMockServerWithAddr(t, addr1)
	defer server2.Close()

	// Create a new client and connect
	client2, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to reconnect: %v", err)
	}
	defer client2.Close()

	// Make a call on the new connection
	_, _, err = client2.Call(ctx, 0x010101, []byte("test"))
	if err != nil {
		t.Errorf("Call after reconnect failed: %v", err)
	}
}

// TestTCPClient_ConnectionPool tests connection pooling behavior
func TestTCPClient_ConnectionPool(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	// Create multiple clients
	clients := make([]*TCPClient, 3)
	for i := range clients {
		var err error
		clients[i], err = NewTCPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
		defer clients[i].Close()
	}

	ctx := context.Background()

	// All clients should be able to make calls
	for i, client := range clients {
		_, _, err := client.Call(ctx, 0x010101, []byte("test"))
		if err != nil {
			t.Errorf("Client %d call failed: %v", i, err)
		}
	}
}

// TestTCPClient_ReceiveLoopTermination tests receive loop termination
func TestTCPClient_ReceiveLoopTermination(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close the server
	server.Close()

	// Wait a bit for the client to detect the closed connection
	time.Sleep(200 * time.Millisecond)

	// Client should still close cleanly
	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !client.IsClosed() {
		t.Error("Client should be closed")
	}
}

// TestTCPClient_ResponseRouting tests response routing to correct request
func TestTCPClient_ResponseRouting(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Send multiple requests with different bodies
	requests := []string{"req1", "req2", "req3"}
	responses := make([]string, len(requests))

	for i, req := range requests {
		msgID := uint32(0x010101)
		reqBody := []byte(req)

		_, respBody, err := client.Call(ctx, msgID, reqBody)
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
			continue
		}

		responses[i] = string(respBody)
	}

	// Verify each response matches its request
	for i, req := range requests {
		if responses[i] != req {
			t.Errorf("Response %d mismatch: got %q, want %q", i, responses[i], req)
		}
	}
}

// TestTCPClient_PendingRequestCleanup tests cleanup of pending requests
func TestTCPClient_PendingRequestCleanup(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Make a call to add a pending request
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgID := uint32(0x010101)
	reqBody := []byte("test")

	// Start a call in background
	errChan := make(chan error, 1)
	go func() {
		_, _, err := client.Call(ctx, msgID, reqBody)
		errChan <- err
	}()

	// Wait a bit for the request to be sent
	time.Sleep(50 * time.Millisecond)

	// Close the client - should clean up pending requests
	client.Close()

	// The call should return an error
	select {
	case <-errChan:
		// Expected
	case <-time.After(time.Second):
		t.Error("Call did not return after client close")
	}
}

// TestTCPClient_ConcurrentClose tests concurrent close operations
func TestTCPClient_ConcurrentClose(t *testing.T) {
	t.Parallel()

	server := startMockServer(t)
	defer server.Close()

	config := &Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close from multiple goroutines
	var wg sync.WaitGroup
	numClosers := 5

	for i := 0; i < numClosers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Close()
		}()
	}

	wg.Wait()

	if !client.IsClosed() {
		t.Error("Client should be closed")
	}
}

// Mock server implementation

type mockServer struct {
	listener    net.Listener
	addr        string
	mu          sync.RWMutex
	requestCount int
	versionValid bool
	closing     chan struct{}
	closeOnce   sync.Once
	closed      bool
}

func startMockServer(t *testing.T) *mockServer {
	t.Helper()
	return startMockServerWithAddr(t, "127.0.0.1:0")
}

func startMockServerWithAddr(t *testing.T, addr string) *mockServer {
	t.Helper()

	// Use IPv4 loopback if no address specified to avoid IPv6 issues
	if addr == "" {
		addr = "127.0.0.1:0"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}

	server := &mockServer{
		listener: listener,
		addr:     listener.Addr().String(),
		closing:  make(chan struct{}),
	}

	go server.run()

	return server
}

func startMockServerWithFrameSize(t *testing.T, frameSize int) *mockServer {
	t.Helper()
	return startMockServerWithAddr(t, "")
}

func startSlowMockServer(t *testing.T) *mockServer {
	t.Helper()
	return startMockServerWithAddr(t, "")
}

func startValidatingMockServer(t *testing.T, validateVersion bool) *mockServer {
	t.Helper()
	server := startMockServerWithAddr(t, "")
	server.versionValid = false
	return server
}

func (s *mockServer) Addr() string {
	return s.addr
}

func (s *mockServer) Close() error {
	s.closeOnce.Do(func() {
		close(s.closing)
		s.closed = true
	})
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *mockServer) run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *mockServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	frameHeader := make([]byte, 4)

	for {
		select {
		case <-s.closing:
			return
		default:
		}

		// Read frame header
		_, err := io.ReadFull(conn, frameHeader)
		if err != nil {
			return
		}

		// Parse frame size
		frameSize := binary.BigEndian.Uint32(frameHeader)
		if frameSize == 0 || frameSize > maxFrameBytes {
			continue
		}

		// Read frame payload
		payload := make([]byte, frameSize)
		_, err = io.ReadFull(conn, payload)
		if err != nil {
			return
		}

		// Parse protocol header
		if len(payload) < 8 {
			continue
		}

		version := payload[0]
		if version != protocol.Version1 {
			continue
		}

		reqID := binary.BigEndian.Uint32(payload[4:8])
		body := payload[8:]

		s.mu.Lock()
		s.requestCount++
		s.mu.Unlock()

		// Send response
		responseFrame := make([]byte, 4+8+len(body))
		binary.BigEndian.PutUint32(responseFrame[0:4], uint32(8+len(body)))
		responseFrame[4] = protocol.Version1
		protocol.PutMsgID(responseFrame[5:8], 0x010201) // Response msg ID
		binary.BigEndian.PutUint32(responseFrame[8:12], reqID)
		copy(responseFrame[12:], body)

		conn.Write(responseFrame)
	}
}

func (s *mockServer) RequestCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.requestCount
}
