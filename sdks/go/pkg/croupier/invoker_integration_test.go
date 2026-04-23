// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestInvoker_integration_lifecycle tests invoker lifecycle
func TestInvoker_integration_lifecycle(t *testing.T) {
	t.Run("Create and Close invoker", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		err := invoker.Close()
		t.Logf("Close error: %v", err)
	})

	t.Run("Multiple Close calls", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic: %v", r)
			}
		}()

		err1 := invoker.Close()
		err2 := invoker.Close()
		err3 := invoker.Close()

		t.Logf("Multiple Close errors: %v, %v, %v", err1, err2, err3)
	})

	t.Run("Close after failed Invoke", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		ctx := context.Background()
		_, _ = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})

		err := invoker.Close()
		t.Logf("Close after failed Invoke error: %v", err)
	})
}

// TestInvoker_concurrent_invocations tests concurrent invocations
func TestInvoker_concurrent_invocations(t *testing.T) {
	t.Run("Concurrent Invoke operations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 50
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx := context.Background()
				_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
				_ = err
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		t.Logf("Concurrent Invoke: %d goroutines in %v", numGoroutines, elapsed)
	})

	t.Run("Concurrent StartJob operations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 30
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx := context.Background()
				jobID, _ := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
				_ = jobID
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent StartJob: %d goroutines", numGoroutines)
	})

	t.Run("Concurrent mixed operations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 40
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx := context.Background()

				switch idx % 3 {
				case 0:
					_, _ = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
				case 1:
					jobID, _ := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
					_ = jobID
				case 2:
					_ = invoker.SetSchema("test.func", map[string]interface{}{"type": "object"})
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent mixed operations: %d goroutines", numGoroutines)
	})
}

// TestInvoker_context_variations tests various context scenarios
func TestInvoker_context_variations(t *testing.T) {
	t.Run("Invoke with cancelled context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with cancelled context error: %v", err)
	})

	t.Run("Invoke with timeout context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout expires

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with expired timeout error: %v", err)
	})

	t.Run("Invoke with context values", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.WithValue(context.Background(), "key", "value")
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with context values error: %v", err)
	})

	t.Run("Invoke with deadline in the past", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		past := time.Now().Add(-1 * time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), past)
		defer cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with past deadline error: %v", err)
	})
}

// TestInvoker_payload_variations tests various payload scenarios
func TestInvoker_payload_variations(t *testing.T) {
	t.Run("Invoke with empty payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "", InvokeOptions{})
		t.Logf("Invoke with empty payload error: %v", err)
	})

	t.Run("Invoke with nil payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", string([]byte{}), InvokeOptions{})
		t.Logf("Invoke with nil payload error: %v", err)
	})

	t.Run("Invoke with large payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		// Create a large JSON payload
		largePayload := `{"data":"` + string(make([]byte, 10000)) + `"}`
		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", largePayload, InvokeOptions{})
		t.Logf("Invoke with large payload error: %v", err)
	})

	t.Run("Invoke with various JSON payloads", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		payloads := []string{
			`{}`,
			`{"key":"value"}`,
			`{"num":42,"bool":true,"null":null}`,
			`{"array":[1,2,3]}`,
			`{"nested":{"key":"value"}}`,
			`{"unicode":"世界🌍"}`,
		}

		ctx := context.Background()
		for i, payload := range payloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with payload %d error: %v", i, err)
		}
	})
}

// TestInvoker_function_id_variations tests various function ID formats
func TestInvoker_function_id_variations(t *testing.T) {
	t.Run("Invoke with various function IDs", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		functionIDs := []string{
			"simple",
			"with.dash",
			"with_underscore",
			"with:colon",
			"with/slash",
			"MixedCase",
			"with123numbers",
			"very.long.function.name.with.many.segments",
		}

		ctx := context.Background()
		for _, funcID := range functionIDs {
			_, err := invoker.Invoke(ctx, funcID, "{}", InvokeOptions{})
			t.Logf("Invoke with function ID '%s' error: %v", funcID, err)
		}
	})
}

// TestInvoker_options_combinations tests various option combinations
func TestInvoker_options_combinations(t *testing.T) {
	t.Run("Invoke with IdempotencyKey variations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		keys := []string{
			"",
			"key-123",
			"uuid-like-123e4567-e89b-12d3-a456-426614174000",
			"very.long.idempotency.key.with.many.segments",
		}

		ctx := context.Background()
		for _, key := range keys {
			opts := InvokeOptions{
				IdempotencyKey: key,
			}
			_, err := invoker.Invoke(ctx, "test.func", "{}", opts)
			t.Logf("Invoke with IdempotencyKey '%s' error: %v", key, err)
		}
	})

	t.Run("Invoke with Timeout variations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		timeouts := []time.Duration{
			0,
			1 * time.Nanosecond,
			1 * time.Microsecond,
			10 * time.Millisecond,
			1 * time.Second,
			30 * time.Second,
		}

		ctx := context.Background()
		for _, timeout := range timeouts {
			opts := InvokeOptions{
				Timeout: timeout,
			}
			_, err := invoker.Invoke(ctx, "test.func", "{}", opts)
			t.Logf("Invoke with timeout %v error: %v", timeout, err)
		}
	})

	t.Run("Invoke with Headers variations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		headerSets := []map[string]string{
			nil,
			{},
			{"X-Custom": "value"},
			{
				"X-Header1": "value1",
				"X-Header2": "value2",
				"X-Header3": "value3",
			},
			{
				"X-Unicode": "世界🌍",
				"X-Empty":   "",
			},
		}

		ctx := context.Background()
		for i, headers := range headerSets {
			opts := InvokeOptions{
				Headers: headers,
			}
			_, err := invoker.Invoke(ctx, "test.func", "{}", opts)
			t.Logf("Invoke with headers set %d error: %v", i, err)
		}
	})

	t.Run("Invoke with Retry options", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		retryConfigs := []*RetryConfig{
			nil,
			{Enabled: false},
			{Enabled: true, MaxAttempts: 1},
			{Enabled: true, MaxAttempts: 3, InitialDelayMs: 100},
			{Enabled: true, MaxAttempts: 5, InitialDelayMs: 1000, MaxDelayMs: 5000},
		}

		ctx := context.Background()
		for i, retry := range retryConfigs {
			opts := InvokeOptions{
				Retry: retry,
			}
			_, err := invoker.Invoke(ctx, "test.func", "{}", opts)
			t.Logf("Invoke with retry config %d error: %v", i, err)
		}
	})
}

// TestInvoker_rapid_operations tests rapid operation sequences
func TestInvoker_rapid_operations(t *testing.T) {
	t.Run("Rapid Invoke operations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		const iterations = 100
		start := time.Now()

		ctx := context.Background()
		for i := 0; i < iterations; i++ {
			_, _ = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Rapid Invoke: %d iterations in %v (avg %v per call)",
			iterations, elapsed, avgTime)
	})

	t.Run("Rapid Create-Invoke-Close cycles", func(t *testing.T) {
		const iterations = 20
		start := time.Now()

		for i := 0; i < iterations; i++ {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
			})

			if invoker != nil {
				ctx := context.Background()
				_, _ = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
				_ = invoker.Close()
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Rapid Create-Invoke-Close: %d cycles in %v (avg %v per cycle)",
			iterations, elapsed, avgTime)
	})
}

// TestInvoker_error_scenarios tests various error scenarios
func TestInvoker_error_scenarios(t *testing.T) {
	t.Run("Invoke after Close", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		_ = invoker.Close()

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke after Close error: %v", err)
	})

	t.Run("StartJob after Close", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		_ = invoker.Close()

		ctx := context.Background()
		jobID, err := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("StartJob after Close - jobID: %s, error: %v", jobID, err)
	})

	t.Run("SetSchema after Close", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}

		_ = invoker.Close()

		err := invoker.SetSchema("test.func", map[string]interface{}{"type": "object"})
		t.Logf("SetSchema after Close error: %v", err)
	})

	t.Run("StreamJob with invalid job ID", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		stream, err := invoker.StreamJob(ctx, "invalid-job-id")
		t.Logf("StreamJob with invalid job ID - stream: %v, error: %v", stream, err)
	})

	t.Run("CancelJob with invalid job ID", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		err := invoker.CancelJob(ctx, "invalid-job-id")
		t.Logf("CancelJob with invalid job ID error: %v", err)
	})
}

// TestInvoker_configuration_tests tests invoker with various configurations
func TestInvoker_configuration_tests(t *testing.T) {
	t.Run("Invoker with different addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:8080",
			"127.0.0.1:8080",
			"example.com:8080",
			"http://localhost:8080",
			"https://example.com:8080",
		}

		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: addr,
			})

			if invoker == nil {
				t.Errorf("Invoker with address '%s' is nil", addr)
			} else {
				_ = invoker.Close()
				t.Logf("Invoker with address '%s' created successfully", addr)
			}
		}
	})

	t.Run("Invoker with Insecure variations", func(t *testing.T) {
		insecureValues := []bool{true, false}

		for _, insecure := range insecureValues {
			invoker := NewInvoker(&InvokerConfig{
				Address:  "localhost:8080",
				Insecure: insecure,
			})

			if invoker == nil {
				t.Errorf("Invoker with Insecure=%v is nil", insecure)
			} else {
				_ = invoker.Close()
				t.Logf("Invoker with Insecure=%v created successfully", insecure)
			}
		}
	})

	t.Run("Invoker with TimeoutSeconds variations", func(t *testing.T) {
		timeouts := []int{0, 1, 10, 30, 60, 300}

		for _, timeout := range timeouts {
			invoker := NewInvoker(&InvokerConfig{
				Address:        "localhost:8080",
				TimeoutSeconds: timeout,
			})

			if invoker == nil {
				t.Errorf("Invoker with TimeoutSeconds=%d is nil", timeout)
			} else {
				_ = invoker.Close()
				t.Logf("Invoker with TimeoutSeconds=%d created successfully", timeout)
			}
		}
	})

	t.Run("Invoker with TLS configurations", func(t *testing.T) {
		configs := []struct {
			name     string
			config   InvokerConfig
		}{
			{
				"CA only",
				InvokerConfig{
					Address:  "localhost:8080",
					CAFile:   "/path/to/ca.crt",
					Insecure: false,
				},
			},
			{
				"Cert and Key",
				InvokerConfig{
					Address:  "localhost:8080",
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
					Insecure: false,
				},
			},
			{
				"Full TLS",
				InvokerConfig{
					Address:  "localhost:8080",
					CAFile:   "/path/to/ca.crt",
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
					Insecure: false,
				},
			},
		}

		for _, tc := range configs {
			invoker := NewInvoker(&tc.config)

			if invoker == nil {
				t.Errorf("Invoker with %s TLS config is nil", tc.name)
			} else {
				_ = invoker.Close()
				t.Logf("Invoker with %s TLS config created successfully", tc.name)
			}
		}
	})
}

// TestInvoker_schema_operations tests schema operations
func TestInvoker_schema_operations(t *testing.T) {
	t.Run("SetSchema with various schemas", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		schemas := []map[string]interface{}{
			{"type": "string"},
			{"type": "number"},
			{"type": "boolean"},
			{"type": "array", "items": map[string]interface{}{"type": "string"}},
			{"type": "object", "properties": map[string]interface{}{
				"name":  map[string]interface{}{"type": "string"},
				"age":   map[string]interface{}{"type": "number"},
				"active": map[string]interface{}{"type": "boolean"},
			}},
			{
				"type": "object",
				"properties": map[string]interface{}{
					"nested": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"value": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		}

		for i, schema := range schemas {
			err := invoker.SetSchema("test.func", schema)
			t.Logf("SetSchema with schema %d error: %v", i, err)
		}
	})

	t.Run("SetSchema for multiple functions", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		if invoker == nil {
			t.Fatal("NewInvoker returned nil")
		}
		defer invoker.Close()

		functions := []string{
			"func1",
			"func2",
			"func3",
		}

		schema := map[string]interface{}{"type": "object"}

		for _, fn := range functions {
			err := invoker.SetSchema(fn, schema)
			t.Logf("SetSchema for function '%s' error: %v", fn, err)
		}
	})
}

// TestInvoker_multiple_invokers tests multiple invoker instances
func TestInvoker_multiple_invokers(t *testing.T) {
	t.Run("Multiple invokers with same config", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		const numInvokers = 10
		invokers := make([]Invoker, numInvokers)

		for i := 0; i < numInvokers; i++ {
			invokers[i] = NewInvoker(config)
		}

		// Close all
		for _, inv := range invokers {
			if inv != nil {
				_ = inv.Close()
			}
		}

		t.Logf("Created and closed %d invokers", numInvokers)
	})

	t.Run("Concurrent invoker creation", func(t *testing.T) {
		const numGoroutines = 20
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				invoker := NewInvoker(&InvokerConfig{
					Address: "http://localhost:19090",
				})

				if invoker != nil {
					_ = invoker.Close()
				}
			}()
		}

		wg.Wait()
		t.Logf("Concurrent invoker creation: %d goroutines", numGoroutines)
	})
}
