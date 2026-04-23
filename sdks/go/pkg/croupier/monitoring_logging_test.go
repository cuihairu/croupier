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

// TestMonitoring_MetricsCollection tests metrics collection scenarios
func TestMonitoring_MetricsCollection(t *testing.T) {
	t.Run("Invocation metrics", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numInvokes = 20

		var successCount, errorCount int
		var totalDuration time.Duration

		for i := 0; i < numInvokes; i++ {
			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			duration := time.Since(start)

			totalDuration += duration
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
			_ = result
		}

		avgDuration := totalDuration / time.Duration(numInvokes)
		successRate := float64(successCount) / float64(numInvokes) * 100

		t.Logf("Invocation metrics:")
		t.Logf("  Total invocations: %d", numInvokes)
		t.Logf("  Success: %d (%.1f%%)", successCount, successRate)
		t.Logf("  Errors: %d", errorCount)
		t.Logf("  Avg duration: %v", avgDuration)
	})

	t.Run("Concurrent invocation metrics", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numGoroutines = 10
		const invokesPerGoroutine = 5

		var wg sync.WaitGroup
		var mu sync.Mutex
		var totalInvokes, totalErrors int

		startTime := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < invokesPerGoroutine; j++ {
					result, err := invoker.Invoke(ctx, "test.function",
						fmt.Sprintf(`{"goroutine":%d,"invoke":%d}`, goroutineID, j), InvokeOptions{})

					mu.Lock()
					totalInvokes++
					if err != nil {
						totalErrors++
					}
					mu.Unlock()
					_ = result
				}
			}(i)
		}

		wg.Wait()
		totalDuration := time.Since(startTime)

		throughput := float64(totalInvokes) / totalDuration.Seconds()

		t.Logf("Concurrent metrics:")
		t.Logf("  Total invocations: %d", totalInvokes)
		t.Logf("  Errors: %d", totalErrors)
		t.Logf("  Duration: %v", totalDuration)
		t.Logf("  Throughput: %.2f invokes/sec", throughput)
	})
}

// TestMonitoring_PerformanceTracking tests performance tracking
func TestMonitoring_PerformanceTracking(t *testing.T) {
	t.Run("Response time distribution", func(t *testing.T) {
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

		durations := make([]time.Duration, numInvokes)

		for i := 0; i < numInvokes; i++ {
			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			durations[i] = time.Since(start)
			_ = result
			_ = err
		}

		// Calculate percentiles
		var sum time.Duration
		for _, d := range durations {
			sum += d
		}
		avg := sum / time.Duration(numInvokes)

		// Simple percentile calculation
		for _, p := range []int{50, 90, 95, 99} {
			percentileValue := percentile(durations, p)
			t.Logf("p%d latency: %v", p, percentileValue)
		}

		t.Logf("Average latency: %v", avg)
	})

	t.Run("Payload size vs duration", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		sizes := []int{
			100,
			500,
			1000,
			5000,
			10000,
		}

		for _, size := range sizes {
			payload := fmt.Sprintf(`{"data":"%s"}`, string(make([]byte, size)))

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Size %d bytes: duration=%v, error=%v, result_len=%d",
				size, duration, err, len(result))
		}
	})
}

// TestMonitoring_ResourceUsage tests resource usage monitoring
func TestMonitoring_ResourceUsage(t *testing.T) {
	t.Run("Memory usage tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numInvokes = 100

		for i := 0; i < numInvokes; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			_ = result
			_ = err

			if i%20 == 0 {
				t.Logf("Completed %d invokes", i)
			}
		}

		t.Logf("Completed %d invokes", numInvokes)
	})

	t.Run("Goroutine tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Start multiple jobs
		const numJobs = 20
		jobIDs := make([]string, numJobs)

		for i := 0; i < numJobs; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", fmt.Sprintf(`{"job":%d}`, i), InvokeOptions{})
			jobIDs[i] = jobID
		}

		t.Logf("Started %d jobs", numJobs)

		// Cancel all jobs
		for _, jobID := range jobIDs {
			invoker.CancelJob(ctx, jobID)
		}
	})
}

// TestMonitoring_ErrorTracking tests error tracking and monitoring
func TestMonitoring_ErrorTracking(t *testing.T) {
	t.Run("Error rate monitoring", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numInvokes = 30

		errorTypes := make(map[string]int)

		for i := 0; i < numInvokes; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})

			if err != nil {
				errorType := fmt.Sprintf("%T", err)
				errorTypes[errorType]++
			}
			_ = result
		}

		t.Logf("Error types encountered:")
		for errorType, count := range errorTypes {
			t.Logf("  %s: %d occurrences", errorType, count)
		}
	})

	t.Run("Retry tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 50,
				MaxDelayMs:     200,
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

		t.Logf("Retry test:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Error: %v", err)
		t.Logf("  Result length: %d", len(result))
	})
}

// TestLogging_LogLevels tests logging at different levels
func TestLogging_LogLevels(t *testing.T) {
	t.Run("Info level logging", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// These operations should generate info logs
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Invoke result: error=%v, result_len=%d", err, len(result))

		schema := map[string]interface{}{"type": "string"}
		err = invoker.SetSchema("test.function", schema)
		t.Logf("SetSchema: error=%v", err)
	})

	t.Run("Error level logging", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// These operations might generate error logs
		result, err := invoker.Invoke(ctx, "invalid.function", "{}", InvokeOptions{})
		t.Logf("Invalid invoke: error=%v, result_len=%d", err, len(result))
	})
}

// TestMonitoring_OperationalMetrics tests operational metrics
func TestMonitoring_OperationalMetrics(t *testing.T) {
	t.Run("Connection pool metrics", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numRequests = 50

		start := time.Now()
		for i := 0; i < numRequests; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			_ = result
			_ = err
		}
		duration := time.Since(start)

		throughput := float64(numRequests) / duration.Seconds()

		t.Logf("Connection pool metrics:")
		t.Logf("  Requests: %d", numRequests)
		t.Logf("  Duration: %v", duration)
		t.Logf("  Throughput: %.2f req/sec", throughput)
	})

	t.Run("Schema operation metrics", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numSchemas = 20

		start := time.Now()
		for i := 0; i < numSchemas; i++ {
			schema := map[string]interface{}{
				"type": "string",
			}
			err := invoker.SetSchema(fmt.Sprintf("function%d", i), schema)
			_ = err
		}
		duration := time.Since(start)

		avgDuration := duration / time.Duration(numSchemas)

		t.Logf("Schema operation metrics:")
		t.Logf("  Schemas set: %d", numSchemas)
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Avg duration: %v", avgDuration)
	})
}

// TestMonitoring_TimeoutTracking tests timeout tracking
func TestMonitoring_TimeoutTracking(t *testing.T) {
	t.Run("Timeout occurrences", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "192.0.2.1:8080",
			TimeoutSeconds: 1,
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

		t.Logf("Timeout test:")
		t.Logf("  Configured timeout: 1s")
		t.Logf("  Actual duration: %v", duration)
		t.Logf("  Error: %v", err)
		t.Logf("  Result length: %d", len(result))
	})

	t.Run("Context timeout tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		timeouts := []time.Duration{
			10 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
		}

		for _, timeout := range timeouts {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Timeout %v: duration=%v, error=%v, result_len=%d",
				timeout, duration, err, len(result))
		}
	})
}

// TestMonitoring_RetryMetrics tests retry-related metrics
func TestMonitoring_RetryMetrics(t *testing.T) {
	t.Run("Retry attempt tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:        true,
				MaxAttempts:    5,
				InitialDelayMs: 20,
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

		t.Logf("Retry metrics:")
		t.Logf("  Max attempts: 5")
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Error: %v", err)
		t.Logf("  Result length: %d", len(result))
	})

	t.Run("Backoff delay tracking", func(t *testing.T) {
		backoffMultipliers := []float64{1.5, 2.0, 3.0}

		for _, mult := range backoffMultipliers {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:           true,
					MaxAttempts:       3,
					InitialDelayMs:    20,
					MaxDelayMs:        200,
					BackoffMultiplier: mult,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker == nil {
				continue
			}

			ctx := context.Background()
			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Backoff %.1f: duration=%v, error=%v, result_len=%d",
				mult, duration, err, len(result))

			invoker.Close()
		}
	})
}

// TestMonitoring_CircuitBreakerMetrics tests circuit breaker patterns
func TestMonitoring_CircuitBreakerMetrics(t *testing.T) {
	t.Run("Failure threshold tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     false,
				MaxAttempts: 1,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numInvokes = 10

		failureCount := 0
		for i := 0; i < numInvokes; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			if err != nil {
				failureCount++
			}
			_ = result
		}

		failureRate := float64(failureCount) / float64(numInvokes) * 100

		t.Logf("Circuit breaker metrics:")
		t.Logf("  Total invocations: %d", numInvokes)
		t.Logf("  Failures: %d", failureCount)
		t.Logf("  Failure rate: %.1f%%", failureRate)
	})
}

// TestMonitoring_JobMetrics tests job-related metrics
func TestMonitoring_JobMetrics(t *testing.T) {
	t.Run("Job lifecycle metrics", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numJobs = 10

		start := time.Now()
		jobIDs := make([]string, numJobs)

		// Start jobs
		for i := 0; i < numJobs; i++ {
			jobID, err := invoker.StartJob(ctx, "test.job", fmt.Sprintf(`{"job":%d}`, i), InvokeOptions{})
			jobIDs[i] = jobID
			_ = err
		}

		startDuration := time.Since(start)

		// Cancel all jobs
		cancelStart := time.Now()
		for _, jobID := range jobIDs {
			invoker.CancelJob(ctx, jobID)
		}
		cancelDuration := time.Since(cancelStart)

		t.Logf("Job metrics:")
		t.Logf("  Jobs started: %d", numJobs)
		t.Logf("  Start duration: %v", startDuration)
		t.Logf("  Cancel duration: %v", cancelDuration)
	})

	t.Run("Job streaming metrics", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		t.Logf("Start job: jobID=%s, error=%v", jobID, err)

		if jobID != "" {
			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("Stream job: error=%v", err)

			if err == nil && eventChan != nil {
				eventCount := 0
				start := time.Now()

				for range eventChan {
					eventCount++
					if eventCount >= 3 {
						break
					}
				}

				duration := time.Since(start)
				t.Logf("Job streaming metrics:")
				t.Logf("  Events received: %d", eventCount)
				t.Logf("  Duration: %v", duration)
			}
		}
	})
}

// TestMonitoring_LongRunningOperations tests long-running operation monitoring
func TestMonitoring_LongRunningOperations(t *testing.T) {
	t.Run("Extended operation tracking", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Track progress over time
		checkpoints := []int{10, 25, 50, 75, 100}
		const totalInvokes = 100

		lastCheckpoint := 0
		for _, checkpoint := range checkpoints {
			for i := lastCheckpoint; i < checkpoint; i++ {
				result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
				_ = result
				_ = err
			}

			progress := float64(checkpoint) / float64(totalInvokes) * 100
			t.Logf("Progress: %d%% (%d/%d invokes)", int(progress), checkpoint, totalInvokes)

			lastCheckpoint = checkpoint
		}
	})
}

// Helper function to calculate percentile
func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Simple selection (not efficient but works for tests)
	index := len(durations) * p / 100
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}
