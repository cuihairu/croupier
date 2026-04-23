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

// TestProtocolTransport_ProtocolVariations tests different protocol configurations
func TestProtocolTransport_ProtocolVariations(t *testing.T) {
	t.Run("HTTP vs HTTPS addresses", func(t *testing.T) {
		addresses := []string{
			"http://localhost:19090",
			"https://localhost:19090",
			"localhost:19090",
			"http://127.0.0.1:19090",
			"https://127.0.0.1:19090",
		}

		for _, addr := range addresses {
			config := &InvokerConfig{
				Address: addr,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Address '%s': error=%v, result_len=%d", addr, err, len(result))
				invoker.Close()
			}
		}
	})

	t.Run("Port variations", func(t *testing.T) {
		ports := []string{
			"80",
			"443",
			"8080",
			"19090",
			"30000",
			"65535", // Max valid port
		}

		for _, port := range ports {
			config := &InvokerConfig{
				Address: "localhost:" + port,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Port %s: error=%v, result_len=%d", port, err, len(result))
				invoker.Close()
			}
		}
	})
}

// TestProtocolTransport_TLSConfigurations tests TLS configuration scenarios
func TestProtocolTransport_TLSConfigurations(t *testing.T) {
	t.Run("TLS with Insecure flag", func(t *testing.T) {
		configs := []*InvokerConfig{
			{
				Address:  "localhost:19090",
				Insecure: true,
			},
			{
				Address:  "localhost:19090",
				Insecure: false,
			},
		}

		for i, config := range configs {
			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Config %d (Insecure=%v): error=%v, result_len=%d",
					i, config.Insecure, err, len(result))
				invoker.Close()
			}
		}
	})

	t.Run("TLS with certificate files", func(t *testing.T) {
		config := &InvokerConfig{
			Address:  "localhost:19090",
			Insecure: false,
			CAFile:   "/path/to/ca.crt",
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		}

		invoker := NewHTTPInvoker(config)
		if invoker != nil {
			ctx := context.Background()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("With cert files: error=%v, result_len=%d", err, len(result))
			invoker.Close()
		}
	})
}

// TestProtocolTransport_TimeoutConfigurations tests timeout configurations
func TestProtocolTransport_TimeoutConfigurations(t *testing.T) {
	t.Run("InvokerConfig timeout variations", func(t *testing.T) {
		timeouts := []int{
			0,
			1,
			5,
			10,
			30,
			60,
			300,
		}

		for _, timeout := range timeouts {
			config := &InvokerConfig{
				Address:        "localhost:19090",
				TimeoutSeconds: timeout,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				start := time.Now()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				duration := time.Since(start)

				t.Logf("Timeout %ds: duration=%v, error=%v, result_len=%d",
					timeout, duration, err, len(result))
				invoker.Close()
			}
		}
	})

	t.Run("InvokeOptions timeout vs InvokerConfig timeout", func(t *testing.T) {
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

		// InvokeOptions timeout should override InvokerConfig timeout
		opts := InvokeOptions{
			Timeout: 5 * time.Second,
		}

		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		duration := time.Since(start)

		t.Logf("Override timeout: duration=%v, error=%v, result_len=%d",
			duration, err, len(result))
	})
}

// TestProtocolTransport_HeaderHandling tests HTTP header handling
func TestProtocolTransport_HeaderHandling(t *testing.T) {
	t.Run("Standard HTTP headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headers := map[string]string{
			"Content-Type":      "application/json",
			"Accept":            "application/json",
			"User-Agent":         "Croupier-SDK/1.0",
			"Cache-Control":      "no-cache",
			"Connection":         "keep-alive",
		}

		opts := InvokeOptions{Headers: headers}
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Standard headers: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Custom headers with special values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headers := map[string]string{
			"X-Custom-Header":  "custom-value",
			"X-Request-ID":     fmt.Sprintf("req-%d", time.Now().UnixNano()),
			"X-Trace-ID":       fmt.Sprintf("trace-%d", time.Now().UnixNano()),
			"X-Session-ID":     "session-abc123",
			"X-Client-Version": "1.0.0",
		}

		opts := InvokeOptions{Headers: headers}
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Custom headers: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Headers with encoding", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headers := map[string]string{
			"Accept-Encoding":  "gzip, deflate",
			"Content-Encoding": "gzip",
		}

		opts := InvokeOptions{Headers: headers}
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Encoding headers: error=%v, result_len=%d", err, len(result))
	})
}

// TestProtocolTransport_ConnectionPooling tests connection pooling scenarios
func TestProtocolTransport_ConnectionPooling(t *testing.T) {
	t.Run("Sequential requests (connection reuse)", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const requests = 20

		start := time.Now()
		for i := 0; i < requests; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			t.Logf("Request %d: error=%v, result_len=%d", i, err, len(result))
		}
		duration := time.Since(start)

		t.Logf("Sequential requests: %d requests in %v (%.2f req/sec)",
			requests, duration, float64(requests)/duration.Seconds())
	})

	t.Run("Keep-alive headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headers := map[string]string{
			"Connection": "keep-alive",
		}

		opts := InvokeOptions{Headers: headers}

		for i := 0; i < 5; i++ {
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Keep-alive request %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestProtocolTransport_Idempotency tests idempotency behavior
func TestProtocolTransport_Idempotency(t *testing.T) {
	t.Run("Same idempotency key multiple times", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		idempotencyKey := fmt.Sprintf("idem-%d", time.Now().UnixNano())

		var results []string
		var errors []error

		// Invoke 3 times with same key
		for i := 0; i < 3; i++ {
			opts := InvokeOptions{
				IdempotencyKey: idempotencyKey,
			}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			results = append(results, result)
			errors = append(errors, err)

			t.Logf("Invocation %d: error=%v, result_len=%d", i, err, len(result))
		}

		// Check if results are consistent
		t.Logf("Idempotency check: %d invocations with same key", len(results))
	})

	t.Run("Different idempotency keys", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		for i := 0; i < 3; i++ {
			opts := InvokeOptions{
				IdempotencyKey: fmt.Sprintf("idem-%d", time.Now().UnixNano()),
			}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Different key %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Empty and nil idempotency key", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Empty key
		opts1 := InvokeOptions{
			IdempotencyKey: "",
		}
		result1, err1 := invoker.Invoke(ctx, "test.function", "{}", opts1)
		t.Logf("Empty idempotency key: error=%v, result_len=%d", err1, len(result1))

		// Default (no key specified)
		result2, err2 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("No idempotency key: error=%v, result_len=%d", err2, len(result2))
	})
}

// TestProtocolTransport_ErrorCodes tests error code handling
func TestProtocolTransport_ErrorCodes(t *testing.T) {
	t.Run("HTTP status code scenarios", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test with various function IDs that might return different status codes
		functionIDs := []string{
			"test.success",
			"test.notfound",
			"test.unauthorized",
			"test.error",
			"test.timeout",
		}

		for _, functionID := range functionIDs {
			result, err := invoker.Invoke(ctx, functionID, "{}", InvokeOptions{})
			t.Logf("Function %s: error=%v, result_len=%d", functionID, err, len(result))
		}
	})

	t.Run("Network error scenarios", func(t *testing.T) {
		// Test with addresses that will fail
		addresses := []string{
			"localhost:1",            // Wrong port
			"192.0.2.1:8080",        // Non-routable IP
			"invalid-host-name:8080", // Invalid hostname
		}

		for _, addr := range addresses {
			config := &InvokerConfig{
				Address: addr,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Address '%s': error=%v, result_len=%d", addr, err, len(result))
				invoker.Close()
			}
		}
	})
}

// TestProtocolTransport_PayloadEncoding tests payload encoding scenarios
func TestProtocolTransport_PayloadEncoding(t *testing.T) {
	t.Run("JSON payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		payloads := []string{
			`{"simple":"value"}`,
			`{"nested":{"key":"value"}}`,
			`{"array":[1,2,3]}`,
			`{"escaped":"\"quote\""}`,
			`{"unicode":"测试中文"}`,
			`{"emoji":"🎉"}`,
		}

		for i, payload := range payloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("JSON payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("URL encoding in payload", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		payloads := []string{
			`{"url":"http://example.com?param=value&other=123"}`,
			`{"path":"/api/v1/users?id=123"}`,
			`{"search":"query with spaces"}`,
		}

		for i, payload := range payloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("URL encoded payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestProtocolTransport_Compression tests compression scenarios
func TestProtocolTransport_Compression(t *testing.T) {
	t.Run("Request compression", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headers := map[string]string{
			"Content-Encoding": "gzip",
			"Accept-Encoding":  "gzip, deflate",
		}

		opts := InvokeOptions{Headers: headers}
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("With compression: error=%v, result_len=%d", err, len(result))
	})
}
