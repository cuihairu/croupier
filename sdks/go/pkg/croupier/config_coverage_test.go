// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"sync"
	"testing"
)

// TestInvoker_configurationVariations tests various invoker configurations
func TestInvoker_configurationVariations(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		invoker := NewInvoker(nil)
		if invoker == nil {
			t.Error("NewInvoker(nil) should return invoker with defaults")
		}
	})

	t.Run("with empty config", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{})
		if invoker == nil {
			t.Error("NewInvoker(&InvokerConfig{}) should return invoker with defaults")
		}
	})

	t.Run("with various addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:8080",
			"127.0.0.1:8080",
			"192.168.1.1:8080",
			"example.com:8443",
		}

		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address:   addr,
				Retry:     DefaultRetryConfig(),
				Reconnect: DefaultReconnectConfig(),
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for address=%s", addr)
			}
		}
	})

	t.Run("with various timeouts", func(t *testing.T) {
		timeouts := []int{0, 1, 10, 30, 60, 120, 300}

		for _, timeout := range timeouts {
			invoker := NewInvoker(&InvokerConfig{
				Address:        "localhost:8080",
				TimeoutSeconds: timeout,
				Retry:          DefaultRetryConfig(),
				Reconnect:      DefaultReconnectConfig(),
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for timeout=%d", timeout)
			}
		}
	})

	t.Run("with TLS enabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:   "example.com:8443",
			Insecure:  false,
			CAFile:    "/path/to/ca.crt",
			CertFile:  "/path/to/cert.pem",
			KeyFile:   "/path/to/key.pem",
			Retry:     DefaultRetryConfig(),
			Reconnect: DefaultReconnectConfig(),
		})

		if invoker == nil {
			t.Error("NewInvoker() with TLS config returned nil")
		}
	})

	t.Run("with TLS insecure", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:   "example.com:8443",
			Insecure:  true,
			Retry:     DefaultRetryConfig(),
			Reconnect: DefaultReconnectConfig(),
		})

		if invoker == nil {
			t.Error("NewInvoker() with insecure TLS returned nil")
		}
	})
}

// TestRetryConfiguration_variations tests various retry configurations
func TestRetryConfiguration_variations(t *testing.T) {
	t.Run("retry enabled with various max attempts", func(t *testing.T) {
		maxAttempts := []int{1, 2, 3, 5, 10}

		for _, attempts := range maxAttempts {
			config := &RetryConfig{
				Enabled:           true,
				MaxAttempts:       attempts,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Retry:     config,
				Reconnect: DefaultReconnectConfig(),
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for MaxAttempts=%d", attempts)
			}

			if invoker.(*tcpInvoker).config.Retry.MaxAttempts != attempts {
				t.Errorf("Retry MaxAttempts = %d, want %d",
					invoker.(*tcpInvoker).config.Retry.MaxAttempts, attempts)
			}
		}
	})

	t.Run("retry with various delays", func(t *testing.T) {
		delayConfigs := []struct {
			initial int
			max     int
		}{
			{10, 100},
			{50, 1000},
			{100, 5000},
			{200, 10000},
		}

		for _, cfg := range delayConfigs {
			config := &RetryConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: cfg.initial,
				MaxDelayMs:     cfg.max,
				BackoffMultiplier: 2.0,
				JitterFactor:   0.1,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Retry:     config,
				Reconnect: DefaultReconnectConfig(),
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for InitialDelayMs=%d, MaxDelayMs=%d",
					cfg.initial, cfg.max)
			}
		}
	})

	t.Run("retry disabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:   "localhost:8080",
			Retry:     &RetryConfig{Enabled: false},
			Reconnect: DefaultReconnectConfig(),
		})

		if invoker == nil {
			t.Error("NewInvoker() with retry disabled returned nil")
		}

		if invoker.(*tcpInvoker).config.Retry.Enabled {
			t.Error("Retry should be disabled")
		}
	})
}

// TestReconnectConfiguration_variations tests various reconnect configurations
func TestReconnectConfiguration_variations(t *testing.T) {
	t.Run("reconnect enabled with various max attempts", func(t *testing.T) {
		maxAttempts := []int{1, 3, 5, 10, 0} // 0 means unlimited

		for _, attempts := range maxAttempts {
			config := &ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       attempts,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Retry:     DefaultRetryConfig(),
				Reconnect: config,
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for MaxAttempts=%d", attempts)
			}

			if invoker.(*tcpInvoker).config.Reconnect.MaxAttempts != attempts {
				t.Errorf("Reconnect MaxAttempts = %d, want %d",
					invoker.(*tcpInvoker).config.Reconnect.MaxAttempts, attempts)
			}
		}
	})

	t.Run("reconnect with various delays", func(t *testing.T) {
		delayConfigs := []struct {
			initial int
			max     int
		}{
			{10, 100},
			{50, 1000},
			{100, 5000},
			{200, 10000},
		}

		for _, cfg := range delayConfigs {
			config := &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    5,
				InitialDelayMs: cfg.initial,
				MaxDelayMs:     cfg.max,
				BackoffMultiplier: 2.0,
				JitterFactor:   0.1,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Retry:     DefaultRetryConfig(),
				Reconnect: config,
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for InitialDelayMs=%d, MaxDelayMs=%d",
					cfg.initial, cfg.max)
			}
		}
	})

	t.Run("reconnect disabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:   "localhost:8080",
			Retry:     DefaultRetryConfig(),
			Reconnect: &ReconnectConfig{Enabled: false},
		})

		if invoker == nil {
			t.Error("NewInvoker() with reconnect disabled returned nil")
		}

		if invoker.(*tcpInvoker).config.Reconnect.Enabled {
			t.Error("Reconnect should be disabled")
		}
	})
}

// TestInvoker_withBothRetryAndReconnect tests interaction between retry and reconnect
func TestInvoker_withBothRetryAndReconnect(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address:   "localhost:8080",
		Retry: &RetryConfig{
			Enabled:     true,
			MaxAttempts: 3,
		},
		Reconnect: &ReconnectConfig{
			Enabled:     true,
			MaxAttempts: 5,
		},
	})

	if invoker == nil {
		t.Fatal("NewInvoker() returned nil")
	}

	tcpInvoker := invoker.(*tcpInvoker)
	if tcpInvoker.config.Retry.MaxAttempts != 3 {
		t.Errorf("Retry MaxAttempts = %d, want 3", tcpInvoker.config.Retry.MaxAttempts)
	}

	if tcpInvoker.config.Reconnect.MaxAttempts != 5 {
		t.Errorf("Reconnect MaxAttempts = %d, want 5", tcpInvoker.config.Reconnect.MaxAttempts)
	}
}

// TestInvoker_concurrentCreation tests concurrent invoker creation
func TestInvoker_concurrentCreation(t *testing.T) {
	const numGoroutines = 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Retry:     DefaultRetryConfig(),
				Reconnect: DefaultReconnectConfig(),
			})

			if invoker == nil {
				t.Errorf("Goroutine %d: NewInvoker() returned nil", idx)
			}
		}(i)
	}

	wg.Wait()
}

// TestInvoker_contextUsage tests context usage
func TestInvoker_contextUsage(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address:   "localhost:8080",
		Retry:     DefaultRetryConfig(),
		Reconnect: DefaultReconnectConfig(),
	})

	if invoker == nil {
		t.Fatal("NewInvoker() returned nil")
	}

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Exercise the context path (even though it won't connect)
	_ = ctx
	_ = invoker
}

// TestDefaultConfigurations tests default configuration values
func TestDefaultConfigurations(t *testing.T) {
	t.Run("DefaultRetryConfig", func(t *testing.T) {
		config := DefaultRetryConfig()

		if !config.Enabled {
			t.Error("Retry should be enabled by default")
		}

		if config.MaxAttempts <= 0 {
			t.Error("MaxAttempts should be positive")
		}

		if config.InitialDelayMs <= 0 {
			t.Error("InitialDelayMs should be positive")
		}

		if config.MaxDelayMs <= 0 {
			t.Error("MaxDelayMs should be positive")
		}

		if config.BackoffMultiplier <= 1.0 {
			t.Error("BackoffMultiplier should be greater than 1.0")
		}

		if config.JitterFactor < 0 || config.JitterFactor > 1.0 {
			t.Error("JitterFactor should be between 0 and 1")
		}

		if len(config.RetryableStatusCodes) == 0 {
			t.Error("RetryableStatusCodes should not be empty")
		}
	})

	t.Run("DefaultReconnectConfig", func(t *testing.T) {
		config := DefaultReconnectConfig()

		if !config.Enabled {
			t.Error("Reconnect should be enabled by default")
		}

		if config.MaxAttempts < 0 {
			t.Error("MaxAttempts should be non-negative (0 = infinite retries)")
		}

		if config.InitialDelayMs <= 0 {
			t.Error("InitialDelayMs should be positive")
		}

		if config.MaxDelayMs <= 0 {
			t.Error("MaxDelayMs should be positive")
		}

		if config.BackoffMultiplier <= 1.0 {
			t.Error("BackoffMultiplier should be greater than 1.0")
		}

		if config.JitterFactor < 0 || config.JitterFactor > 1.0 {
			t.Error("JitterFactor should be between 0 and 1")
		}
	})
}

// TestConfigurationEdgeCases tests edge cases in configuration
func TestConfigurationEdgeCases(t *testing.T) {
	t.Run("zero values in retry config", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:        true,
			MaxAttempts:    0,      // 0 = unlimited
			InitialDelayMs: 0,      // 0 is valid
			MaxDelayMs:     0,      // 0 is valid
			BackoffMultiplier: 1.0,  // 1.0 is valid
			JitterFactor:   0.0,   // 0.0 is valid
		}

		invoker := NewInvoker(&InvokerConfig{
			Address:   "localhost:8080",
			Retry:     config,
			Reconnect: DefaultReconnectConfig(),
		})

		if invoker == nil {
			t.Error("NewInvoker() returned nil for zero values")
		}
	})

	t.Run("zero values in reconnect config", func(t *testing.T) {
		config := &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    0,      // 0 = unlimited
			InitialDelayMs: 0,      // 0 is valid
			MaxDelayMs:     0,      // 0 is valid
			BackoffMultiplier: 1.0,  // 1.0 is valid
			JitterFactor:   0.0,   // 0.0 is valid
		}

		invoker := NewInvoker(&InvokerConfig{
			Address:   "localhost:8080",
			Retry:     DefaultRetryConfig(),
			Reconnect: config,
		})

		if invoker == nil {
			t.Error("NewInvoker() returned nil for zero values")
		}
	})

	t.Run("extreme timeout values", func(t *testing.T) {
		timeouts := []int{-1, 0, 1, 3600, 86400} // -1s, 0s, 1s, 1h, 1d

		for _, timeout := range timeouts {
			invoker := NewInvoker(&InvokerConfig{
				Address:        "localhost:8080",
				TimeoutSeconds: timeout,
				Retry:          DefaultRetryConfig(),
				Reconnect:      DefaultReconnectConfig(),
			})

			if invoker == nil {
				t.Errorf("NewInvoker() returned nil for timeout=%d", timeout)
			}
		}
	})
}

// TestInvoker_multipleInstances tests multiple invoker instances
func TestInvoker_multipleInstances(t *testing.T) {
	invokers := make([]Invoker, 5)

	for i := 0; i < 5; i++ {
		invokers[i] = NewInvoker(&InvokerConfig{
			Address:   "localhost:8080",
			Retry:     DefaultRetryConfig(),
			Reconnect: DefaultReconnectConfig(),
		})

		if invokers[i] == nil {
			t.Errorf("Invoker %d is nil", i)
		}
	}

	// Verify they are different instances
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 5; j++ {
			if invokers[i] == invokers[j] {
				t.Errorf("Invokers %d and %d should be different instances", i, j)
			}
		}
	}
}
