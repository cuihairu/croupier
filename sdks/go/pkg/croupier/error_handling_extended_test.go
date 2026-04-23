// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// TestErrorHandling_NetworkErrors tests network-related error scenarios
func TestErrorHandling_NetworkErrors(t *testing.T) {
	t.Run("Connection refused errors", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090", // Non-existent server
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		if err != nil {
			t.Logf("Expected connection error: %v", err)
			// Check if it's a connection error
			var netErr net.Error
			if errors.As(err, &netErr) {
				t.Logf("Confirmed network error: %v", netErr)
			}
		} else {
			t.Log("Unexpected success with non-existent server")
			t.Logf("Result: %s", string(result))
		}
	})

	t.Run("Timeout errors", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:9999",
			TimeoutSeconds: 1,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()

		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		duration := time.Since(start)

		if err != nil {
			t.Logf("Timeout error after %v: %v", duration, err)
		} else {
			t.Logf("Unexpected success in %v", duration)
			t.Logf("Result: %s", string(result))
		}
	})

	t.Run("Invalid address formats", func(t *testing.T) {
		invalidAddresses := []string{
			"",
			"invalid-address",
			"http://",
			"localhost:abc",
			"99999", // Port too high
			":8080",  // Missing host
		}

		for _, addr := range invalidAddresses {
			config := &InvokerConfig{
				Address: addr,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Unexpected success with invalid address: '%s'", addr)
				invoker.Close()
			} else {
				t.Logf("Expected failure with invalid address: '%s'", addr)
			}
		}
	})
}

// TestErrorHandling_PayloadErrors tests payload-related error scenarios
func TestErrorHandling_PayloadErrors(t *testing.T) {
	t.Run("Malformed JSON payloads", func(t *testing.T) {
		malformedPayloads := []string{
			"{",
			"}",
			"{{",
			"}}",
			"invalid",
			"{invalid}",
			"{'key': 'value'}", // Single quotes
			"{key: value}",      // Unquoted keys
			"null",
			"undefined",
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		for i, payload := range malformedPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Payload %d ('%s'): error=%v, result_len=%d",
				i, payload, err, len(result))
		}
	})

	t.Run("Empty and nil payloads", func(t *testing.T) {
		payloads := []string{
			"",
			"{}",
			"[]",
			"null",
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		for i, payload := range payloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Payload %d ('%s'): error=%v, result_len=%d",
				i, payload, err, len(result))
		}
	})

	t.Run("Very large payloads", func(t *testing.T) {
		sizes := []int{
			1024,        // 1 KB
			10 * 1024,   // 10 KB
			100 * 1024,  // 100 KB
			1024 * 1024, // 1 MB
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		for _, size := range sizes {
			largePayload := fmt.Sprintf(`{"data":"%s"}`, string(make([]byte, size)))
			start := time.Now()

			result, err := invoker.Invoke(ctx, "test.function", largePayload, InvokeOptions{})

			duration := time.Since(start)
			t.Logf("Large payload (%d bytes): error=%v, duration=%v, result_len=%d",
				size, err, duration, len(result))
		}
	})
}

// TestErrorHandling_ContextErrors tests context-related error scenarios
func TestErrorHandling_ContextErrors(t *testing.T) {
	t.Run("Already cancelled context", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before using

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Cancelled context: error=%v, result_len=%d", err, len(result))
		if err != nil {
			t.Logf("Error type: %T", err)
		}
	})

	t.Run("Context with very short deadline", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		deadlines := []time.Duration{
			time.Nanosecond,
			time.Microsecond,
			time.Millisecond,
		}

		for _, deadline := range deadlines {
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()

			time.Sleep(deadline * 2) // Ensure deadline passes

			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

			t.Logf("Deadline %v: error=%v, result_len=%d", deadline, err, len(result))
		}
	})

	t.Run("Context cancellation during invocation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel in a goroutine after a short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Cancellation during invoke: error=%v, result_len=%d", err, len(result))
	})
}

// TestErrorHandling_InvokerErrors tests invoker-specific error scenarios
func TestErrorHandling_InvokerErrors(t *testing.T) {
	t.Run("Operations on closed invoker", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		// Close the invoker
		err := invoker.Close()
		t.Logf("First close: %v", err)

		ctx := context.Background()

		// Try to invoke on closed invoker
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Invoke after close: error=%v, result_len=%d", err, len(result))

		// Try to start job on closed invoker
		jobID, err := invoker.StartJob(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("StartJob after close: error=%v, jobID=%s", err, jobID)

		// Close again
		err = invoker.Close()
		t.Logf("Second close: %v", err)
	})

	t.Run("Invalid function IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		invalidFunctionIDs := []string{
			"",
			" ",
			"invalid function id",
			"function/with/slashes",
			"function\\with\\backslashes",
			"function\nwith\nnewlines",
			"function\twith\ttabs",
			string(make([]byte, 10000)), // Very long
		}

		ctx := context.Background()

		for _, funcID := range invalidFunctionIDs {
			result, err := invoker.Invoke(ctx, funcID, "{}", InvokeOptions{})
			t.Logf("Invalid function ID (len=%d): error=%v, result_len=%d",
				len(funcID), err, len(result))
		}
	})
}

// TestErrorHandling_RetryErrors tests retry-related error scenarios
func TestErrorHandling_RetryErrors(t *testing.T) {
	t.Run("Retry with all attempts failing", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090", // Non-existent server
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    5,
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
		t.Logf("All retries failed after %v: error=%v, result_len=%d",
			duration, err, len(result))
	})

	t.Run("Retry with zero attempts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 0, // Should fail immediately
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
		t.Logf("Zero retry attempts took %v: error=%v, result_len=%d",
			duration, err, len(result))
	})

	t.Run("Retry with negative delays", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: -100,
				MaxDelayMs:     -50,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Negative delays: error=%v, result_len=%d", err, len(result))
	})
}

// TestErrorHandling_JobErrors tests job-related error scenarios
func TestErrorHandling_JobErrors(t *testing.T) {
	t.Run("Invalid job IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		invalidJobIDs := []string{
			"",
			"non-existent-job",
			"invalid/job/id",
			string(make([]byte, 1000)),
		}

		ctx := context.Background()

		for _, jobID := range invalidJobIDs {
			// Try to stream job events
			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("StreamJob for invalid ID (len=%d): error=%v, channel=%v",
				len(jobID), err, eventChan != nil)

			// Try to cancel job
			err = invoker.CancelJob(ctx, jobID)
			t.Logf("CancelJob for invalid ID (len=%d): error=%v",
				len(jobID), err)
		}
	})

	t.Run("Job operations on closed invoker", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		invoker.Close()

		ctx := context.Background()
		jobID := "test-job-123"

		// Try various job operations on closed invoker
		eventChan, err := invoker.StreamJob(ctx, jobID)
		t.Logf("StreamJob on closed invoker: error=%v, channel=%v", err, eventChan != nil)

		err = invoker.CancelJob(ctx, jobID)
		t.Logf("CancelJob on closed invoker: error=%v", err)
	})
}

// TestErrorHandling_IOErrors tests I/O-related error scenarios
func TestErrorHandling_IOErrors(t *testing.T) {
	t.Run("Response with unexpected EOF", func(t *testing.T) {
		// This test checks handling of incomplete responses
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Make a request that might get interrupted
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				t.Logf("Got EOF error as expected: %v", err)
			} else {
				t.Logf("Got non-EOF error: %v (type: %T)", err, err)
			}
		} else {
			t.Logf("Request succeeded, result length: %d", len(result))
		}
	})

	t.Run("Operations with very short timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 0,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Zero timeout config: error=%v, result_len=%d", err, len(result))
	})
}

// TestErrorHandling_ConfigurationErrors tests configuration-related error scenarios
func TestErrorHandling_ConfigurationErrors(t *testing.T) {
	t.Run("Invalid TLS configurations", func(t *testing.T) {
		invalidTLSConfigs := []*InvokerConfig{
			{
				Address:  "localhost:19090",
				Insecure: false,
				CAFile:   "/nonexistent/ca.crt",
			},
			{
				Address:  "localhost:19090",
				Insecure: false,
				CertFile: "/nonexistent/cert.pem",
				KeyFile:  "/nonexistent/key.pem",
			},
			{
				Address:  "localhost:19090",
				Insecure: false,
				CAFile:   "/nonexistent/ca.crt",
				CertFile: "/nonexistent/cert.pem",
				KeyFile:  "/nonexistent/key.pem",
			},
		}

		for i, config := range invalidTLSConfigs {
			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Config %d: Unexpected success with invalid TLS config", i)
				invoker.Close()
			} else {
				t.Logf("Config %d: Expected failure with invalid TLS config", i)
			}
		}
	})

	t.Run("Invalid timeout values", func(t *testing.T) {
		invalidTimeouts := []int{
			-1,
			-100,
			0,
			1000000, // Very large timeout
		}

		for _, timeout := range invalidTimeouts {
			config := &InvokerConfig{
				Address:        "localhost:19090",
				TimeoutSeconds: timeout,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Timeout %d: Invoker created", timeout)
				invoker.Close()
			} else {
				t.Logf("Timeout %d: Invoker creation failed", timeout)
			}
		}
	})
}
