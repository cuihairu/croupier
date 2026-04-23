// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestLifecycle_Initialization tests invoker initialization scenarios
func TestLifecycle_Initialization(t *testing.T) {
	t.Run("Basic initialization", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		err := invoker.Close()
		t.Logf("Basic init and close: error=%v", err)
	})

	t.Run("Initialization with timeout", func(t *testing.T) {
		timeouts := []int{0, 1, 10, 30, 60, 300}

		for _, timeout := range timeouts {
			config := &InvokerConfig{
				Address:        "localhost:19090",
				TimeoutSeconds: timeout,
			}

			invoker := NewHTTPInvoker(config)
			if invoker == nil {
				t.Fatalf("Timeout %d: NewHTTPInvoker returned nil", timeout)
			}

			ctx := context.Background()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Timeout %d: error=%v, result_len=%d", timeout, err, len(result))

			invoker.Close()
		}
	})

	t.Run("Initialization with TLS configs", func(t *testing.T) {
		configs := []*InvokerConfig{
			{
				Address:  "localhost:19090",
				Insecure: true,
			},
			{
				Address:  "localhost:19090",
				Insecure: false,
			},
			{
				Address:  "localhost:19090",
				CAFile:   "/path/to/ca.crt",
				CertFile: "/path/to/cert.pem",
				KeyFile:  "/path/to/key.pem",
			},
		}

		for i, config := range configs {
			invoker := NewHTTPInvoker(config)
			if invoker == nil {
				t.Fatalf("Config %d: NewHTTPInvoker returned nil", i)
			}

			err := invoker.Close()
			t.Logf("TLS config %d: close error=%v", i, err)
		}
	})

	t.Run("Initialization with retry config", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("With retry config: error=%v, result_len=%d", err, len(result))

		invoker.Close()
	})

	t.Run("Initialization with reconnect config", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:       true,
				MaxAttempts:   3,
				InitialDelayMs: 100,
				MaxDelayMs:    1000,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		err := invoker.Close()
		t.Logf("With reconnect config: close error=%v", err)
	})
}

// TestLifecycle_ConcurrentInitialization tests concurrent initialization scenarios
func TestLifecycle_ConcurrentInitialization(t *testing.T) {
	t.Run("Parallel invoker creation", func(t *testing.T) {
		const numInvokers = 50
		var wg sync.WaitGroup

		for i := 0; i < numInvokers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				config := &InvokerConfig{
					Address: fmt.Sprintf("localhost:%d", 19090+idx%10),
				}

				invoker := NewHTTPInvoker(config)
				if invoker != nil {
					ctx := context.Background()
					result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
					_ = result
					_ = err
					invoker.Close()
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Created and closed %d invokers concurrently", numInvokers)
	})

	t.Run("Rapid create and destroy cycles", func(t *testing.T) {
		const cycles = 100

		for i := 0; i < cycles; i++ {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				invoker.Close()
			}
		}

		t.Logf("Completed %d create/destroy cycles", cycles)
	})
}

// TestLifecycle_StateTransitions tests state transition scenarios
func TestLifecycle_StateTransitions(t *testing.T) {
	t.Run("From initialized to active", func(t *testing.T) {
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
		t.Logf("Initialized to active: error=%v, result_len=%d", err, len(result))
	})

	t.Run("From active to closed", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Before close: error=%v, result_len=%d", err, len(result))

		closeErr := invoker.Close()
		t.Logf("Closed: error=%v", closeErr)

		// Operations after close
		result2, err2 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("After close: error=%v, result_len=%d", err2, len(result2))
	})

	t.Run("Multiple close calls", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		// Close multiple times
		var errs []error
		for i := 0; i < 5; i++ {
			err := invoker.Close()
			errs = append(errs, err)
			t.Logf("Close call %d: error=%v", i, err)
		}
	})
}

// TestLifecycle_ResourceAcquisition tests resource acquisition patterns
func TestLifecycle_ResourceAcquisition(t *testing.T) {
	t.Run("Connection pooling", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numRequests = 50

		start := time.Now()
		for i := 0; i < numRequests; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			_ = result
			_ = err
		}
		duration := time.Since(start)

		t.Logf("Connection pooling: %d requests in %v (%.2f req/sec)",
			numRequests, duration, float64(numRequests)/duration.Seconds())
	})

	t.Run("Schema resource management", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Set multiple schemas
		for i := 0; i < 10; i++ {
			schema := map[string]interface{}{
				"type": "string",
			}
			err := invoker.SetSchema(fmt.Sprintf("function%d", i), schema)
			t.Logf("Set schema %d: error=%v", i, err)
		}
	})

	t.Run("Job resource management", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numJobs = 20

		// Start multiple jobs
		jobIDs := make([]string, numJobs)
		for i := 0; i < numJobs; i++ {
			jobID, err := invoker.StartJob(ctx, "test.job", fmt.Sprintf(`{"job":%d}`, i), InvokeOptions{})
			jobIDs[i] = jobID
			t.Logf("Started job %d: jobID=%s, error=%v", i, jobID, err)
		}

		// Cancel all jobs
		for _, jobID := range jobIDs {
			if jobID != "" {
				err := invoker.CancelJob(ctx, jobID)
				t.Logf("Cancelled job %s: error=%v", jobID, err)
			}
		}
	})
}

// TestLifecycle_CleanupOperations tests cleanup operations
func TestLifecycle_CleanupOperations(t *testing.T) {
	t.Run("Cleanup after errors", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()

		// Invoke with invalid function (will error)
		result1, err1 := invoker.Invoke(ctx, "invalid.function", "{}", InvokeOptions{})
		t.Logf("Invalid function: error=%v, result_len=%d", err1, len(result1))

		// Cleanup should still work
		closeErr := invoker.Close()
		t.Logf("Cleanup after error: close error=%v", closeErr)
	})

	t.Run("Cleanup with pending jobs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()

		// Start some jobs
		jobIDs := make([]string, 5)
		for i := 0; i < 5; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
			jobIDs[i] = jobID
		}

		// Close without cancelling jobs
		closeErr := invoker.Close()
		t.Logf("Close with pending jobs: error=%v", closeErr)
	})

	t.Run("Cleanup with active streams", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()

		jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		if jobID != "" {
			eventChan, _ := invoker.StreamJob(ctx, jobID)

			// Close while stream is active
			closeErr := invoker.Close()
			t.Logf("Close with active stream: error=%v", closeErr)

			// Channel should be closed
			if eventChan != nil {
				_, ok := <-eventChan
				t.Logf("Stream channel open after close: %v", ok)
			}
		}
	})
}

// TestLifecycle_ErrorDuringInit tests error scenarios during initialization
func TestLifecycle_ErrorDuringInit(t *testing.T) {
	t.Run("Invalid address format", func(t *testing.T) {
		addresses := []string{
			"",
			"invalid-address",
			"http://",
			":19090",
		}

		for _, addr := range addresses {
			config := &InvokerConfig{
				Address: addr,
			}

			invoker := NewHTTPInvoker(config)
			// Should still create invoker, but will fail on invoke
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Address '%s': error=%v, result_len=%d", addr, err, len(result))
				invoker.Close()
			}
		}
	})

	t.Run("Non-existent certificate files", func(t *testing.T) {
		config := &InvokerConfig{
			Address:  "localhost:19090",
			CAFile:   "/nonexistent/ca.crt",
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		}

		invoker := NewHTTPInvoker(config)
		if invoker != nil {
			ctx := context.Background()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Non-existent cert files: error=%v, result_len=%d", err, len(result))
			invoker.Close()
		}
	})
}

// TestLifecycle_IdempotentOperations tests idempotent lifecycle operations
func TestLifecycle_IdempotentOperations(t *testing.T) {
	t.Run("Idempotent close", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		// Close multiple times should be safe
		var errs []error
		for i := 0; i < 10; i++ {
			err := invoker.Close()
			errs = append(errs, err)
		}

		t.Logf("Multiple closes: %d attempts, errors=%v", len(errs), errs)
	})

	t.Run("Idempotent schema setting", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "string",
		}

		// Set same schema multiple times
		var errs []error
		for i := 0; i < 5; i++ {
			err := invoker.SetSchema("test.function", schema)
			errs = append(errs, err)
		}

		t.Logf("Multiple schema sets: %d attempts, errors=%v", len(errs), errs)
	})
}

// TestLifecycle_ConcurrentAccess tests concurrent access during lifecycle
func TestLifecycle_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent operations during initialization", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numGoroutines = 20
		var wg sync.WaitGroup

		// Start operations immediately after creation
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, idx), InvokeOptions{})
				_ = result
				_ = err
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent ops during init: %d goroutines", numGoroutines)
	})

	t.Run("Concurrent operations during shutdown", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()
		const numGoroutines = 20
		var wg sync.WaitGroup

		// Start operations and close concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, idx), InvokeOptions{})
				_ = result
				_ = err
			}(i)
		}

		// Close while operations are in flight
		go func() {
			time.Sleep(10 * time.Millisecond)
			invoker.Close()
		}()

		wg.Wait()
		t.Logf("Concurrent ops during shutdown: %d goroutines", numGoroutines)
	})

	t.Run("Concurrent schema operations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 20
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				schema := map[string]interface{}{
					"type": "string",
				}
				err := invoker.SetSchema(fmt.Sprintf("function%d", idx), schema)
				_ = err
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent schema ops: %d operations", numGoroutines)
	})
}

// TestLifecycle_MemoryLeaks tests for potential memory leaks
func TestLifecycle_MemoryLeaks(t *testing.T) {
	t.Run("Repeated lifecycle cycles", func(t *testing.T) {
		const cycles = 50

		for i := 0; i < cycles; i++ {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				_ = result
				_ = err
				invoker.Close()
			}
		}

		t.Logf("Completed %d lifecycle cycles", cycles)
	})

	t.Run("Schema accumulation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Set many schemas
		const numSchemas = 100
		for i := 0; i < numSchemas; i++ {
			schema := map[string]interface{}{
				"type": "string",
			}
			err := invoker.SetSchema(fmt.Sprintf("function%d", i), schema)
			if i%10 == 0 {
				t.Logf("Set schema %d: error=%v", i, err)
			}
		}

		t.Logf("Set %d schemas", numSchemas)
	})
}

// TestLifecycle_TimeoutHandling tests timeout handling during lifecycle
func TestLifecycle_TimeoutHandling(t *testing.T) {
	t.Run("Invoke timeout during initialization", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 1,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Timeout during init phase: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Slow close operation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		// Start some jobs
		ctx := context.Background()
		for i := 0; i < 10; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
			_ = jobID
		}

		// Close with pending jobs
		start := time.Now()
		err := invoker.Close()
		duration := time.Since(start)

		t.Logf("Close with pending jobs: duration=%v, error=%v", duration, err)
	})
}

// TestLifecycle_ConfigurationValidation tests configuration validation
func TestLifecycle_ConfigurationValidation(t *testing.T) {
	t.Run("Valid configurations", func(t *testing.T) {
		configs := []*InvokerConfig{
			{Address: "http://localhost:19090"},
			{Address: "http://localhost:19090", TimeoutSeconds: 30},
			{Address: "http://localhost:19090", Insecure: true},
			{
				Address:  "localhost:19090",
				CAFile:   "/path/to/ca.crt",
				CertFile: "/path/to/cert.pem",
				KeyFile:  "/path/to/key.pem",
			},
			{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:     true,
					MaxAttempts: 3,
				},
			},
			{
				Address: "http://localhost:19090",
				Reconnect: &ReconnectConfig{
					Enabled: true,
				},
			},
		}

		for i, config := range configs {
			invoker := NewHTTPInvoker(config)
			if invoker == nil {
				t.Errorf("Config %d: failed to create invoker", i)
			} else {
				invoker.Close()
				t.Logf("Config %d: valid", i)
			}
		}
	})

	t.Run("Retry configuration boundaries", func(t *testing.T) {
		testCases := []struct {
			name     string
			retryCfg *RetryConfig
		}{
			{"Zero max attempts", &RetryConfig{Enabled: true, MaxAttempts: 0}},
			{"Negative max attempts", &RetryConfig{Enabled: true, MaxAttempts: -1}},
			{"Large max attempts", &RetryConfig{Enabled: true, MaxAttempts: 1000}},
			{"Zero delay", &RetryConfig{Enabled: true, MaxAttempts: 3, InitialDelayMs: 0}},
			{"Negative delay", &RetryConfig{Enabled: true, MaxAttempts: 3, InitialDelayMs: -100}},
			{"Large delay", &RetryConfig{Enabled: true, MaxAttempts: 3, InitialDelayMs: 1000000}},
		}

		for _, tc := range testCases {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Retry:   tc.retryCfg,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				// Only verify that the invoker can be created with these configurations
				// Don't actually invoke since extreme values (1000 retries, 16min delays) will timeout
				t.Logf("%s: invoker created successfully with config", tc.name)
				invoker.Close()
			} else {
				t.Errorf("%s: failed to create invoker", tc.name)
			}
		}
	})
}
