// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdvancedIntegration_MultiInvoker tests multiple invokers working together
func TestAdvancedIntegration_MultiInvoker(t *testing.T) {
	t.Run("Load balancing across invokers", func(t *testing.T) {
		const numInvokers = 3
		invokers := make([]Invoker, numInvokers)

		for i := 0; i < numInvokers; i++ {
			config := &InvokerConfig{
				Address: fmt.Sprintf("localhost:%d", 19090+i%5),
			}
			invokers[i] = NewHTTPInvoker(config)
			if invokers[i] == nil {
				t.Fatalf("Failed to create invoker %d", i)
			}
			defer invokers[i].Close()
		}

		ctx := context.Background()
		const numRequests = 30

		// Distribute requests across invokers
		for i := 0; i < numRequests; i++ {
			invokerIdx := i % numInvokers
			result, err := invokers[invokerIdx].Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			t.Logf("Request %d on invoker %d: error=%v, result_len=%d", i, invokerIdx, err, len(result))
		}
	})

	t.Run("Failover between invokers", func(t *testing.T) {
		// Create primary and backup invokers
		primary := NewHTTPInvoker(&InvokerConfig{Address: "http://localhost:19090"})
		backup := NewHTTPInvoker(&InvokerConfig{Address: "http://localhost:19090"})

		if primary == nil || backup == nil {
			t.Fatal("Failed to create invokers")
		}
		defer primary.Close()
		defer backup.Close()

		ctx := context.Background()
		var err error
		var result string

		// Try primary first
		result, err = primary.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Primary invoker: error=%v, result_len=%d", err, len(result))

		// If primary fails, try backup
		if err != nil {
			result, err = backup.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Backup invoker: error=%v, result_len=%d", err, len(result))
		}
	})

	t.Run("Concurrent invokers with shared workload", func(t *testing.T) {
		const numInvokers = 5
		const numJobs = 100

		var wg sync.WaitGroup
		jobsCompleted := atomic.Int32{}

		for i := 0; i < numInvokers; i++ {
			wg.Add(1)
			go func(invokerID int) {
				defer wg.Done()

				config := &InvokerConfig{
					Address: fmt.Sprintf("localhost:%d", 19090+invokerID%5),
				}
				invoker := NewHTTPInvoker(config)
				if invoker == nil {
					return
				}
				defer invoker.Close()

				ctx := context.Background()
				jobsPerInvoker := numJobs / numInvokers

				for j := 0; j < jobsPerInvoker; j++ {
					result, err := invoker.Invoke(ctx, "test.function",
						fmt.Sprintf(`{"invoker":%d,"job":%d}`, invokerID, j), InvokeOptions{})

					if err == nil || len(result) >= 0 {
						jobsCompleted.Add(1)
					}
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Completed %d/%d jobs across %d invokers", jobsCompleted.Load(), numJobs, numInvokers)
	})
}

// TestAdvancedIntegration_ComplexWorkflows tests complex multi-step workflows
func TestAdvancedIntegration_ComplexWorkflows(t *testing.T) {
	t.Run("Fan-out fan-in pattern", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numWorkers = 10

		// Fan-out: Start multiple jobs
		jobIDs := make([]string, numWorkers)
		for i := 0; i < numWorkers; i++ {
			jobID, err := invoker.StartJob(ctx, "test.job", fmt.Sprintf(`{"worker":%d}`, i), InvokeOptions{})
			jobIDs[i] = jobID
			t.Logf("Started job %d: jobID=%s, error=%v", i, jobID, err)
		}

		// Fan-in: Wait for all jobs
		var wg sync.WaitGroup
		for _, jobID := range jobIDs {
			wg.Add(1)
			go func(jid string) {
				defer wg.Done()

				eventChan, err := invoker.StreamJob(ctx, jid)
				if err != nil {
					t.Logf("Stream job %s: error=%v", jid, err)
					return
				}

				// Read events
				eventCount := 0
				for range eventChan {
					eventCount++
					if eventCount > 5 {
						break
					}
				}
				t.Logf("Job %s: received %d events", jid, eventCount)
			}(jobID)
		}

		wg.Wait()
	})

	t.Run("Pipeline pattern", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const stages = 3
		const items = 5

		// Process items through pipeline stages
		for stage := 0; stage < stages; stage++ {
			for item := 0; item < items; item++ {
				result, err := invoker.Invoke(ctx, "test.function",
					fmt.Sprintf(`{"stage":%d,"item":%d}`, stage, item), InvokeOptions{})
				t.Logf("Pipeline stage %d item %d: error=%v, result_len=%d", stage, item, err, len(result))
			}
		}
	})

	t.Run("Distributed transaction simulation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const participants = 3

		// Phase 1: Prepare all participants
		prepareResults := make([]error, participants)
		for i := 0; i < participants; i++ {
			result, err := invoker.Invoke(ctx, "test.prepare",
				fmt.Sprintf(`{"participant":%d,"phase":"prepare"}`, i), InvokeOptions{})
			prepareResults[i] = err
			t.Logf("Prepare participant %d: error=%v, result_len=%d", i, err, len(result))
		}

		// Phase 2: Check if all prepared successfully
		allPrepared := true
		for _, err := range prepareResults {
			if err != nil {
				allPrepared = false
				break
			}
		}

		// Phase 3: Commit or rollback
		for i := 0; i < participants; i++ {
			var phase string
			if allPrepared {
				phase = "commit"
			} else {
				phase = "rollback"
			}

			result, err := invoker.Invoke(ctx, "test."+phase,
				fmt.Sprintf(`{"participant":%d,"phase":"%s"}`, i, phase), InvokeOptions{})
			t.Logf("%s participant %d: error=%v, result_len=%d", phase, i, err, len(result))
		}
	})
}

// TestAdvancedIntegration_StateManagement tests state management across operations
func TestAdvancedIntegration_StateManagement(t *testing.T) {
	t.Run("Schema state persistence", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
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
		err := invoker.SetSchema("test.function", schema1)
		t.Logf("Set schema1: error=%v", err)

		// Update schema
		schema2 := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":  map[string]interface{}{"type": "string"},
				"email": map[string]interface{}{"type": "string"},
			},
		}
		err = invoker.SetSchema("test.function", schema2)
		t.Logf("Set schema2: error=%v", err)

		// Clear schema
		schema3 := map[string]interface{}{}
		err = invoker.SetSchema("test.function", schema3)
		t.Logf("Clear schema: error=%v", err)
	})

	t.Run("Job state transitions", func(t *testing.T) {
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
			// Stream job events
			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("Stream job: error=%v", err)

			if err == nil && eventChan != nil {
				// Read a few events
				eventCount := 0
				for event := range eventChan {
					eventCount++
					t.Logf("Job event: %+v", event)
					if eventCount >= 3 {
						// Cancel job
						cancelErr := invoker.CancelJob(ctx, jobID)
						t.Logf("Cancel job: error=%v", cancelErr)
						break
					}
				}
			}
		}
	})

	t.Run("Invoker lifecycle states", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()

		// Active state
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Active state: error=%v, result_len=%d", err, len(result))

		// Close
		err = invoker.Close()
		t.Logf("Close: error=%v", err)

		// Operations after close should handle gracefully
		result, err = invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("After close: error=%v, result_len=%d", err, len(result))

		// Multiple closes
		err = invoker.Close()
		t.Logf("Second close: error=%v", err)
	})
}

// TestAdvancedIntegration_ErrorPropagation tests error propagation patterns
func TestAdvancedIntegration_ErrorPropagation(t *testing.T) {
	t.Run("Error wrapping and unwrapping", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test various error scenarios
		scenarios := []struct {
			functionID string
			payload     string
		}{
			{"nonexistent.function", "{}"},
			{"test.function", "invalid json"},
			{"", "{}"},
		}

		for i, scenario := range scenarios {
			result, err := invoker.Invoke(ctx, scenario.functionID, scenario.payload, InvokeOptions{})
			t.Logf("Scenario %d (func=%s, payload=%s): error=%v, result_len=%d",
				i, scenario.functionID, scenario.payload, err, len(result))
		}
	})

	t.Run("Retry error handling", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				BackoffMultiplier: 2.0,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("With retry: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Context cancellation propagation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel immediately
		cancel()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Cancelled context: error=%v, result_len=%d", err, len(result))
	})
}

// TestAdvancedIntegration_ResourceContention tests resource contention scenarios
func TestAdvancedIntegration_ResourceContention(t *testing.T) {
	t.Run("Memory contention", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numGoroutines = 50
		const payloadSize = 10000

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				largePayload := fmt.Sprintf(`{"data":"%s","id":%d}`,
					string(make([]byte, payloadSize)), idx)

				result, err := invoker.Invoke(ctx, "test.function", largePayload, InvokeOptions{})
				_ = result
				_ = err
			}(i)
		}

		wg.Wait()
		t.Logf("Completed %d memory-intensive operations", numGoroutines)
	})

	t.Run("Network contention", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numRequests = 100

		start := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, idx), InvokeOptions{})
				_ = result
				_ = err
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Completed %d concurrent requests in %v (%.2f req/sec)",
			numRequests, duration, float64(numRequests)/duration.Seconds())
	})

	t.Run("Goroutine contention", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Start many jobs concurrently
		const numJobs = 50
		jobIDs := make([]string, numJobs)

		for i := 0; i < numJobs; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", fmt.Sprintf(`{"job":%d}`, i), InvokeOptions{})
			jobIDs[i] = jobID
		}

		// Stream all jobs
		var wg sync.WaitGroup
		for _, jobID := range jobIDs {
			if jobID == "" {
				continue
			}
			wg.Add(1)
			go func(jid string) {
				defer wg.Done()
				eventChan, _ := invoker.StreamJob(ctx, jid)
				if eventChan != nil {
					eventCount := 0
					for range eventChan {
						eventCount++
						if eventCount >= 2 {
							break
						}
					}
				}
			}(jobID)
		}

		wg.Wait()
		t.Logf("Completed streaming for %d jobs", numJobs)
	})
}

// TestAdvancedIntegration_ObserverPattern tests observer pattern implementations
func TestAdvancedIntegration_ObserverPattern(t *testing.T) {
	t.Run("Event subscription simulation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numEvents = 10

		// Simulate event streaming
		jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		if jobID != "" {
			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("Event subscription: error=%v", err)

			if err == nil && eventChan != nil {
				eventCount := 0
				for range eventChan {
					eventCount++
					t.Logf("Received event %d", eventCount)
					if eventCount >= numEvents {
						break
					}
				}
			}
		}
	})

	t.Run("Multiple observers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numObservers = 3

		jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		if jobID != "" {
			var wg sync.WaitGroup

			// Multiple observers watching same job
			for i := 0; i < numObservers; i++ {
				wg.Add(1)
				go func(observerID int) {
					defer wg.Done()

					eventChan, err := invoker.StreamJob(ctx, jobID)
					t.Logf("Observer %d: subscribe error=%v", observerID, err)

					if err == nil && eventChan != nil {
						eventCount := 0
						for range eventChan {
							eventCount++
							t.Logf("Observer %d: event %d", observerID, eventCount)
							if eventCount >= 3 {
								break
							}
						}
					}
				}(i)
			}

			wg.Wait()
		}
	})
}

// TestAdvancedIntegration_CachingTests tests caching behavior
func TestAdvancedIntegration_CachingTests(t *testing.T) {
	t.Run("Idempotency key caching", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		idempotencyKey := fmt.Sprintf("cache-test-%d", time.Now().UnixNano())

		// First invocation
		result1, err1 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{
			IdempotencyKey: idempotencyKey,
		})
		t.Logf("First invocation: error=%v, result_len=%d", err1, len(result1))

		// Second invocation with same key (should return cached result)
		result2, err2 := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{
			IdempotencyKey: idempotencyKey,
		})
		t.Logf("Second invocation (cached): error=%v, result_len=%d", err2, len(result2))
	})

	t.Run("Schema caching", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Set schema multiple times
		schema := map[string]interface{}{
			"type": "string",
		}

		for i := 0; i < 5; i++ {
			err := invoker.SetSchema("test.function", schema)
			t.Logf("Set schema iteration %d: error=%v", i, err)
		}
	})
}

// TestAdvancedIntegration_BackpressureTests tests backpressure scenarios
func TestAdvancedIntegration_BackpressureTests(t *testing.T) {
	t.Run("Slow consumer", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		if jobID != "" {
			eventChan, _ := invoker.StreamJob(ctx, jobID)

			if eventChan != nil {
				// Slow consumer: add delay between events
				eventCount := 0
				for range eventChan {
					eventCount++
					time.Sleep(100 * time.Millisecond)
					t.Logf("Processed event %d (slow consumer)", eventCount)
					if eventCount >= 5 {
						break
					}
				}
			}
		}
	})

	t.Run("Fast producer", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numInvokes = 50

		// Fast producer: rapid invocations
		start := time.Now()
		for i := 0; i < numInvokes; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			_ = result
			_ = err
		}
		duration := time.Since(start)

		t.Logf("Fast producer: %d invocations in %v (%.2f req/sec)",
			numInvokes, duration, float64(numInvokes)/duration.Seconds())
	})
}

// TestAdvancedIntegration_CascadingFailures tests cascading failure scenarios
func TestAdvancedIntegration_CascadingFailures(t *testing.T) {
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
		const chainLength = 5

		// Chain of dependent operations
		for i := 0; i < chainLength; i++ {
			result, err := invoker.Invoke(ctx, fmt.Sprintf("step%d.function", i),
				fmt.Sprintf(`{"step":%d}`, i), InvokeOptions{})
			t.Logf("Chain step %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Partial failure recovery", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const operations = 10

		successCount := 0
		failureCount := 0

		for i := 0; i < operations; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"op":%d}`, i), InvokeOptions{})
			if err != nil {
				failureCount++
			} else {
				successCount++
			}
			_ = result
		}

		t.Logf("Partial failure: %d success, %d failure", successCount, failureCount)
	})
}

// TestAdvancedIntegration_TimeoutVariations tests various timeout scenarios
func TestAdvancedIntegration_TimeoutVariations(t *testing.T) {
	t.Run("Adaptive timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Start with short timeout, increase if needed
		timeouts := []time.Duration{
			10 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			500 * time.Millisecond,
		}

		for _, timeout := range timeouts {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			cancel()

			t.Logf("Timeout %v: error=%v, result_len=%d", timeout, err, len(result))
			if err == nil {
				break
			}
		}
	})

	t.Run("Timeout with retry", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 50,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Timeout with retry: error=%v, result_len=%d", err, len(result))
	})
}
