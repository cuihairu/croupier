// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestPerformanceBenchmark_InvocationBenchmarks tests invocation performance
func TestPerformanceBenchmark_InvocationBenchmarks(t *testing.T) {
	t.Run("Single invocation benchmark", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const iterations = 1000

		start := time.Now()
		successCount := 0

		for i := 0; i < iterations; i++ {
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			if err == nil {
				successCount++
			}
			_ = result
		}

		duration := time.Since(start)
		avgLatency := duration / time.Duration(iterations)
		throughput := float64(iterations) / duration.Seconds()

		t.Logf("Single invocation benchmark:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Success: %d (%.1f%%)", successCount, float64(successCount)*100/float64(iterations))
		t.Logf("  Total time: %v", duration)
		t.Logf("  Avg latency: %v", avgLatency)
		t.Logf("  Throughput: %.2f ops/sec", throughput)
	})

	t.Run("Concurrent invocation benchmark", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const goroutines = 10
		const invokesPerGoroutine = 100

		start := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < invokesPerGoroutine; j++ {
					result, err := invoker.Invoke(ctx, "test.function",
						fmt.Sprintf(`{"goroutine":%d,"invoke":%d}`, idx, j),
						InvokeOptions{})
					_ = result
					_ = err
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		totalInvokes := goroutines * invokesPerGoroutine
		throughput := float64(totalInvokes) / duration.Seconds()

		t.Logf("Concurrent invocation benchmark:")
		t.Logf("  Goroutines: %d", goroutines)
		t.Logf("  Invokes per goroutine: %d", invokesPerGoroutine)
		t.Logf("  Total invokes: %d", totalInvokes)
		t.Logf("  Total time: %v", duration)
		t.Logf("  Throughput: %.2f ops/sec", throughput)
	})
}

// TestPerformanceBenchmark_ResourceUsageBenchmarks tests resource usage patterns
func TestPerformanceBenchmark_ResourceUsageBenchmarks(t *testing.T) {
	t.Run("Memory allocation rate", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const iterations = 500

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		for i := 0; i < iterations; i++ {
			invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"id":%d}`, i), InvokeOptions{})
		}

		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
		avgPerInvoke := allocatedBytes / iterations

		t.Logf("Memory allocation rate:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Total allocated: %d bytes", allocatedBytes)
		t.Logf("  Avg per invoke: %d bytes", avgPerInvoke)
	})

	t.Run("Goroutine creation rate", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		const numGoroutines = 100

		start := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				time.Sleep(time.Microsecond)
				_ = idx
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Goroutine creation rate:")
		t.Logf("  Goroutines: %d", numGoroutines)
		t.Logf("  Total time: %v", duration)
		t.Logf("  Rate: %.2f goroutines/sec", float64(numGoroutines)/duration.Seconds())
	})
}

// TestPerformanceBenchmark_OperationComparison compares operation performance
func TestPerformanceBenchmark_OperationComparison(t *testing.T) {
	t.Run("Invoke vs StartJob performance", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

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

		// Benchmark Invoke
		startInvoke := time.Now()
		for i := 0; i < iterations; i++ {
			invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		}
		invokeDuration := time.Since(startInvoke)

		// Benchmark StartJob
		startJob := time.Now()
		for i := 0; i < iterations; i++ {
			invoker.StartJob(ctx, "test.job", "{}", InvokeOptions{})
		}
		jobDuration := time.Since(startJob)

		t.Logf("Operation comparison:")
		t.Logf("  Invoke: %v (%.2f ops/sec)", invokeDuration,
			float64(iterations)/invokeDuration.Seconds())
		t.Logf("  StartJob: %v (%.2f ops/sec)", jobDuration,
			float64(iterations)/jobDuration.Seconds())
	})

	t.Run("With retry vs without retry", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		const iterations = 30

		// With retry
		config1 := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}
		invoker1 := NewHTTPInvoker(config1)
		if invoker1 != nil {
			defer invoker1.Close()

			ctx := context.Background()
			start := time.Now()

			for i := 0; i < iterations; i++ {
				invoker1.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			}

			withRetryDuration := time.Since(start)
			t.Logf("With retry: %v (%.2f ops/sec)", withRetryDuration,
				float64(iterations)/withRetryDuration.Seconds())
		}

		// Without retry
		config2 := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled: false,
			},
		}
		invoker2 := NewHTTPInvoker(config2)
		if invoker2 != nil {
			defer invoker2.Close()

			ctx := context.Background()
			start := time.Now()

			for i := 0; i < iterations; i++ {
				invoker2.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			}

			withoutRetryDuration := time.Since(start)
			t.Logf("Without retry: %v (%.2f ops/sec)", withoutRetryDuration,
				float64(iterations)/withoutRetryDuration.Seconds())
		}
	})
}

// TestPerformanceBenchmark_ScalabilityTests tests scalability characteristics
func TestPerformanceBenchmark_ScalabilityTests(t *testing.T) {
	t.Run("Linear scalability test", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		ctx := context.Background()
		const baseInvokers = 1
		const invokesPerInvoker = 10

		invokerCounts := []int{1, 2, 5}

		for _, count := range invokerCounts {
			start := time.Now()
			var wg sync.WaitGroup

			for i := 0; i < count; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					config := &InvokerConfig{
						Address: fmt.Sprintf("localhost:%d", 19090+idx%10),
					}

					invoker := NewHTTPInvoker(config)
					if invoker != nil {
						for j := 0; j < invokesPerInvoker; j++ {
							invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
						}
						invoker.Close()
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(start)

			totalOps := count * invokesPerInvoker
			t.Logf("%d invokers: %v (%.2f ops/sec)", count, duration,
				float64(totalOps)/duration.Seconds())
		}
	})

	t.Run("Payload size impact", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		sizes := []struct{
			name string
			size int
		}{
			{"small (100B)", 100},
			{"medium (1KB)", 1024},
			{"large (10KB)", 10240},
			{"xlarge (100KB)", 102400},
		}

		for _, sizeInfo := range sizes {
			payload := fmt.Sprintf(`{"data":"%s"}`,
				string(make([]byte, sizeInfo.size)))

			iterations := 10
			if sizeInfo.size > 10000 {
				iterations = 5
			}

			start := time.Now()
			for i := 0; i < iterations; i++ {
				invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			}
			duration := time.Since(start)

			avgDuration := duration / time.Duration(iterations)
			t.Logf("%s: %v avg (%.2f MB/sec throughput)",
				sizeInfo.name, avgDuration,
				float64(sizeInfo.size)/(avgDuration.Seconds()*1024*1024))
		}
	})
}

// TestPerformanceBenchmark_ConcurrencyScaling tests concurrency scaling
func TestPerformanceBenchmark_ConcurrencyScaling(t *testing.T) {
	t.Run("Optimal concurrency level", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const invokesPerLevel = 50

		concurrencyLevels := []int{1, 2, 5, 10, 20}

		for _, concurrency := range concurrencyLevels {
			start := time.Now()
			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					for j := 0; j < invokesPerLevel; j++ {
						invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
					}
				}()
			}

			wg.Wait()
			duration := time.Since(start)

			totalInvokes := concurrency * invokesPerLevel
			throughput := float64(totalInvokes) / duration.Seconds()

			t.Logf("Concurrency %d: %v total, %.2f ops/sec",
				concurrency, duration, throughput)
		}
	})
}

// TestPerformanceBenchmark_ColdStartPerformance tests cold start scenarios
func TestPerformanceBenchmark_ColdStartPerformance(t *testing.T) {
	t.Run("Cold invoker creation", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		const iterations = 20

		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()

			config := &InvokerConfig{
				Address: "http://localhost:19090",
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				invoker.Close()
			}

			duration := time.Since(start)
			totalDuration += duration
		}

		avgDuration := totalDuration / time.Duration(iterations)
		t.Logf("Cold start avg: %v", avgDuration)
	})

	t.Run("Warm vs cold invoker", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		ctx := context.Background()
		const iterations = 100

		// Cold (first call)
		coldStart := time.Now()
		invoker := NewHTTPInvoker(config)
		if invoker != nil {
			invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			coldDuration := time.Since(coldStart)

			// Warm (subsequent calls)
			warmStart := time.Now()
			for i := 0; i < iterations; i++ {
				invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			}
			warmDuration := time.Since(warmStart)

			avgWarmDuration := warmDuration / time.Duration(iterations)

			t.Logf("Cold start: %v", coldDuration)
			t.Logf("Warm avg: %v", avgWarmDuration)
			t.Logf("Cold/Slowdown ratio: %.2fx",
				float64(coldDuration)/float64(avgWarmDuration))

			invoker.Close()
		}
	})
}

// TestPerformanceBenchmark_BatchOperations benchmarks batch operation patterns
func TestPerformanceBenchmark_BatchOperations(t *testing.T) {
	t.Run("Batch invoke performance", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		batchSizes := []int{1, 5, 10, 20}
		const iterationsPerBatch = 5

		for _, batchSize := range batchSizes {
			start := time.Now()

			for b := 0; b < iterationsPerBatch; b++ {
				for i := 0; i < batchSize; i++ {
					invoker.Invoke(ctx, "test.function",
						fmt.Sprintf(`{"batch":%d,"item":%d}`, b, i),
						InvokeOptions{})
				}
			}

			duration := time.Since(start)
			totalOps := batchSize * iterationsPerBatch

			t.Logf("Batch size %d: %v for %d ops (%.2f ops/sec)",
				batchSize, duration, totalOps, float64(totalOps)/duration.Seconds())
		}
	})

	t.Run("Sequential vs parallel batches", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		ctx := context.Background()
		const batchSize = 20

		// Sequential
		invoker1 := NewHTTPInvoker(config)
		if invoker1 != nil {
			defer invoker1.Close()

			start := time.Now()
			for i := 0; i < batchSize; i++ {
				invoker1.Invoke(ctx, "test.function", fmt.Sprintf(`{"id":%d}`, i), InvokeOptions{})
			}
			sequentialDuration := time.Since(start)

			t.Logf("Sequential (%d): %v (%.2f ops/sec)",
				batchSize, sequentialDuration,
				float64(batchSize)/sequentialDuration.Seconds())
		}

		// Parallel
		invoker2 := NewHTTPInvoker(config)
		if invoker2 != nil {
			defer invoker2.Close()

			start := time.Now()
			var wg sync.WaitGroup

			for i := 0; i < batchSize; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					invoker2.Invoke(ctx, "test.function", fmt.Sprintf(`{"id":%d}`, idx), InvokeOptions{})
				}(i)
			}

			wg.Wait()
			parallelDuration := time.Since(start)

			t.Logf("Parallel (%d): %v (%.2f ops/sec)",
				batchSize, parallelDuration,
				float64(batchSize)/parallelDuration.Seconds())
		}
	})
}

// TestPerformanceBenchmark_MemoryEfficiency tests memory efficiency
func TestPerformanceBenchmark_MemoryEfficiency(t *testing.T) {
	t.Run("Memory reuse patterns", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const iterations = 1000

		// Test with reused options
		reusedOpts := InvokeOptions{
			IdempotencyKey: "test-key",
		}

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		for i := 0; i < iterations; i++ {
			invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"id":%d}`, i), reusedOpts)
		}

		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		allocated := m2.TotalAlloc - m1.TotalAlloc
		avgPerInvoke := allocated / iterations

		t.Logf("Memory reuse (%d iterations):", iterations)
		t.Logf("  Total allocated: %d bytes", allocated)
		t.Logf("  Avg per invoke: %d bytes", avgPerInvoke)
	})

	t.Run("Large payload efficiency", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Pre-build payload
		largePayload := fmt.Sprintf(`{"data":"%s"}`, string(make([]byte, 100000)))
		const iterations = 10

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		start := time.Now()
		for i := 0; i < iterations; i++ {
			invoker.Invoke(ctx, "test.function", largePayload, InvokeOptions{})
		}
		duration := time.Since(start)

		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		allocated := m2.TotalAlloc - m1.TotalAlloc

		t.Logf("Large payload (%d bytes):", len(largePayload))
		t.Logf("  Total time: %v", duration)
		t.Logf("  Avg per invoke: %v", duration/time.Duration(iterations))
		t.Logf("  Memory allocated: %d bytes", allocated)
	})
}

// TestPerformanceBenchmark_LatencyDistribution tests latency distribution
func TestPerformanceBenchmark_LatencyDistribution(t *testing.T) {
	t.Run("Percentile latency measurement", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping benchmark in short mode")
		}

		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const samples = 100

		latencies := make([]time.Duration, samples)

		for i := 0; i < samples; i++ {
			start := time.Now()
			invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			latencies[i] = time.Since(start)
		}

		// Calculate percentiles
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		p50 := latencies[samples*50/100]
		p90 := latencies[samples*90/100]
		p95 := latencies[samples*95/100]
		p99 := latencies[samples*99/100]

		t.Logf("Latency percentiles (%d samples):", samples)
		t.Logf("  p50: %v", p50)
		t.Logf("  p90: %v", p90)
		t.Logf("  p95: %v", p95)
		t.Logf("  p99: %v", p99)
	})
}
