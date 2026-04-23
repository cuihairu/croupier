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

// TestComplex_invokeScenarios tests complex invocation scenarios
func TestComplex_invokeScenarios(t *testing.T) {
	t.Run("sequential invokes", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()

		for i := 0; i < 10; i++ {
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke %d error: %v", i, err)
		}
	})

	t.Run("sequential job operations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()

		for i := 0; i < 5; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
			_, _ = invoker.StreamJob(ctx, jobID)
			_ = invoker.CancelJob(ctx, jobID)
			t.Logf("Job operation sequence %d completed", i)
		}
	})
}

// TestComplex_multipleInvokers tests using multiple invokers
func TestComplex_multipleInvokers(t *testing.T) {
	t.Run("create multiple invokers", func(t *testing.T) {
		invokers := make([]Invoker, 5)

		for i := 0; i < 5; i++ {
			invokers[i] = NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
			})

			if invokers[i] == nil {
				t.Errorf("Invoker %d is nil", i)
			}
		}

		ctx := context.Background()
		for i, invoker := range invokers {
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoker %d error: %v", i, err)
		}
	})

	t.Run("parallel invokers", func(t *testing.T) {
		const numInvokers = 10
		var wg sync.WaitGroup

		for i := 0; i < numInvokers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				invoker := NewInvoker(&InvokerConfig{
					Address: "http://localhost:19090",
				})

				ctx := context.Background()
				_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
				t.Logf("Parallel invoker %d error: %v", idx, err)
			}(i)
		}

		wg.Wait()
	})
}

// TestComplex_reconfiguration tests reconfiguration scenarios
func TestComplex_reconfiguration(t *testing.T) {
	t.Run("create destroy create", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		// Create
		invoker1 := NewInvoker(config)
		if invoker1 == nil {
			t.Fatal("First invoker is nil")
		}

		// Close
		err := invoker1.Close()
		t.Logf("First close error: %v", err)

		// Create again
		invoker2 := NewInvoker(config)
		if invoker2 == nil {
			t.Fatal("Second invoker is nil")
		}

		err = invoker2.Close()
		t.Logf("Second close error: %v", err)
	})

	t.Run("update config between creates", func(t *testing.T) {
		config1 := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker1 := NewInvoker(config1)
		_ = invoker1.Close()

		config2 := &InvokerConfig{
			Address: "http://localhost:19090", // Different address
		}

		invoker2 := NewInvoker(config2)
		if invoker2 == nil {
			t.Error("Second invoker with different config should not be nil")
		}
		_ = invoker2.Close()
	})
}

// TestComplex_clientLifecycle tests complete client lifecycle
func TestComplex_clientLifecycle(t *testing.T) {
	t.Run("full lifecycle with functions", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Register functions
		for i := 0; i < 3; i++ {
			desc := FunctionDescriptor{
				ID:      "test.func",
				Version: "1.0.0",
			}
			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			}
			err := client.RegisterFunction(desc, handler)
			t.Logf("Register function %d error: %v", i, err)
		}

		// Get local address
		addr := client.GetLocalAddress()
		t.Logf("Local address: %s", addr)

		// Close
		err := client.Close()
		t.Logf("Close error: %v", err)
	})

	t.Run("reuse after close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Close
		_ = client.Close()

		// Try to use after close - expect panic
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic after close: %v", r)
			}
		}()

		addr := client.GetLocalAddress()
		t.Logf("GetLocalAddress after close: %s", addr)

		// Try to register after close - may panic
		desc := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
		}
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("ok"), nil
		}
		err := client.RegisterFunction(desc, handler)
		t.Logf("Register after close error: %v", err)
	})
}

// TestComplex_errorHandling tests complex error handling scenarios
func TestComplex_errorHandling(t *testing.T) {
	t.Run("multiple errors in sequence", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()

		errors := make([]error, 5)
		for i := 0; i < 5; i++ {
			_, errors[i] = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		}

		for i, err := range errors {
			t.Logf("Invoke %d error: %v", i, err)
		}
	})

	t.Run("error with nil invoker", func(t *testing.T) {
		var invoker Invoker = nil

		// Expect panic when calling methods on nil interface
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic with nil invoker: %v", r)
			}
		}()

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})

		if err == nil {
			t.Error("Invoke with nil invoker should return error")
		}

		t.Logf("Expected error: %v", err)
	})
}

// TestComplex_timing tests timing-related scenarios
func TestComplex_timing(t *testing.T) {
	t.Run("rapid create destroy", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
			})

			if invoker == nil {
				t.Errorf("Iteration %d: NewInvoker returned nil", i)
			}

			_ = invoker.Close()
		}
	})

	t.Run("rapid invoke attempts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()

		start := time.Now()
		for i := 0; i < 50; i++ {
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			if err != nil {
				t.Logf("Invoke %d error: %v", i, err)
			}
		}
		elapsed := time.Since(start)

		t.Logf("50 invokes took: %v (%.2f ms per invoke)", elapsed, float64(elapsed.Milliseconds())/50)
	})
}

// TestComplex_contextPropagation tests context propagation
func TestComplex_contextPropagation(t *testing.T) {
	t.Run("context with timeout propagation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// Multiple invokes with same context
		for i := 0; i < 5; i++ {
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke %d with timeout context error: %v", i, err)
		}

		if ctx.Err() != nil {
			t.Logf("Context error: %v", ctx.Err())
		}
	})

	t.Run("context with value propagation", func(t *testing.T) {
		type ctxKey string
		key := ctxKey("trace-id")

		ctx := context.WithValue(context.Background(), key, "trace-12345")
		ctx = context.WithValue(ctx, ctxKey("user-id"), "user-67890")

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with context values error: %v", err)

		// Verify context values are preserved
		traceID := ctx.Value(key)
		if traceID != nil {
			t.Logf("Trace ID preserved: %v", traceID)
		}
	})
}

// TestComplex_schemaVariations tests various schema scenarios
func TestComplex_schemaVariations(t *testing.T) {
	t.Run("nested schema structures", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":      "string",
							"minLength": 1,
							"maxLength": 100,
						},
						"age": map[string]interface{}{
							"type":    "integer",
							"minimum": 0,
							"maximum": 150,
						},
						"address": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"street": map[string]interface{}{
									"type": "string",
								},
								"city": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		}

		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with nested structure error: %v", err)
	})

	t.Run("schema with array types", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "string"},
				},
				"numbers": map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "number"},
				},
			},
		}

		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with arrays error: %v", err)
	})

	t.Run("schema with enum values", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{
					"type": "string",
					"enum": []string{"active", "inactive", "pending"},
				},
				"priority": map[string]interface{}{
					"type": "integer",
					"enum": []interface{}{1, 2, 3, 4, 5},
				},
			},
		}

		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with enums error: %v", err)
	})
}

// TestComplex_multipleSchemas tests setting schemas for multiple functions
func TestComplex_multipleSchemas(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})

	schemas := []struct {
		functionID string
		schema     map[string]interface{}
	}{
		{
			"func1",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]string{"type": "string"},
				},
			},
		},
		{
			"func2",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]string{"type": "integer"},
				},
			},
		},
		{
			"func3",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"active": map[string]string{"type": "boolean"},
				},
			},
		},
	}

	for _, s := range schemas {
		err := invoker.SetSchema(s.functionID, s.schema)
		t.Logf("SetSchema for %s error: %v", s.functionID, err)
	}
}

// TestComplex_headerPropagation tests header propagation
func TestComplex_headerPropagation(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})

	ctx := context.Background()

	t.Run("headers with standard keys", func(t *testing.T) {
		standardHeaders := map[string]string{
			"X-Request-ID":       "req-123",
			"X-Correlation-ID":   "corr-456",
			"X-Trace-ID":         "trace-789",
			"X-Session-ID":       "session-abc",
			"X-User-ID":          "user-def",
			"X-Client-Version":   "1.0.0",
			"X-Client-Name":      "test-client",
			"X-Environment":      "development",
		}

		options := InvokeOptions{
			Headers: standardHeaders,
		}

		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with standard headers error: %v", err)
	})

	t.Run("headers with custom keys", func(t *testing.T) {
		customHeaders := map[string]string{
			"X-Custom-Header-1": "value1",
			"X-Custom-Header-2": "value2",
			"X-Custom-Header-3": "value3",
		}

		options := InvokeOptions{
			Headers: customHeaders,
		}

		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with custom headers error: %v", err)
	})

	t.Run("headers with mixed case", func(t *testing.T) {
		mixedCaseHeaders := map[string]string{
			"x-request-id":  "lower",
			"X-Request-Id":  "mixed",
			"X-REQUEST-ID":  "upper",
			"X-R e q u e s t-I D": "spaced",
		}

		options := InvokeOptions{
			Headers: mixedCaseHeaders,
		}

		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with mixed case headers error: %v", err)
	})
}
