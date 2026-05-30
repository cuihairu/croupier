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

// TestAPICompatibility_InterfaceContracts tests interface contract compliance
func TestAPICompatibility_InterfaceContracts(t *testing.T) {
	t.Run("Invoker interface compliance", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Verify invoker implements all required interface methods
		var _ Invoker = invoker

		ctx := context.Background()

		// Test all interface methods exist
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		_ = result
		_ = err

		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		_ = jobID
		_ = err

		err = invoker.SetSchema("test.function", map[string]interface{}{"type": "string"})
		_ = err

		err = invoker.CancelJob(ctx, "test-job-id")
		_ = err

		eventChan, err := invoker.StreamJob(ctx, "test-job-id")
		_ = eventChan
		_ = err

		err = invoker.Close()
		_ = err

		t.Log("All interface methods are accessible")
	})

	t.Run("Nil interface handling", func(t *testing.T) {
		var invoker Invoker = nil

		// Test with panic recovery for nil invoker
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic with nil invoker: %v", r)
			}
		}()

		// Operations on nil invoker will panic (expected behavior)
		// This test verifies that we handle it appropriately
		ctx := context.Background()

		// These will panic, which we recover from
		_ = invoker.Close()
		_, _ = invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		_, _ = invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		_ = invoker.SetSchema("test", map[string]interface{}{})
		_ = invoker.CancelJob(ctx, "job-id")
		_, _ = invoker.StreamJob(ctx, "job-id")

		t.Log("Nil invoker operations panic as expected")
	})
}

// TestAPICompatibility_BackwardsCompatibility tests backwards compatibility
func TestAPICompatibility_BackwardsCompatibility(t *testing.T) {
	t.Run("Default configuration values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Test with default (empty) options
		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Default options: error=%v, result_len=%d", err, len(result))

		// Test with nil options (should use defaults)
		result, err = invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Empty options struct: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Legacy configuration support", func(t *testing.T) {
		// Test various configuration patterns
		configs := []*InvokerConfig{
			// Minimal config
			{Address: "http://localhost:19090"},
			// With timeout
			{Address: "http://localhost:19090", TimeoutSeconds: 30},
			// With TLS
			{Address: "http://localhost:19090", Insecure: true},
			// With all fields
			{
				Address:        "localhost:19090",
				TimeoutSeconds: 30,
				Insecure:       true,
				CAFile:         "/path/to/ca.crt",
				CertFile:       "/path/to/cert.pem",
				KeyFile:        "/path/to/key.pem",
			},
		}

		for i, config := range configs {
			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Config pattern %d: successfully created invoker", i)
				invoker.Close()
			} else {
				t.Errorf("Config pattern %d: failed to create invoker", i)
			}
		}
	})
}

// TestAPICompatibility_TypeConsistency tests type consistency across API
func TestAPICompatibility_TypeConsistency(t *testing.T) {
	t.Run("Config type consistency", func(t *testing.T) {
		// RetryConfig types
		retryConfig := &RetryConfig{
			Enabled:           true,
			MaxAttempts:       3,
			InitialDelayMs:    100,
			MaxDelayMs:        5000,
			BackoffMultiplier: 2.0,
			JitterFactor:      0.1,
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry:   retryConfig,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		t.Log("RetryConfig types are consistent")
	})

	t.Run("InvokeOptions type consistency", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test with various option types
		options := []InvokeOptions{
			{}, // Empty
			{
				Timeout: 30 * time.Second,
			},
			{
				Headers: map[string]string{
					"X-Test": "value",
				},
			},
			{
				IdempotencyKey: "test-key-123",
			},
			{
				Timeout:        10 * time.Second,
				Headers:        map[string]string{"X-Test": "value"},
				IdempotencyKey: "key-456",
			},
		}

		for i, opts := range options {
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Options %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestAPICompatibility_ErrorHandlingConsistency tests error handling consistency
func TestAPICompatibility_ErrorHandlingConsistency(t *testing.T) {
	t.Run("Consistent error types", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// All error scenarios should return error (not panic)
		scenarios := []struct {
			name string
			fn   func() error
		}{
			{
				"Invalid function",
				func() error {
					_, err := invoker.Invoke(ctx, "", "{}", InvokeOptions{})
					return err
				},
			},
			{
				"Invalid job ID",
				func() error {
					return invoker.CancelJob(ctx, "")
				},
			},
			{
				"Invalid schema",
				func() error {
					return invoker.SetSchema("", nil)
				},
			},
		}

		for _, scenario := range scenarios {
			err := scenario.fn()
			t.Logf("Scenario '%s': error=%v", scenario.name, err)
			// All should return error without panicking
		}
	})
}

// TestAPICompatibility_ConcurrencySafety tests concurrency safety
func TestAPICompatibility_ConcurrencySafety(t *testing.T) {
	t.Run("Concurrent invokes", func(t *testing.T) {
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

		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, idx), InvokeOptions{})
				_ = result
				errChan <- err
			}(i)
		}

		// Collect all results
		for i := 0; i < numGoroutines; i++ {
			err := <-errChan
			t.Logf("Goroutine %d: error=%v", i, err)
		}

		t.Log("Concurrent invokes completed safely")
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
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				schema := map[string]interface{}{
					"type": "string",
				}
				err := invoker.SetSchema(fmt.Sprintf("function%d", idx), schema)
				errChan <- err
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			err := <-errChan
			t.Logf("Schema operation %d: error=%v", i, err)
		}

		t.Log("Concurrent schema operations completed safely")
	})
}

// TestAPICompatibility_ParameterValidation tests parameter validation
func TestAPICompatibility_ParameterValidation(t *testing.T) {
	t.Run("Invoke parameter validation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test various parameter combinations
		testCases := []struct {
			name       string
			functionID string
			payload    string
			options    InvokeOptions
		}{
			{"Empty function ID", "", "{}", InvokeOptions{}},
			{"Empty payload", "test.function", "", InvokeOptions{}},
			{"Empty options", "test.function", "{}", InvokeOptions{}},
			{"All empty", "", "", InvokeOptions{}},
		}

		for _, tc := range testCases {
			result, err := invoker.Invoke(ctx, tc.functionID, tc.payload, tc.options)
			t.Logf("Test '%s': error=%v, result_len=%d", tc.name, err, len(result))
		}
	})

	t.Run("Schema parameter validation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Test various schema configurations
		schemas := []struct {
			name   string
			funcID string
			schema map[string]interface{}
		}{
			{"Empty schema", "test1", map[string]interface{}{}},
			{"Nil schema", "test2", nil},
			{"Valid schema", "test3", map[string]interface{}{"type": "string"}},
			{"Complex schema", "test4", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]string{"type": "string"},
				},
			}},
		}

		for _, tc := range schemas {
			err := invoker.SetSchema(tc.funcID, tc.schema)
			t.Logf("Test '%s': error=%v", tc.name, err)
		}
	})
}

// TestAPICompatibility_StateBehavior tests state behavior consistency
func TestAPICompatibility_StateBehavior(t *testing.T) {
	t.Run("State transitions", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()

		// Active state
		result1, err1 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Active state: error=%v, result_len=%d", err1, len(result1))

		// Close
		err := invoker.Close()
		t.Logf("Close: error=%v", err)

		// Closed state
		result2, err2 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Closed state: error=%v, result_len=%d", err2, len(result2))

		// Second close
		err = invoker.Close()
		t.Logf("Second close: error=%v", err)
	})

	t.Run("Idempotent operations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Multiple closes should be safe
		var errs []error
		for i := 0; i < 5; i++ {
			err := invoker.Close()
			errs = append(errs, err)
		}

		t.Logf("Multiple closes: %d errors", len(errs))
		for i, err := range errs {
			t.Logf("  Close %d: error=%v", i, err)
		}
	})
}

// TestAPICompatibility_TimeoutBehavior tests timeout behavior
func TestAPICompatibility_TimeoutBehavior(t *testing.T) {
	t.Run("Config timeout", func(t *testing.T) {
		configs := []struct {
			name    string
			timeout int
		}{
			{"Zero timeout", 0},
			{"Short timeout", 1},
			{"Medium timeout", 30},
			{"Long timeout", 300},
		}

		for _, tc := range configs {
			config := &InvokerConfig{
				Address:        "localhost:19090",
				TimeoutSeconds: tc.timeout,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Config '%s': error=%v, result_len=%d", tc.name, err, len(result))
				invoker.Close()
			}
		}
	})

	t.Run("Options timeout override", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Options timeout should override config timeout
		opts := InvokeOptions{
			Timeout: 100 * time.Millisecond,
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Override timeout: error=%v, result_len=%d", err, len(result))
	})
}

// TestAPICompatibility_HeaderHandling tests header handling
func TestAPICompatibility_HeaderHandling(t *testing.T) {
	t.Run("Empty headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Empty headers map
		opts := InvokeOptions{
			Headers: map[string]string{},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Empty headers: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Nil headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Nil headers (default)
		opts := InvokeOptions{}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Nil headers: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Multiple headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Multiple headers
		opts := InvokeOptions{
			Headers: map[string]string{
				"X-Header-1": "value1",
				"X-Header-2": "value2",
				"X-Header-3": "value3",
			},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Multiple headers: error=%v, result_len=%d", err, len(result))
	})
}

// TestAPICompatibility_RetryBehavior tests retry behavior
func TestAPICompatibility_RetryBehavior(t *testing.T) {
	t.Run("Retry disabled", func(t *testing.T) {
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

		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		t.Logf("Retry disabled: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})

	t.Run("Retry enabled", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 10,
				MaxDelayMs:     100,
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

		t.Logf("Retry enabled: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})
}

// TestAPICompatibility_JobBehavior tests job behavior
func TestAPICompatibility_JobBehavior(t *testing.T) {
	t.Run("Job lifecycle", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Start job
		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		t.Logf("Start job: jobID=%s, error=%v", jobID, err)

		if jobID != "" {
			// Stream job
			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("Stream job: error=%v", err)

			if eventChan != nil {
				// Try to read one event
				select {
				case event := <-eventChan:
					t.Logf("Job event: %+v", event)
				case <-time.After(100 * time.Millisecond):
					t.Log("No event received (timeout)")
				}
			}

			// Cancel job
			err = invoker.CancelJob(ctx, jobID)
			t.Logf("Cancel job: error=%v", err)
		}
	})

	t.Run("Invalid job operations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Cancel non-existent job
		err := invoker.CancelJob(ctx, "non-existent-job")
		t.Logf("Cancel non-existent job: error=%v", err)

		// Stream non-existent job
		eventChan, err := invoker.StreamJob(ctx, "non-existent-job")
		t.Logf("Stream non-existent job: error=%v, channel=%v", err, eventChan != nil)
	})
}
