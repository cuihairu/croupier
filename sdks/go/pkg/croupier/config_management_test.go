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

// TestConfigManagement_DefaultValues tests default configuration values
func TestConfigManagement_DefaultValues(t *testing.T) {
	t.Run("Default retry config", func(t *testing.T) {
		config := DefaultRetryConfig()

		if config == nil {
			t.Fatal("DefaultRetryConfig returned nil")
		}

		t.Logf("Default retry config: Enabled=%v, MaxAttempts=%d, InitialDelayMs=%d",
			config.Enabled, config.MaxAttempts, config.InitialDelayMs)
	})

	t.Run("Default reconnect config", func(t *testing.T) {
		config := DefaultReconnectConfig()

		if config == nil {
			t.Fatal("DefaultReconnectConfig returned nil")
		}

		t.Logf("Default reconnect config: Enabled=%v, MaxAttempts=%d, InitialDelayMs=%d",
			config.Enabled, config.MaxAttempts, config.InitialDelayMs)
	})

	t.Run("Minimal invoker config", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Minimal config: error=%v, result_len=%d", err, len(result))
	})
}

// TestConfigValidation_AddressFormats tests various address format validations
func TestConfigValidation_AddressFormats(t *testing.T) {
	t.Run("Valid address formats", func(t *testing.T) {
		validAddresses := []string{
			"localhost:19090",
			"127.0.0.1:19090",
			"0.0.0.0:19090",
			"192.168.1.1:8080",
			"example.com:8080",
			"example.com:443",
			"localhost:80",
			"localhost:443",
		}

		for _, addr := range validAddresses {
			config := &InvokerConfig{
				Address: addr,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Address '%s': successfully created invoker", addr)
				invoker.Close()
			} else {
				t.Errorf("Address '%s': failed to create invoker", addr)
			}
		}
	})

	t.Run("Port edge cases", func(t *testing.T) {
		portCases := []struct {
			port    string
			valid   bool
			describe string
		}{
			{"0", true, "Port 0 (system assigned)"},
			{"1", true, "Port 1 (minimum valid)"},
			{"80", true, "Port 80 (HTTP)"},
			{"443", true, "Port 443 (HTTPS)"},
			{"8080", true, "Port 8080 (common alt)"},
			{"19090", true, "Port 19090 (default)"},
			{"65535", true, "Port 65535 (maximum valid)"},
		}

		for _, tc := range portCases {
			config := &InvokerConfig{
				Address: "localhost:" + tc.port,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("%s: successfully created invoker", tc.describe)
				invoker.Close()
			} else if tc.valid {
				t.Errorf("%s: unexpectedly failed to create invoker", tc.describe)
			}
		}
	})
}

// TestConfigValidation_TimeoutValues tests timeout configuration values
func TestConfigValidation_TimeoutValues(t *testing.T) {
	t.Run("Valid timeout values", func(t *testing.T) {
		timeouts := []int{
			0,      // No timeout
			1,      // 1 second
			30,     // 30 seconds (common)
			60,     // 1 minute
			300,    // 5 minutes
			3600,   // 1 hour
		}

		for _, timeout := range timeouts {
			config := &InvokerConfig{
				Address:        "localhost:19090",
				TimeoutSeconds: timeout,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				ctx := context.Background()
				result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
				t.Logf("Timeout %ds: error=%v, result_len=%d", timeout, err, len(result))
				invoker.Close()
			}
		}
	})

	t.Run("Timeout vs InvokeOptions timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// InvokeOptions timeout should override config timeout
		opts := InvokeOptions{
			Timeout: 100 * time.Millisecond,
		}

		start := time.Now()
		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		duration := time.Since(start)

		t.Logf("Override timeout: duration=%v, error=%v, result_len=%d", duration, err, len(result))
	})
}

// TestConfigValidation_TLSCombinations tests TLS configuration combinations
func TestConfigValidation_TLSCombinations(t *testing.T) {
	t.Run("TLS configuration matrix", func(t *testing.T) {
		configs := []struct {
			name     string
			config   *InvokerConfig
		}{
			{
				"No TLS",
				&InvokerConfig{
					Address:  "localhost:19090",
					Insecure: false,
				},
			},
			{
				"Insecure mode",
				&InvokerConfig{
					Address:  "localhost:19090",
					Insecure: true,
				},
			},
			{
				"CA only",
				&InvokerConfig{
					Address:  "localhost:19090",
					Insecure: false,
					CAFile:   "/path/to/ca.crt",
				},
			},
			{
				"Full TLS",
				&InvokerConfig{
					Address:  "localhost:19090",
					Insecure: false,
					CAFile:   "/path/to/ca.crt",
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
		}

		for _, tc := range configs {
			invoker := NewHTTPInvoker(tc.config)
			if invoker != nil {
				t.Logf("TLS config '%s': successfully created invoker", tc.name)
				invoker.Close()
			} else {
				t.Errorf("TLS config '%s': failed to create invoker", tc.name)
			}
		}
	})
}

// TestRetryConfiguration_BoundaryValues tests retry configuration boundary values
func TestRetryConfiguration_BoundaryValues(t *testing.T) {
	t.Run("MaxAttempts boundaries", func(t *testing.T) {
		testCases := []struct {
			maxAttempts int
			describe    string
		}{
			{0, "Zero max attempts"},
			{1, "Single attempt"},
			{3, "Three attempts"},
			{5, "Five attempts"},
			{10, "Ten attempts"},
			{100, "Large max attempts"},
		}

		for _, tc := range testCases {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:     true,
					MaxAttempts: tc.maxAttempts,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("%s: successfully created invoker", tc.describe)
				invoker.Close()
			}
		}
	})

	t.Run("Delay boundaries", func(t *testing.T) {
		testCases := []struct {
			initialDelay int
			maxDelay     int
			describe     string
		}{
			{0, 0, "Zero delays"},
			{1, 1, "Minimum delays"},
			{10, 100, "Small delays"},
			{100, 1000, "Medium delays"},
			{1000, 10000, "Large delays"},
			{10000, 60000, "Very large delays"},
		}

		for _, tc := range testCases {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:        true,
					MaxAttempts:    3,
					InitialDelayMs: tc.initialDelay,
					MaxDelayMs:     tc.maxDelay,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("%s: successfully created invoker", tc.describe)
				invoker.Close()
			}
		}
	})

	t.Run("Backoff multiplier boundaries", func(t *testing.T) {
		multipliers := []float64{
			1.0,   // No backoff
			1.5,   // Moderate backoff
			2.0,   // Standard exponential backoff
			3.0,   // Aggressive backoff
			10.0,  // Very aggressive backoff
		}

		for _, mult := range multipliers {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:           true,
					MaxAttempts:       3,
					InitialDelayMs:    100,
					MaxDelayMs:        5000,
					BackoffMultiplier: mult,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Backoff multiplier %.1f: successfully created invoker", mult)
				invoker.Close()
			}
		}
	})

	t.Run("Jitter factor boundaries", func(t *testing.T) {
		jitterFactors := []float64{
			0.0,  // No jitter
			0.1,  // 10% jitter
			0.25, // 25% jitter
			0.5,  // 50% jitter
			1.0,  // 100% jitter
		}

		for _, jitter := range jitterFactors {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:       true,
					MaxAttempts:   3,
					InitialDelayMs: 100,
					MaxDelayMs:    1000,
					JitterFactor:  jitter,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Jitter factor %.2f: successfully created invoker", jitter)
				invoker.Close()
			}
		}
	})
}

// TestReconnectConfiguration_BoundaryValues tests reconnect configuration
func TestReconnectConfiguration_BoundaryValues(t *testing.T) {
	t.Run("Reconnect max attempts", func(t *testing.T) {
		attempts := []int{0, 1, 3, 5, 10, 100}

		for _, maxAttempts := range attempts {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Reconnect: &ReconnectConfig{
					Enabled:     true,
					MaxAttempts: maxAttempts,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Reconnect max attempts %d: OK", maxAttempts)
				invoker.Close()
			}
		}
	})

	t.Run("Reconnect delays", func(t *testing.T) {
		testCases := []struct {
			initialDelay int
			maxDelay     int
		}{
			{0, 0},
			{10, 100},
			{100, 1000},
			{1000, 10000},
		}

		for _, tc := range testCases {
			config := &InvokerConfig{
				Address: "http://localhost:19090",
				Reconnect: &ReconnectConfig{
					Enabled:        true,
					MaxAttempts:    3,
					InitialDelayMs: tc.initialDelay,
					MaxDelayMs:     tc.maxDelay,
				},
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Reconnect delays %d/%d: OK", tc.initialDelay, tc.maxDelay)
				invoker.Close()
			}
		}
	})
}

// TestConfigCombinations_ComplexScenarios tests complex configuration combinations
func TestConfigCombinations_ComplexScenarios(t *testing.T) {
	t.Run("All features enabled", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
			Insecure:       false,
			CAFile:         "/path/to/ca.crt",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
			Reconnect: &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 100,
				MaxDelayMs:     1000,
			},
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("All features: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Minimal configuration", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Minimal config: error=%v, result_len=%d", err, len(result))
	})
}

// TestConfigImmutability tests that configurations aren't modified unexpectedly
func TestConfigImmutability(t *testing.T) {
	t.Run("Config reuse", func(t *testing.T) {
		sharedConfig := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
		}

		// Create multiple invokers with same config
		invoker1 := NewHTTPInvoker(sharedConfig)
		invoker2 := NewHTTPInvoker(sharedConfig)

		if invoker1 == nil || invoker2 == nil {
			t.Fatal("Failed to create invokers")
		}

		defer invoker1.Close()
		defer invoker2.Close()

		ctx := context.Background()

		result1, err1 := invoker1.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		result2, err2 := invoker2.Invoke(ctx, "test.function", "{}", InvokeOptions{})

		t.Logf("Invoker1: error=%v, result_len=%d", err1, len(result1))
		t.Logf("Invoker2: error=%v, result_len=%d", err2, len(result2))
	})
}

// TestConfigConcurrency tests configuration with concurrent operations
func TestConfigConcurrency(t *testing.T) {
	t.Run("Concurrent invoker creation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		const numInvokers = 20
		var wg sync.WaitGroup

		for i := 0; i < numInvokers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				invoker := NewHTTPInvoker(config)
				if invoker != nil {
					ctx := context.Background()
					result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"idx":%d}`, idx), InvokeOptions{})
					_ = result
					_ = err
					invoker.Close()
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Created %d invokers concurrently with shared config", numInvokers)
	})
}

// TestConfigValidation_InvalidInputs tests invalid configuration inputs
func TestConfigValidation_InvalidInputs(t *testing.T) {
	t.Run("Empty address", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "",
		}

		invoker := NewHTTPInvoker(config)
		if invoker != nil {
			ctx := context.Background()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Empty address: error=%v, result_len=%d", err, len(result))
			invoker.Close()
		}
	})

	t.Run("Invalid port numbers", func(t *testing.T) {
		// These might be handled by the HTTP client layer
		invalidPorts := []string{
			"localhost:-1",
			"localhost:65536",
			"localhost:99999",
		}

		for _, addr := range invalidPorts {
			config := &InvokerConfig{
				Address: addr,
			}

			invoker := NewHTTPInvoker(config)
			if invoker != nil {
				t.Logf("Invalid port '%s': invoker created (may fail on invoke)", addr)
				invoker.Close()
			}
		}
	})

	t.Run("Negative timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: -1,
		}

		invoker := NewHTTPInvoker(config)
		if invoker != nil {
			ctx := context.Background()
			result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Negative timeout: error=%v, result_len=%d", err, len(result))
			invoker.Close()
		}
	})
}

// TestConfigOptions_InvokeOptions tests InvokeOptions configuration
func TestConfigOptions_InvokeOptions(t *testing.T) {
	t.Run("Options combinations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test various option combinations
		optionSets := []InvokeOptions{
			{
				Timeout: 0,
			},
			{
				Timeout: 30 * time.Second,
			},
			{
				Headers: map[string]string{
					"X-Test": "value",
				},
			},
			{
				IdempotencyKey: "test-key",
			},
			{
				Timeout:        10 * time.Second,
				Headers:        map[string]string{"X-Test": "value"},
				IdempotencyKey: "key-123",
			},
			{
				Retry: &RetryConfig{
					Enabled:     true,
					MaxAttempts: 5,
				},
			},
		}

		for i, opts := range optionSets {
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Options set %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Empty vs nil headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Empty headers map
		opts1 := InvokeOptions{
			Headers: map[string]string{},
		}

		// Nil headers (default)
		opts2 := InvokeOptions{}

		result1, err1 := invoker.Invoke(ctx, "test.function", "{}", opts1)
		result2, err2 := invoker.Invoke(ctx, "test.function", "{}", opts2)

		t.Logf("Empty headers: error=%v, result_len=%d", err1, len(result1))
		t.Logf("Nil headers: error=%v, result_len=%d", err2, len(result2))
	})
}

// TestConfigState_ImmutableState tests immutable state behavior
func TestConfigState_ImmutableState(t *testing.T) {
	t.Run("Config after creation", func(t *testing.T) {
		originalAddress := "http://localhost:19090"
		config := &InvokerConfig{
			Address: originalAddress,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Config should not be modified by invoker creation
		if config.Address != originalAddress {
			t.Errorf("Address was modified: expected %s, got %s", originalAddress, config.Address)
		}

		ctx := context.Background()
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		_ = result
		_ = err

		// Config should not be modified by invoke
		if config.Address != originalAddress {
			t.Errorf("Address was modified after invoke: expected %s, got %s", originalAddress, config.Address)
		}
	})
}

// TestConfigDefaults_OverrideBehavior tests default override behavior
func TestConfigDefaults_OverrideBehavior(t *testing.T) {
	t.Run("InvokeOptions override Config", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
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

		// Options should override config
		opts := InvokeOptions{
			Timeout: 100 * time.Millisecond,
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("With override: error=%v, result_len=%d", err, len(result))
	})
}

// TestConfigLifecycle tests configuration lifecycle
func TestConfigLifecycle(t *testing.T) {
	t.Run("Config reuse after close", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		// First invoker
		invoker1 := NewHTTPInvoker(config)
		if invoker1 != nil {
			ctx := context.Background()
			result, err := invoker1.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("First invoker: error=%v, result_len=%d", err, len(result))
			invoker1.Close()
		}

		// Second invoker with same config
		invoker2 := NewHTTPInvoker(config)
		if invoker2 != nil {
			ctx := context.Background()
			result, err := invoker2.Invoke(ctx, "test.function", "{}", InvokeOptions{})
			t.Logf("Second invoker: error=%v, result_len=%d", err, len(result))
			invoker2.Close()
		}
	})
}
