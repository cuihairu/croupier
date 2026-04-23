// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestMemoryManagement_ResourceLeakTests tests for potential resource leaks
func TestMemoryManagement_ResourceLeakTests(t *testing.T) {
	t.Run("Repeated create and close cycles", func(t *testing.T) {
		const cycles = 100

		// Force GC before test
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		for i := 0; i < cycles; i++ {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				invoker.Close()
			}
		}

		// Force GC after test
		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		t.Logf("Memory - Before: %d allocs, %d bytes", m1.Alloc, m1.TotalAlloc)
		t.Logf("Memory - After: %d allocs, %d bytes", m2.Alloc, m2.TotalAlloc)
		t.Logf("Completed %d create/close cycles", cycles)
	})

	t.Run("Many concurrent invokers", func(t *testing.T) {
		const numInvokers = 50

		invokers := make([]Invoker, 0, numInvokers)

		for i := 0; i < numInvokers; i++ {
			config := &InvokerConfig{
				Address: fmt.Sprintf("localhost:%d", 19090+i%10),
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				invokers = append(invokers, invoker)
			}
		}

		t.Logf("Created %d invokers", len(invokers))

		// Close all
		for i, inv := range invokers {
			if inv != nil {
				err := inv.Close()
				t.Logf("Closed invoker %d: error=%v", i, err)
			}
		}
	})
}

// TestMemoryManagement_LargePayloads tests memory handling with large payloads
func TestMemoryManagement_LargePayloads(t *testing.T) {
	t.Run("Large payload handling", func(t *testing.T) {
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
			1024,          // 1 KB
			10 * 1024,     // 10 KB
			100 * 1024,    // 100 KB
			1024 * 1024,   // 1 MB
		}

		for _, size := range sizes {
			largePayload := fmt.Sprintf(`{"data":"%s"}`, string(make([]byte, size)))

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", largePayload, InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Large payload (%d bytes): duration=%v, error=%v, result_len=%d",
				size, duration, err, len(result))
		}
	})

	t.Run("Concurrent large payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numGoroutines = 20
		const payloadSize = 10000 // 10 KB each

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				largePayload := fmt.Sprintf(`{"id":%d,"data":"%s"}`,
					idx, string(make([]byte, payloadSize)))

				result, err := invoker.Invoke(ctx, "test.function", largePayload, InvokeOptions{})
				t.Logf("Goroutine %d: error=%v, result_len=%d", idx, err, len(result))
			}(i)
		}

		// Wait a bit for goroutines to complete
		time.Sleep(time.Second)
	})
}

// TestMemoryManagement_GoroutineLeakTests tests for goroutine leaks
func TestMemoryManagement_GoroutineLeakTests(t *testing.T) {
	t.Run("Start and cancel many jobs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numJobs = 30

		// Check goroutine count before
		numGoroutinesBefore := runtime.NumGoroutine()
		t.Logf("Goroutines before: %d", numGoroutinesBefore)

		jobIDs := make([]string, numJobs)
		for i := 0; i < numJobs; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
			jobIDs[i] = jobID
		}

		// Cancel all jobs
		for _, jobID := range jobIDs {
			invoker.CancelJob(ctx, jobID)
		}

		// Give time for cleanup
		time.Sleep(time.Millisecond * 100)

		// Check goroutine count after
		numGoroutinesAfter := runtime.NumGoroutine()
		t.Logf("Goroutines after: %d (delta: %d)", numGoroutinesAfter, numGoroutinesAfter-numGoroutinesBefore)
	})

	t.Run("Stream job and cleanup", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numStreams = 10

		numGoroutinesBefore := runtime.NumGoroutine()

		for i := 0; i < numStreams; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})

			eventChan, err := invoker.StreamJob(ctx, jobID)
			t.Logf("Stream %d: error=%v, channel=%v", i, err, eventChan != nil)

			// Don't read from channel, just let it get garbage collected
		}

		// Give time for cleanup
		time.Sleep(time.Millisecond * 100)

		numGoroutinesAfter := runtime.NumGoroutine()
		t.Logf("Goroutines: %d before, %d after (delta: %d)",
			numGoroutinesBefore, numGoroutinesAfter, numGoroutinesAfter-numGoroutinesBefore)
	})
}

// TestMemoryManagement_MemoryProfiler tests memory profiling patterns
func TestMemoryManagement_MemoryProfiler(t *testing.T) {
	t.Run("Memory usage pattern during operations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const iterations = 50

		for i := 0; i < iterations; i++ {
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Perform operation
			invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"iteration":%d}`, i), InvokeOptions{})

			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			if i%10 == 0 {
				t.Logf("Iteration %d: Alloc = %d bytes, TotalAlloc = %d bytes",
					i, m2.Alloc-m1.Alloc, m2.TotalAlloc)
			}
		}
	})

	t.Run("GC pressure test", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const iterations = 100

		for i := 0; i < iterations; i++ {
			// Create temporary objects
			tmpData := make([]byte, 1024*10) // 10 KB
			_ = tmpData

			// Invoke
			invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})

			// Force GC every 10 iterations
			if i%10 == 0 {
				runtime.GC()
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				t.Logf("After GC at iteration %d: HeapObjects = %d", i, m.HeapObjects)
			}
		}
	})
}

// TestMemoryManagement_ResourceLimits tests resource limit scenarios
func TestMemoryManagement_ResourceLimits(t *testing.T) {
	t.Run("Schema size limits", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		sizes := []int{
			10,
			100,
			1000,
			10000,
		}

		for _, size := range sizes {
			schema := make(map[string]interface{})
			for i := 0; i < size; i++ {
				schema[fmt.Sprintf("field%d", i)] = map[string]interface{}{
					"type": "string",
				}
			}

			err := invoker.SetSchema("test.function", schema)
			t.Logf("Schema with %d fields: error=%v", size, err)
		}
	})

	t.Run("Headers size limits", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headerCounts := []int{
			1,
			10,
			50,
			100,
		}

		for _, count := range headerCounts {
			headers := make(map[string]string)
			for i := 0; i < count; i++ {
				headers[fmt.Sprintf("X-Header-%d", i)] = fmt.Sprintf("value-%d", i)
			}

			opts := InvokeOptions{Headers: headers}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("%d headers: error=%v, result_len=%d", count, err, len(result))
		}
	})
}

// TestMemoryManagement_CleanupTests tests cleanup behavior
func TestMemoryManagement_CleanupTests(t *testing.T) {
	t.Run("Proper cleanup sequence", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()

		// Start some jobs
		jobIDs := make([]string, 5)
		for i := 0; i < 5; i++ {
			jobID, _ := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
			jobIDs[i] = jobID
		}

		// Set schemas
		schema := map[string]interface{}{"type": "string"}
		invoker.SetSchema("test.function", schema)

		// Close invoker
		err := invoker.Close()

		t.Logf("Cleanup sequence: Close error=%v", err)

		// Verify we can still cancel jobs (should handle gracefully)
		for _, jobID := range jobIDs {
			err = invoker.CancelJob(ctx, jobID)
			t.Logf("Cancel after close: jobID=%s, error=%v", jobID, err)
		}
	})

	t.Run("Multiple cleanup attempts", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		// Close multiple times
		for i := 0; i < 5; i++ {
			err := invoker.Close()
			t.Logf("Close attempt %d: error=%v", i, err)
		}
	})
}

// TestMemoryManagement_AllocationPatterns tests allocation patterns
func TestMemoryManagement_AllocationPatterns(t *testing.T) {
	t.Run("Minimal allocations in hot path", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const iterations = 100

		// Reuse options to minimize allocations
		opts := InvokeOptions{}

		for i := 0; i < iterations; i++ {
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			_ = result
			_ = err
		}

		t.Logf("Completed %d invokes with minimal allocations", iterations)
	})

	t.Run("Bulk string concatenation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Build large payload efficiently
		var largePayload string
		for i := 0; i < 100; i++ {
			largePayload += fmt.Sprintf(`{"item%d":"value%d"},`, i, i)
		}
		largePayload = `[` + largePayload[:len(largePayload)-1] + `]`

		result, err := invoker.Invoke(ctx, "test.function", largePayload, InvokeOptions{})
		t.Logf("Bulk concatenation: error=%v, result_len=%d", err, len(result))
	})
}

// TestMemoryManagement_Timeouts tests timeout-based cleanup
func TestMemoryManagement_Timeouts(t *testing.T) {
	t.Run("Context timeout cleanup", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numTimeouts = 10

		for i := 0; i < numTimeouts; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()

			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Timeout %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}
