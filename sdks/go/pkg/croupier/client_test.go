package croupier

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("with valid config", func(t *testing.T) {
		t.Parallel()

		config := &ClientConfig{
			ServiceID:      "test-service",
			ServiceVersion: "1.0.0",
			AgentAddr:      "localhost:19090",
		}
		cli := NewClient(config)

		if cli == nil {
			t.Fatal("NewClient returned nil")
		}

		c, ok := cli.(*client)
		if !ok {
			t.Fatal("NewClient did not return *client type")
		}

		if c.config != config {
			t.Error("config not set correctly")
		}

		if c.handlers == nil {
			t.Error("handlers map not initialized")
		}

		if c.descriptors == nil {
			t.Error("descriptors map not initialized")
		}

		if c.stopCh == nil {
			t.Error("stopCh not initialized")
		}
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		t.Parallel()

		cli := NewClient(nil)
		c, ok := cli.(*client)
		if !ok {
			t.Fatal("NewClient did not return *client type")
		}

		if c.config == nil {
			t.Fatal("config should be set to default")
		}

		if c.config.AgentAddr != "localhost:19090" {
			t.Errorf("expected default AgentAddr, got %q", c.config.AgentAddr)
		}

		if !c.config.Insecure {
			t.Error("expected default Insecure to be true")
		}
	})

	t.Run("with logging disabled", func(t *testing.T) {
		t.Parallel()

		config := &ClientConfig{
			ServiceID:      "test-service",
			DisableLogging: true,
		}
		cli := NewClient(config)
		c, ok := cli.(*client)
		if !ok {
			t.Fatal("NewClient did not return *client type")
		}

		if _, ok := c.logger.(*NoOpLogger); !ok {
			t.Error("expected NoOpLogger when DisableLogging is true")
		}
	})
}

func TestClient_RegisterFunction(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
	}
	cli := NewClient(config)
	c := cli.(*client)

	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}

	t.Run("successful registration", func(t *testing.T) {
		t.Parallel()

		newCli := NewClient(config)
		desc := FunctionDescriptor{
			ID:      "test.function",
			Version: "1.0.0",
		}

		err := newCli.RegisterFunction(desc, handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		nc := newCli.(*client)
		if _, ok := nc.handlers["test.function"]; !ok {
			t.Error("handler not registered")
		}

		if _, ok := nc.descriptors["test.function"]; !ok {
			t.Error("descriptor not registered")
		}
	})

	t.Run("empty function ID returns error", func(t *testing.T) {
		t.Parallel()

		newCli := NewClient(config)
		desc := FunctionDescriptor{
			ID: "",
		}

		err := newCli.RegisterFunction(desc, handler)
		if err == nil {
			t.Error("expected error for empty function ID")
		}

		if err != nil && err.Error() != "function ID cannot be empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("default version is set", func(t *testing.T) {
		t.Parallel()

		newCli := NewClient(config)
		desc := FunctionDescriptor{
			ID:      "test.version",
			Version: "",
		}

		err := newCli.RegisterFunction(desc, handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		nc := newCli.(*client)
		if nc.descriptors["test.version"].Version != "1.0.0" {
			t.Errorf("expected default version 1.0.0, got %q", nc.descriptors["test.version"].Version)
		}
	})

	t.Run("cannot register while running", func(t *testing.T) {
		t.Parallel()

		newCli := NewClient(config)
		nc := newCli.(*client)
		nc.running.Store(true)

		desc := FunctionDescriptor{
			ID:      "test.running",
			Version: "1.0.0",
		}

		err := newCli.RegisterFunction(desc, handler)
		if err == nil {
			t.Error("expected error when registering while running")
		}
	})

	_ = c
}

func TestClient_Stop(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		ServiceID: "test-service",
	}
	cli := NewClient(config)
	c := cli.(*client)

	// Set running state
	c.running.Store(true)
	c.connected.Store(true)

	err := cli.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.running.Load() {
		t.Error("expected running to be false after Stop")
	}

	if c.connected.Load() {
		t.Error("expected connected to be false after Stop")
	}
}

func TestClient_Close(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		ServiceID: "test-service",
	}
	cli := NewClient(config)
	c := cli.(*client)

	c.handlers = map[string]FunctionHandler{"test": nil}

	err := cli.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.handlers != nil {
		t.Error("expected handlers to be nil after Close")
	}
}

func TestClient_convertToLocalFunctions(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		ServiceID: "test-service",
	}
	cli := NewClient(config)
	c := cli.(*client)

	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}

	c.descriptors["f1"] = FunctionDescriptor{ID: "f1", Version: "1.0.0"}
	c.descriptors["f2"] = FunctionDescriptor{ID: "f2", Version: "2.0.0"}
	c.handlers["f1"] = handler
	c.handlers["f2"] = handler

	funcs := c.convertToLocalFunctions()

	if len(funcs) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(funcs))
	}

	funcByID := make(map[string]LocalFunctionDescriptor)
	for _, f := range funcs {
		funcByID[f.ID] = f
	}

	if f := funcByID["f1"]; f.Version != "1.0.0" {
		t.Errorf("expected f1 version 1.0.0, got %q", f.Version)
	}

	if f := funcByID["f2"]; f.Version != "2.0.0" {
		t.Errorf("expected f2 version 2.0.0, got %q", f.Version)
	}
}

// Test DefaultClientConfig
func TestDefaultClientConfig(t *testing.T) {
	t.Parallel()

	config := DefaultClientConfig()

	if config.AgentAddr != "localhost:19090" {
		t.Errorf("expected AgentAddr localhost:19090, got %q", config.AgentAddr)
	}

	if config.Env != "development" {
		t.Errorf("expected Env development, got %q", config.Env)
	}

	if config.ServiceVersion != "1.0.0" {
		t.Errorf("expected ServiceVersion 1.0.0, got %q", config.ServiceVersion)
	}

	if config.TimeoutSeconds != 30 {
		t.Errorf("expected TimeoutSeconds 30, got %d", config.TimeoutSeconds)
	}

	if config.HeartbeatInterval != 60 {
		t.Errorf("expected HeartbeatInterval 60, got %d", config.HeartbeatInterval)
	}

	if !config.Insecure {
		t.Error("expected Insecure to be true")
	}

	if config.Headers == nil {
		t.Error("expected Headers to be initialized")
	}

	if config.ProviderLang != "go" {
		t.Errorf("expected ProviderLang go, got %q", config.ProviderLang)
	}

	if config.ProviderSDK != "croupier-go-sdk" {
		t.Errorf("expected ProviderSDK croupier-go-sdk, got %q", config.ProviderSDK)
	}

	if config.LogLevel != "INFO" {
		t.Errorf("expected LogLevel INFO, got %q", config.LogLevel)
	}

	if config.Reconnect == nil {
		t.Error("expected Reconnect to be initialized")
	}

	if config.Retry == nil {
		t.Error("expected Retry to be initialized")
	}

	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected MaxFileSize 10485760, got %d", config.MaxFileSize)
	}
}

// Test DefaultReconnectConfig
func TestDefaultReconnectConfig(t *testing.T) {
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

	if config.MaxDelayMs != 30000 {
		t.Errorf("expected MaxDelayMs 30000, got %d", config.MaxDelayMs)
	}

	if config.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier 2.0, got %f", config.BackoffMultiplier)
	}

	if config.JitterFactor != 0.2 {
		t.Errorf("expected JitterFactor 0.2, got %f", config.JitterFactor)
	}
}

// Test DefaultRetryConfig
func TestDefaultRetryConfig(t *testing.T) {
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

	if config.MaxDelayMs != 5000 {
		t.Errorf("expected MaxDelayMs 5000, got %d", config.MaxDelayMs)
	}

	expectedCodes := []int32{14, 13, 2, 10, 4}
	if len(config.RetryableStatusCodes) != len(expectedCodes) {
		t.Errorf("expected %d retryable codes, got %d", len(expectedCodes), len(config.RetryableStatusCodes))
	}

	for i, code := range expectedCodes {
		if config.RetryableStatusCodes[i] != code {
			t.Errorf("expected code %d at index %d, got %d", code, i, config.RetryableStatusCodes[i])
		}
	}
}

// Test FunctionDescriptor defaults and validation
func TestFunctionDescriptor(t *testing.T) {
	t.Parallel()

	t.Run("empty version gets default", func(t *testing.T) {
		t.Parallel()

		desc := FunctionDescriptor{
			ID: "test.function",
		}

		if desc.Version != "" {
			t.Error("expected empty version initially")
		}

		// Simulate RegisterFunction's default version logic
		version := desc.Version
		if version == "" {
			version = "1.0.0"
		}

		if version != "1.0.0" {
			t.Errorf("expected default version 1.0.0, got %q", version)
		}
	})

	t.Run("valid descriptor", func(t *testing.T) {
		t.Parallel()

		desc := FunctionDescriptor{
			ID:        "player.ban",
			Version:   "1.2.3",
			Category:  "moderation",
			Risk:      "high",
			Entity:    "player",
			Operation: "update",
			Enabled:   true,
		}

		if desc.ID != "player.ban" {
			t.Errorf("expected ID player.ban, got %q", desc.ID)
		}

		if desc.Risk != "high" {
			t.Errorf("expected Risk high, got %q", desc.Risk)
		}

		if !desc.Enabled {
			t.Error("expected Enabled to be true")
		}
	})
}

// Test InvokerConfig defaults
func TestInvokerConfig(t *testing.T) {
	t.Parallel()

	config := InvokerConfig{
		Address:        "localhost:8080",
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	if config.Address != "localhost:8080" {
		t.Errorf("expected Address localhost:8080, got %q", config.Address)
	}

	if config.TimeoutSeconds != 30 {
		t.Errorf("expected TimeoutSeconds 30, got %d", config.TimeoutSeconds)
	}

	if !config.Insecure {
		t.Error("expected Insecure to be true")
	}
}

// Test TaskEvent structure
func TestTaskEvent(t *testing.T) {
	t.Parallel()

	event := TaskEvent{
		EventType: "completed",
		TaskID:    "task-123",
		Payload:   `{"result":"success"}`,
		Done:      true,
	}

	if event.EventType != "completed" {
		t.Errorf("expected EventType completed, got %q", event.EventType)
	}

	if event.TaskID != "task-123" {
		t.Errorf("expected TaskID task-123, got %q", event.TaskID)
	}

	if !event.Done {
		t.Error("expected Done to be true")
	}
}

// Test InvokeOptions
func TestInvokeOptions(t *testing.T) {
	t.Parallel()

	opts := InvokeOptions{
		IdempotencyKey: "key-123",
		Timeout:        10 * time.Second,
		Headers: map[string]string{
			"X-Custom": "value",
		},
	}

	if opts.IdempotencyKey != "key-123" {
		t.Errorf("expected IdempotencyKey key-123, got %q", opts.IdempotencyKey)
	}

	if opts.Timeout != 10*time.Second {
		t.Errorf("expected Timeout 10s, got %v", opts.Timeout)
	}

	if opts.Headers["X-Custom"] != "value" {
		t.Errorf("expected header value, got %q", opts.Headers["X-Custom"])
	}
}

// Test LocalFunctionDescriptor
func TestLocalFunctionDescriptor(t *testing.T) {
	t.Parallel()

	desc := LocalFunctionDescriptor{
		ID:      "player.get",
		Version: "1.0.0",
	}

	if desc.ID != "player.get" {
		t.Errorf("expected ID player.get, got %q", desc.ID)
	}

	if desc.Version != "1.0.0" {
		t.Errorf("expected Version 1.0.0, got %q", desc.Version)
	}
}

// Test Client_Connect idempotent
func TestClient_ConnectIdempotent(t *testing.T) {
	t.Parallel()

	c := &client{
		config: &ClientConfig{
			AgentAddr: "127.0.0.1:19090",
		},
		handlers:    map[string]FunctionHandler{},
		descriptors: map[string]FunctionDescriptor{},
		stopCh:      make(chan struct{}),
		logger:      &NoOpLogger{},
	}
	c.connected.Store(true) // Already connected

	// Should return nil without trying to connect again
	err := c.Connect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error when already connected: %v", err)
	}
}

// Test Client_Connect with no handlers
func TestClient_ConnectNoHandlers(t *testing.T) {
	t.Parallel()

	c := &client{
		config: &ClientConfig{
			AgentAddr: "127.0.0.1:19090",
		},
		handlers:    map[string]FunctionHandler{},
		descriptors: map[string]FunctionDescriptor{},
		stopCh:      make(chan struct{}),
		logger:      &NoOpLogger{},
	}

	// Should fail to connect (no actual agent)
	err := c.Connect(context.Background())
	if err == nil {
		t.Error("expected connection error (no agent running)")
	}
}

// Test Client_Serve without connect
func TestClient_ServeWithoutConnect(t *testing.T) {
	t.Parallel()

	c := &client{
		config: &ClientConfig{
			AgentAddr: "127.0.0.1:19090",
		},
		handlers:    map[string]FunctionHandler{},
		descriptors: map[string]FunctionDescriptor{},
		stopCh:      make(chan struct{}),
		logger:      &NoOpLogger{},
	}

	// Should try to connect and fail
	err := c.Serve(context.Background())
	if err == nil {
		t.Error("expected error when serving without connection")
	}
}

// Test Client_convertToLocalFunctions with empty descriptors
func TestClient_convertToLocalFunctionsWithEmptyDescriptors(t *testing.T) {
	t.Parallel()

	c := &client{
		descriptors: map[string]FunctionDescriptor{},
		handlers:    map[string]FunctionHandler{},
	}

	funcs := c.convertToLocalFunctions()
	if len(funcs) != 0 {
		t.Errorf("expected 0 functions, got %d", len(funcs))
	}
}

// Test Client_convertToLocalFunctions with missing handlers
func TestClient_convertToLocalFunctionsWithMissingHandlers(t *testing.T) {
	t.Parallel()

	c := &client{
		descriptors: map[string]FunctionDescriptor{
			"f1": {ID: "f1", Version: "1.0.0"},
		},
		handlers: map[string]FunctionHandler{},
	}

	funcs := c.convertToLocalFunctions()
	// Functions without handlers should not be included
	if len(funcs) != 0 {
		t.Errorf("expected 0 functions (no handlers), got %d", len(funcs))
	}
}

// Test Client_convertToLocalFunctions with multiple descriptors
func TestClient_convertToLocalFunctionsWithMultipleDescriptors(t *testing.T) {
	t.Parallel()

	c := &client{
		descriptors: map[string]FunctionDescriptor{
			"func1": {ID: "func1", Version: "1.0.0"},
			"func2": {ID: "func2", Version: "2.0.0"},
			"func3": {ID: "func3", Version: "3.0.0"},
		},
		handlers: map[string]FunctionHandler{
			"func1": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
			"func2": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
			"func3": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
		},
	}

	localFuncs := c.convertToLocalFunctions()
	if len(localFuncs) != 3 {
		t.Errorf("expected 3 functions, got %d", len(localFuncs))
	}

	// Check each function
	funcMap := make(map[string]LocalFunctionDescriptor)
	for _, fn := range localFuncs {
		funcMap[fn.ID] = fn
	}

	if funcMap["func1"].Version != "1.0.0" {
		t.Errorf("expected func1 version 1.0.0, got %s", funcMap["func1"].Version)
	}
	if funcMap["func2"].Version != "2.0.0" {
		t.Errorf("expected func2 version 2.0.0, got %s", funcMap["func2"].Version)
	}
	if funcMap["func3"].Version != "3.0.0" {
		t.Errorf("expected func3 version 3.0.0, got %s", funcMap["func3"].Version)
	}
}

// Test Client_convertToLocalFunctions with empty version
func TestClient_convertToLocalFunctionsWithEmptyVersion(t *testing.T) {
	t.Parallel()

	c := &client{
		descriptors: map[string]FunctionDescriptor{
			"func1": {ID: "func1", Version: ""},       // Empty version
			"func2": {ID: "func2", Version: "custom"}, // Custom version
		},
		handlers: map[string]FunctionHandler{
			"func1": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
			"func2": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
		},
	}

	localFuncs := c.convertToLocalFunctions()
	if len(localFuncs) != 2 {
		t.Errorf("expected 2 functions, got %d", len(localFuncs))
	}

	// Check that empty version gets default "1.0.0"
	funcMap := make(map[string]LocalFunctionDescriptor)
	for _, fn := range localFuncs {
		funcMap[fn.ID] = fn
	}

	if funcMap["func1"].Version != "1.0.0" {
		t.Errorf("expected func1 version 1.0.0 (default), got %s", funcMap["func1"].Version)
	}
	if funcMap["func2"].Version != "custom" {
		t.Errorf("expected func2 version 'custom', got %s", funcMap["func2"].Version)
	}
}

// Test Client_convertToLocalFunctions with handlers but no descriptors
func TestClient_convertToLocalFunctionsNoDescriptors(t *testing.T) {
	t.Parallel()

	c := &client{
		descriptors: map[string]FunctionDescriptor{},
		handlers: map[string]FunctionHandler{
			"func1": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
		},
	}

	localFuncs := c.convertToLocalFunctions()
	if len(localFuncs) != 0 {
		t.Errorf("expected 0 functions (no descriptors), got %d", len(localFuncs))
	}
}

// Test Client_RegisterFunctionWithDuplicate tests registering same function twice
func TestClient_RegisterFunctionWithDuplicate(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		ServiceID: "test-service",
	}
	cli := NewClient(config)
	c := cli.(*client)

	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}

	desc := FunctionDescriptor{
		ID:      "test.function",
		Version: "1.0.0",
	}

	// Register first time
	err := cli.RegisterFunction(desc, handler)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Register second time - should overwrite
	err = cli.RegisterFunction(desc, handler)
	if err != nil {
		t.Errorf("second registration failed: %v", err)
	}

	// Verify only one entry exists
	if len(c.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(c.handlers))
	}
	if len(c.descriptors) != 1 {
		t.Errorf("expected 1 descriptor, got %d", len(c.descriptors))
	}
}

// Test Client_convertToLocalFunctions with mixed handlers
func TestClient_convertToLocalFunctionsWithMixedHandlers(t *testing.T) {
	t.Parallel()

	c := &client{
		descriptors: map[string]FunctionDescriptor{
			"f1": {ID: "f1", Version: "1.0.0"},
			"f2": {ID: "f2", Version: "2.0.0"},
			"f3": {ID: "f3", Version: "3.0.0"},
		},
		handlers: map[string]FunctionHandler{
			"f1": func(ctx context.Context, payload []byte) ([]byte, error) {
				return payload, nil
			},
			// f2 handler missing
			"f3": func(ctx context.Context, payload []byte) ([]byte, error) {
				return payload, nil
			},
		},
	}

	funcs := c.convertToLocalFunctions()

	// Should only include functions with handlers
	if len(funcs) != 2 {
		t.Errorf("expected 2 functions (only those with handlers), got %d", len(funcs))
	}

	funcByID := make(map[string]LocalFunctionDescriptor)
	for _, f := range funcs {
		funcByID[f.ID] = f
	}

	if _, ok := funcByID["f1"]; !ok {
		t.Error("expected f1 in result (has handler)")
	}
	if _, ok := funcByID["f2"]; ok {
		t.Error("expected f2 NOT in result (no handler)")
	}
	if _, ok := funcByID["f3"]; !ok {
		t.Error("expected f3 in result (has handler)")
	}
}

// Test Client_ConnectErrorCases tests Connect error cases
func TestClient_ConnectErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("with invalid agent address", func(t *testing.T) {
		t.Parallel()

		config := &ClientConfig{
			ServiceID:      "test-service",
			AgentAddr:      "invalid-address:abc",
			Insecure:       true,
			TimeoutSeconds: 1,
		}
		cli := NewClient(config)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := cli.Connect(ctx)
		if err == nil {
			t.Error("expected error with invalid address")
		}
	})

	t.Run("connection timeout", func(t *testing.T) {
		t.Parallel()

		config := &ClientConfig{
			ServiceID:      "test-service",
			AgentAddr:      "192.0.2.1:9999", // Non-routable IP
			Insecure:       true,
			TimeoutSeconds: 1,
		}
		cli := NewClient(config)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := cli.Connect(ctx)
		if err == nil {
			t.Error("expected error with timeout")
		}
	})
}

// Test Client_StopNotRunning tests Stop when not running
func TestClient_StopNotRunning(t *testing.T) {
	t.Parallel()

	c := &client{
		config: &ClientConfig{},
		stopCh: make(chan struct{}),
		logger: &NoOpLogger{},
	}
	c.running.Store(false)

	// Should not panic or error
	c.Stop()
}

// Test Client_StopWhileRunning tests Stop while service is running
func TestClient_StopWhileRunning(t *testing.T) {
	t.Parallel()

	c := &client{
		config: &ClientConfig{},
		stopCh: make(chan struct{}),
		logger: &NoOpLogger{},
	}
	c.running.Store(true)

	// Stop should close the stopCh and set running to false
	c.Stop()

	if c.running.Load() {
		t.Error("expected running to be false after Stop")
	}

	select {
	case <-c.stopCh:
		// Channel should be closed
	default:
		t.Error("expected stopCh to be closed")
	}
}

// Test Client_RegisterFunctionWithEmptyID tests registering with empty ID
func TestClient_RegisterFunctionWithEmptyID(t *testing.T) {
	t.Parallel()

	c := &client{
		config:   &ClientConfig{},
		handlers: make(map[string]FunctionHandler),
		logger:   &NoOpLogger{},
	}

	desc := FunctionDescriptor{
		ID: "", // Empty ID
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}

	err := c.RegisterFunction(desc, handler)
	if err == nil {
		t.Error("expected error with empty function ID")
	}
}

// Test Client_RegisterFunctionWithDefaultVersion tests version defaulting
func TestClient_RegisterFunctionWithDefaultVersion(t *testing.T) {
	t.Parallel()

	c := &client{
		config:      &ClientConfig{},
		handlers:    make(map[string]FunctionHandler),
		descriptors: make(map[string]FunctionDescriptor),
		logger:      &NoOpLogger{},
	}

	desc := FunctionDescriptor{
		ID: "test.fn", // No version set
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}

	err := c.RegisterFunction(desc, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that version was defaulted
	storedDesc, ok := c.descriptors["test.fn"]
	if !ok {
		t.Fatal("function descriptor not stored")
	}
	if storedDesc.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", storedDesc.Version)
	}
}

// Test DefaultLogger
func TestDefaultLogger(t *testing.T) {
	t.Parallel()

	t.Run("with debug mode", func(t *testing.T) {
		t.Parallel()

		logger := NewDefaultLogger(true, os.Stdout)

		// Should not panic
		logger.Debugf("debug message")
		logger.Infof("info message")
		logger.Warnf("warn message")
		logger.Errorf("error message")
	})

	t.Run("without debug mode", func(t *testing.T) {
		t.Parallel()

		logger := NewDefaultLogger(false, os.Stdout)

		// Should not panic
		logger.Debugf("debug message")
		logger.Infof("info message")
		logger.Warnf("warn message")
		logger.Errorf("error message")
	})
}

// Test NoOpLogger behavior in client
func TestClientNoOpLogger(t *testing.T) {
	t.Parallel()

	logger := &NoOpLogger{}

	// Should not panic and produce no output
	logger.Debugf("debug message")
	logger.Infof("info message")
	logger.Warnf("warn message")
	logger.Errorf("error message")
}

// Test NoOpLogger in client
func TestClient_NoOpLogger(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		DisableLogging: true,
	})

	c := cli.(*client)
	if _, ok := c.logger.(*NoOpLogger); !ok {
		t.Error("expected NoOpLogger when DisableLogging is true")
	}

	// Logger methods should not panic
	c.logger.Debugf("test debug")
	c.logger.Infof("test info")
	c.logger.Warnf("test warn")
	c.logger.Errorf("test error")
}

// ========== Additional Client Tests ==========

func TestClient_RegisterFunction_Multiple(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
	})

	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}

	// Register multiple functions
	cli.RegisterFunction(FunctionDescriptor{ID: "fn1", Version: "1.0.0"}, handler)
	cli.RegisterFunction(FunctionDescriptor{ID: "fn2", Version: "1.0.0"}, handler)
	cli.RegisterFunction(FunctionDescriptor{ID: "fn3", Version: "1.0.0"}, handler)

	c := cli.(*client)
	if len(c.descriptors) != 3 {
		t.Errorf("expected 3 descriptors, got %d", len(c.descriptors))
	}

	if len(c.handlers) != 3 {
		t.Errorf("expected 3 handlers, got %d", len(c.handlers))
	}
}

func TestClient_RegisterFunction_Replace(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
	})

	callCount := 0
	handler1 := func(ctx context.Context, payload []byte) ([]byte, error) {
		callCount = 1
		return payload, nil
	}
	handler2 := func(ctx context.Context, payload []byte) ([]byte, error) {
		callCount = 2
		return payload, nil
	}

	// Register function, then replace it
	cli.RegisterFunction(FunctionDescriptor{ID: "fn1", Version: "1.0.0"}, handler1)
	cli.RegisterFunction(FunctionDescriptor{ID: "fn1", Version: "2.0.0"}, handler2)

	c := cli.(*client)
	if len(c.descriptors) != 1 {
		t.Errorf("expected 1 descriptor, got %d", len(c.descriptors))
	}

	// Verify the handler is the second one
	c.handlers["fn1"](context.Background(), []byte{})
	if callCount != 2 {
		t.Error("expected second handler to be registered")
	}
}

func TestClient_Stop_WhenNotStarted(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
	})

	// Stop should not panic when not started
	cli.Stop()
}

func TestClient_Close_WhenNotStarted(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
	})

	// Close should not panic when not started
	err := cli.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

func TestFunctionDescriptor_Fields(t *testing.T) {
	t.Parallel()

	desc := FunctionDescriptor{
		ID:        "test.function",
		Version:   "1.0.0",
		Category:  "test",
		Risk:      "low",
		Entity:    "player",
		Operation: "read",
		Enabled:   true,
	}

	if desc.ID != "test.function" {
		t.Errorf("unexpected ID: %s", desc.ID)
	}
	if desc.Version != "1.0.0" {
		t.Errorf("unexpected Version: %s", desc.Version)
	}
	if desc.Category != "test" {
		t.Errorf("unexpected Category: %s", desc.Category)
	}
}

func TestClientConfig_Defaults(t *testing.T) {
	t.Parallel()

	config := DefaultClientConfig()

	if config.ServiceID == "" {
		// ServiceID may be empty by default
	}
	if config.ProviderLang != "go" {
		t.Errorf("expected ProviderLang 'go', got %s", config.ProviderLang)
	}
}

func TestClient_Connect_Timeout(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
		AgentAddr:      "localhost:19999", // Non-existent server
		TimeoutSeconds: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := cli.Connect(ctx)
	if err == nil {
		t.Error("expected error when connecting to non-existent server")
		cli.Stop()
	}
}

func TestClient_Serve_WithoutConnect(t *testing.T) {
	t.Parallel()

	cli := NewClient(&ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
	})

	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}
	cli.RegisterFunction(FunctionDescriptor{ID: "fn1", Version: "1.0.0"}, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Serve should work even without Connect (it will listen locally)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cli.Stop()
	}()

	cli.Serve(ctx)
}
