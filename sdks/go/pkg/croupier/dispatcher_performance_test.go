// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDispatcher_performance_benchmarks tests dispatcher performance
func TestDispatcher_performance_benchmarks(t *testing.T) {
	t.Run("Enqueue performance", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		const iterations = 10000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			dispatcher.Enqueue(func() {})
		}

		elapsed := time.Since(start)
		opsPerSec := float64(iterations) / elapsed.Seconds()

		t.Logf("Enqueued %d items in %v (%.0f ops/sec)", iterations, elapsed, opsPerSec)

		if opsPerSec < 10000 {
			t.Errorf("Enqueue performance too low: %.0f ops/sec, expected > 10000", opsPerSec)
		}
	})

	t.Run("EnqueueWithData performance", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		const iterations = 10000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			EnqueueWithData(dispatcher, func(val int) {
				// Callback
			}, i)
		}

		elapsed := time.Since(start)
		opsPerSec := float64(iterations) / elapsed.Seconds()

		t.Logf("EnqueueWithData %d items in %v (%.0f ops/sec)", iterations, elapsed, opsPerSec)
	})

	t.Run("ProcessQueueWithLimit performance", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const iterations = 10000
		for i := 0; i < iterations; i++ {
			dispatcher.Enqueue(func() {})
		}

		start := time.Now()
		processed := dispatcher.ProcessQueueWithLimit(10000)
		elapsed := time.Since(start)

		opsPerSec := float64(processed) / elapsed.Seconds()

		t.Logf("Processed %d items in %v (%.0f ops/sec)", processed, elapsed, opsPerSec)
	})
}

// TestDispatcher_concurrent_stress tests dispatcher under concurrent stress
func TestDispatcher_concurrent_stress(t *testing.T) {
	t.Run("Concurrent Enqueue from many goroutines", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		const numGoroutines = 100
		const itemsPerGoroutine = 1000
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < itemsPerGoroutine; j++ {
					dispatcher.Enqueue(func() {})
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		totalItems := numGoroutines * itemsPerGoroutine
		opsPerSec := float64(totalItems) / elapsed.Seconds()

		t.Logf("Concurrent Enqueue: %d goroutines × %d items = %d items in %v (%.0f ops/sec)",
			numGoroutines, itemsPerGoroutine, totalItems, elapsed, opsPerSec)
	})

	t.Run("Concurrent Enqueue and Process", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const numGoroutines = 50
		const itemsPerGoroutine = 100
		var wg sync.WaitGroup

		start := time.Now()
		var processedCount int32

		// Producers
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < itemsPerGoroutine; j++ {
					dispatcher.Enqueue(func() {
						atomic.AddInt32(&processedCount, 1)
					})
				}
			}()
		}

		// Consumers
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < itemsPerGoroutine; k++ {
					dispatcher.ProcessQueueWithLimit(100)
					time.Sleep(1 * time.Millisecond)
				}
			}()
		}

		wg.Wait()
		elapsed := time.Since(start)

		t.Logf("Concurrent Enqueue+Process: completed in %v, processed %d items",
			elapsed, atomic.LoadInt32(&processedCount))
	})

	t.Run("Concurrent Clear operations", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const numGoroutines = 20
		const iterations = 100
		var wg sync.WaitGroup

		// First populate the queue
		for i := 0; i < 10000; i++ {
			dispatcher.Enqueue(func() {})
		}

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					dispatcher.Clear()
				}
			}()
		}

		wg.Wait()
		elapsed := time.Since(start)

		t.Logf("Concurrent Clear: %d goroutines × %d iterations in %v",
			numGoroutines, iterations, elapsed)
	})
}

// TestDispatcher_memory_tests tests memory behavior
func TestDispatcher_memory_tests(t *testing.T) {
	t.Run("Large queue memory usage", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer ResetDispatcher()

		const largeItemCount = 100000

		t.Logf("Enqueueing %d items...", largeItemCount)
		for i := 0; i < largeItemCount; i++ {
			dispatcher.Enqueue(func() {})
		}

		count := dispatcher.GetPendingCount()
		t.Logf("Queue contains %d items", count)

		if count != largeItemCount {
			t.Errorf("Expected %d items, got %d", largeItemCount, count)
		}

		// Process all
		start := time.Now()
		processed := dispatcher.ProcessQueueWithLimit(largeItemCount)
		elapsed := time.Since(start)

		t.Logf("Processed %d items in %v", processed, elapsed)
	})

	t.Run("Rapid Enqueue and Clear cycles", func(t *testing.T) {
		dispatcher := GetDispatcher()

		const cycles = 1000
		const itemsPerCycle = 100

		start := time.Now()

		for i := 0; i < cycles; i++ {
			// Enqueue items
			for j := 0; j < itemsPerCycle; j++ {
				dispatcher.Enqueue(func() {})
			}

			// Clear
			dispatcher.Clear()
		}

		elapsed := time.Since(start)
		avgCycleTime := elapsed / time.Duration(cycles)

		t.Logf("Rapid Enqueue+Clear: %d cycles in %v (avg %v per cycle)",
			cycles, elapsed, avgCycleTime)
	})
}

// TestDispatcher_limits tests dispatcher limits and boundaries
func TestDispatcher_limits(t *testing.T) {
	t.Run("SetMaxProcessPerFrame with various limits", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		// Enqueue many items
		for i := 0; i < 1000; i++ {
			dispatcher.Enqueue(func() {})
		}

		limits := []int{1, 10, 50, 100, 500, 1000}

		for _, limit := range limits {
			dispatcher.SetMaxProcessPerFrame(limit)

			// Re-populate queue
			for i := 0; i < 1000; i++ {
				dispatcher.Enqueue(func() {})
			}

			start := time.Now()
			processed := dispatcher.ProcessQueueWithLimit(10000)
			elapsed := time.Since(start)

			t.Logf("Limit=%d: processed %d items in %v", limit, processed, elapsed)
		}
	})

	t.Run("ProcessQueueWithLimit boundary values", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		testCases := []struct {
			name      string
			queueSize int
			limit     int
		}{
			{"Zero limit", 100, 0},
			{"Limit greater than queue", 10, 100},
			{"Limit equals queue", 50, 50},
			{"Limit half of queue", 100, 50},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Populate queue
				for i := 0; i < tc.queueSize; i++ {
					dispatcher.Enqueue(func() {})
				}

				processed := dispatcher.ProcessQueueWithLimit(tc.limit)
				t.Logf("Queue=%d, Limit=%d, Processed=%d", tc.queueSize, tc.limit, processed)

				dispatcher.Clear()
			})
		}
	})
}

// TestDispatcher_realistic_workloads tests realistic usage patterns
func TestDispatcher_realistic_workloads(t *testing.T) {
	t.Run("Burst workload", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		const burstSize = 1000
		const numBursts = 10

		for burst := 0; burst < numBursts; burst++ {
			// Burst of items
			for i := 0; i < burstSize; i++ {
				dispatcher.Enqueue(func() {})
			}

			// Process some
			dispatcher.ProcessQueueWithLimit(burstSize / 2)
		}

		remaining := dispatcher.GetPendingCount()
		t.Logf("After %d bursts of %d items: %d remaining", numBursts, burstSize, remaining)
	})

	t.Run("Sustained load", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		const duration = 2 * time.Second
		const ratePerSec = 1000

		start := time.Now()
		var enqueued int32

		// Producer
		stopCh := make(chan bool)
		go func() {
			ticker := time.NewTicker(time.Second / time.Duration(ratePerSec))
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					dispatcher.Enqueue(func() {})
					atomic.AddInt32(&enqueued, 1)
				case <-stopCh:
					return
				}
			}
		}()

		// Consumer
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					dispatcher.ProcessQueueWithLimit(500)
				case <-stopCh:
					return
				}
			}
		}()

		time.Sleep(duration)
		close(stopCh)

		elapsed := time.Since(start)
		totalEnqueued := atomic.LoadInt32(&enqueued)
		actualRate := float64(totalEnqueued) / elapsed.Seconds()

		t.Logf("Sustained load: %d items in %v (%.0f items/sec)",
			totalEnqueued, elapsed, actualRate)
	})
}

// TestDispatcher_callback_execution tests callback execution behavior
func TestDispatcher_callback_execution(t *testing.T) {
	t.Run("Callback execution order", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		const itemCount = 100
		order := make([]int, 0, itemCount)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < itemCount; i++ {
			idx := i
			wg.Add(1)
			dispatcher.Enqueue(func() {
				defer wg.Done()
				mu.Lock()
				order = append(order, idx)
				mu.Unlock()
			})
		}

		dispatcher.ProcessQueueWithLimit(itemCount)
		wg.Wait()

		t.Logf("Processed %d callbacks", len(order))
	})

	t.Run("Callback panic recovery", func(t *testing.T) {
		dispatcher := GetDispatcher()
		defer dispatcher.Clear()

		// Enqueue a callback that panics
		dispatcher.Enqueue(func() {
			panic("intentional panic for testing")
		})

		// Enqueue normal callbacks
		for i := 0; i < 10; i++ {
			dispatcher.Enqueue(func() {
				// Normal callback
			})
		}

		// Process - should not crash
		dispatcher.ProcessQueueWithLimit(20)
		t.Log("Processed panicking callback without crashing")
	})
}

// TestDispatcher_multiple_instances tests multiple dispatcher instances
func TestDispatcher_multiple_instances(t *testing.T) {
	t.Run("GetDispatcher always returns same instance", func(t *testing.T) {
		d1 := GetDispatcher()
		d2 := GetDispatcher()
		d3 := GetDispatcher()

		if d1 != d2 || d2 != d3 {
			t.Error("GetDispatcher should return same instance")
		}
	})

	t.Run("ResetDispatcher creates fresh state", func(t *testing.T) {
		dispatcher := GetDispatcher()

		// Add items
		for i := 0; i < 100; i++ {
			dispatcher.Enqueue(func() {})
		}

		count1 := dispatcher.GetPendingCount()

		// Reset
		ResetDispatcher()

		dispatcher2 := GetDispatcher()
		count2 := dispatcher2.GetPendingCount()

		t.Logf("Before reset: %d items, After reset: %d items", count1, count2)

		if count2 != 0 {
			t.Errorf("After reset, expected 0 items, got %d", count2)
		}
	})
}

// TestDispatcher_isInitialized tests initialization state
func TestDispatcher_isInitialized(t *testing.T) {
	t.Run("IsInitialized after GetDispatcher", func(t *testing.T) {
		dispatcher := GetDispatcher()
		initialized := dispatcher.IsInitialized()

		t.Logf("Dispatcher initialized: %v", initialized)
	})

	t.Run("Multiple Initialize calls", func(t *testing.T) {
		dispatcher := GetDispatcher()

		// Call Initialize multiple times
		for i := 0; i < 10; i++ {
			dispatcher.Initialize()
		}

		// Should still work
		dispatcher.Enqueue(func() {})
		count := dispatcher.GetPendingCount()

		t.Logf("After multiple Initialize calls, queue has %d items", count)
	})
}
