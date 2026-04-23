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

// TestJobOperations_BasicScenarios tests basic job operation scenarios
func TestJobOperations_BasicScenarios(t *testing.T) {
	t.Run("StartJob with minimal configuration", func(t *testing.T) {
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

		t.Logf("StartJob result: jobID=%s, error=%v", jobID, err)
	})

	t.Run("StartJob with custom options", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		opts := InvokeOptions{
			IdempotencyKey: fmt.Sprintf("job-key-%d", time.Now().UnixNano()),
			Timeout:        30 * 1000 * 1000 * 1000, // 30s
			Headers: map[string]string{
				"X-Job-Type":    "test",
				"X-Priority":    "high",
				"X-Attempt":     "1",
				"X-Request-ID":  fmt.Sprintf("req-%d", time.Now().UnixNano()),
			},
		}

		ctx := context.Background()
		jobID, err := invoker.StartJob(ctx, "test.job.withOptions", "{}", opts)

		t.Logf("StartJob with options: jobID=%s, error=%v", jobID, err)
	})

	t.Run("StartJob with various payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		payloads := []string{
			"{",
			"{}",
			`{"data":"test"}`,
			`{"number":42}`,
			`{"nested":{"key":"value"}}`,
			`{"array":[1,2,3]}`,
			"",
		}

		ctx := context.Background()

		for i, payload := range payloads {
			jobID, err := invoker.StartJob(ctx, "test.job", payload, InvokeOptions{})
			t.Logf("Payload %d (len=%d): jobID=%s, error=%v", i, len(payload), jobID, err)
		}
	})
}

// TestJobOperations_StreamJob tests job streaming scenarios
func TestJobOperations_StreamJob(t *testing.T) {
	t.Run("StreamJob with valid job ID", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// First start a job
		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		if err != nil {
			t.Logf("StartJob failed: %v", err)
		}

		t.Logf("Started job: %s", jobID)

		// Try to stream the job
		eventChan, err := invoker.StreamJob(ctx, jobID)
		t.Logf("StreamJob: error=%v, channel=%v", err, eventChan != nil)

		if eventChan != nil {
			// Try to read from channel with timeout
			select {
			case event, ok := <-eventChan:
				if ok {
					t.Logf("Received event: Type=%s, JobID=%s, Payload=%s, Error=%s",
						event.EventType, event.JobID, event.Payload, event.Error)
				} else {
					t.Log("Event channel closed")
				}
			case <-time.After(time.Second):
				t.Log("No event received within 1 second")
			}
		}
	})

	t.Run("StreamJob with timeout context", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()

		jobID := "test-job-timeout"
		eventChan, err := invoker.StreamJob(ctx, jobID)

		t.Logf("StreamJob with timeout: error=%v, channel=%v", err, eventChan != nil)
	})

	t.Run("StreamJob multiple jobs concurrently", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numJobs = 3

		for i := 0; i < numJobs; i++ {
			jobID := fmt.Sprintf("test-job-%d", i)
			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("StreamJob %d: jobID=%s, error=%v, channel=%v",
				i, jobID, err, eventChan != nil)
		}
	})
}

// TestJobOperations_CancelJob tests job cancellation scenarios
func TestJobOperations_CancelJob(t *testing.T) {
	t.Run("CancelJob immediately after start", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Start a job
		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		t.Logf("Started job: %s, error=%v", jobID, err)

		// Cancel immediately
		err = invoker.CancelJob(ctx, jobID)
		t.Logf("CancelJob result: error=%v", err)
	})

	t.Run("CancelJob with various job IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		jobIDs := []string{
			"",
			"non-existent-job",
			"job-123",
			"job/with/slashes",
			fmt.Sprintf("job-%d", time.Now().UnixNano()),
		}

		for _, jobID := range jobIDs {
			err := invoker.CancelJob(ctx, jobID)
			t.Logf("CancelJob for '%s' (len=%d): error=%v", jobID, len(jobID), err)
		}
	})

	t.Run("CancelJob with context values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		type contextKey string
		ctx := context.WithValue(context.Background(), contextKey("userID"), "user-123")
		ctx = context.WithValue(ctx, contextKey("reason"), "user_request")

		jobID := "test-job-context"
		err := invoker.CancelJob(ctx, jobID)

		t.Logf("CancelJob with context: jobID=%s, error=%v", jobID, err)
	})
}

// TestJobOperations_ErrorScenarios tests job operation error scenarios
func TestJobOperations_ErrorScenarios(t *testing.T) {
	t.Run("StartJob with invalid function IDs", func(t *testing.T) {
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
			"function\nwith\nnewlines",
		}

		ctx := context.Background()

		for _, funcID := range invalidFunctionIDs {
			jobID, err := invoker.StartJob(ctx, funcID, "{}", InvokeOptions{})
			t.Logf("Invalid function ID '%s': jobID=%s, error=%v", funcID, jobID, err)
		}
	})

	t.Run("StartJob with cancelled context", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		t.Logf("StartJob with cancelled context: jobID=%s, error=%v", jobID, err)
	})

	t.Run("Job operations on closed invoker", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		// Close the invoker
		err := invoker.Close()
		t.Logf("Close invoker: %v", err)

		ctx := context.Background()
		jobID := "test-job-closed"

		// Try StartJob
		startedJobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		t.Logf("StartJob on closed invoker: jobID=%s, error=%v", startedJobID, err)

		// Try StreamJob
		eventChan, err := invoker.StreamJob(ctx, jobID)
		t.Logf("StreamJob on closed invoker: error=%v, channel=%v", err, eventChan != nil)

		// Try CancelJob
		err = invoker.CancelJob(ctx, jobID)
		t.Logf("CancelJob on closed invoker: error=%v", err)
	})
}

// TestJobOperations_PerformancePatterns tests performance-related patterns
func TestJobOperations_PerformancePatterns(t *testing.T) {
	t.Run("Rapid job creation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numJobs = 20
		ctx := context.Background()

		start := time.Now()
		successful := 0

		for i := 0; i < numJobs; i++ {
			jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
			if err == nil {
				successful++
				t.Logf("Job %d started: %s", i, jobID)
			}
		}

		duration := time.Since(start)
		t.Logf("Rapid job creation: %d/%d successful in %v (%.2f jobs/sec)",
			successful, numJobs, duration, float64(numJobs)/duration.Seconds())
	})

	t.Run("Concurrent job operations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numGoroutines = 10
		ctx := context.Background()

		start := time.Now()
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer func() { done <- true }()

				jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
				t.Logf("Goroutine %d: jobID=%s, error=%v", idx, jobID, err)

				// Try to cancel
				if jobID != "" {
					_ = invoker.CancelJob(ctx, jobID)
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		duration := time.Since(start)
		t.Logf("Concurrent job operations: %d goroutines completed in %v", numGoroutines, duration)
	})
}

// TestJobOperations_IntegrationPatterns tests integration patterns with jobs
func TestJobOperations_IntegrationPatterns(t *testing.T) {
	t.Run("Invoke and job operations together", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Regular invoke
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Invoke result: len=%d, error=%v", len(result), err)

		// Start job
		jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		t.Logf("StartJob result: jobID=%s, error=%v", jobID, err)

		// Stream job
		eventChan, err := invoker.StreamJob(ctx, jobID)
		t.Logf("StreamJob result: error=%v, channel=%v", err, eventChan != nil)

		// Cancel job
		err = invoker.CancelJob(ctx, jobID)
		t.Logf("CancelJob result: error=%v", err)
	})

	t.Run("Job with idempotency", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		idempotencyKey := fmt.Sprintf("idempotent-job-%d", time.Now().UnixNano())
		opts := InvokeOptions{
			IdempotencyKey: idempotencyKey,
		}

		ctx := context.Background()

		// Start job with idempotency key
		jobID1, err1 := invoker.StartJob(ctx, "test.job", "{}", opts)
		t.Logf("First StartJob: jobID=%s, error=%v", jobID1, err1)

		// Try to start again with same key
		jobID2, err2 := invoker.StartJob(ctx, "test.job", "{}", opts)
		t.Logf("Second StartJob (same key): jobID=%s, error=%v", jobID2, err2)

		// Check if job IDs are the same (idempotency)
		if jobID1 == jobID2 {
			t.Log("Idempotency verified: same job ID returned")
		}
	})
}
