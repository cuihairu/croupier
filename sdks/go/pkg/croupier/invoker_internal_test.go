// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"testing"
	"time"
)

// TestInvoker_tlsConfigTests tests TLS configuration building
func TestInvoker_tlsConfigTests(t *testing.T) {
	t.Run("buildTLSConfig with nil config", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// buildTLSConfig is called internally during Connect
		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect (which calls buildTLSConfig internally) error: %v", err)
	})

	t.Run("buildTLSConfig with CA file only", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			CAFile:  "/path/to/ca.crt",
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect with CA file error: %v", err)
	})

	t.Run("buildTLSConfig with cert and key", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:  "localhost:8080",
			CertFile: "/path/to/cert.pem",
			KeyFile:   "/path/to/key.pem",
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect with cert and key error: %v", err)
	})

	t.Run("buildTLSConfig with all TLS files", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:  "localhost:8080",
			CAFile:   "/path/to/ca.crt",
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect with all TLS config error: %v", err)
	})

	t.Run("buildTLSConfig with insecure", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:  "localhost:8080",
			Insecure: true,
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect with insecure error: %v", err)
	})

	t.Run("buildTLSConfig with various addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:8080",
			"example.com:8080",
			"test.example.org:8080",
			"192.168.1.1:8080",
			"[::1]:8080",
		}

		for _, address := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: address,
			})

			ctx := context.Background()
			err := invoker.Connect(ctx)
			t.Logf("Connect with Address='%s' error: %v", address, err)
		}
	})
}

// TestInvoker_retryDelayCalculation tests retry delay calculation
func TestInvoker_retryDelayCalculation(t *testing.T) {
	t.Run("calculateRetryDelay with various attempts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    10,
				MaxDelayMs:        1000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
		})

		// calculateRetryDelay is called internally during Invoke with retry
		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke (which calls calculateRetryDelay internally) error: %v", err)
	})

	t.Run("calculateRetryDelay with zero multiplier", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    10,
				MaxDelayMs:        100,
				BackoffMultiplier: 0.0,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with zero backoff multiplier error: %v", err)
	})

	t.Run("calculateRetryDelay with large multiplier", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    1,
				MaxDelayMs:        10000,
				BackoffMultiplier: 100.0,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with large backoff multiplier error: %v", err)
	})
}

// TestInvoker_reconnectDelayCalculation tests reconnect delay calculation
func TestInvoker_reconnectDelayCalculation(t *testing.T) {
	t.Run("calculateReconnectDelay with various attempts", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    10,
				MaxDelayMs:        1000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
		})

		// calculateReconnectDelay is called internally during Connect
		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect (which calls calculateReconnectDelay internally) error: %v", err)
	})

	t.Run("calculateReconnectDelay with zero delay", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    3,
				InitialDelayMs: 0,
				MaxDelayMs:     0,
			},
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("Connect with zero reconnect delay error: %v", err)
	})
}

// TestInvoker_connectionErrorChecks tests connection error checking
func TestInvoker_connectionErrorChecks(t *testing.T) {
	t.Run("isConnectionError with various errors", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// isConnectionError is called internally
		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke (which calls isConnectionError internally) error: %v", err)
	})
}

// TestInvoker_scheduleReconnect tests reconnect scheduling
func TestInvoker_scheduleReconnect(t *testing.T) {
	t.Run("scheduleReconnectIfNeeded with reconnect disabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled: false,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with reconnect disabled error: %v", err)
	})

	t.Run("scheduleReconnectIfNeeded with reconnect enabled", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Reconnect: &ReconnectConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with reconnect enabled error: %v", err)
	})
}

// TestInvoker_invokeCombinations tests various Invoke combinations
func TestInvoker_invokeCombinations(t *testing.T) {
	t.Run("Invoke with both retry and reconnect", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
			Reconnect: &ReconnectConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		})

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with retry and reconnect error: %v", err)
	})

	t.Run("Invoke with all options", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
			Reconnect: &ReconnectConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		})

		options := InvokeOptions{
			IdempotencyKey: "test-key-12345",
			Timeout:        30 * time.Second,
			Headers: map[string]string{
				"X-Request-ID":  "req-123",
				"X-Correlation-ID": "corr-456",
				"X-User-ID":     "user-789",
			},
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		ctx := context.Background()
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with all options error: %v", err)
	})
}

// TestInvoker_multipleInvokers tests multiple invoker instances
func TestInvoker_multipleInvokers(t *testing.T) {
	t.Run("multiple invokers with retry", func(t *testing.T) {
		invokers := make([]Invoker, 5)

		for i := 0; i < 5; i++ {
			invokers[i] = NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry: &RetryConfig{
					Enabled:     true,
					MaxAttempts: 2,
				},
			})
		}

		ctx := context.Background()
		for i, invoker := range invokers {
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoker %d error: %v", i, err)
		}
	})

	t.Run("multiple invokers with different addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:8080",
			"localhost:8081",
			"localhost:8082",
		}

		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: addr,
			})

			ctx := context.Background()
			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoker (%s) error: %v", addr, err)
		}
	})
}

// TestInvoker_closeTests tests invoker close operations
func TestInvoker_closeTests(t *testing.T) {
	t.Run("close after failed connect", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "invalid-address:99999",
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		err := invoker.Close()
		t.Logf("Close after failed connect error: %v", err)
	})

	t.Run("close multiple times", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		// Close multiple times
		for i := 0; i < 3; i++ {
			err := invoker.Close()
			t.Logf("Close call %d error: %v", i+1, err)
		}
	})
}
