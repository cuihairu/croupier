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

const (
	Second = 1 * time.Second
)

// TestRetryMechanisms_BasicScenarios tests basic retry scenarios
func TestRetryMechanisms_BasicScenarios(t *testing.T) {
	t.Run("Invoke with retry enabled", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 100,
				MaxDelayMs:     1000,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Retry enabled: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Invoke with retry disabled", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled: false,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Retry disabled: error=%v, result_len=%d", err, len(result))
	})
}

// TestRetryMechanisms_MaxAttempts tests different max attempts configurations
func TestRetryMechanisms_MaxAttempts(t *testing.T) {
	t.Run("Retry with 1 attempt", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 1,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("1 attempt: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})

	t.Run("Retry with 3 attempts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("3 attempts: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})

	t.Run("Retry with 5 attempts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("5 attempts: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})
}

// TestRetryMechanisms_DelayStrategies tests different delay strategies
func TestRetryMechanisms_DelayStrategies(t *testing.T) {
	t.Run("Retry with exponential backoff", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("Exponential backoff: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})

	t.Run("Retry with linear backoff", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    200,
				MaxDelayMs:        1000,
				BackoffMultiplier: 1.0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("Linear backoff: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})
}

// TestRetryMechanisms_WithJitter tests retry with jitter
func TestRetryMechanisms_WithJitter(t *testing.T) {
	t.Run("Retry with jitter", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 100,
				MaxDelayMs:     1000,
				JitterFactor:   0.1,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("With jitter: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Retry without jitter", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 100,
				MaxDelayMs:     1000,
				JitterFactor:   0.0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Without jitter: error=%v, result_len=%d", err, len(result))
	})
}

// TestRetryMechanisms_WithOverride tests retry with InvokeOptions override
func TestRetryMechanisms_WithOverride(t *testing.T) {
	t.Run("Override retry configuration at invoke time", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Override with different retry config
		opts := InvokeOptions{
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 2,
			},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("With override: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Disable retry at invoke time", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Override to disable retry
		opts := InvokeOptions{
			Retry: &RetryConfig{
				Enabled: false,
			},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Disabled at invoke: error=%v, result_len=%d", err, len(result))
	})
}

// TestRetryMechanisms_ContextCancellation tests retry with context cancellation
func TestRetryMechanisms_ContextCancellation(t *testing.T) {
	t.Run("Cancel context during retry", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    10,
				InitialDelayMs: 500,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after a short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("Cancelled during retry: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})
}

// TestRetryMechanisms_ConcurrentOperations tests retry with concurrent operations
func TestRetryMechanisms_ConcurrentOperations(t *testing.T) {
	t.Run("Concurrent invokes with retry", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 100,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer func() { done <- true }()

				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

				t.Logf("Goroutine %d: error=%v, result_len=%d", idx, err, len(result))
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		duration := time.Since(start)
		t.Logf("Concurrent retries: %d goroutines in %v", numGoroutines, duration)
	})
}

// TestRetryMechanisms_EdgeCases tests retry edge cases
func TestRetryMechanisms_EdgeCases(t *testing.T) {
	t.Run("Retry with zero initial delay", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Zero initial delay: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Retry with very large delay", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    2,
				InitialDelayMs: 100000, // 100 seconds
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Large delay: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Retry with zero max attempts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Zero max attempts: error=%v, result_len=%d", err, len(result))
	})
}

// TestRetryMechanisms_PerformancePatterns tests retry performance patterns
func TestRetryMechanisms_PerformancePatterns(t *testing.T) {
	t.Run("Measure retry latency", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 100,
				MaxDelayMs:     1000,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		const iterations = 5
		totalDuration := time.Duration(0)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			duration := time.Since(start)

			totalDuration += duration
			t.Logf("Iteration %d: duration=%v, error=%v", i, duration, err)
		}

		avgDuration := totalDuration / time.Duration(iterations)
		t.Logf("Average retry latency: %v", avgDuration)
	})
}

// TestRetryMechanisms_Idempotency tests retry with idempotency
func TestRetryMechanisms_Idempotency(t *testing.T) {
	t.Run("Retry with idempotency key", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		idempotencyKey := fmt.Sprintf("idempotent-%d", time.Now().UnixNano())

		opts := InvokeOptions{
			IdempotencyKey: idempotencyKey,
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("With idempotency key: error=%v, result_len=%d", err, len(result))

		// Try again with same key
		result2, err2 := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Same idempotency key: error=%v, result_len=%d", err2, len(result2))
	})
}
