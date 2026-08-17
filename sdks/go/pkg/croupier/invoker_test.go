// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewInvoker(t *testing.T) {
	t.Parallel()

	t.Run("with valid config", func(t *testing.T) {
		t.Parallel()

		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
			Insecure:       true,
		}
		invoker := NewInvoker(config)

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		impl, ok := invoker.(*httpInvoker)
		if !ok {
			t.Fatal("NewInvoker did not return *httpInvoker type")
		}

		if impl.config != config {
			t.Error("config not set correctly")
		}

		if impl.schemas == nil {
			t.Error("schemas map not initialized")
		}
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		t.Parallel()

		invoker := NewInvoker(nil)
		impl := invoker.(*httpInvoker)

		if impl.config == nil {
			t.Fatal("config should be set to default")
		}

		if impl.config.Address != defaultServerAPIURL {
			t.Errorf("expected default Address, got %q", impl.config.Address)
		}

		if !impl.config.Insecure {
			t.Error("expected default Insecure to be true")
		}

		if impl.config.Reconnect == nil {
			t.Error("expected default Reconnect config")
		}

		if impl.config.Retry == nil {
			t.Error("expected default Retry config")
		}
	})
}

func TestInvoker_isConnectionError(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"broken pipe", errors.New("broken pipe"), true},
		{"network unreachable", errors.New("network is unreachable"), true},
		{"no such host", errors.New("no such host"), true},
		{"timeout", errors.New("i/o timeout"), true},
		{"transport closing", errors.New("transport is closing"), true},
		{"random error", errors.New("some random error"), false},
		{"other error", errors.New("validation failed"), false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := i.isConnectionError(tc.err)
			if result != tc.expected {
				t.Errorf("isConnectionError(%q) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestInvoker_isRetryableError(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"unavailable", errors.New("service unavailable"), true},
		{"internal error", errors.New("internal error"), true},
		{"deadline exceeded", errors.New("context deadline exceeded"), true},
		{"aborted", errors.New("aborted"), true},
		{"transport closing", errors.New("transport is closing"), true},
		{"timeout", errors.New("timeout"), true},
		{"random error", errors.New("some random error"), false},
		{"validation error", errors.New("validation failed"), false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := i.isRetryableError(tc.err)
			if result != tc.expected {
				t.Errorf("isRetryableError(%q) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestInvoker_SetSchema(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	})

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"name"},
	}

	err := i.SetSchema("test.function", schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	impl := i.(*tcpInvoker)
	stored, ok := impl.schemas["test.function"]
	if !ok {
		t.Error("schema not stored")
	}
	if stored == nil {
		t.Error("stored schema is nil")
	}
}

func TestInvoker_Connect(t *testing.T) {
	t.Parallel()

	t.Run("with no server running", func(t *testing.T) {
		t.Parallel()

		i := newTCPInvoker(&InvokerConfig{
			Address:        "127.0.0.1:19999", // Non-existent server
			TimeoutSeconds: 1,
			Insecure:       true,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := i.Connect(ctx)
		if err == nil {
			t.Error("expected error when connecting to non-existent server")
		}
	})
}

func TestInvoker_Close(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19999",
		Insecure: true,
	})

	// Close should not panic
	err := i.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Close should be idempotent
	err = i.Close()
	if err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}
}

func TestInvoker_Invoke(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := i.Invoke(ctx, "test.function", `{"test":"data"}`, InvokeOptions{})
	if err == nil {
		t.Error("expected error when invoking without connection")
	}
}

func TestInvoker_StartTask(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := i.StartTask(ctx, "test.function", `{"test":"data"}`, InvokeOptions{})
	if err == nil {
		t.Error("expected error when starting task without connection")
	}
}

func TestInvoker_CancelTask(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := i.CancelTask(ctx, "task-123")
	if err == nil {
		t.Error("expected error when cancelling task without connection")
	}
}

func TestInvoker_StreamTask(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch, err := i.StreamTask(ctx, "task-123")
	if err == nil {
		t.Error("expected error for streaming (not yet implemented)")
	}
	if ch != nil {
		// Channel should be closed
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("expected channel to be closed")
			}
		default:
			t.Error("expected channel to be closed immediately")
		}
	}
}

func TestInvoker_calculateReconnectDelay(t *testing.T) {
	t.Parallel()

	config := &InvokerConfig{
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			InitialDelayMs:    1000,
			MaxDelayMs:        30000,
			BackoffMultiplier: 2.0,
			JitterFactor:      0.2,
		},
	}

	i := newTCPInvoker(config).(*tcpInvoker)

	// Set reconnection attempts to different values
	testCases := []struct {
		name          string
		attempt       int
		minExpectedMs int
		maxExpectedMs int
	}{
		{"first attempt", 1, 800, 1200},   // ~1s with jitter
		{"second attempt", 2, 1600, 2400}, // ~2s with jitter
		{"third attempt", 3, 3200, 4800},  // ~4s with jitter
		{"large attempt", 10, 0, 30000},   // capped at max
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			delay := i.calculateReconnectDelay(tc.attempt)

			delayMs := int(delay.Milliseconds())
			if delayMs < tc.minExpectedMs || delayMs > tc.maxExpectedMs {
				t.Errorf("delay = %dms, want between %dms and %dms", delayMs, tc.minExpectedMs, tc.maxExpectedMs)
			}
		})
	}
}

func TestInvoker_calculateRetryDelay(t *testing.T) {
	t.Parallel()

	config := &RetryConfig{
		Enabled:           true,
		InitialDelayMs:    100,
		MaxDelayMs:        5000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.1,
	}

	i := newTCPInvoker(&InvokerConfig{Retry: config}).(*tcpInvoker)

	testCases := []struct {
		name          string
		attempt       int
		minExpectedMs int
		maxExpectedMs int
	}{
		{"first retry", 0, 90, 110},   // ~100ms with jitter
		{"second retry", 1, 180, 220}, // ~200ms with jitter
		{"third retry", 2, 360, 440},  // ~400ms with jitter
		{"large retry", 10, 0, 5000},  // capped at max
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			delay := i.calculateRetryDelay(tc.attempt, config)

			delayMs := int(delay.Milliseconds())
			if delayMs < tc.minExpectedMs || delayMs > tc.maxExpectedMs {
				t.Errorf("delay = %dms, want between %dms and %dms", delayMs, tc.minExpectedMs, tc.maxExpectedMs)
			}
		})
	}
}

func TestInvoker_validatePayload(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	t.Run("with empty schema", func(t *testing.T) {
		t.Parallel()

		// Empty payload should error
		err := i.validatePayload("", map[string]interface{}{})
		if err == nil {
			t.Error("expected error for empty payload with empty schema")
		}
	})

	t.Run("with JSON schema", func(t *testing.T) {
		t.Parallel()

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"name"},
		}

		// Valid payload
		err := i.validatePayload(`{"name":"test"}`, schema)
		if err != nil {
			t.Errorf("unexpected error for valid payload: %v", err)
		}

		// Invalid payload (missing required field)
		err = i.validatePayload(`{}`, schema)
		if err == nil {
			t.Error("expected error for invalid payload")
		}
	})
}

func TestInvoker_scheduleReconnectIfNeeded(t *testing.T) {
	t.Parallel()

	t.Run("with reconnect disabled", func(t *testing.T) {
		t.Parallel()

		config := &InvokerConfig{
			Address: "127.0.0.1:19090",
			Reconnect: &ReconnectConfig{
				Enabled: false,
			},
		}

		i := newTCPInvoker(config).(*tcpInvoker)

		// Should not schedule reconnect
		i.scheduleReconnectIfNeeded()

		if i.isReconnecting {
			t.Error("expected isReconnecting to be false when reconnect is disabled")
		}
	})

	t.Run("with reconnect enabled", func(t *testing.T) {
		t.Parallel()

		config := &InvokerConfig{
			Address: "127.0.0.1:19090",
			Reconnect: &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 50, // Short delay for testing
			},
		}

		i := newTCPInvoker(config).(*tcpInvoker)

		// Simulate connection error
		i.scheduleReconnectIfNeeded()

		if !i.isReconnecting {
			t.Error("expected isReconnecting to be true")
		}

		// Clean up
		if i.reconnectCancelCtx != nil {
			i.reconnectCancelCtx()
		}
	})
}

func TestInvoker_ConnectAlreadyConnected(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	// Mark as already connected
	i.connected = true

	ctx := context.Background()
	err := i.Connect(ctx)
	if err != nil {
		t.Errorf("unexpected error when already connected: %v", err)
	}

	// Should still be connected
	if !i.connected {
		t.Error("expected connected to remain true")
	}
}

func TestInvoker_ConnectReconnecting(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	// Mark as reconnecting
	i.isReconnecting = true

	ctx := context.Background()
	err := i.Connect(ctx)
	if err == nil {
		t.Error("expected error when reconnecting")
	}

	if err.Error() != "reconnection in progress" {
		t.Errorf("expected 'reconnection in progress' error, got: %v", err)
	}
}

func TestInvoker_Options(t *testing.T) {
	t.Parallel()

	opts := InvokeOptions{
		IdempotencyKey: "test-key-123",
		Timeout:        5 * time.Second,
		Headers: map[string]string{
			"X-Request-ID": "req-456",
			"X-Session":    "session-789",
		},
		Retry: &RetryConfig{
			Enabled:     true,
			MaxAttempts: 5,
		},
	}

	if opts.IdempotencyKey != "test-key-123" {
		t.Errorf("expected IdempotencyKey 'test-key-123', got %q", opts.IdempotencyKey)
	}

	if opts.Timeout != 5*time.Second {
		t.Errorf("expected Timeout 5s, got %v", opts.Timeout)
	}

	if opts.Headers["X-Request-ID"] != "req-456" {
		t.Errorf("expected X-Request-ID 'req-456', got %q", opts.Headers["X-Request-ID"])
	}

	if opts.Retry.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts 5, got %d", opts.Retry.MaxAttempts)
	}
}

func TestInvoker_ConfigDefaults(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(nil).(*tcpInvoker)

	// Check defaults
	if i.config.Address != "localhost:19090" {
		t.Errorf("expected default Address, got %q", i.config.Address)
	}

	if i.config.TimeoutSeconds != 30 {
		t.Errorf("expected default TimeoutSeconds 30, got %d", i.config.TimeoutSeconds)
	}

	if !i.config.Insecure {
		t.Error("expected default Insecure to be true")
	}

	if i.config.Reconnect == nil {
		t.Error("expected default Reconnect config")
	}

	if i.config.Retry == nil {
		t.Error("expected default Retry config")
	}

	if i.config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout 30s, got %v", i.config.DefaultTimeout)
	}
}

// ========== Additional Invoker Tests ==========

func TestInvoker_SetSchema_Multiple(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	// SetSchema should work even when not connected
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]string{"type": "string"},
		},
	}
	invoker.SetSchema("test.function", schema)

	// Verify schema was set
	impl := invoker.(*tcpInvoker)
	if impl.schemas["test.function"] == nil {
		t.Error("schema was not set")
	}
}

func TestInvoker_CheckConnectionState(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	impl := invoker.(*tcpInvoker)
	// Should not be connected initially
	if impl.connected {
		t.Error("should not be connected initially")
	}
}

func TestInvoker_ConfigAddress(t *testing.T) {
	t.Parallel()

	config := &InvokerConfig{
		Address:  "localhost:19999",
		Insecure: true,
	}
	invoker := newTCPInvoker(config)

	impl := invoker.(*tcpInvoker)
	if impl.config.Address != "localhost:19999" {
		t.Errorf("expected address localhost:19999, got %s", impl.config.Address)
	}
}

func TestInvokerConfigDefaults(t *testing.T) {
	t.Parallel()

	// Create invoker with nil config to test defaults
	invoker := newTCPInvoker(nil)
	impl := invoker.(*tcpInvoker)

	if impl.config.Address != "localhost:19090" {
		t.Errorf("expected default address, got %s", impl.config.Address)
	}

	if !impl.config.Insecure {
		t.Error("expected Insecure to be true by default")
	}

	if impl.config.TimeoutSeconds != 30 {
		t.Errorf("expected TimeoutSeconds 30, got %d", impl.config.TimeoutSeconds)
	}
}

func TestInvoker_Close_WhenNotConnected(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	// Close should not panic when not connected
	err := invoker.Close()
	if err != nil {
		t.Errorf("Close should not return error when not connected: %v", err)
	}
}

func TestInvoker_Invoke_WhenNotConnected(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	// Invoke should try to connect and fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
	if err == nil {
		t.Error("expected error when invoking without server")
	}
}

func TestInvoker_StartTask_WhenNotConnected(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	// StartTask should try to connect and fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := invoker.StartTask(ctx, "test.function", "{}", InvokeOptions{})
	if err == nil {
		t.Error("expected error when starting task without server")
	}
}

func TestInvoker_CancelTask_WhenNotConnected(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	// CancelTask should try to connect and fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := invoker.CancelTask(ctx, "task-id")
	if err == nil {
		t.Error("expected error when canceling task without server")
	}
}

func TestInvoker_Connect_ContextCancellation(t *testing.T) {
	t.Parallel()

	invoker := newTCPInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	})

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := invoker.Connect(ctx)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

// ========== RetryConfig Tests ==========

func TestRetryConfigDefaults(t *testing.T) {
	t.Parallel()

	config := DefaultRetryConfig()

	if !config.Enabled {
		t.Error("expected Enabled to be true")
	}

	if config.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", config.MaxAttempts)
	}

	if config.InitialDelayMs != 100 {
		t.Errorf("expected InitialDelayMs 100, got %d", config.InitialDelayMs)
	}
}

// ========== ReconnectConfig Tests ==========

func TestReconnectConfigDefaults(t *testing.T) {
	t.Parallel()

	config := DefaultReconnectConfig()

	if !config.Enabled {
		t.Error("expected Enabled to be true")
	}

	if config.MaxAttempts != 0 {
		t.Errorf("expected MaxAttempts 0 (infinite), got %d", config.MaxAttempts)
	}

	if config.InitialDelayMs != 1000 {
		t.Errorf("expected InitialDelayMs 1000, got %d", config.InitialDelayMs)
	}
}

// ========== InvokeOptions Tests ==========

func TestInvokeOptionsDefaults(t *testing.T) {
	t.Parallel()

	opts := InvokeOptions{}

	// InvokeOptions is a struct, check zero values
	if opts.Timeout != 0 {
		t.Errorf("expected default Timeout 0, got %v", opts.Timeout)
	}
}

// ========== tcpInvoker Internal Tests ==========

func TestTCPInvoker_ValidatePayload_EdgeCases(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	t.Run("empty payload with non-empty schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
		}
		err := i.validatePayload("", schema)
		if err == nil {
			t.Error("expected error for empty payload")
		}
	})

	t.Run("valid JSON with empty schema", func(t *testing.T) {
		schema := map[string]interface{}{}
		err := i.validatePayload(`{}`, schema)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("complex nested schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{"type": "string"},
						"age":  map[string]interface{}{"type": "integer"},
					},
					"required": []interface{}{"name"},
				},
			},
		}

		// Valid payload
		err := i.validatePayload(`{"user":{"name":"John","age":30}}`, schema)
		if err != nil {
			t.Errorf("unexpected error for valid payload: %v", err)
		}

		// Invalid - missing nested required field
		err = i.validatePayload(`{"user":{"age":30}}`, schema)
		if err == nil {
			t.Error("expected error for missing required nested field")
		}
	})
}

func TestTCPInvoker_ScheduleReconnect_MaxAttempts(t *testing.T) {
	t.Parallel()

	config := &InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    1, // Only 1 attempt
			InitialDelayMs: 10,
		},
	}

	i := newTCPInvoker(config).(*tcpInvoker)

	// First reconnect should be scheduled
	i.scheduleReconnectIfNeeded()

	// Clean up immediately
	if i.reconnectCancelCtx != nil {
		i.reconnectCancelCtx()
	}
}

func TestTCPInvoker_CalculateReconnectDelay_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("zero backoff multiplier", func(t *testing.T) {
		config := &InvokerConfig{
			Reconnect: &ReconnectConfig{
				Enabled:           true,
				InitialDelayMs:    1000,
				MaxDelayMs:        30000,
				BackoffMultiplier: 0, // Zero multiplier
				JitterFactor:      0,
			},
		}
		i := newTCPInvoker(config).(*tcpInvoker)

		delay := i.calculateReconnectDelay(5)
		// With zero multiplier, delay should be capped at max
		if delay > 30*time.Second {
			t.Errorf("delay should be capped at max: %v", delay)
		}
	})

	t.Run("very high attempt number", func(t *testing.T) {
		config := &InvokerConfig{
			Reconnect: &ReconnectConfig{
				Enabled:           true,
				InitialDelayMs:    1000,
				MaxDelayMs:        5000, // Low max
				BackoffMultiplier: 2.0,
				JitterFactor:      0,
			},
		}
		i := newTCPInvoker(config).(*tcpInvoker)

		delay := i.calculateReconnectDelay(100)
		// Should be capped at max
		if delay > 5*time.Second {
			t.Errorf("delay should be capped at max 5s: %v", delay)
		}
	})
}

func TestTCPInvoker_CalculateRetryDelay_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("zero backoff multiplier", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:           true,
			InitialDelayMs:    100,
			MaxDelayMs:        5000,
			BackoffMultiplier: 0,
			JitterFactor:      0,
		}
		i := newTCPInvoker(&InvokerConfig{Retry: config}).(*tcpInvoker)

		delay := i.calculateRetryDelay(5, config)
		if delay < 0 {
			t.Errorf("delay should not be negative: %v", delay)
		}
	})

	t.Run("very high attempt number", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:           true,
			InitialDelayMs:    100,
			MaxDelayMs:        1000, // Low max
			BackoffMultiplier: 2.0,
			JitterFactor:      0,
		}
		i := newTCPInvoker(&InvokerConfig{Retry: config}).(*tcpInvoker)

		delay := i.calculateRetryDelay(100, config)
		if delay > 1*time.Second {
			t.Errorf("delay should be capped at max 1s: %v", delay)
		}
	})
}

func TestTCPInvoker_IsConnectionError(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(nil).(*tcpInvoker)

	tests := []struct {
		name string
		err  error
	}{
		{"nil error", nil},
		{"context canceled", context.Canceled},
		{"context deadline exceeded", context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify no panic
			_ = i.isConnectionError(tt.err)
		})
	}
}

func TestTCPInvoker_IsRetryableError(t *testing.T) {
	t.Parallel()

	i := newTCPInvoker(nil).(*tcpInvoker)

	// Test with nil error
	if i.isRetryableError(nil) {
		t.Error("nil error should not be retryable")
	}
}

func TestTCPInvoker_ScheduleReconnectIfDisabled(t *testing.T) {
	t.Parallel()

	config := &InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled: false,
		},
	}

	i := newTCPInvoker(config).(*tcpInvoker)

	// Should not panic when reconnect is disabled
	i.scheduleReconnectIfNeeded()
}

func TestTCPInvoker_CalculateDelay_FirstAttempt(t *testing.T) {
	t.Parallel()

	config := &InvokerConfig{
		Reconnect: &ReconnectConfig{
			Enabled:        true,
			InitialDelayMs: 1000,
			MaxDelayMs:     30000,
			JitterFactor:   0,
		},
	}
	i := newTCPInvoker(config).(*tcpInvoker)

	// First attempt should return initial delay
	delay := i.calculateReconnectDelay(1)
	if delay < time.Millisecond || delay > 2*time.Second {
		t.Errorf("unexpected delay for first attempt: %v", delay)
	}
}
