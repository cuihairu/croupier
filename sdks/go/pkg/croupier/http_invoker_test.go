package croupier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHTTPInvoker(t *testing.T) {
	t.Parallel()

	t.Run("with valid config", func(t *testing.T) {
		t.Parallel()

		config := &InvokerConfig{
			Address:        "localhost:18780",
			TimeoutSeconds: 30,
			Insecure:       true,
		}
		invoker := NewHTTPInvoker(config)

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		impl, ok := invoker.(*httpInvoker)
		if !ok {
			t.Fatal("NewHTTPInvoker did not return *httpInvoker type")
		}

		if impl.config != config {
			t.Error("config not set correctly")
		}

		if impl.schemas == nil {
			t.Error("schemas map not initialized")
		}

		if impl.httpClient == nil {
			t.Error("httpClient not initialized")
		}
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		t.Parallel()

		invoker := NewHTTPInvoker(nil)
		impl := invoker.(*httpInvoker)

		if impl.config == nil {
			t.Fatal("config should be set to default")
		}

		if impl.config.Address != "localhost:18780" {
			t.Errorf("expected default Address, got %q", impl.config.Address)
		}

		if !impl.config.Insecure {
			t.Error("expected default Insecure to be true")
		}
	})
}

func TestHTTPInvoker_Connect(t *testing.T) {
	t.Parallel()

	t.Run("successful connect", func(t *testing.T) {
		t.Parallel()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  "localhost:18780",
			Insecure: true,
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		impl := invoker.(*httpInvoker)
		if !impl.connected {
			t.Error("expected connected to be true")
		}
	})

	t.Run("connect is idempotent", func(t *testing.T) {
		t.Parallel()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  "localhost:18780",
			Insecure: true,
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		if err != nil {
			t.Fatalf("first connect failed: %v", err)
		}

		err = invoker.Connect(ctx)
		if err != nil {
			t.Fatalf("second connect should not fail: %v", err)
		}
	})
}

func TestHTTPInvoker_Close(t *testing.T) {
	t.Parallel()

	t.Run("close when connected", func(t *testing.T) {
		t.Parallel()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  "localhost:18780",
			Insecure: true,
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		err := invoker.Close()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		impl := invoker.(*httpInvoker)
		if impl.connected {
			t.Error("expected connected to be false")
		}
	})

	t.Run("close when not connected", func(t *testing.T) {
		t.Parallel()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  "localhost:18780",
			Insecure: true,
		})

		err := invoker.Close()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		t.Parallel()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  "localhost:18780",
			Insecure: true,
		})

		_ = invoker.Connect(context.Background())

		_ = invoker.Close()
		err := invoker.Close()
		if err != nil {
			t.Fatalf("second close should not fail: %v", err)
		}
	})
}

func TestHTTPInvoker_SetSchema(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

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

	impl := invoker.(*httpInvoker)
	stored, ok := impl.schemas["test.function"]
	if !ok {
		t.Error("schema not stored")
	}
	if stored == nil {
		t.Error("stored schema is nil")
	}
}

func TestHTTPInvoker_IsConnected(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	// Should not be connected initially
	if invoker.(*httpInvoker).IsConnected() {
		t.Error("should not be connected initially")
	}

	// Connect
	_ = invoker.Connect(context.Background())

	// Should be connected now
	if !invoker.(*httpInvoker).IsConnected() {
		t.Error("should be connected after Connect")
	}
}

func TestHTTPInvoker_GetAddress(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:19999",
		Insecure: true,
	})

	addr := invoker.(*httpInvoker).GetAddress()
	if addr == "" {
		t.Error("expected address to be set")
	}
}

func TestHTTPInvoker_SetDefaultGameEnv(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	impl := invoker.(*httpInvoker)
	impl.SetDefaultGameEnv("test-game", "production")

	if impl.defaultGameID != "test-game" {
		t.Errorf("expected defaultGameID test-game, got %s", impl.defaultGameID)
	}

	if impl.defaultEnv != "production" {
		t.Errorf("expected defaultEnv production, got %s", impl.defaultEnv)
	}
}

func TestHTTPInvoker_Invoke_WithMockServer(t *testing.T) {
	t.Parallel()

	t.Run("successful invocation", func(t *testing.T) {
		t.Parallel()

		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method and path
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			// Return success response
			resp := map[string]interface{}{
				"code":    0,
				"message": "success",
				"data":    map[string]interface{}{"result": "ok"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		// Create invoker with server URL
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  server.URL,
			Insecure: true,
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		result, err := invoker.Invoke(ctx, "test.function", `{"input":"data"}`, InvokeOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("invocation with idempotency key", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read request body
			var reqBody map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&reqBody)

			// Verify idempotency key is present
			if _, ok := reqBody["idempotencyKey"]; !ok {
				t.Error("expected idempotencyKey in request")
			}

			resp := map[string]interface{}{
				"code":    0,
				"message": "success",
				"data":    "ok",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  server.URL,
			Insecure: true,
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		opts := InvokeOptions{
			IdempotencyKey: "test-key-123",
		}
		_, err := invoker.Invoke(ctx, "test.function", `{}`, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invocation with server error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"code":    500,
				"message": "internal error",
				"data":    nil,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  server.URL,
			Insecure: true,
			Retry:    &RetryConfig{Enabled: false},
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		_, err := invoker.Invoke(ctx, "test.function", `{}`, InvokeOptions{})
		if err == nil {
			t.Error("expected error for server error response")
		}
	})

	t.Run("invocation with HTTP error status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  server.URL,
			Insecure: true,
			Retry:    &RetryConfig{Enabled: false},
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		_, err := invoker.Invoke(ctx, "test.function", `{}`, InvokeOptions{})
		if err == nil {
			t.Error("expected error for HTTP 500")
		}
	})
}

func TestHTTPInvoker_Invoke_WithSchemaValidation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  server.URL,
		Insecure: true,
	})

	// Set schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"name"},
	}
	_ = invoker.SetSchema("test.function", schema)

	ctx := context.Background()
	_ = invoker.Connect(ctx)

	t.Run("valid payload", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.function", `{"name":"test"}`, InvokeOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid payload - missing required field", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.function", `{}`, InvokeOptions{})
		if err == nil {
			t.Error("expected error for invalid payload")
		}
	})

	t.Run("invalid payload - wrong type", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.function", `{"name":123}`, InvokeOptions{})
		if err == nil {
			t.Error("expected error for wrong type")
		}
	})
}

func TestHTTPInvoker_StartTask(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    "task completed",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  server.URL,
		Insecure: true,
	})

	ctx := context.Background()
	_ = invoker.Connect(ctx)

	taskID, err := invoker.StartTask(ctx, "test.function", `{"input":"data"}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if taskID == "" {
		t.Error("expected non-empty task ID")
	}
}

func TestHTTPInvoker_StreamTask(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	ctx := context.Background()

	_, err := invoker.StreamTask(ctx, "task-123")
	if err == nil {
		t.Error("expected error for StreamTask (not supported)")
	}
}

func TestHTTPInvoker_CancelTask(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	ctx := context.Background()

	err := invoker.CancelTask(ctx, "task-123")
	if err == nil {
		t.Error("expected error for CancelTask (not supported)")
	}
}

func TestHTTPInvoker_parseJSONPayload(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(nil).(*httpInvoker)

	t.Run("valid JSON object", func(t *testing.T) {
		result := invoker.parseJSONPayload(`{"key":"value"}`)
		obj, ok := result.(map[string]interface{})
		if !ok {
			t.Error("expected map[string]interface{}")
		}
		if obj["key"] != "value" {
			t.Errorf("expected value, got %v", obj["key"])
		}
	})

	t.Run("valid JSON array", func(t *testing.T) {
		result := invoker.parseJSONPayload(`[1,2,3]`)
		arr, ok := result.([]interface{})
		if !ok {
			t.Error("expected []interface{}")
		}
		if len(arr) != 3 {
			t.Errorf("expected 3 elements, got %d", len(arr))
		}
	})

	t.Run("invalid JSON returns string", func(t *testing.T) {
		result := invoker.parseJSONPayload(`not json`)
		str, ok := result.(string)
		if !ok {
			t.Error("expected string")
		}
		if str != "not json" {
			t.Errorf("expected 'not json', got %s", str)
		}
	})
}

func TestHTTPInvoker_ValidatePayload(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(nil).(*httpInvoker)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"name"},
	}

	t.Run("valid payload", func(t *testing.T) {
		err := invoker.validatePayload("test.function", `{"name":"test"}`, schema)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid JSON payload", func(t *testing.T) {
		err := invoker.validatePayload("test.function", `not json`, schema)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		err := invoker.validatePayload("test.function", `{}`, schema)
		if err == nil {
			t.Error("expected error for missing required field")
		}
	})
}

func TestHTTPInvoker_Invoke_WithHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for custom headers
		if r.Header.Get("X-Custom") != "value" {
			t.Error("expected X-Custom header")
		}
		if r.Header.Get("X-Game-ID") != "test-game" {
			t.Error("expected X-Game-ID header")
		}

		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  server.URL,
		Insecure: true,
	})

	ctx := context.Background()
	_ = invoker.Connect(ctx)

	opts := InvokeOptions{
		Headers: map[string]string{
			"X-Custom":  "value",
			"X-Game-ID": "test-game",
		},
	}
	_, err := invoker.Invoke(ctx, "test.function", `{}`, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPInvoker_Invoke_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:        server.URL,
		Insecure:       true,
		TimeoutSeconds: 1,
		Retry:          &RetryConfig{Enabled: false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = invoker.Connect(ctx)

	_, err := invoker.Invoke(ctx, "test.function", `{}`, InvokeOptions{})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestHTTPInvoker_Retry(t *testing.T) {
	t.Parallel()

	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			// First 2 attempts fail
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Third attempt succeeds
		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: server.URL,
		Retry: &RetryConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 10,
			MaxDelayMs:     100,
		},
	})

	ctx := context.Background()
	_ = invoker.Connect(ctx)

	_, err := invoker.Invoke(ctx, "test.function", `{}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}

	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

// TestHTTPInvoker_StartTask_Comprehensive covers edge cases for StartTask
func TestHTTPInvoker_StartTask_Comprehensive(t *testing.T) {
	t.Parallel()

	t.Run("returns generated task ID on success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"code":    0,
				"message": "success",
				"data":    map[string]interface{}{"status": "started"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  server.URL,
			Insecure: true,
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		taskID, err := invoker.StartTask(ctx, "test.async.function", `{"input":"data"}`, InvokeOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify task ID format (should start with "http-task-")
		if taskID == "" {
			t.Error("expected non-empty task ID")
		}
		if len(taskID) < 10 {
			t.Errorf("task ID too short: %s", taskID)
		}
	})

	t.Run("propagates invoke errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"code":    500,
				"message": "internal server error",
				"data":    nil,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:  server.URL,
			Insecure: true,
			Retry:    &RetryConfig{Enabled: false},
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		_, err := invoker.StartTask(ctx, "test.function", `{}`, InvokeOptions{})
		if err == nil {
			t.Error("expected error for failed invocation")
		}
	})

	t.Run("handles timeout during StartTask", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			resp := map[string]interface{}{
				"code":    0,
				"message": "success",
				"data":    nil,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:        server.URL,
			Insecure:       true,
			TimeoutSeconds: 1,
			Retry:          &RetryConfig{Enabled: false},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_ = invoker.Connect(ctx)

		_, err := invoker.StartTask(ctx, "test.function", `{}`, InvokeOptions{})
		if err == nil {
			t.Error("expected timeout error")
		}
	})
}

// TestHTTPInvoker_StreamTask_ErrorCases tests error scenarios for StreamTask
func TestHTTPInvoker_StreamTask_ErrorCases(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	ctx := context.Background()

	t.Run("returns not supported error", func(t *testing.T) {
		t.Parallel()

		_, err := invoker.StreamTask(ctx, "any-task-id")
		if err == nil {
			t.Error("expected error for StreamTask (not supported)")
		}

		expectedErr := "not supported for HTTP invoker"
		if err.Error() == "" || !containsString(err.Error(), expectedErr) {
			t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("error is consistent across different task IDs", func(t *testing.T) {
		t.Parallel()

		taskIDs := []string{"", "task-123", "http-task-456", "invalid"}

		for _, taskID := range taskIDs {
			_, err := invoker.StreamTask(ctx, taskID)
			if err == nil {
				t.Errorf("expected error for task ID %q", taskID)
			}
		}
	})
}

// TestHTTPInvoker_CancelTask_ErrorCases tests error scenarios for CancelTask
func TestHTTPInvoker_CancelTask_ErrorCases(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	ctx := context.Background()

	t.Run("returns not supported error", func(t *testing.T) {
		t.Parallel()

		err := invoker.CancelTask(ctx, "any-task-id")
		if err == nil {
			t.Error("expected error for CancelTask (not supported)")
		}

		expectedErr := "not supported for HTTP invoker"
		if err.Error() == "" || !containsString(err.Error(), expectedErr) {
			t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("error is consistent across different task IDs", func(t *testing.T) {
		t.Parallel()

		taskIDs := []string{"", "task-123", "http-task-456", "running-task"}

		for _, taskID := range taskIDs {
			err := invoker.CancelTask(ctx, taskID)
			if err == nil {
				t.Errorf("expected error for task ID %q", taskID)
			}
		}
	})

	t.Run("CancelTask when not connected", func(t *testing.T) {
		t.Parallel()

		invoker2 := NewHTTPInvoker(&InvokerConfig{
			Address:  "localhost:18780",
			Insecure: true,
		})
		// Don't connect

		err := invoker2.CancelTask(ctx, "task-123")
		if err == nil {
			t.Error("expected error when not connected")
		}
	})
}

// TestHTTPInvoker_TaskOperations_NotSupported tests all unsupported task operations
func TestHTTPInvoker_TaskOperations_NotSupported(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  "localhost:18780",
		Insecure: true,
	})

	ctx := context.Background()

	// Test that both StreamTask and CancelTask return consistent errors
	streamErrChan := make(chan error, 1)
	cancelErrChan := make(chan error, 1)

	go func() {
		_, err := invoker.StreamTask(ctx, "test-task")
		streamErrChan <- err
	}()

	go func() {
		err := invoker.CancelTask(ctx, "test-task")
		cancelErrChan <- err
	}()

	streamErr := <-streamErrChan
	cancelErr := <-cancelErrChan

	if streamErr == nil || cancelErr == nil {
		t.Error("both operations should return errors")
	}

	// Both should mention "not supported"
	if !containsString(streamErr.Error(), "not supported") {
		t.Errorf("StreamTask error should mention 'not supported', got: %v", streamErr)
	}
	if !containsString(cancelErr.Error(), "not supported") {
		t.Errorf("CancelTask error should mention 'not supported', got: %v", cancelErr)
	}
}

// TestHTTPInvoker_StartTask_UniqueIdGeneration tests task ID uniqueness
func TestHTTPInvoker_StartTask_UniqueIdGeneration(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    nil,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:  server.URL,
		Insecure: true,
	})

	ctx := context.Background()
	_ = invoker.Connect(ctx)

	// Generate multiple task IDs and verify they are unique
	taskIDs := make(map[string]bool)
	for i := 0; i < 10; i++ {
		taskID, err := invoker.StartTask(ctx, "test.function", `{}`, InvokeOptions{})
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}

		if taskIDs[taskID] {
			t.Errorf("duplicate task ID generated: %s", taskID)
		}
		taskIDs[taskID] = true
	}

	if len(taskIDs) != 10 {
		t.Errorf("expected 10 unique task IDs, got %d", len(taskIDs))
	}
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
