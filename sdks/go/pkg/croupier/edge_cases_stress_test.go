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

// TestEdgeCases_zeroAndEmptyValues tests zero and empty value handling
func TestEdgeCases_zeroAndEmptyValues(t *testing.T) {
	t.Run("Invoke with empty payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "", InvokeOptions{})
		t.Logf("Invoke with empty payload error: %v", err)
	})

	t.Run("Invoke with whitespace payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		whitespacePayloads := []string{" ", "  ", "\t", "\n", "\r", " \t\n\r"}
		ctx := context.Background()

		for _, payload := range whitespacePayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with whitespace payload '%s' error: %v", payload, err)
		}
	})

	t.Run("Invoke with empty function ID", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "", "{}", InvokeOptions{})
		t.Logf("Invoke with empty function ID error: %v", err)
	})

	t.Run("InvokeOptions with zero timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		options := InvokeOptions{
			Timeout: 0,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with zero timeout error: %v", err)
	})

	t.Run("InvokeOptions with empty headers map", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		options := InvokeOptions{
			Headers: map[string]string{},
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with empty headers map error: %v", err)
	})
}

// TestEdgeCases_extremeValues tests extreme value handling
func TestEdgeCases_extremeValues(t *testing.T) {
	t.Run("RetryConfig with extreme MaxAttempts", func(t *testing.T) {
		maxAttempts := []int{-100, -1, 0, 1, 100, 1000, 1000000}

		for _, attempts := range maxAttempts {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:     true,
					MaxAttempts: attempts,
				},
			})

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with MaxAttempts=%d error: %v", attempts, err)
		}
	})

	t.Run("RetryConfig with extreme delays", func(t *testing.T) {
		delays := []int{-1000, -1, 0, 1, 10, 1000, 1000000}

		for _, delay := range delays {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:        true,
					MaxAttempts:    2,
					InitialDelayMs: delay,
					MaxDelayMs:     delay * 10,
				},
			})

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with InitialDelayMs=%d error: %v", delay, err)
		}
	})

	t.Run("RetryConfig with extreme multipliers", func(t *testing.T) {
		multipliers := []float64{-10.0, -1.0, 0.0, 0.001, 0.1, 1.0, 2.0, 10.0, 100.0, 1000.0}

		for _, mult := range multipliers {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:           true,
					MaxAttempts:       2,
					BackoffMultiplier: mult,
				},
			})

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with BackoffMultiplier=%.3f error: %v", mult, err)
		}
	})

	t.Run("ReconnectConfig with extreme MaxAttempts", func(t *testing.T) {
		maxAttempts := []int{-100, -1, 0, 1, 100, 1000}

		for _, attempts := range maxAttempts {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Reconnect: &ReconnectConfig{
					Enabled:     true,
					MaxAttempts: attempts,
				},
			})

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with Reconnect MaxAttempts=%d error: %v", attempts, err)
		}
	})
}

// TestEdgeCases_specialCharacters tests special character handling
func TestEdgeCases_specialCharacters(t *testing.T) {
	t.Run("Invoke with special characters in payload", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		specialPayloads := []string{
			`{"text": "Null byte: \x00"}`,
			`{"text": "Bell: \a"}`,
			`{"text": "Backspace: \b"}`,
			`{"text": "Form feed: \f"}`,
			`{"text": "Vertical tab: \v"}`,
			`{"text": "ESC: \x1b"}`,
			`{"text": "Quote: \""}`,
			`{"text": "Backslash: \\"}`,
			`{"text": "Slash: /"}`,
			`{"text": "Percent: %"}`,
		}

		ctx := context.Background()
		for _, payload := range specialPayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with special char payload error: %v", err)
		}
	})

	t.Run("Invoke with Unicode edge cases", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		unicodePayloads := []string{
			`{"text": "Combining: cafe\u0301"}`,
			`{"text": "Zero width: \u200b"}`,
			`{"text": "Non-breaking space: \u00a0"}`,
			`{"text": "Soft hyphen: \u00ad"}`,
			`{"text": "Emoji: 😀 🎉 🚀"}`,
			`{"text": "RTL text: \u05d0\u05d1\u05d2"}`,
			`{"text": "BOM: \ufeff"}`,
		}

		ctx := context.Background()
		for _, payload := range unicodePayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with Unicode payload error: %v", err)
		}
	})

	t.Run("Headers with special characters", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		specialHeaders := map[string]string{
			"X-Special":      "value with \n newlines \t tabs",
			"X-Unicode":      "世界 🌍 Привет",
			"X-URL-Encoded":  "name=value&foo=bar",
			"X-JSON":         `{"key":"value"}`,
			"X-Empty-Value":  "",
			"X-Space-Only":   "   ",
			"X-With-Quotes":  `"quoted"`,
		}

		options := InvokeOptions{
			Headers: specialHeaders,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with special headers error: %v", err)
	})
}

// TestEdgeCases_invalidInputs tests invalid input handling
func TestEdgeCases_invalidInputs(t *testing.T) {
	t.Run("Invoke with invalid JSON structures", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		invalidPayloads := []string{
			`{"missing": }`,
			`{"extra": }`,
			`[1,2,3}`,
			`"just string"`,
			`12345`,
			`true`,
			`null`,
			``,
			`   {}   `,
			`{}}`,
			`{{}}`,
			`[][]`,
		}

		ctx := context.Background()
		for _, payload := range invalidPayloads {
			_, err := invoker.Invoke(ctx, "test.func", payload, InvokeOptions{})
			t.Logf("Invoke with invalid JSON '%s' error: %v", payload, err)
		}
	})

	t.Run("StartJob with invalid contexts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// Context with very short timeout
		ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel1()
		time.Sleep(10 * time.Millisecond)
		_, err1 := invoker.StartJob(ctx1, "test.func", "{}", InvokeOptions{})
		t.Logf("StartJob with expired context error: %v", err1)

		// Cancelled context
		ctx2, cancel2 := context.WithCancel(context.Background())
		cancel2()
		_, err2 := invoker.StartJob(ctx2, "test.func", "{}", InvokeOptions{})
		t.Logf("StartJob with cancelled context error: %v", err2)
	})

	t.Run("SetSchema with invalid schemas", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		invalidSchemas := []map[string]interface{}{
			{"type": 123},                           // Wrong type for type field
			{"type": "invalid"},                     // Invalid type
			{"properties": "not an object"},        // Wrong type for properties
			{"additionalProperties": "not bool"},  // Wrong type
			{"required": 123},                       // Wrong type for required
			{"items": "not an object"},              // Wrong type for items
		}

		for i, schema := range invalidSchemas {
			err := invoker.SetSchema("test.func", schema)
			t.Logf("SetSchema with invalid schema %d error: %v", i, err)
		}
	})
}

// TestEdgeCases_resourceLimits tests resource limit scenarios
func TestEdgeCases_resourceLimits(t *testing.T) {
	t.Run("Invoke with many header entries", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// Create many headers
		headers := make(map[string]string)
		for i := 0; i < 200; i++ {
			headers[fmt.Sprintf("X-Header-%d", i)] = fmt.Sprintf("Value-%d", i)
		}

		options := InvokeOptions{
			Headers: headers,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with 200 headers error: %v", err)
	})

	t.Run("Invoke with very long header values", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		longValue := string(make([]byte, 10000))
		for i := range longValue {
			longValue = "a" + longValue[:i]
		}

		headers := map[string]string{
			"X-Long-Header": longValue,
		}

		options := InvokeOptions{
			Headers: headers,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with very long header value error: %v", err)
	})

	t.Run("Invoke with very long function ID", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// Create a very long function ID
		longID := ""
		for i := 0; i < 1000; i++ {
			longID += fmt.Sprintf("segment%d.", i)
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, longID, "{}", InvokeOptions{})
		t.Logf("Invoke with very long function ID error: %v", err)
	})
}

// TestEdgeCases_timeRelatedTests tests time-related edge cases
func TestEdgeCases_timeRelatedTests(t *testing.T) {
	t.Run("Invoke with very short timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		timeouts := []time.Duration{
			0,
			1 * time.Nanosecond,
			1 * time.Microsecond,
			100 * time.Microsecond,
		}

		ctx := context.Background()
		for _, timeout := range timeouts {
			options := InvokeOptions{
				Timeout: timeout,
			}

			_, err := invoker.Invoke(ctx, "test.func", "{}", options)
			t.Logf("Invoke with timeout=%v error: %v", timeout, err)
		}
	})

	t.Run("Invoke with very long timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		longTimeouts := []time.Duration{
			1 * time.Hour,
			24 * time.Hour,
			365 * 24 * time.Hour,
		}

		ctx := context.Background()
		for _, timeout := range longTimeouts {
			options := InvokeOptions{
				Timeout: timeout,
			}

			_, err := invoker.Invoke(ctx, "test.func", "{}", options)
			t.Logf("Invoke with timeout=%v error: %v", timeout, err)
		}
	})

	t.Run("Invoke with context deadline in the past", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		past := time.Now().Add(-1 * time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), past)
		defer cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with past deadline error: %v", err)
	})

	t.Run("Invoke with context deadline far in future", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		future := time.Now().Add(365 * 24 * time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), future)
		defer cancel()

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with far future deadline error: %v", err)
	})
}

// TestEdgeCases_nillAndZeroPointers tests nil and zero pointer handling
func TestEdgeCases_nillAndZeroPointers(t *testing.T) {
	t.Run("InvokeOptions with nil Retry", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		options := InvokeOptions{
			Retry: nil,
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with nil Retry error: %v", err)
	})

	t.Run("InvokerConfig with nil Retry and Reconnect", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:   "localhost:8080",
			Retry:     nil,
			Reconnect: nil,
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with nil configs error: %v", err)
	})

	t.Run("NewInvoker with nil config", func(t *testing.T) {
		invoker := NewInvoker(nil)
		if invoker == nil {
			t.Error("NewInvoker(nil) should return non-nil")
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with nil config error: %v", err)
	})
}

// TestEdgeCases_rapidOperations tests rapid operation sequences
func TestEdgeCases_rapidOperations(t *testing.T) {
	t.Run("rapid Invoke attempts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		start := time.Now()

		for i := 0; i < 100; i++ {
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			if err != nil {
				t.Logf("Invoke %d error: %v", i, err)
			}
		}

		elapsed := time.Since(start)
		t.Logf("100 rapid Invokes took: %v (%.2f ms per Invoke)",
			elapsed, float64(elapsed.Milliseconds())/100)
	})

	t.Run("rapid StartJob attempts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		start := time.Now()

		jobIDs := make([]string, 50)
		for i := 0; i < 50; i++ {
			jobID, err := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
			jobIDs[i] = jobID
			if err != nil {
				t.Logf("StartJob %d error: %v", i, err)
			}
		}

		elapsed := time.Since(start)
		t.Logf("50 rapid StartJobs took: %v (%.2f ms per StartJob)",
			elapsed, float64(elapsed.Milliseconds())/50)
	})

	t.Run("rapid Create and Close invokers", func(t *testing.T) {
		start := time.Now()

		for i := 0; i < 50; i++ {
			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
			})
			if invoker != nil {
				_ = invoker.Close()
			}
		}

		elapsed := time.Since(start)
		t.Logf("50 Create/Close cycles took: %v (%.2f ms per cycle)",
			elapsed, float64(elapsed.Milliseconds())/50)
	})
}

// TestEdgeCases_concurrentStressTests tests concurrent stress scenarios
func TestEdgeCases_concurrentStressTests(t *testing.T) {
	t.Run("concurrent everything", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		const numGoroutines = 100
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx := context.Background()

				switch idx % 4 {
				case 0:
					_, _ = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
				case 1:
					jobID, _ := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
					_, _ = invoker.StreamJob(ctx, jobID)
				case 2:
					jobID, _ := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
					_ = invoker.CancelJob(ctx, jobID)
				case 3:
					_ = invoker.SetSchema("test.func", map[string]interface{}{"type": "object"})
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent everything test completed")
	})

	t.Run("concurrent with same invoker and different contexts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, _ = invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent with different contexts test completed")
	})
}
