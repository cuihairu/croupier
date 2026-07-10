package croupier

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewInvoker_NilConfig(t *testing.T) {
	invoker := NewInvoker(nil)
	if invoker == nil {
		t.Fatal("NewInvoker(nil) returned nil")
	}

	impl := invoker.(*tcpInvoker)
	if impl.config.Address != "localhost:19090" {
		t.Errorf("expected default address localhost:19090, got %s", impl.config.Address)
	}
	if !impl.config.Insecure {
		t.Error("expected default Insecure=true")
	}
	if impl.config.TimeoutSeconds != 30 {
		t.Errorf("expected default TimeoutSeconds=30, got %d", impl.config.TimeoutSeconds)
	}
	if impl.config.Reconnect == nil {
		t.Error("expected default Reconnect config")
	}
	if impl.config.Retry == nil {
		t.Error("expected default Retry config")
	}
	if impl.config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout=30s, got %v", impl.config.DefaultTimeout)
	}
}

func TestNewInvoker_WithConfig(t *testing.T) {
	config := &InvokerConfig{
		Address:        "10.0.0.1:19090",
		TimeoutSeconds: 60,
		Insecure:       false,
		Reconnect:      DefaultReconnectConfig(),
		Retry:          DefaultRetryConfig(),
	}
	invoker := NewInvoker(config)
	impl := invoker.(*tcpInvoker)

	if impl.config != config {
		t.Error("config not set correctly")
	}
	if impl.schemas == nil {
		t.Error("schemas map not initialized")
	}
}

func TestNewInvoker_PartialConfig(t *testing.T) {
	config := &InvokerConfig{
		Address:        "10.0.0.1:19090",
		TimeoutSeconds: 10,
	}
	invoker := NewInvoker(config)
	impl := invoker.(*tcpInvoker)

	if impl.config.Reconnect == nil {
		t.Error("expected Reconnect to be set to default")
	}
	if impl.config.Retry == nil {
		t.Error("expected Retry to be set to default")
	}
	if impl.config.DefaultTimeout != 10*time.Second {
		t.Errorf("expected DefaultTimeout=10s, got %v", impl.config.DefaultTimeout)
	}
}

func TestInvoker_SetSchema(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{Address: "localhost:19090", Insecure: true})

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"name"},
	}

	err := invoker.SetSchema("test.function", schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	impl := invoker.(*tcpInvoker)
	if impl.schemas["test.function"] == nil {
		t.Error("schema not stored")
	}
}

func TestInvoker_SetSchema_Multiple(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{Address: "localhost:19090", Insecure: true})

	for i := 0; i < 5; i++ {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field": map[string]interface{}{"type": "string"},
			},
		}
		err := invoker.SetSchema("func."+string(rune('0'+i)), schema)
		if err != nil {
			t.Fatalf("unexpected error for func %d: %v", i, err)
		}
	}

	impl := invoker.(*tcpInvoker)
	if len(impl.schemas) != 5 {
		t.Errorf("expected 5 schemas, got %d", len(impl.schemas))
	}
}

func TestInvoker_isConnectionError(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)

	tests := []struct {
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
		{"connection unavailable", errors.New("connection unavailable"), true},
		{"random error", errors.New("some random error"), false},
		{"validation error", errors.New("validation failed"), false},
		{"case insensitive", errors.New("Connection Refused"), true},
		{"case insensitive timeout", errors.New("TIMEOUT occurred"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := i.isConnectionError(tc.err)
			if result != tc.expected {
				t.Errorf("isConnectionError(%q) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestInvoker_isRetryableError(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)

	tests := []struct {
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
		{"case insensitive", errors.New("UNAVAILABLE"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := i.isRetryableError(tc.err)
			if result != tc.expected {
				t.Errorf("isRetryableError(%q) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestInvoker_calculateReconnectDelay(t *testing.T) {
	config := &InvokerConfig{
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			InitialDelayMs:    1000,
			MaxDelayMs:        30000,
			BackoffMultiplier: 2.0,
			JitterFactor:      0.2,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)

	tests := []struct {
		name          string
		attempt       int
		minExpectedMs int
		maxExpectedMs int
	}{
		{"first attempt", 1, 800, 1200},
		{"second attempt", 2, 1600, 2400},
		{"third attempt", 3, 3200, 4800},
		{"large attempt", 10, 0, 30000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delay := i.calculateReconnectDelay(tc.attempt)
			delayMs := int(delay.Milliseconds())
			if delayMs < tc.minExpectedMs || delayMs > tc.maxExpectedMs {
				t.Errorf("delay = %dms, want between %dms and %dms", delayMs, tc.minExpectedMs, tc.maxExpectedMs)
			}
		})
	}
}

func TestInvoker_calculateReconnectDelay_ZeroBackoff(t *testing.T) {
	config := &InvokerConfig{
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			InitialDelayMs:    1000,
			MaxDelayMs:        5000,
			BackoffMultiplier: 0,
			JitterFactor:      0,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)

	delay := i.calculateReconnectDelay(5)
	if delay > 5*time.Second {
		t.Errorf("delay should be capped at max: %v", delay)
	}
}

func TestInvoker_calculateReconnectDelay_HighAttempt(t *testing.T) {
	config := &InvokerConfig{
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			InitialDelayMs:    1000,
			MaxDelayMs:        5000,
			BackoffMultiplier: 2.0,
			JitterFactor:      0,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)

	delay := i.calculateReconnectDelay(100)
	if delay > 5*time.Second {
		t.Errorf("delay should be capped at max 5s: %v", delay)
	}
}

func TestInvoker_calculateRetryDelay(t *testing.T) {
	config := &RetryConfig{
		Enabled:           true,
		InitialDelayMs:    100,
		MaxDelayMs:        5000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.1,
	}
	i := NewInvoker(&InvokerConfig{Retry: config}).(*tcpInvoker)

	tests := []struct {
		name          string
		attempt       int
		minExpectedMs int
		maxExpectedMs int
	}{
		{"first retry", 0, 90, 110},
		{"second retry", 1, 180, 220},
		{"third retry", 2, 360, 440},
		{"large retry", 10, 0, 5000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delay := i.calculateRetryDelay(tc.attempt, config)
			delayMs := int(delay.Milliseconds())
			if delayMs < tc.minExpectedMs || delayMs > tc.maxExpectedMs {
				t.Errorf("delay = %dms, want between %dms and %dms", delayMs, tc.minExpectedMs, tc.maxExpectedMs)
			}
		})
	}
}

func TestInvoker_calculateRetryDelay_ZeroBackoff(t *testing.T) {
	config := &RetryConfig{
		Enabled:           true,
		InitialDelayMs:    100,
		MaxDelayMs:        5000,
		BackoffMultiplier: 0,
		JitterFactor:      0,
	}
	i := NewInvoker(&InvokerConfig{Retry: config}).(*tcpInvoker)

	delay := i.calculateRetryDelay(5, config)
	if delay < 0 {
		t.Errorf("delay should not be negative: %v", delay)
	}
}

func TestInvoker_calculateRetryDelay_HighAttempt(t *testing.T) {
	config := &RetryConfig{
		Enabled:           true,
		InitialDelayMs:    100,
		MaxDelayMs:        1000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	i := NewInvoker(&InvokerConfig{Retry: config}).(*tcpInvoker)

	delay := i.calculateRetryDelay(100, config)
	if delay > 1*time.Second {
		t.Errorf("delay should be capped at max 1s: %v", delay)
	}
}

func TestInvoker_validatePayload_EmptySchema(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)

	// Empty payload with empty schema should error
	err := i.validatePayload("", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty payload with empty schema")
	}

	// Non-empty payload with empty schema should pass
	err = i.validatePayload(`{"key":"value"}`, map[string]interface{}{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvoker_validatePayload_JSONSchema(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)

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
}

func TestInvoker_validatePayload_NestedSchema(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)

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

	// Valid nested payload
	err := i.validatePayload(`{"user":{"name":"John","age":30}}`, schema)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid nested payload
	err = i.validatePayload(`{"user":{"age":30}}`, schema)
	if err == nil {
		t.Error("expected error for missing required nested field")
	}
}

func TestInvoker_scheduleReconnect_Disabled(t *testing.T) {
	config := &InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled: false,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)

	i.scheduleReconnectIfNeeded()

	if i.isReconnecting {
		t.Error("expected isReconnecting=false when reconnect is disabled")
	}
}

func TestInvoker_scheduleReconnect_AlreadyReconnecting(t *testing.T) {
	config := &InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 50,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)
	i.isReconnecting = true

	i.scheduleReconnectIfNeeded()

	// Should still be reconnecting (no change)
	if !i.isReconnecting {
		t.Error("expected isReconnecting to remain true")
	}
}

func TestInvoker_scheduleReconnect_MaxAttemptsReached(t *testing.T) {
	config := &InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    2,
			InitialDelayMs: 50,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)
	i.reconnectAttempts = 2 // Already at max

	i.scheduleReconnectIfNeeded()

	if i.isReconnecting {
		t.Error("should not schedule reconnect when max attempts reached")
	}
}

func TestInvoker_scheduleReconnect_Enabled(t *testing.T) {
	config := &InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 50,
		},
	}
	i := NewInvoker(config).(*tcpInvoker)

	i.scheduleReconnectIfNeeded()

	if !i.isReconnecting {
		t.Error("expected isReconnecting=true")
	}
	if i.reconnectAttempts != 1 {
		t.Errorf("expected reconnectAttempts=1, got %d", i.reconnectAttempts)
	}

	// Clean up
	if i.reconnectCancelCtx != nil {
		i.reconnectCancelCtx()
	}
}

func TestInvoker_Connect_AlreadyConnected(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)
	i.connected = true

	ctx := context.Background()
	err := i.Connect(ctx)
	if err != nil {
		t.Errorf("unexpected error when already connected: %v", err)
	}
	if !i.connected {
		t.Error("expected connected to remain true")
	}
}

func TestInvoker_Connect_Reconnecting(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)
	i.isReconnecting = true

	ctx := context.Background()
	err := i.Connect(ctx)
	if err == nil {
		t.Error("expected error when reconnecting")
	}
	if err.Error() != "reconnection in progress" {
		t.Errorf("expected 'reconnection in progress', got: %v", err)
	}
}

func TestInvoker_Connect_FailedConnection(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          &RetryConfig{Enabled: false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := i.Connect(ctx)
	if err == nil {
		t.Error("expected error when connecting to non-existent server")
	}
}

func TestInvoker_Close(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true})

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

func TestInvoker_Close_WithReconnectContext(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address: "127.0.0.1:19090",
		Reconnect: &ReconnectConfig{
			Enabled:        true,
			InitialDelayMs: 100,
		},
	}).(*tcpInvoker)

	// Start a reconnection
	i.scheduleReconnectIfNeeded()

	err := i.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if i.connected {
		t.Error("expected connected=false after close")
	}
	if i.isReconnecting {
		t.Error("expected isReconnecting=false after close")
	}
}

func TestInvoker_buildTLSConfig_Insecure(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		Insecure: true,
	}).(*tcpInvoker)

	// buildTLSConfig is only called when !Insecure, so we test the secure path
	// For insecure, we just verify the invoker was created correctly
	if !i.config.Insecure {
		t.Error("expected Insecure=true")
	}
}

func TestInvoker_buildTLSConfig_WithCAFile(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address: "localhost:19090",
		CAFile:  "/nonexistent/ca.crt",
	}).(*tcpInvoker)

	_, err := i.buildTLSConfig()
	if err == nil {
		t.Error("expected error for non-existent CA file")
	}
}

func TestInvoker_buildTLSConfig_WithCertFiles(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:  "localhost:19090",
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}).(*tcpInvoker)

	_, err := i.buildTLSConfig()
	if err == nil {
		t.Error("expected error for non-existent cert files")
	}
}

func TestInvoker_buildTLSConfig_NoFiles(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address: "localhost:19090",
	}).(*tcpInvoker)

	cfg, err := i.buildTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MinVersion != 0x0303 { // TLS 1.2
		t.Errorf("expected TLS 1.2 min version, got %x", cfg.MinVersion)
	}
}

func TestInvoker_buildTLSConfig_AddressParsing(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"host:port", "example.com:8080"},
		{"ip:port", "192.168.1.1:8080"},
		{"ipv6", "[::1]:8080"},
		{"no port", "example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := NewInvoker(&InvokerConfig{
				Address: tc.address,
			}).(*tcpInvoker)

			cfg, err := i.buildTLSConfig()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.ServerName == "" {
				t.Error("expected ServerName to be set")
			}
		})
	}
}

func TestInvoker_executeWithRetry_Disabled(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Retry: &RetryConfig{Enabled: false},
	}).(*tcpInvoker)

	called := 0
	result, err := i.executeWithRetry(context.Background(), InvokeOptions{}, func() (string, error) {
		called++
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if called != 1 {
		t.Errorf("expected function called once, got %d", called)
	}
}

func TestInvoker_executeWithRetry_Success(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 10,
		},
	}).(*tcpInvoker)

	called := 0
	result, err := i.executeWithRetry(context.Background(), InvokeOptions{}, func() (string, error) {
		called++
		return "success", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
	if called != 1 {
		t.Errorf("expected function called once, got %d", called)
	}
}

func TestInvoker_executeWithRetry_RetryThenSuccess(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 10,
			JitterFactor:   0,
		},
		Reconnect: &ReconnectConfig{Enabled: false},
	}).(*tcpInvoker)

	called := 0
	result, err := i.executeWithRetry(context.Background(), InvokeOptions{}, func() (string, error) {
		called++
		if called < 3 {
			return "", errors.New("service unavailable")
		}
		return "recovered", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "recovered" {
		t.Errorf("expected 'recovered', got %q", result)
	}
	if called != 3 {
		t.Errorf("expected function called 3 times, got %d", called)
	}
}

func TestInvoker_executeWithRetry_NonRetryableError(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 10,
		},
		Reconnect: &ReconnectConfig{Enabled: false},
	}).(*tcpInvoker)

	called := 0
	_, err := i.executeWithRetry(context.Background(), InvokeOptions{}, func() (string, error) {
		called++
		return "", errors.New("validation failed")
	})

	if err == nil {
		t.Error("expected error")
	}
	if called != 1 {
		t.Errorf("expected function called once (non-retryable), got %d", called)
	}
}

func TestInvoker_executeWithRetry_ContextCancelled(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    5,
			InitialDelayMs: 1000,
		},
		Reconnect: &ReconnectConfig{Enabled: false},
	}).(*tcpInvoker)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := i.executeWithRetry(ctx, InvokeOptions{}, func() (string, error) {
		return "", errors.New("unavailable")
	})

	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestInvoker_executeWithRetry_OptionsRetry(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    5,
			InitialDelayMs: 100,
		},
		Reconnect: &ReconnectConfig{Enabled: false},
	}).(*tcpInvoker)

	// Options retry should override config retry
	opts := InvokeOptions{
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    2,
			InitialDelayMs: 10,
		},
	}

	called := 0
	_, err := i.executeWithRetry(context.Background(), opts, func() (string, error) {
		called++
		return "", errors.New("unavailable")
	})

	if err == nil {
		t.Error("expected error")
	}
	// Should only retry once (2 attempts total) because options.MaxAttempts=2
	if called != 2 {
		t.Errorf("expected 2 attempts, got %d", called)
	}
}

func TestInvoker_Invoke_NotConnected(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          &RetryConfig{Enabled: false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := i.Invoke(ctx, "test.function", `{"test":"data"}`, InvokeOptions{})
	if err == nil {
		t.Error("expected error when invoking without connection")
	}
}

func TestInvoker_StartTask_NotConnected(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          &RetryConfig{Enabled: false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := i.StartTask(ctx, "test.function", `{"test":"data"}`, InvokeOptions{})
	if err == nil {
		t.Error("expected error when starting task without connection")
	}
}

func TestInvoker_CancelTask_NotConnected(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          &RetryConfig{Enabled: false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := i.CancelTask(ctx, "task-123")
	if err == nil {
		t.Error("expected error when cancelling task without connection")
	}
}

func TestInvoker_StreamTask_NotConnected(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          &RetryConfig{Enabled: false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := i.StreamTask(ctx, "task-123")
	if err == nil {
		t.Error("expected error for streaming without connection")
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

func TestInvoker_connect_AlreadyConnected(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)
	i.connected = true

	ctx := context.Background()
	err := i.connect(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvoker_connect_Reconnecting(t *testing.T) {
	i := NewInvoker(&InvokerConfig{Address: "127.0.0.1:19090", Insecure: true}).(*tcpInvoker)
	i.isReconnecting = true

	ctx := context.Background()
	err := i.connect(ctx)
	if err == nil {
		t.Error("expected error")
	}
	if err.Error() != "reconnection in progress" {
		t.Errorf("expected 'reconnection in progress', got: %v", err)
	}
}

func TestInvoker_connectLocked_Failure(t *testing.T) {
	i := NewInvoker(&InvokerConfig{
		Address:        "127.0.0.1:19999",
		TimeoutSeconds: 1,
		Insecure:       true,
	}).(*tcpInvoker)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := i.connectLocked(ctx)
	if err == nil {
		t.Error("expected error when connecting to non-existent server")
	}
}

func TestInvoker_DefaultRetryConfig_Values(t *testing.T) {
	cfg := DefaultRetryConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelayMs != 100 {
		t.Errorf("expected InitialDelayMs=100, got %d", cfg.InitialDelayMs)
	}
	if cfg.MaxDelayMs != 5000 {
		t.Errorf("expected MaxDelayMs=5000, got %d", cfg.MaxDelayMs)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier=2.0, got %f", cfg.BackoffMultiplier)
	}
}

func TestInvoker_DefaultReconnectConfig_Values(t *testing.T) {
	cfg := DefaultReconnectConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if cfg.MaxAttempts != 0 {
		t.Errorf("expected MaxAttempts=0, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelayMs != 1000 {
		t.Errorf("expected InitialDelayMs=1000, got %d", cfg.InitialDelayMs)
	}
	if cfg.MaxDelayMs != 30000 {
		t.Errorf("expected MaxDelayMs=30000, got %d", cfg.MaxDelayMs)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier=2.0, got %f", cfg.BackoffMultiplier)
	}
}

func TestInvokeOptions_Fields(t *testing.T) {
	opts := InvokeOptions{
		IdempotencyKey: "key-123",
		Timeout:        5 * time.Second,
		Headers: map[string]string{
			"X-Request-ID": "req-456",
		},
		Retry: &RetryConfig{Enabled: true, MaxAttempts: 5},
	}

	if opts.IdempotencyKey != "key-123" {
		t.Errorf("expected IdempotencyKey='key-123', got %q", opts.IdempotencyKey)
	}
	if opts.Timeout != 5*time.Second {
		t.Errorf("expected Timeout=5s, got %v", opts.Timeout)
	}
	if opts.Headers["X-Request-ID"] != "req-456" {
		t.Errorf("expected X-Request-ID='req-456', got %q", opts.Headers["X-Request-ID"])
	}
	if opts.Retry.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", opts.Retry.MaxAttempts)
	}
}
