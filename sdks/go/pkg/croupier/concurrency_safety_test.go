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

// TestConcurrencySafety_RaceConditions tests for race conditions
func TestConcurrencySafety_RaceConditions(t *testing.T) {
	t.Run("Concurrent invoker creation", func(t *testing.T) {
		const numGoroutines = 100

		var wg sync.WaitGroup
		var successCount int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				config := &InvokerConfig{
					Address: fmt.Sprintf("localhost:%d", 19090+idx%10),
				}

				invoker := NewHTTPInvoker(config)
				if invoker != nil {
					defer invoker.Close()
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Created %d/%d invokers concurrently", successCount, numGoroutines)
	})

	t.Run("Concurrent invoke calls", func(t *testing.T) {
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

		var wg sync.WaitGroup
		var successCount, errorCount int32

		for i := 0; i < numInvokes; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&errorCount, 1)
				}
				t.Logf("Invoke %d: result_len=%d, error=%v", idx, len(result), err)
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent invokes: %d success, %d errors", successCount, errorCount)
	})

	t.Run("Concurrent close calls", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		const numCloseCalls = 50
		var wg sync.WaitGroup

		for i := 0; i < numCloseCalls; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := invoker.Close()
				t.Logf("Close call %d: error=%v", idx, err)
			}(i)
		}

		wg.Wait()
		t.Logf("Completed %d concurrent close calls", numCloseCalls)
	})
}

// TestConcurrencySafety_SharedState tests shared state safety
func TestConcurrencySafety_SharedState(t *testing.T) {
	t.Run("Shared invoker with state changes", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const operations = 200

		var wg sync.WaitGroup
		var invokeCount, schemaCount int32

		for i := 0; i < operations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				switch idx % 3 {
				case 0:
					// Invoke
					result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
					t.Logf("Op %d (invoke): result_len=%d, error=%v", idx, len(result), err)
					atomic.AddInt32(&invokeCount, 1)
				case 1:
					// SetSchema
					schema := map[string]interface{}{"type": "string"}
					err := invoker.SetSchema("test.function", schema)
					t.Logf("Op %d (schema): error=%v", idx, err)
					atomic.AddInt32(&schemaCount, 1)
				case 2:
					// StartJob
					jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
					t.Logf("Op %d (job): jobID=%s, error=%v", idx, jobID, err)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("State changes: %d invokes, %d schemas", invokeCount, schemaCount)
	})
}

// TestConcurrencySafety_ResourceContention tests resource contention scenarios
func TestConcurrencySafety_ResourceContention(t *testing.T) {
	t.Run("High contention on single invoker", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 2,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numGoroutines = 200

		start := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"id":%d}`, idx), InvokeOptions{})
				t.Logf("Goroutine %d: error=%v, result_len=%d", idx, err, len(result))
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("High contention: %d goroutines in %v (%.2f ops/sec)",
			numGoroutines, duration, float64(numGoroutines)/duration.Seconds())
	})

	t.Run("Multiple invokers with shared configuration", func(t *testing.T) {
		baseConfig := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		const numInvokers = 20
		invokers := make([]Invoker, numInvokers)

		// Create multiple invokers
		for i := 0; i < numInvokers; i++ {
			invokers[i] = NewHTTPInvoker(baseConfig)
		}

		ctx := context.Background()
		var wg sync.WaitGroup

		// Use all invokers concurrently
		for i := 0; i < numInvokers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				if invokers[idx] != nil {
					result, err := invokers[idx].Invoke(ctx, "test.function", "{}", InvokeOptions{})
					t.Logf("Invoker %d: error=%v, result_len=%d", idx, err, len(result))
					invokers[idx].Close()
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Completed %d concurrent invokers", numInvokers)
	})
}

// TestConcurrencySafety_DeadlockScenarios tests potential deadlock scenarios
func TestConcurrencySafety_DeadlockScenarios(t *testing.T) {
	t.Run("Concurrent close and invoke", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}

		ctx := context.Background()
		const numOperations = 50
		var wg sync.WaitGroup

		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				if idx%2 == 0 {
					// Invoke
					result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
					t.Logf("Concurrent invoke %d: error=%v, result_len=%d", idx, err, len(result))
				} else {
					// Close
					err := invoker.Close()
					t.Logf("Concurrent close %d: error=%v", idx, err)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Completed mixed close/invoke operations")
	})

	t.Run("Bidirectional operations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numOps = 30
		var wg sync.WaitGroup

		// StartJob and CancelJob concurrently
		for i := 0; i < numOps; i++ {
			wg.Add(2)

			// StartJob goroutine
			go func(idx int) {
				defer wg.Done()
				jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
				t.Logf("StartJob %d: jobID=%s, error=%v", idx, jobID, err)
			}(i)

			// CancelJob goroutine
			go func(idx int) {
				defer wg.Done()
				jobID := fmt.Sprintf("job-%d", idx)
				err := invoker.CancelJob(ctx, jobID)
				t.Logf("CancelJob %d: error=%v", idx, err)
			}(i)
		}

		wg.Wait()
		t.Logf("Completed bidirectional job operations")
	})
}

// TestConcurrencySafety_MemorySafety tests memory safety under concurrency
func TestConcurrencySafety_MemorySafety(t *testing.T) {
	t.Run("No data races on options", func(t *testing.T) {
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

		// Shared options struct (potential data race)
		sharedOpts := InvokeOptions{
			Headers: map[string]string{
				"X-Shared": "value",
			},
		}

		var wg sync.WaitGroup
		for i := 0; i < numInvokes; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Each goroutine uses the shared options
				result, err := invoker.Invoke(ctx, "test.function", "{}", sharedOpts)
				t.Logf("Shared opts %d: error=%v, result_len=%d", idx, err, len(result))
			}(i)
		}

		wg.Wait()
		t.Logf("No data races with shared options")
	})

	t.Run("Safe context usage", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Create contexts with different lifetimes
		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Create context per goroutine
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Per-goroutine ctx %d: error=%v, result_len=%d", idx, err, len(result))
			}(i)
		}

		wg.Wait()
		t.Logf("Safe per-goroutine context usage")
	})
}

// TestConcurrencySafety_StressTests stress tests for concurrency
func TestConcurrencySafety_StressTests(t *testing.T) {
	t.Run("Rapid create and destroy", func(t *testing.T) {
		const iterations = 100

		for i := 0; i < iterations; i++ {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				// Immediate use and close
				ctx := context.Background()
				invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				invoker.Close()
			}
		}

		t.Logf("Completed %d rapid create/use/destroy cycles", iterations)
	})

	t.Run("Wave pattern concurrency", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const waves = 5
		const goroutinesPerWave = 20

		for wave := 0; wave < waves; wave++ {
			var wg sync.WaitGroup

			for i := 0; i < goroutinesPerWave; i++ {
				wg.Add(1)
				go func(waveIdx, goroutineIdx int) {
					defer wg.Done()

					result, err := invoker.Invoke(ctx, "test.function",
						fmt.Sprintf(`{"wave":%d,"goroutine":%d}`, waveIdx, goroutineIdx),
						InvokeOptions{})
					t.Logf("Wave %d, Goroutine %d: error=%v, result_len=%d",
						waveIdx, goroutineIdx, err, len(result))
				}(wave, i)
			}

			wg.Wait()
			t.Logf("Wave %d completed", wave)
		}
	})
}

// TestConcurrencySafety_AtomicOperations tests atomic operation patterns
func TestConcurrencySafety_AtomicOperations(t *testing.T) {
	t.Run("Atomic counter usage", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		var successCount, failureCount int64
		const numInvokes = 100

		var wg sync.WaitGroup
		for i := 0; i < numInvokes; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failureCount, 1)
				}
				_ = result
			}(i)
		}

		wg.Wait()
		t.Logf("Atomic counters: %d success, %d failure", successCount, failureCount)
	})

	t.Run("Atomic pointer swaps", func(t *testing.T) {
		var invokerPtr atomic.Value
		initialInvoker := NewHTTPInvoker(&InvokerConfig{Address: "http://localhost:19090"})
		invokerPtr.Store(initialInvoker)

		const numSwaps = 50
		var wg sync.WaitGroup

		for i := 0; i < numSwaps; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Load current invoker
				invoker := invokerPtr.Load().(Invoker)

				// Use it
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Atomic swap %d: error=%v, result_len=%d", idx, err, len(result))

				// Swap with new invoker
				newInvoker := NewHTTPInvoker(&InvokerConfig{Address: "http://localhost:19090"})
				invokerPtr.Store(newInvoker)
			}(i)
		}

		wg.Wait()

		// Cleanup final invoker
		finalInvoker := invokerPtr.Load().(Invoker)
		finalInvoker.Close()

		t.Logf("Completed %d atomic pointer swaps", numSwaps)
	})
}

// TestConcurrencySafety_ChannelUsage tests safe channel usage patterns
func TestConcurrencySafety_ChannelUsage(t *testing.T) {
	t.Run("Concurrent job streaming", func(t *testing.T) {
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

		var wg sync.WaitGroup
		jobIDs := make([]string, numJobs)

		// Start multiple jobs
		for i := 0; i < numJobs; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				jobID, err := invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
				jobIDs[idx] = jobID
				t.Logf("Started job %d: %s, error=%v", idx, jobID, err)
			}(i)
		}

		wg.Wait()

		// Stream from all jobs concurrently
		for _, jobID := range jobIDs {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				eventChan, err := invoker.StreamJob(ctx, id)
				t.Logf("Stream job %s: error=%v, channel=%v", id, err, eventChan != nil)

				if eventChan != nil {
					select {
					case event := <-eventChan:
						t.Logf("Event from %s: Type=%s", id, event.EventType)
					case <-time.After(time.Millisecond * 100):
						t.Log("No event within timeout")
					}
				}
			}(jobID)
		}

		wg.Wait()
		t.Logf("Concurrent job streaming completed")
	})
}
