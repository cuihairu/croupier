// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestInvoker_errorScenarios tests invoker error scenarios
func TestInvoker_errorScenarios(t *testing.T) {
	t.Run("Invoke with invalid address", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "invalid-address-99999:99999",
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})

		t.Logf("Invoke with invalid address error: %v", err)
	})

	t.Run("Invoke with very short timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout has passed

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with timed out context error: %v", err)
	})

	t.Run("Invoke with cancelled context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with cancelled context error: %v", err)
	})

	t.Run("Invoke with very large payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		largePayload := string(make([]byte, 10*1024*1024)) // 10MB

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", largePayload, InvokeOptions{})
		t.Logf("Invoke with 10MB payload error: %v", err)
	})

	t.Run("Invoke with special JSON payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		specialPayloads := []string{
			`{"null": null}`,
			`{"empty": ""}`,
			`{"array": [1,2,3]}`,
			`{"nested": {"a": {"b": {"c": 1}}}}`,
			`{"unicode": "世界😀"}`,
			`{"escape": "\n\t\r"}`,
			`{"number": 123.456}`,
			`{"bool": true}`,
		}

		ctx := context.Background()
		for _, payload := range specialPayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			// Safely truncate payload for logging
			displayPayload := payload
			if len(displayPayload) > 20 {
				displayPayload = displayPayload[:20] + "..."
			}
			t.Logf("Invoke with payload '%s' error: %v", displayPayload, err)
		}
	})
}

// TestInvoker_invokeWithRetry tests Invoke with retry configuration
func TestInvoker_invokeWithRetry(t *testing.T) {
	t.Run("Invoke with retry enabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 10,
				MaxDelayMs:     100,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with retry error: %v", err)
	})

	t.Run("Invoke with retry and short timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    5,
				InitialDelayMs: 1,
				MaxDelayMs:     10,
			},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with retry and timeout error: %v", err)
	})

	t.Run("Invoke with zero delay retry", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    2,
				InitialDelayMs: 0,
				MaxDelayMs:     0,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with zero delay retry error: %v", err)
	})

	t.Run("Invoke with very high retry attempts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    100,
				InitialDelayMs: 1,
				MaxDelayMs:     5,
			},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with high retry attempts error: %v", err)
	})
}

// TestInvoker_taskOperations tests task operations
func TestInvoker_taskOperations(t *testing.T) {
	t.Run("StartTask with invalid address", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "invalid-address:99999",
		})

		ctx := context.Background()
		taskID, err := invoker.StartTask(ctx, "test.func", "{}", InvokeOptions{})

		t.Logf("StartTask error: %v, taskID: %s", err, taskID)
	})

	t.Run("StartTask with timeout context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond)

		taskID, err := invoker.StartTask(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("StartTask with timeout error: %v, taskID: %s", err, taskID)
	})

	t.Run("StartTask with retry", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		})

		ctx := context.Background()
		taskID, err := invoker.StartTask(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("StartTask with retry error: %v, taskID: %s", err, taskID)
	})

	t.Run("StreamTask with invalid task ID", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		events, err := invoker.StreamTask(ctx, "invalid-task-id-99999")

		if events != nil {
			t.Log("StreamTask returned channel")
		}

		t.Logf("StreamTask error: %v", err)
	})

	t.Run("StreamTask with timeout context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		events, err := invoker.StreamTask(ctx, "test-task")

		if events != nil {
			t.Log("StreamTask returned channel")
		}

		t.Logf("StreamTask with timeout error: %v", err)
	})

	t.Run("CancelTask with invalid task ID", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		err := invoker.CancelTask(ctx, "invalid-task-id-99999")

		t.Logf("CancelTask error: %v", err)
	})

	t.Run("CancelTask with timeout context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		err := invoker.CancelTask(ctx, "test-task")

		t.Logf("CancelTask with timeout error: %v", err)
	})

	t.Run("CancelTask with retry", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		})

		ctx := context.Background()
		err := invoker.CancelTask(ctx, "test-task")

		t.Logf("CancelTask with retry error: %v", err)
	})
}

// TestInvoker_connectWithReconnect tests invoker with reconnect configuration
func TestInvoker_connectWithReconnect(t *testing.T) {
	t.Run("Invoke with reconnect enabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    5,
				InitialDelayMs: 10,
				MaxDelayMs:     100,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with reconnect error: %v", err)
	})

	t.Run("Invoke with both retry and reconnect", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
			Reconnect: &ReconnectConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with retry and reconnect error: %v", err)
	})

	t.Run("Invoke with zero reconnect delay", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    2,
				InitialDelayMs: 0,
				MaxDelayMs:     0,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with zero reconnect delay error: %v", err)
	})
}

// TestInvoker_schemaOperations tests schema operations
func TestInvoker_schemaOperations(t *testing.T) {
	t.Run("SetSchema with complex schema", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field1": map[string]interface{}{
					"type":      "string",
					"minLength": 1,
					"maxLength": 100,
				},
				"field2": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
					"maximum": 1000,
				},
				"field3": map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "string"},
				},
			},
			"required": []string{"field1", "field2"},
		}

		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with complex schema error: %v", err)
	})

	t.Run("SetSchema with empty schema", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		err := invoker.SetSchema("test.func", map[string]interface{}{})
		t.Logf("SetSchema with empty schema error: %v", err)
	})

	t.Run("SetSchema with nil schema", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		var schema map[string]interface{} = nil
		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with nil schema error: %v", err)
	})
}

// TestInvoker_variousOptions tests InvokeOptions variations
func TestInvoker_variousOptions(t *testing.T) {
	t.Run("Invoke with idempotency key", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		options := InvokeOptions{
			IdempotencyKey: "unique-key-12345",
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with idempotency key error: %v", err)
	})

	t.Run("Invoke with very long timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		options := InvokeOptions{
			Timeout: 24 * time.Hour,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with long timeout error: %v", err)
	})

	t.Run("Invoke with zero timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		options := InvokeOptions{
			Timeout: 0,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with zero timeout error: %v", err)
	})

	t.Run("Invoke with many headers", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		headers := make(map[string]string)
		for i := 0; i < 100; i++ {
			headers[fmt.Sprintf("X-Header-%d", i)] = fmt.Sprintf("value-%d", i)
		}

		options := InvokeOptions{
			Headers: headers,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with 100 headers error: %v", err)
	})
}

// TestInvoker_concurrentInvokes tests concurrent invoke operations
func TestInvoker_concurrentInvokes(t *testing.T) {
	t.Run("concurrent Invoke operations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		const numGoroutines = 20
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				ctx := context.Background()
				_, errors[idx] = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			}(i)
		}

		// Wait a bit for goroutines to complete
		time.Sleep(500 * time.Millisecond)

		errorCount := 0
		for i, err := range errors {
			if err != nil {
				errorCount++
				t.Logf("Goroutine %d error: %v", i, err)
			}
		}

		t.Logf("Concurrent invokes: %d/%d had errors", errorCount, numGoroutines)
	})
}
