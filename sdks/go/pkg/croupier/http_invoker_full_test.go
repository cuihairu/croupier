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

// TestHTTPInvoker_advancedScenarios tests advanced HTTP invoker scenarios
func TestHTTPInvoker_advancedScenarios(t *testing.T) {
	t.Run("Invoke with very large headers", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		headers := make(map[string]string)
		for i := 0; i < 50; i++ {
			headers[fmt.Sprintf("X-Header-%d", i)] = fmt.Sprintf("Value-%d-with-some-longer-text", i)
		}

		options := InvokeOptions{
			Headers: headers,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with 50 headers error: %v", err)
	})

	t.Run("Invoke with header value containing special characters", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		headers := map[string]string{
			"X-Special":     "value with \n newlines \t tabs",
			"X-Unicode":     "世界 🌍 Привет",
			"X-URL-Encoded": "name=value&foo=bar",
			"X-JSON":        `{"key":"value"}`,
		}

		options := InvokeOptions{
			Headers: headers,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with special header values error: %v", err)
	})

	t.Run("Invoke with very long idempotency key", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		longKey := string(make([]byte, 1000))
		for i := range longKey {
			longKey = "a" + longKey[:i]
		}

		options := InvokeOptions{
			IdempotencyKey: longKey[:100],
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with long idempotency key error: %v", err)
	})

	t.Run("Invoke with various timeouts", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		timeouts := []time.Duration{
			1 * time.Nanosecond,
			1 * time.Microsecond,
			1 * time.Millisecond,
			100 * time.Millisecond,
			1 * time.Second,
			30 * time.Second,
			5 * time.Minute,
		}

		for _, timeout := range timeouts {
			options := InvokeOptions{
				Timeout: timeout,
			}

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", options)
			t.Logf("Invoke with timeout=%v error: %v", timeout, err)
		}
	})
}

// TestHTTPInvoker_payloadVariations tests payload variations
func TestHTTPInvoker_payloadVariations(t *testing.T) {
	t.Run("Invoke with JSON array payload", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		payloads := []string{
			`[]`,
			`[1, 2, 3]`,
			`["a", "b", "c"]`,
			`[{"key": "value"}, {"key2": "value2"}]`,
			`[null, true, false, "string", 123]`,
		}

		ctx := context.Background()
		for _, payload := range payloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with array payload '%s' error: %v", payload, err)
		}
	})

	t.Run("Invoke with nested JSON payload", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		payloads := []string{
			`{"nested": {"deep": {"deeper": "value"}}}`,
			`{"array": [{"a": 1}, {"b": 2}]}`,
			`{"mixed": {"arr": [1, 2, 3], "obj": {"x": 1}}}`,
			`{"empty": {}, "null": null, "bool": true}`,
		}

		ctx := context.Background()
		for _, payload := range payloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with nested payload error: %v", err)
		}
	})

	t.Run("Invoke with malformed JSON", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		malformedPayloads := []string{
			`{`,
			`}`,
			`{"unclosed": `,
			`{"missing": value}`,
			`{"extra": "comma",}`,
			`'single quotes'`,
			`undefined`,
		}

		ctx := context.Background()
		for _, payload := range malformedPayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with malformed JSON '%s' error: %v", payload, err)
		}
	})

	t.Run("Invoke with numeric JSON values", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		payloads := []string{
			`{"int": 42}`,
			`{"float": 3.14}`,
			`{"negative": -123}`,
			`{"zero": 0}`,
			`{"large": 9999999999}`,
			`{"scientific": 1.23e10}`,
			`{"exp": -2.5e-3}`,
		}

		ctx := context.Background()
		for _, payload := range payloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with numeric payload '%s' error: %v", payload, err)
		}
	})
}

// TestHTTPInvoker_functionIDVariations tests function ID variations
func TestHTTPInvoker_functionIDVariations(t *testing.T) {
	t.Run("Invoke with various function IDs", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		functionIDs := []string{
			"simple",
			"dot.separated.id",
			"with_underscore",
			"with-dash",
			"with:colon",
			"with/slash",
			"MixedCase",
			"with123numbers",
			"UPPERCASE",
			"withCamelCase",
			"snake_case",
			"kebab-case",
		}

		ctx := context.Background()
		for _, funcID := range functionIDs {
			_, err := invoker.Invoke(ctx, funcID, "{}", InvokeOptions{})
			t.Logf("Invoke with functionID='%s' error: %v", funcID, err)
		}
	})

	t.Run("Invoke with very long function ID", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		longID := "a.very.long.function.name." +
			"that.keeps.going.on.and.on." +
			"with.many.dots.and.segments"

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, longID, "{}", InvokeOptions{})
		t.Logf("Invoke with long functionID error: %v", err)
	})

	t.Run("Invoke with empty and whitespace function IDs", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ids := []string{
			"",
			" ",
			"  ",
			"\t",
			"\n",
		}

		ctx := context.Background()
		for _, funcID := range ids {
			_, err := invoker.Invoke(ctx, funcID, "{}", InvokeOptions{})
			t.Logf("Invoke with whitespace functionID='%s' error: %v", funcID, err)
		}
	})
}

// TestHTTPInvoker_contextScenarios tests context scenarios
func TestHTTPInvoker_contextScenarios(t *testing.T) {
	t.Run("Invoke with deadline exceeded context", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		deadline := time.Now().Add(-1 * time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with exceeded deadline error: %v", err)
	})

	t.Run("Invoke with very short deadline", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		deadline := time.Now().Add(1 * time.Microsecond)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		time.Sleep(10 * time.Millisecond)

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with short deadline error: %v", err)
	})

	t.Run("Invoke with context chain", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		// Create a chain of contexts
		ctx1 := context.Background()
		ctx2 := context.WithValue(ctx1, "key1", "value1")
		ctx3, cancel := context.WithTimeout(ctx2, 1*time.Hour)
		defer cancel()

		_, err := invoker.Invoke(ctx3, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with context chain error: %v", err)

		if ctx3.Value("key1") == "value1" {
			t.Log("Context values preserved through chain")
		}
	})
}

// TestHTTPInvoker_schemaOperations tests schema operations with HTTP invoker
func TestHTTPInvoker_schemaOperations(t *testing.T) {
	t.Run("SetSchema with various schema types", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		schemas := []struct {
			name   string
			schema map[string]interface{}
		}{
			{
				"empty schema",
				map[string]interface{}{},
			},
			{
				"string schema",
				map[string]interface{}{
					"type": "string",
				},
			},
			{
				"number schema",
				map[string]interface{}{
					"type": "number",
				},
			},
			{
				"boolean schema",
				map[string]interface{}{
					"type": "boolean",
				},
			},
			{
				"array schema",
				map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "string"},
				},
			},
			{
				"object schema",
				map[string]interface{}{
					"type":       "object",
					"properties": map[string]string{"name": "string"},
				},
			},
		}

		for _, tc := range schemas {
			err := invoker.SetSchema("test.func", tc.schema)
			t.Logf("SetSchema with %s error: %v", tc.name, err)
		}
	})

	t.Run("SetSchema for multiple functions", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		functions := []string{
			"func1",
			"func2",
			"func3",
			"func4",
			"func5",
		}

		schema := map[string]interface{}{
			"type": "object",
		}

		for _, funcID := range functions {
			err := invoker.SetSchema(funcID, schema)
			t.Logf("SetSchema for '%s' error: %v", funcID, err)
		}
	})

	t.Run("SetSchema with complex validation rules", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"email": map[string]interface{}{
					"type":   "string",
					"format": "email",
				},
				"age": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
					"maximum": 150,
				},
				"url": map[string]interface{}{
					"type":   "string",
					"format": "uri",
				},
			},
			"required": []string{"email", "age"},
		}

		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with complex validation error: %v", err)
	})
}

// TestHTTPInvoker_retryCombinations tests retry combinations
func TestHTTPInvoker_retryCombinations(t *testing.T) {
	t.Run("Invoke with retry and custom options retry", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 2,
			},
		})

		options := InvokeOptions{
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with both config and options retry error: %v", err)
	})

	t.Run("Invoke with retry disabled in options", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		})

		options := InvokeOptions{
			Retry: &RetryConfig{
				Enabled: false,
			},
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with retry disabled in options error: %v", err)
	})
}

// TestHTTPInvoker_concurrentInvokes tests concurrent Invoke operations
func TestHTTPInvoker_concurrentInvokes(t *testing.T) {
	t.Run("concurrent Invoke with same invoker", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		const numGoroutines = 50
		errors := make([]error, numGoroutines)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				ctx := context.Background()
				_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})

				mu.Lock()
				errors[idx] = err
				mu.Unlock()
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		errorCount := 0
		for _, err := range errors {
			if err != nil {
				errorCount++
			}
		}

		t.Logf("Concurrent invokes: %d/%d had errors", errorCount, numGoroutines)
	})

	t.Run("concurrent Schema operations", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		const numGoroutines = 20
		errors := make([]error, numGoroutines)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				schema := map[string]interface{}{
					"type": "object",
				}
				err := invoker.SetSchema("test.func", schema)

				mu.Lock()
				errors[idx] = err
				mu.Unlock()
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		errorCount := 0
		for _, err := range errors {
			if err != nil {
				errorCount++
			}
		}

		t.Logf("Concurrent SetSchema: %d/%d had errors", errorCount, numGoroutines)
	})
}

// TestHTTPInvoker_addressVariations tests address variations
func TestHTTPInvoker_addressVariations(t *testing.T) {
	t.Run("Invoke with various HTTP addresses", func(t *testing.T) {
		addresses := []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
			"http://0.0.0.0:8080",
			// "http://192.168.1.1:8080", // Removed: causes long timeout
			"http://example.com:8080",
			"https://localhost:8443",
			"http://localhost:8080/api",
			"http://localhost:8080/api/v1",
		}

		for _, addr := range addresses {
			invoker := NewHTTPInvoker(&InvokerConfig{
				Address: addr,
			})

			// Add timeout to prevent long waits
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with address='%s' error: %v", addr, err)
		}
	})

	t.Run("Invoke with IPv6 addresses", func(t *testing.T) {
		addresses := []string{
			"http://[::1]:8080",
			"http://[2001:db8::1]:8080",
			"http://[fe80::1]:8080",
		}

		for _, addr := range addresses {
			invoker := NewHTTPInvoker(&InvokerConfig{
				Address: addr,
			})

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with IPv6='%s' error: %v", addr, err)
		}
	})
}

// TestHTTPInvoker_configurationCombinations tests configuration combinations
func TestHTTPInvoker_configurationCombinations(t *testing.T) {
	t.Run("Invoker with all configuration options", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:        "http://localhost:8080",
			TimeoutSeconds: 30,
			Insecure:       true,
			CAFile:         "/path/to/ca.crt",
			CertFile:       "/path/to/cert.pem",
			KeyFile:        "/path/to/key.pem",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
			Reconnect: &ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    1000,
				MaxDelayMs:        10000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.2,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with full config error: %v", err)
	})

	t.Run("Invoker with minimal configuration", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with minimal config error: %v", err)
	})
}

// TestHTTPInvoker_errorPaths tests error paths
func TestHTTPInvoker_errorPaths(t *testing.T) {
	t.Run("Invoke after Close", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		// Close the invoker
		err := invoker.Close()
		t.Logf("Close error: %v", err)

		// Try to invoke after close
		ctx := context.Background()
		_, err = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke after Close error: %v", err)
	})

	t.Run("multiple Close calls", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		// Close multiple times
		for i := 0; i < 3; i++ {
			err := invoker.Close()
			t.Logf("Close call %d error: %v", i+1, err)
		}
	})

	t.Run("StartJob with invalid options", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		options := InvokeOptions{
			Timeout: -1 * time.Second, // Invalid timeout
		}

		ctx := context.Background()
		jobID, err := invoker.StartJob(ctx, "test.func", "{}", options)
		t.Logf("StartJob with invalid timeout error: %v, jobID: %s", err, jobID)
	})
}

// TestHTTPInvoker_validationCoverage tests validation functions
func TestHTTPInvoker_validationCoverage(t *testing.T) {
	t.Run("validatePayload with various valid payloads", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		validPayloads := []string{
			`{}`,
			`{"key": "value"}`,
			`{"number": 42}`,
			`{"float": 3.14}`,
			`{"bool": true}`,
			`{"null": null}`,
			`{"array": []}`,
			`{"object": {}}`,
			`{"string": ""}`,
		}

		for _, payload := range validPayloads {
			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with valid payload '%s' error: %v", payload, err)
		}
	})
}
