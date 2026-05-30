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

// TestContextScenarios_Timeouts tests various timeout scenarios
func TestContextScenarios_Timeouts(t *testing.T) {
	t.Run("Invoke with very short timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		timeouts := []time.Duration{
			1 * time.Nanosecond,
			1 * time.Microsecond,
			1 * time.Millisecond,
			10 * time.Millisecond,
		}

		for _, timeout := range timeouts {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Timeout %v: duration=%v, error=%v, result_len=%d",
				timeout, duration, err, len(result))
		}
	})

	t.Run("Invoke with deadline", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		deadlines := []time.Time{
			time.Now().Add(1 * time.Millisecond),
			time.Now().Add(10 * time.Millisecond),
			time.Now().Add(-1 * time.Hour), // Already passed
		}

		for _, deadline := range deadlines {
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()

			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

			timeUntilDeadline := time.Until(deadline)
			t.Logf("Deadline %v (in %v): error=%v, result_len=%d",
				deadline.Format("15:04:05.000"), timeUntilDeadline, err, len(result))
		}
	})
}

// TestContextScenarios_Cancellation tests context cancellation scenarios
func TestContextScenarios_Cancellation(t *testing.T) {
	t.Run("Cancel context immediately", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Immediate cancel: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Cancel context after delay", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		delays := []time.Duration{
			1 * time.Millisecond,
			5 * time.Millisecond,
			10 * time.Millisecond,
		}

		for _, delay := range delays {
			ctx, cancel := context.WithCancel(context.Background())

			// Cancel after delay
			go func() {
				time.Sleep(delay)
				cancel()
			}()

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Cancel after %v: duration=%v, error=%v, result_len=%d",
				delay, duration, err, len(result))
		}
	})
}

// TestContextScenarios_Values tests context values scenarios
func TestContextScenarios_Values(t *testing.T) {
	t.Run("Invoke with context values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		type contextKey string

		ctx := context.Background()
		ctx = context.WithValue(ctx, contextKey("requestID"), "req-12345")
		ctx = context.WithValue(ctx, contextKey("userID"), "user-67890")
		ctx = context.WithValue(ctx, contextKey("sessionID"), "sess-abcde")
		ctx = context.WithValue(ctx, contextKey("traceID"), "trace-xyz")
		ctx = context.WithValue(ctx, contextKey("clientIP"), "192.168.1.1")

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Context values: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Invoke with nested context values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		type contextKey string

		// Create nested context with multiple values
		ctx1 := context.WithValue(context.Background(), contextKey("level1"), "value1")
		ctx2 := context.WithValue(ctx1, contextKey("level2"), "value2")
		ctx3 := context.WithValue(ctx2, contextKey("level3"), "value3")

		result, err := invoker.Invoke(ctx3, "test.function", "{}", InvokeOptions{})
		t.Logf("Nested context values: error=%v, result_len=%d", err, len(result))
	})
}

// TestContextScenarios_Background tests background context scenarios
func TestContextScenarios_Background(t *testing.T) {
	t.Run("Invoke with background context", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Background context: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Invoke with TODO context", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.TODO()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("TODO context: error=%v, result_len=%d", err, len(result))
	})
}

// TestContextScenarios_ChainedContexts tests chained context operations
func TestContextScenarios_ChainedContexts(t *testing.T) {
	t.Run("WithCancel then WithTimeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithTimeout(ctx1, time.Second)
		defer cancel2()
		defer cancel1()

		result, err := invoker.Invoke(ctx2, "test.function", "{}", InvokeOptions{})
		t.Logf("Chained cancel+timeout: error=%v, result_len=%d", err, len(result))
	})

	t.Run("WithTimeout then WithValue", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		type contextKey string
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second)
		ctx2 := context.WithValue(ctx1, contextKey("key"), "value")
		defer cancel1()

		result, err := invoker.Invoke(ctx2, "test.function", "{}", InvokeOptions{})
		t.Logf("Chained timeout+value: error=%v, result_len=%d", err, len(result))
	})

	t.Run("WithValue then WithDeadline", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		type contextKey string
		ctx1 := context.WithValue(context.Background(), contextKey("key1"), "value1")
		ctx2, cancel := context.WithDeadline(ctx1, time.Now().Add(time.Second))
		defer cancel()

		result, err := invoker.Invoke(ctx2, "test.function", "{}", InvokeOptions{})
		t.Logf("Chained value+deadline: error=%v, result_len=%d", err, len(result))
	})
}

// TestContextScenarios_ConcurrentOperations tests concurrent context operations
func TestContextScenarios_ConcurrentOperations(t *testing.T) {
	t.Run("Multiple concurrent invokes with different contexts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer func() { done <- true }()

				// Each goroutine uses a different context type
				var ctx context.Context
				switch idx % 4 {
				case 0:
					ctx = context.Background()
				case 1:
					ctx, _ = context.WithTimeout(context.Background(), time.Second)
				case 2:
					type contextKey string
					ctx = context.WithValue(context.Background(), contextKey("idx"), idx)
				case 3:
					ctx, _ = context.WithCancel(context.Background())
				}

				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Goroutine %d: error=%v, result_len=%d", idx, err, len(result))
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		t.Logf("Completed %d concurrent invokes with different contexts", numGoroutines)
	})
}

// TestContextScenarios_ErrorPropagation tests error propagation through contexts
func TestContextScenarios_ErrorPropagation(t *testing.T) {
	t.Run("Context cancellation error", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		if err != nil {
			if err == context.Canceled {
				t.Log("Got expected context.Canceled error")
			} else {
				t.Logf("Got error: %v (type: %T)", err, err)
			}
		} else {
			t.Log("No error with cancelled context")
		}

		t.Logf("Result length: %d", len(result))
	})

	t.Run("Context deadline exceeded error", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		time.Sleep(time.Millisecond) // Ensure timeout expires

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		if err != nil {
			if err == context.DeadlineExceeded {
				t.Log("Got expected context.DeadlineExceeded error")
			} else {
				t.Logf("Got error: %v (type: %T)", err, err)
			}
		} else {
			t.Log("No error with expired deadline")
		}

		t.Logf("Result length: %d", len(result))
	})
}

// TestContextScenarios_TimeoutValues tests various timeout value scenarios
func TestContextScenarios_TimeoutValues(t *testing.T) {
	t.Run("Invoke with InvokeOptions timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		timeouts := []time.Duration{
			time.Nanosecond,
			time.Millisecond,
			100 * time.Millisecond,
			time.Second,
		}

		for _, timeout := range timeouts {
			opts := InvokeOptions{
				Timeout: timeout,
			}

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			duration := time.Since(start)

			t.Logf("InvokeOptions timeout %v: duration=%v, error=%v, result_len=%d",
				timeout, duration, err, len(result))
		}
	})
}

// TestContextScenarios_ContextChecks tests context state checks
func TestContextScenarios_ContextChecks(t *testing.T) {
	t.Run("Check context Done channel", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())

		// Check Done before cancel
		select {
		case <-ctx.Done():
			t.Log("Context done before cancel")
		default:
			t.Log("Context not done yet")
		}

		cancel()

		// Check Done after cancel
		select {
		case <-ctx.Done():
			t.Log("Context done after cancel")
		default:
			t.Log("Context still not done")
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("After cancel: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Check context Err method", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())

		errBefore := ctx.Err()
		t.Logf("Context.Err() before cancel: %v", errBefore)

		cancel()

		errAfter := ctx.Err()
		t.Logf("Context.Err() after cancel: %v", errAfter)

		if errAfter == context.Canceled {
			t.Log("Confirmed context.Canceled")
		}
	})
}

// TestContextScenarios_RealWorldPatterns tests real-world context usage patterns
func TestContextScenarios_RealWorldPatterns(t *testing.T) {
	t.Run("HTTP request context pattern", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Simulate HTTP request context with timeout and values
		type contextKey string
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		ctx = context.WithValue(ctx, contextKey("requestID"), fmt.Sprintf("req-%d", time.Now().UnixNano()))
		ctx = context.WithValue(ctx, contextKey("userID"), "user-123")
		ctx = context.WithValue(ctx, contextKey("remoteAddr"), "192.168.1.100:12345")
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.function", `{"input":"test"}`, InvokeOptions{})
		t.Logf("HTTP request pattern: error=%v, result_len=%d", err, len(result))
	})

	t.Run("RPC call context pattern", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Simulate RPC call context
		type contextKey string
		ctx := context.Background()
		ctx = context.WithValue(ctx, contextKey("rpcService"), "TestService")
		ctx = context.WithValue(ctx, contextKey("rpcMethod"), "TestMethod")
		ctx = context.WithValue(ctx, contextKey("rpcCallID"), fmt.Sprintf("call-%d", time.Now().UnixNano()))

		result, err := invoker.Invoke(ctx, "test.function", `{"method":"test"}`, InvokeOptions{})
		t.Logf("RPC call pattern: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Batch processing context pattern", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Simulate batch processing with deadline
		type contextKey string
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Minute))
		ctx = context.WithValue(ctx, contextKey("batchID"), fmt.Sprintf("batch-%d", time.Now().UnixNano()))
		ctx = context.WithValue(ctx, contextKey("batchSize"), 100)
		defer cancel()

		const batchSize = 5
		for i := 0; i < batchSize; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"item":%d}`, i), InvokeOptions{})
			t.Logf("Batch item %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}
