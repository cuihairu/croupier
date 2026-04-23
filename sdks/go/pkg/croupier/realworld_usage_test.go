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

// TestRealWorldUsage_Microservice tests microservice usage patterns
func TestRealWorldUsage_Microservice(t *testing.T) {
	t.Run("Service discovery pattern", func(t *testing.T) {
		services := []struct {
			name    string
			address string
		}{
			{"user-service", "localhost:8081"},
			{"order-service", "localhost:8082"},
			{"payment-service", "localhost:8083"},
			{"inventory-service", "localhost:8084"},
		}

		ctx := context.Background()
		var wg sync.WaitGroup

		for _, svc := range services {
			wg.Add(1)
			go func(service struct{ name, address string }) {
				defer wg.Done()

				config := &InvokerConfig{
					Address: service.address,
					Retry: &RetryConfig{
						Enabled:     true,
						MaxAttempts: 3,
					},
				}

				invoker := NewHTTPInvoker(config)
				if invoker != nil {
					defer invoker.Close()

					result, err := invoker.Invoke(ctx, "health.check", "{}", InvokeOptions{})
					t.Logf("%s health check: error=%v, result_len=%d",
						service.name, err, len(result))
				}
			}(svc)
		}

		wg.Wait()
	})

	t.Run("API gateway pattern", func(t *testing.T) {
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

		ctx := context.Background()

		// Simulate API gateway routing
		requests := []struct {
			path    string
			payload string
		}{
			{"/api/v1/users", `{"action":"list"}`},
			{"/api/v1/users/123", `{"action":"get"}`},
			{"/api/v1/orders", `{"action":"create"}`},
			{"/api/v1/products", `{"action":"search"}`},
		}

		for _, req := range requests {
			functionID := fmt.Sprintf("gateway.route%s", req.path)
			result, err := invoker.Invoke(ctx, functionID, req.payload, InvokeOptions{})
			t.Logf("Gateway %s: error=%v, result_len=%d", req.path, err, len(result))
		}
	})
}

// TestRealWorldUsage_BatchProcessing tests batch processing patterns
func TestRealWorldUsage_BatchProcessing(t *testing.T) {
	t.Run("Bulk data processing", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate batch processing
		const batchSize = 10
		batches := 3

		for batch := 0; batch < batches; batch++ {
			payloads := make([]string, batchSize)
			for i := 0; i < batchSize; i++ {
				payloads[i] = fmt.Sprintf(`{"batch":%d,"item":%d}`, batch, i)
			}

			// Process batch
			for i, payload := range payloads {
				result, err := invoker.Invoke(ctx, "batch.process", payload, InvokeOptions{})
				t.Logf("Batch %d, Item %d: error=%v, result_len=%d",
					batch, i, err, len(result))
			}
		}
	})

	t.Run("Parallel batch processing", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		const numBatches = 5
		const itemsPerBatch = 5

		var wg sync.WaitGroup
		start := time.Now()

		for batch := 0; batch < numBatches; batch++ {
			wg.Add(1)
			go func(batchID int) {
				defer wg.Done()

				for item := 0; item < itemsPerBatch; item++ {
					payload := fmt.Sprintf(`{"batch":%d,"item":%d}`, batchID, item)
					result, err := invoker.Invoke(ctx, "parallel.process", payload, InvokeOptions{})
					t.Logf("Parallel batch %d, item %d: error=%v, result_len=%d",
						batchID, item, err, len(result))
				}
			}(batch)
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Parallel batch processing: %d batches × %d items in %v",
			numBatches, itemsPerBatch, duration)
	})
}

// TestRealWorldUsage_EventDriven tests event-driven architecture patterns
func TestRealWorldUsage_EventDriven(t *testing.T) {
	t.Run("Event publishing", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		events := []struct {
		 eventType string
			payload   string
		}{
			{"user.created", `{"userId":"123","name":"John"}`},
			{"order.placed", `{"orderId":"456","total":99.99}`},
			{"payment.processed", `{"paymentId":"789","amount":49.99}`},
			{"email.sent", `{"to":"user@example.com","subject":"Welcome"}`},
		}

		for _, event := range events {
			functionID := fmt.Sprintf("event.%s", event.eventType)
			result, err := invoker.Invoke(ctx, functionID, event.payload, InvokeOptions{})
			t.Logf("Event %s: error=%v, result_len=%d", event.eventType, err, len(result))
		}
	})

	t.Run("Event streaming", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Start a streaming job
		jobID, err := invoker.StartJob(ctx, "stream.events", `{"type":"continuous"}`, InvokeOptions{})
		t.Logf("Started streaming job: jobID=%s, error=%v", jobID, err)

		// Try to stream events
		eventChan, err := invoker.StreamJob(ctx, jobID)
		t.Logf("Stream events: error=%v, channel=%v", err, eventChan != nil)

		if eventChan != nil {
			select {
			case event := <-eventChan:
				t.Logf("Received stream event: Type=%s, JobID=%s", event.EventType, event.JobID)
			case <-time.After(time.Second):
				t.Log("No stream event received within timeout")
			}
		}
	})
}

// TestRealWorldUsage_Caching tests caching patterns
func TestRealWorldUsage_Caching(t *testing.T) {
	t.Run("Cache-aside pattern", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate cache miss - fetch from service
		keys := []string{"user:123", "user:456", "user:789"}

		for _, key := range keys {
			payload := fmt.Sprintf(`{"key":"%s","source":"database"}`, key)
			result, err := invoker.Invoke(ctx, "cache.get", payload, InvokeOptions{})
			t.Logf("Cache get %s: error=%v, result_len=%d", key, err, len(result))

			// Simulate cache write
			payload = fmt.Sprintf(`{"key":"%s","ttl":3600}`, key)
			result, err = invoker.Invoke(ctx, "cache.set", payload, InvokeOptions{})
			t.Logf("Cache set %s: error=%v, result_len=%d", key, err, len(result))
		}
	})

	t.Run("Cache invalidation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Invalidate multiple cache entries
		keys := []string{"user:123", "user:456", "session:abc"}
		payload := fmt.Sprintf(`{"keys":[%q, %q, %q]}`, keys[0], keys[1], keys[2])

		result, err := invoker.Invoke(ctx, "cache.invalidate", payload, InvokeOptions{})
		t.Logf("Cache invalidate: error=%v, result_len=%d", err, len(result))
	})
}

// TestRealWorldUsage_RateLimiting tests rate limiting scenarios
func TestRealWorldUsage_RateLimiting(t *testing.T) {
	t.Run("Token bucket rate limiting", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate rate-limited requests
		const requests = 10
		for i := 0; i < requests; i++ {
			payload := fmt.Sprintf(`{"requestId":%d,"tokens":1}`, i)
			result, err := invoker.Invoke(ctx, "ratelimit.consume", payload, InvokeOptions{})

			// Simulate delay between requests
			if i < requests-1 {
				time.Sleep(10 * time.Millisecond)
			}

			t.Logf("Request %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Sliding window rate limiting", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Burst of requests
		const burst = 5
		for i := 0; i < burst; i++ {
			payload := fmt.Sprintf(`{"timestamp":%d}`, time.Now().UnixMilli())
			result, err := invoker.Invoke(ctx, "ratelimit.check", payload, InvokeOptions{})
			t.Logf("Burst request %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestRealWorldUsage_CircuitBreaker tests circuit breaker patterns
func TestRealWorldUsage_CircuitBreaker(t *testing.T) {
	t.Run("Circuit breaker states", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     false, // Circuit breaker should handle retries
				MaxAttempts: 1,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate circuit breaker state transitions
		states := []string{"closed", "half-open", "open"}

		for _, state := range states {
			payload := fmt.Sprintf(`{"circuit":"test-service","state":"%s"}`, state)
			result, err := invoker.Invoke(ctx, "circuitbreaker.state", payload, InvokeOptions{})
			t.Logf("Circuit breaker %s: error=%v, result_len=%d", state, err, len(result))
		}
	})

	t.Run("Circuit breaker fallback", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Primary service
		result, err := invoker.Invoke(ctx, "service.primary", `{"data":"test"}`, InvokeOptions{})
		t.Logf("Primary service: error=%v, result_len=%d", err, len(result))

		// Fallback service
		result, err = invoker.Invoke(ctx, "service.fallback", `{"data":"test"}`, InvokeOptions{})
		t.Logf("Fallback service: error=%v, result_len=%d", err, len(result))
	})
}

// TestRealWorldUsage_SagaPattern tests saga pattern for distributed transactions
func TestRealWorldUsage_SagaPattern(t *testing.T) {
	t.Run("Saga orchestration", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Saga steps
		steps := []struct {
			name    string
			payload string
		}{
			{"reserve-inventory", `{"productId":"123","quantity":1}`},
			{"process-payment", `{"orderId":"456","amount":99.99}`},
			{"confirm-order", `{"orderId":"456","status":"confirmed"}`},
		}

		completedSteps := 0
		for i, step := range steps {
			result, err := invoker.Invoke(ctx, fmt.Sprintf("saga.%s", step.name), step.payload, InvokeOptions{})
			if err == nil {
				completedSteps++
				t.Logf("Step %d (%s): success, result_len=%d", i, step.name, len(result))
			} else {
				t.Logf("Step %d (%s): failed, error=%v", i, step.name, err)

				// Compensating transactions
				for j := i - 1; j >= 0; j-- {
					compensateStep := steps[j]
					compPayload := fmt.Sprintf(`{"original":%s,"action":"compensate"}`, compensateStep.payload)
					result, err := invoker.Invoke(ctx, fmt.Sprintf("saga.%s.compensate", compensateStep.name), compPayload, InvokeOptions{})
					t.Logf("Compensate %s: error=%v, result_len=%d", compensateStep.name, err, len(result))
				}
				break
			}
		}

		t.Logf("Saga completed: %d/%d steps", completedSteps, len(steps))
	})
}

// TestRealWorldUsage_Monitoring tests monitoring and observability patterns
func TestRealWorldUsage_Monitoring(t *testing.T) {
	t.Run("Metrics collection", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Simulate metric collection
		metrics := []string{"request.count", "request.latency", "error.rate", "active.connections"}

		for _, metric := range metrics {
			payload := fmt.Sprintf(`{"metric":"%s","timestamp":%d}`, metric, time.Now().Unix())
			result, err := invoker.Invoke(ctx, "metrics.record", payload, InvokeOptions{})
			t.Logf("Metric %s: error=%v, result_len=%d", metric, err, len(result))
		}
	})

	t.Run("Health checks", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Different health check types
		checks := []struct {
			name string
			payload string
		}{
			{"liveness", `{"type":"liveness"}`},
			{"readiness", `{"type":"readiness"}`},
			{"startup", `{"type":"startup"}`},
		}

		for _, check := range checks {
			result, err := invoker.Invoke(ctx, fmt.Sprintf("health.%s", check.name), check.payload, InvokeOptions{})
			t.Logf("Health check %s: error=%v, result_len=%d", check.name, err, len(result))
		}
	})
}
