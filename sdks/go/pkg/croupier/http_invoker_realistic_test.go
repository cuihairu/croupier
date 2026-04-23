// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestHTTPInvoker_realistic_api_calls tests realistic API call scenarios
func TestHTTPInvoker_realistic_api_calls(t *testing.T) {
	t.Run("Invoke with complete request-response cycle", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		payload := `{"userId":"123","action":"getProfile"}`
		result, err := invoker.Invoke(ctx, "user.getProfile", payload, InvokeOptions{})

		t.Logf("Invoke result: %s, error: %v", string(result), err)
	})

	t.Run("Invoke with structured JSON payload", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		type Request struct {
			UserID   string   `json:"userId"`
			Action   string   `json:"action"`
			Items    []string `json:"items"`
			Metadata map[string]string `json:"metadata"`
		}

		req := Request{
			UserID: "user-123",
			Action: "batchProcess",
			Items:  []string{"item1", "item2", "item3"},
			Metadata: map[string]string{
				"source":  "web",
				"version": "1.0",
			},
		}

		payload, _ := json.Marshal(req)

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "batch.process", string(payload), InvokeOptions{})

		t.Logf("Structured invoke - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoke with custom headers for authentication", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		opts := InvokeOptions{
			Headers: map[string]string{
				"Authorization":     "Bearer token123",
				"X-API-Key":         "api-key-456",
				"X-Request-ID":      "req-789",
				"X-Client-Version":  "1.0.0",
				"X-Client-Platform": "web",
			},
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "auth.secure", "{}", opts)

		t.Logf("Invoke with auth headers - result: %s, error: %v", string(result), err)
	})

	t.Run("Invoke with idempotency for duplicate requests", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		idempotencyKey := "unique-request-id-12345"

		opts := InvokeOptions{
			IdempotencyKey: idempotencyKey,
		}

		ctx := context.Background()

		// First request
		result1, err1 := invoker.Invoke(ctx, "payment.process", `{"amount":100}`, opts)
		t.Logf("First invoke - result: %s, error: %v", string(result1), err1)

		// Duplicate request with same key
		result2, err2 := invoker.Invoke(ctx, "payment.process", `{"amount":100}`, opts)
		t.Logf("Second invoke (same key) - result: %s, error: %v", string(result2), err2)
	})
}

// TestHTTPInvoker_job_operations tests job-based operations
func TestHTTPInvoker_job_operations(t *testing.T) {
	t.Run("StartJob and check job lifecycle", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		payload := `{"task":"longRunningOperation","params":{"iterations":100}}`

		jobID, err := invoker.StartJob(ctx, "job.longTask", payload, InvokeOptions{})
		t.Logf("StartJob - jobID: %s, error: %v", jobID, err)

		if jobID != "" {
			// Try to get job status
			stream, err := invoker.StreamJob(ctx, jobID)
			t.Logf("StreamJob - stream: %v, error: %v", stream, err)
		}
	})

	t.Run("StartJob with timeout", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		payload := `{"task":"quickTask"}`
		jobID, err := invoker.StartJob(ctx, "job.quick", payload, InvokeOptions{})

		t.Logf("StartJob with timeout - jobID: %s, error: %v", jobID, err)
	})

	t.Run("StartJob multiple jobs concurrently", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numJobs = 10

		jobIDs := make([]string, numJobs)
		for i := 0; i < numJobs; i++ {
			payload := fmt.Sprintf(`{"task":"task%d","iteration":%d}`, i, i)
			jobID, err := invoker.StartJob(ctx, "job.batch", payload, InvokeOptions{})
			jobIDs[i] = jobID
			t.Logf("Job %d - ID: %s, error: %v", i, jobID, err)
		}

		t.Logf("Started %d jobs", numJobs)
	})
}

// TestHTTPInvoker_retry_scenarios tests retry behavior
func TestHTTPInvoker_retry_scenarios(t *testing.T) {
	t.Run("Invoke with retry enabled", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				MaxDelayMs:        1000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.flaky", "{}", InvokeOptions{})

		t.Logf("Invoke with retry - result: %s, error: %v", string(result), err)
	})

	t.Run("Invoke with retry override in options", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		opts := InvokeOptions{
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
				InitialDelayMs: 50,
			},
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.retry", "{}", opts)

		t.Logf("Invoke with retry override - result: %s, error: %v", string(result), err)
	})

	t.Run("Invoke with retry disabled", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
			Retry: &RetryConfig{
				Enabled: false,
			},
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.noretry", "{}", InvokeOptions{})

		t.Logf("Invoke with retry disabled - result: %s, error: %v", string(result), err)
	})
}

// TestHTTPInvoker_timeout_scenarios tests various timeout scenarios
func TestHTTPInvoker_timeout_scenarios(t *testing.T) {
	t.Run("Invoke with short timeout", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		opts := InvokeOptions{
			Timeout: 10 * time.Millisecond,
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.slow", "{}", opts)

		t.Logf("Invoke with short timeout - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoke with long timeout", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		opts := InvokeOptions{
			Timeout: 30 * time.Second,
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.fast", "{}", opts)

		t.Logf("Invoke with long timeout - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoke with timeout from context", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.timeout", "{}", InvokeOptions{})

		t.Logf("Invoke with context timeout - result length: %d, error: %v", len(result), err)
	})
}

// TestHTTPInvoker_error_handling tests error handling scenarios
func TestHTTPInvoker_error_handling(t *testing.T) {
	t.Run("Invoke with connection error", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:9999", // Non-existent server
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})

		t.Logf("Invoke with connection error - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoke with invalid function ID format", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "", "{}", InvokeOptions{})

		t.Logf("Invoke with empty function ID - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoke with malformed JSON", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.func", `{invalid json`, InvokeOptions{})

		t.Logf("Invoke with malformed JSON - result length: %d, error: %v", len(result), err)
	})
}

// TestHTTPInvoker_schema_validation tests schema validation scenarios
func TestHTTPInvoker_schema_validation(t *testing.T) {
	t.Run("SetSchema and invoke with valid payload", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":  map[string]interface{}{"type": "string"},
				"age":   map[string]interface{}{"type": "number"},
				"email": map[string]interface{}{"type": "string"},
			},
			"required": []string{"name", "email"},
		}

		err := invoker.SetSchema("user.create", schema)
		t.Logf("SetSchema error: %v", err)

		ctx := context.Background()
		validPayload := `{"name":"John","email":"john@example.com","age":30}`
		result, invokeErr := invoker.Invoke(ctx, "user.create", validPayload, InvokeOptions{})

		t.Logf("Invoke with valid payload - result length: %d, error: %v", len(result), invokeErr)
	})

	t.Run("SetSchema for multiple functions", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schemas := []struct {
			function string
			schema   map[string]interface{}
		}{
			{
				"user.create",
				map[string]interface{}{"type": "object"},
			},
			{
				"user.update",
				map[string]interface{}{"type": "object"},
			},
			{
				"user.delete",
				map[string]interface{}{"type": "object"},
			},
		}

		for _, s := range schemas {
			err := invoker.SetSchema(s.function, s.schema)
			t.Logf("SetSchema for '%s' error: %v", s.function, err)
		}
	})

	t.Run("Update existing schema", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Set initial schema
		schema1 := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		}

		err1 := invoker.SetSchema("test.func", schema1)
		t.Logf("SetSchema (first) error: %v", err1)

		// Update schema
		schema2 := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
				"age":  map[string]interface{}{"type": "number"},
			},
			"required": []string{"name"},
		}

		err2 := invoker.SetSchema("test.func", schema2)
		t.Logf("SetSchema (update) error: %v", err2)
	})
}

// TestHTTPInvoker_concurrent_requests tests concurrent request handling
func TestHTTPInvoker_concurrent_requests(t *testing.T) {
	t.Run("Concurrent Invoke requests", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numRequests = 20
		results := make(chan string, numRequests)
		errors := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(idx int) {
				ctx := context.Background()
				payload := fmt.Sprintf(`{"requestId":%d}`, idx)
				result, err := invoker.Invoke(ctx, "test.concurrent", payload, InvokeOptions{})

				results <- string(result)
				errors <- err
			}(i)
		}

		// Collect results
		successCount := 0
		for i := 0; i < numRequests; i++ {
			<-results
			if <-errors == nil {
				successCount++
			}
		}

		t.Logf("Concurrent Invoke: %d/%d succeeded", successCount, numRequests)
	})

	t.Run("Concurrent StartJob requests", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numJobs = 10
		jobIDs := make([]string, numJobs)

		for i := 0; i < numJobs; i++ {
			ctx := context.Background()
			payload := fmt.Sprintf(`{"jobId":%d}`, i)
			jobID, err := invoker.StartJob(ctx, "job.concurrent", payload, InvokeOptions{})
			jobIDs[i] = jobID
			t.Logf("Job %d - ID: %s, error: %v", i, jobID, err)
		}

		t.Logf("Started %d concurrent jobs", numJobs)
	})
}

// TestHTTPInvoker_various_configs tests invoker with various configurations
func TestHTTPInvoker_various_configs(t *testing.T) {
	t.Run("Invoker with all config options", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address:        "http://localhost:8080",
			TimeoutSeconds: 30,
			Insecure:       false,
			CAFile:         "/path/to/ca.crt",
			CertFile:       "/path/to/cert.pem",
			KeyFile:        "/path/to/key.pem",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
			Reconnect: &ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    1000,
				MaxDelayMs:        30000,
				BackoffMultiplier: 1.5,
				JitterFactor:      0.2,
			},
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.config", "{}", InvokeOptions{})

		t.Logf("Invoke with full config - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoker with minimal config", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.minimal", "{}", InvokeOptions{})

		t.Logf("Invoke with minimal config - result length: %d, error: %v", len(result), err)
	})

	t.Run("Invoker with different address formats", func(t *testing.T) {
		addresses := []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
			"https://example.com:8080",
		}

		for _, addr := range addresses {
			invoker := NewHTTPInvoker(&InvokerConfig{
				Address: addr,
			})

			if invoker == nil {
				t.Errorf("Invoker with address '%s' is nil", addr)
				continue
			}

			ctx := context.Background()
			result, err := invoker.Invoke(ctx, "test.addr", "{}", InvokeOptions{})

			_ = invoker.Close()
			t.Logf("Address '%s' - result length: %d, error: %v", addr, len(result), err)
		}
	})
}

// TestHTTPInvoker_performance_patterns tests performance-related patterns
func TestHTTPInvoker_performance_patterns(t *testing.T) {
	t.Run("Rapid sequential requests", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const iterations = 50
		start := time.Now()

		ctx := context.Background()
		successCount := 0

		for i := 0; i < iterations; i++ {
			_, err := invoker.Invoke(ctx, "test.rapid", fmt.Sprintf(`{"iter":%d}`, i), InvokeOptions{})
			if err == nil {
				successCount++
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Rapid requests: %d iterations in %v (avg %v per request), %d succeeded",
			iterations, elapsed, avgTime, successCount)
	})

	t.Run("Batch operations", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const batchSize = 20
		ctx := context.Background()

		start := time.Now()

		for i := 0; i < batchSize; i++ {
			payload := fmt.Sprintf(`{"batchItem":%d}`, i)
			_, _ = invoker.Invoke(ctx, "batch.process", payload, InvokeOptions{})
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(batchSize)

		t.Logf("Batch operations: %d items in %v (avg %v per item)",
			batchSize, elapsed, avgTime)
	})
}
