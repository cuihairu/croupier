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

// TestTaskOperations_BasicScenarios tests basic task operation scenarios
func TestTaskOperations_BasicScenarios(t *testing.T) {
	t.Run("StartTask with minimal configuration", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})

		t.Logf("StartTask result: taskID=%s, error=%v", taskID, err)
	})

	t.Run("StartTask with custom options", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		opts := InvokeOptions{
			IdempotencyKey: fmt.Sprintf("task-key-%d", time.Now().UnixNano()),
			Timeout:        30 * 1000 * 1000 * 1000, // 30s
			Headers: map[string]string{
				"X-Task-Type":  "test",
				"X-Priority":   "high",
				"X-Attempt":    "1",
				"X-Request-ID": fmt.Sprintf("req-%d", time.Now().UnixNano()),
			},
		}

		ctx := context.Background()
		taskID, err := invoker.StartTask(ctx, "test.task.withOptions", "{}", opts)

		t.Logf("StartTask with options: taskID=%s, error=%v", taskID, err)
	})

	t.Run("StartTask with various payloads", func(t *testing.T) {
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
			taskID, err := invoker.StartTask(ctx, "test.task", payload, InvokeOptions{})
			t.Logf("Payload %d (len=%d): taskID=%s, error=%v", i, len(payload), taskID, err)
		}
	})
}

// TestTaskOperations_StreamTask tests task streaming scenarios
func TestTaskOperations_StreamTask(t *testing.T) {
	t.Run("StreamTask with valid task ID", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// First start a task
		taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
		if err != nil {
			t.Logf("StartTask failed: %v", err)
		}

		t.Logf("Started task: %s", taskID)

		// Try to stream the task
		eventChan, err := invoker.StreamTask(ctx, taskID)
		t.Logf("StreamTask: error=%v, channel=%v", err, eventChan != nil)

		if eventChan != nil {
			// Try to read from channel with timeout
			select {
			case event, ok := <-eventChan:
				if ok {
					t.Logf("Received event: Type=%s, TaskID=%s, Payload=%s, Error=%s",
						event.EventType, event.TaskID, event.Payload, event.Error)
				} else {
					t.Log("Event channel closed")
				}
			case <-time.After(time.Second):
				t.Log("No event received within 1 second")
			}
		}
	})

	t.Run("StreamTask with timeout context", func(t *testing.T) {
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

		taskID := "test-task-timeout"
		eventChan, err := invoker.StreamTask(ctx, taskID)

		t.Logf("StreamTask with timeout: error=%v, channel=%v", err, eventChan != nil)
	})

	t.Run("StreamTask multiple tasks concurrently", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numTasks = 3

		for i := 0; i < numTasks; i++ {
			taskID := fmt.Sprintf("test-task-%d", i)
			eventChan, err := invoker.StreamTask(ctx, taskID)
			t.Logf("StreamTask %d: taskID=%s, error=%v, channel=%v",
				i, taskID, err, eventChan != nil)
		}
	})
}

// TestTaskOperations_CancelTask tests task cancellation scenarios
func TestTaskOperations_CancelTask(t *testing.T) {
	t.Run("CancelTask immediately after start", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Start a task
		taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
		t.Logf("Started task: %s, error=%v", taskID, err)

		// Cancel immediately
		err = invoker.CancelTask(ctx, taskID)
		t.Logf("CancelTask result: error=%v", err)
	})

	t.Run("CancelTask with various task IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		taskIDs := []string{
			"",
			"non-existent-task",
			"task-123",
			"task/with/slashes",
			fmt.Sprintf("task-%d", time.Now().UnixNano()),
		}

		for _, taskID := range taskIDs {
			err := invoker.CancelTask(ctx, taskID)
			t.Logf("CancelTask for '%s' (len=%d): error=%v", taskID, len(taskID), err)
		}
	})

	t.Run("CancelTask with context values", func(t *testing.T) {
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

		taskID := "test-task-context"
		err := invoker.CancelTask(ctx, taskID)

		t.Logf("CancelTask with context: taskID=%s, error=%v", taskID, err)
	})
}

// TestTaskOperations_ErrorScenarios tests task operation error scenarios
func TestTaskOperations_ErrorScenarios(t *testing.T) {
	t.Run("StartTask with invalid function IDs", func(t *testing.T) {
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
			taskID, err := invoker.StartTask(ctx, funcID, "{}", InvokeOptions{})
			t.Logf("Invalid function ID '%s': taskID=%s, error=%v", funcID, taskID, err)
		}
	})

	t.Run("StartTask with cancelled context", func(t *testing.T) {
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

		taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
		t.Logf("StartTask with cancelled context: taskID=%s, error=%v", taskID, err)
	})

	t.Run("Task operations on closed invoker", func(t *testing.T) {
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
		taskID := "test-task-closed"

		// Try StartTask
		startedTaskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
		t.Logf("StartTask on closed invoker: taskID=%s, error=%v", startedTaskID, err)

		// Try StreamTask
		eventChan, err := invoker.StreamTask(ctx, taskID)
		t.Logf("StreamTask on closed invoker: error=%v, channel=%v", err, eventChan != nil)

		// Try CancelTask
		err = invoker.CancelTask(ctx, taskID)
		t.Logf("CancelTask on closed invoker: error=%v", err)
	})
}

// TestTaskOperations_PerformancePatterns tests performance-related patterns
func TestTaskOperations_PerformancePatterns(t *testing.T) {
	t.Run("Rapid task creation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numTasks = 20
		ctx := context.Background()

		start := time.Now()
		successful := 0

		for i := 0; i < numTasks; i++ {
			taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
			if err == nil {
				successful++
				t.Logf("Task %d started: %s", i, taskID)
			}
		}

		duration := time.Since(start)
		t.Logf("Rapid task creation: %d/%d successful in %v (%.2f tasks/sec)",
			successful, numTasks, duration, float64(numTasks)/duration.Seconds())
	})

	t.Run("Concurrent task operations", func(t *testing.T) {
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

				taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
				t.Logf("Goroutine %d: taskID=%s, error=%v", idx, taskID, err)

				// Try to cancel
				if taskID != "" {
					_ = invoker.CancelTask(ctx, taskID)
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		duration := time.Since(start)
		t.Logf("Concurrent task operations: %d goroutines completed in %v", numGoroutines, duration)
	})
}

// TestTaskOperations_IntegrationPatterns tests integration patterns with tasks
func TestTaskOperations_IntegrationPatterns(t *testing.T) {
	t.Run("Invoke and task operations together", func(t *testing.T) {
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

		// Start task
		taskID, err := invoker.StartTask(ctx, "test.task", "{}", InvokeOptions{})
		t.Logf("StartTask result: taskID=%s, error=%v", taskID, err)

		// Stream task
		eventChan, err := invoker.StreamTask(ctx, taskID)
		t.Logf("StreamTask result: error=%v, channel=%v", err, eventChan != nil)

		// Cancel task
		err = invoker.CancelTask(ctx, taskID)
		t.Logf("CancelTask result: error=%v", err)
	})

	t.Run("Task with idempotency", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		idempotencyKey := fmt.Sprintf("idempotent-task-%d", time.Now().UnixNano())
		opts := InvokeOptions{
			IdempotencyKey: idempotencyKey,
		}

		ctx := context.Background()

		// Start task with idempotency key
		taskID1, err1 := invoker.StartTask(ctx, "test.task", "{}", opts)
		t.Logf("First StartTask: taskID=%s, error=%v", taskID1, err1)

		// Try to start again with same key
		taskID2, err2 := invoker.StartTask(ctx, "test.task", "{}", opts)
		t.Logf("Second StartTask (same key): taskID=%s, error=%v", taskID2, err2)

		// Check if task IDs are the same (idempotency)
		if taskID1 == taskID2 {
			t.Log("Idempotency verified: same task ID returned")
		}
	})
}
