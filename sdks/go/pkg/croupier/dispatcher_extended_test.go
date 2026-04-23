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

// TestDispatcher_enqueueWithData tests EnqueueWithData functionality
func TestDispatcher_enqueueWithData(t *testing.T) {
	t.Run("EnqueueWithData with single item", func(t *testing.T) {
		dispatcher := GetDispatcher()

		data := struct {
			Name  string
			Value int
		}{
			Name:  "test",
			Value: 42,
		}

		EnqueueWithData(dispatcher, func(d struct {
			Name  string
			Value int
		}) {
			// Callback for enqueued data
		}, data)
		dispatcher.Clear()
	})

	t.Run("EnqueueWithData with multiple items", func(t *testing.T) {
		dispatcher := GetDispatcher()

		items := []struct {
			ID    int
			Name  string
		}{
			{ID: 1, Name: "first"},
			{ID: 2, Name: "second"},
			{ID: 3, Name: "third"},
		}

		for _, item := range items {
			EnqueueWithData(dispatcher, func(item struct {
				ID    int
				Name  string
			}) {
				// Callback for enqueued item
			}, item)
		}

		count := dispatcher.GetPendingCount()
		t.Logf("Enqueued %d items, pending count: %d", len(items), count)

		dispatcher.Clear()
	})

	t.Run("EnqueueWithData with various data types", func(t *testing.T) {
		dispatcher := GetDispatcher()

		// Enqueue different data types
		EnqueueWithData(dispatcher, func(s string) {
			// Callback for string
		}, "test string")
		EnqueueWithData(dispatcher, func(i int) {
			// Callback for int
		}, 42)
		EnqueueWithData(dispatcher, func(f float64) {
			// Callback for float
		}, 3.14)
		EnqueueWithData(dispatcher, func(b bool) {
			// Callback for bool
		}, true)
		EnqueueWithData(dispatcher, func(v any) {
			// Callback for nil
		}, nil)
		EnqueueWithData(dispatcher, func(s []int) {
			// Callback for slice
		}, []int{1, 2, 3})
		EnqueueWithData(dispatcher, func(m map[string]int) {
			// Callback for map
		}, map[string]int{"key": 42})

		count := dispatcher.GetPendingCount()
		t.Logf("Enqueued 7 different data types, pending count: %d", count)

		dispatcher.Clear()
	})

	t.Run("EnqueueWithData with struct pointers", func(t *testing.T) {
		dispatcher := GetDispatcher()

		type TestData struct {
			Field1 string
			Field2 int
		}

		data := &TestData{
			Field1: "test",
			Field2: 123,
		}

		EnqueueWithData(dispatcher, func(d *TestData) {
			// Callback for struct pointer
		}, data)

		count := dispatcher.GetPendingCount()
		t.Logf("Enqueued struct pointer, pending count: %d", count)

		dispatcher.Clear()
	})
}

// TestDispatcher_concurrentEnqueueData tests concurrent EnqueueWithData
func TestDispatcher_concurrentEnqueueData(t *testing.T) {
	t.Run("concurrent EnqueueWithData operations", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				data := struct {
					Index int
					Value string
				}{
					Index: idx,
					Value: fmt.Sprintf("value-%d", idx),
				}

				EnqueueWithData(dispatcher, func(d struct {
					Index int
					Value string
				}) {
					// Callback for concurrent data
				}, data)
			}(i)
		}

		wg.Wait()

		count := dispatcher.GetPendingCount()
		t.Logf("Concurrent EnqueueWithData: %d items enqueued", count)

		dispatcher.Clear()
	})

	t.Run("concurrent Enqueue and EnqueueWithData", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				if idx%2 == 0 {
					dispatcher.Enqueue(func() {})
				} else {
					EnqueueWithData(dispatcher, func(i int) {
						// Callback for concurrent data
					}, idx)
				}
			}(i)
		}

		wg.Wait()

		count := dispatcher.GetPendingCount()
		t.Logf("Mixed concurrent operations: %d items pending", count)

		dispatcher.Clear()
	})
}

// TestDispatcher_processQueueTests tests ProcessQueue variations
func TestDispatcher_processQueueTests(t *testing.T) {
	t.Run("ProcessQueueWithLimit with various limits", func(t *testing.T) {
		dispatcher := GetDispatcher()

		// Enqueue some items
		for i := 0; i < 20; i++ {
			dispatcher.Enqueue(func() {})
		}

		// Process with limit
		processed := dispatcher.ProcessQueueWithLimit(5)
		t.Logf("Processed %d items with limit 5", processed)

		remaining := dispatcher.GetPendingCount()
		t.Logf("Remaining items: %d", remaining)

		dispatcher.Clear()
	})

	t.Run("ProcessQueueWithLimit with zero limit", func(t *testing.T) {
		dispatcher := GetDispatcher()

		for i := 0; i < 10; i++ {
			dispatcher.Enqueue(func() {})
		}

		processed := dispatcher.ProcessQueueWithLimit(0)
		t.Logf("Processed with zero limit: %d", processed)

		dispatcher.Clear()
	})

	t.Run("ProcessQueueWithLimit with large limit", func(t *testing.T) {
		dispatcher := GetDispatcher()

		for i := 0; i < 10; i++ {
			dispatcher.Enqueue(func() {})
		}

		processed := dispatcher.ProcessQueueWithLimit(1000)
		t.Logf("Processed with large limit: %d", processed)

		dispatcher.Clear()
	})
}

// TestDispatcher_clearTests tests Clear functionality
func TestDispatcher_clearTests(t *testing.T) {
	t.Run("Clear with empty queue", func(t *testing.T) {
		dispatcher := GetDispatcher()

		count := dispatcher.GetPendingCount()
		t.Logf("Pending count before Clear: %d", count)

		dispatcher.Clear()

		count = dispatcher.GetPendingCount()
		t.Logf("Pending count after Clear: %d", count)
	})

	t.Run("Clear with items in queue", func(t *testing.T) {
		dispatcher := GetDispatcher()

		for i := 0; i < 50; i++ {
			dispatcher.Enqueue(func() {})
		}

		count := dispatcher.GetPendingCount()
		t.Logf("Pending count before Clear: %d", count)

		dispatcher.Clear()

		count = dispatcher.GetPendingCount()
		t.Logf("Pending count after Clear: %d", count)

		if count != 0 {
			t.Errorf("Expected 0 items after Clear, got %d", count)
		}
	})

	t.Run("multiple Clear calls", func(t *testing.T) {
		dispatcher := GetDispatcher()

		for i := 0; i < 10; i++ {
			dispatcher.Enqueue(func() {})
		}

		// Clear multiple times
		dispatcher.Clear()
		dispatcher.Clear()
		dispatcher.Clear()

		count := dispatcher.GetPendingCount()
		t.Logf("Pending count after multiple Clears: %d", count)
	})
}

// TestDispatcher_maxProcessPerFrame tests SetMaxProcessPerFrame
func TestDispatcher_maxProcessPerFrame(t *testing.T) {
	t.Run("SetMaxProcessPerFrame with various values", func(t *testing.T) {
		dispatcher := GetDispatcher()

		limits := []int{1, 5, 10, 50, 100, 1000}

		for _, limit := range limits {
			dispatcher.SetMaxProcessPerFrame(limit)
			t.Logf("Set max process per frame to %d", limit)
		}
	})

	t.Run("SetMaxProcessPerFrame with zero", func(t *testing.T) {
		dispatcher := GetDispatcher()

		dispatcher.SetMaxProcessPerFrame(0)
		t.Log("Set max process per frame to 0")
	})

	t.Run("SetMaxProcessPerFrame with negative value", func(t *testing.T) {
		dispatcher := GetDispatcher()

		dispatcher.SetMaxProcessPerFrame(-1)
		t.Log("Set max process per frame to -1")
	})
}

// TestGoroutineID tests goroutine ID functionality
func TestGoroutineID(t *testing.T) {
	t.Run("getGoroutineID from main goroutine", func(t *testing.T) {
		dispatcher := GetDispatcher()

		isMain := dispatcher.IsMainGoroutine()
		t.Logf("IsMainGoroutine in test: %v", isMain)
	})

	t.Run("getGoroutineID from spawned goroutine", func(t *testing.T) {
		dispatcher := GetDispatcher()

		var result bool
		done := make(chan bool)

		go func() {
			result = dispatcher.IsMainGoroutine()
			done <- true
		}()

		<-done
		t.Logf("IsMainGoroutine in spawned goroutine: %v", result)
	})

	t.Run("multiple goroutines check IsMainGoroutine", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const numGoroutines = 10
		results := make([]bool, numGoroutines)
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = dispatcher.IsMainGoroutine()
			}(i)
		}

		wg.Wait()

		mainCount := 0
		for _, isMain := range results {
			if isMain {
				mainCount++
			}
		}

		t.Logf("IsMainGoroutine results: %d/%d reported as main", mainCount, numGoroutines)
	})
}

// TestDispatcher_initializationTests tests initialization
func TestDispatcher_initializationTests(t *testing.T) {
	t.Run("GetDispatcher returns same instance", func(t *testing.T) {
		d1 := GetDispatcher()
		d2 := GetDispatcher()

		if d1 != d2 {
			t.Error("GetDispatcher should return same instance")
		}
	})

	t.Run("IsInitialized", func(t *testing.T) {
		dispatcher := GetDispatcher()

		initialized := dispatcher.IsInitialized()
		t.Logf("Dispatcher initialized: %v", initialized)
	})

	t.Run("Initialize multiple times", func(t *testing.T) {
		dispatcher := GetDispatcher()

		dispatcher.Initialize()
		dispatcher.Initialize()
		dispatcher.Initialize()

		t.Log("Multiple Initialize calls completed")
	})

	t.Run("ResetDispatcher", func(t *testing.T) {
		// Enqueue some items
		dispatcher := GetDispatcher()
		for i := 0; i < 10; i++ {
			dispatcher.Enqueue(func() {})
		}

		count := dispatcher.GetPendingCount()
		t.Logf("Pending count before Reset: %d", count)

		// Reset
		ResetDispatcher()

		count = dispatcher.GetPendingCount()
		t.Logf("Pending count after Reset: %d", count)
	})
}

// TestDispatcher_performanceTests tests dispatcher performance
func TestDispatcher_performanceTests(t *testing.T) {
	t.Run("rapid Enqueue operations", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const iterations = 1000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			dispatcher.Enqueue(func() {})
		}

		elapsed := time.Since(start)
		t.Logf("Enqueued %d items in %v (%.2f µs per item)",
			iterations, elapsed, float64(elapsed.Microseconds())/float64(iterations))

		dispatcher.Clear()
	})

	t.Run("rapid EnqueueWithData operations", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const iterations = 1000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			EnqueueWithData(dispatcher, func(val int) {
				// Callback for rapid enqueues
			}, i)
		}

		elapsed := time.Since(start)
		t.Logf("EnqueueWithData %d items in %v (%.2f µs per item)",
			iterations, elapsed, float64(elapsed.Microseconds())/float64(iterations))

		dispatcher.Clear()
	})

	t.Run("ProcessQueueWithLimit performance", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const iterations = 1000
		for i := 0; i < iterations; i++ {
			dispatcher.Enqueue(func() {})
		}

		start := time.Now()
		processed := dispatcher.ProcessQueueWithLimit(1000)
		elapsed := time.Since(start)

		t.Logf("Processed %d items in %v (%.2f µs per item)",
			processed, elapsed, float64(elapsed.Microseconds())/float64(processed))
	})
}

// TestParseJSONPayload_extremeCases tests extreme JSON parsing cases
func TestParseJSONPayload_extremeCases(t *testing.T) {
	t.Run("parseJSONPayload with very large numbers", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		largeNums := []string{
			`{"num": 999999999999999999999}`,
			`{"num": -999999999999999999999}`,
			`{"num": 1.7976931348623157e+308}`,  // Max float64
			`{"num": -1.7976931348623157e+308}`,
			`{"num": 5e-324}`,                // Min float64
		}

		ctx := context.Background()
		for _, payload := range largeNums {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with large number '%s' error: %v", payload, err)
		}
	})

	t.Run("parseJSONPayload with escape sequences", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		escapedPayloads := []string{
			`{"text": "Line1\\nLine2\\tTabbed"}`,
			`{"path": "C:\\\\Users\\\\test"}`,
			`{"quote": "He said \\"hello\\""}`,
			`{"backslash": "Path\\\\to\\\\file"}`,
			`{"unicode": "\\u0048\\u0065\\u006C\\u006C\\u006F"}`,
		}

		ctx := context.Background()
		for _, payload := range escapedPayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with escaped '%s' error: %v", payload, err)
		}
	})

	t.Run("parseJSONPayload with whitespace variations", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		whitespacePayloads := []string{
			`{"key":"value"}`,
			` { "key" : "value" } `,
			`
			{
				"key": "value"
			}`,
			`{"key":"value"}   `,
			`  {"key":"value"}  `,
		}

		ctx := context.Background()
		for _, payload := range whitespacePayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with whitespace payload error: %v", err)
		}
	})
}
