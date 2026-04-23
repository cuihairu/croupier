// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"testing"
)

// TestConfig_allCombinations tests all configuration combinations
func TestConfig_allCombinations(t *testing.T) {
	t.Run("ClientConfig with all timeout values", func(t *testing.T) {
		timeouts := []int{-10, -1, 0, 1, 10, 30, 60, 120, 300, 3600}

		for _, timeout := range timeouts {
			config := ClientConfig{
				TimeoutSeconds: timeout,
			}

			client := NewClient(&config)
			if client == nil {
				t.Errorf("Client with timeout=%d should not be nil", timeout)
			}

			_ = client.Close()
		}
	})

	t.Run("ClientConfig with all Insecure values", func(t *testing.T) {
		insecureValues := []bool{true, false}

		for _, insecure := range insecureValues {
			config := ClientConfig{
				Insecure: insecure,
			}

			client := NewClient(&config)
			if client == nil {
				t.Errorf("Client with insecure=%v should not be nil", insecure)
			}

			_ = client.Close()
		}
	})

	t.Run("ClientConfig TLS combinations", func(t *testing.T) {
		tlsConfigs := []struct {
			name   string
			config ClientConfig
		}{
			{
				"CA only",
				ClientConfig{
					CAFile: "/path/to/ca.crt",
				},
			},
			{
				"Cert and Key",
				ClientConfig{
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
			{
				"Full TLS",
				ClientConfig{
					CAFile:             "/path/to/ca.crt",
					CertFile:           "/path/to/cert.pem",
					KeyFile:            "/path/to/key.pem",
					ServerName:         "example.com",
					InsecureSkipVerify: false,
				},
			},
			{
				"InsecureSkipVerify",
				ClientConfig{
					CertFile:           "/path/to/cert.pem",
					KeyFile:            "/path/to/key.pem",
					InsecureSkipVerify: true,
				},
			},
		}

		for _, tc := range tlsConfigs {
			client := NewClient(&tc.config)
			if client == nil {
				t.Errorf("Client with TLS config '%s' should not be nil", tc.name)
			}

			_ = client.Close()
		}
	})
}

// TestInvokerConfig_variations tests InvokerConfig variations
func TestInvokerConfig_variations(t *testing.T) {
	t.Run("InvokerConfig with all timeout values", func(t *testing.T) {
		timeouts := []int{-10, -1, 0, 1, 10, 30, 60, 120, 300, 3600}

		for _, timeout := range timeouts {
			config := InvokerConfig{
				TimeoutSeconds: timeout,
			}

			invoker := NewInvoker(&config)
			if invoker == nil {
				t.Errorf("Invoker with timeout=%d should not be nil", timeout)
			}

			_ = invoker.Close()
		}
	})

	t.Run("InvokerConfig with all Insecure values", func(t *testing.T) {
		insecureValues := []bool{true, false}

		for _, insecure := range insecureValues {
			config := InvokerConfig{
				Insecure: insecure,
			}

			invoker := NewInvoker(&config)
			if invoker == nil {
				t.Errorf("Invoker with insecure=%v should not be nil", insecure)
			}

			_ = invoker.Close()
		}
	})

	t.Run("InvokerConfig TLS combinations", func(t *testing.T) {
		tlsConfigs := []struct {
			name   string
			config InvokerConfig
		}{
			{
				"CA only",
				InvokerConfig{
					Address: "http://localhost:19090",
					CAFile:  "/path/to/ca.crt",
				},
			},
			{
				"Cert and Key",
				InvokerConfig{
					Address:  "localhost:8080",
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
			{
				"Full TLS",
				InvokerConfig{
					Address:  "localhost:8080",
					CAFile:   "/path/to/ca.crt",
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
					Insecure: false,
				},
			},
			{
				"Insecure",
				InvokerConfig{
					Address:  "localhost:8080",
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
					Insecure: true,
				},
			},
		}

		for _, tc := range tlsConfigs {
			invoker := NewInvoker(&tc.config)
			if invoker == nil {
				t.Errorf("Invoker with TLS config '%s' should not be nil", tc.name)
			}

			_ = invoker.Close()
		}
	})
}

// TestRetryConfig_allValues tests RetryConfig with all value ranges
func TestRetryConfig_allValues(t *testing.T) {
	t.Run("RetryConfig with boundary MaxAttempts", func(t *testing.T) {
		maxAttempts := []int{
			-100, -10, -1, 0, 1, 2, 3, 5, 10, 100, 1000, 1000000,
		}

		for _, attempts := range maxAttempts {
			config := &RetryConfig{
				Enabled:     true,
				MaxAttempts: attempts,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry:   config,
			})

			if invoker == nil {
				t.Errorf("Invoker with MaxAttempts=%d should not be nil", attempts)
			}

			_ = invoker.Close()
		}
	})

	t.Run("RetryConfig with boundary delays", func(t *testing.T) {
		delays := []int{
			-1000, -100, -10, -1, 0, 1, 10, 100, 1000, 10000, 100000, 1000000,
		}

		for _, delay := range delays {
			config := &RetryConfig{
				Enabled:        true,
				MaxAttempts:    2,
				InitialDelayMs: delay,
				MaxDelayMs:     delay * 10,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry:   config,
			})

			if invoker == nil {
				t.Errorf("Invoker with InitialDelayMs=%d should not be nil", delay)
			}

			_ = invoker.Close()
		}
	})

	t.Run("RetryConfig with boundary multipliers", func(t *testing.T) {
		multipliers := []float64{
			-100.0, -10.0, -1.0, -0.1, 0.0, 0.001, 0.01, 0.1, 1.0, 1.5, 2.0,
			5.0, 10.0, 100.0, 1000.0,
		}

		for _, mult := range multipliers {
			config := &RetryConfig{
				Enabled:           true,
				MaxAttempts:       2,
				BackoffMultiplier: mult,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry:   config,
			})

			if invoker == nil {
				t.Errorf("Invoker with BackoffMultiplier=%.3f should not be nil", mult)
			}

			_ = invoker.Close()
		}
	})

	t.Run("RetryConfig with boundary jitter", func(t *testing.T) {
		jitters := []float64{
			-1.0, -0.5, -0.1, 0.0, 0.1, 0.2, 0.5, 1.0, 2.0,
		}

		for _, jitter := range jitters {
			config := &RetryConfig{
				Enabled:      true,
				MaxAttempts: 2,
				JitterFactor: jitter,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address: "http://localhost:19090",
				Retry:   config,
			})

			if invoker == nil {
				t.Errorf("Invoker with JitterFactor=%.2f should not be nil", jitter)
			}

			_ = invoker.Close()
		}
	})
}

// TestReconnectConfig_allValues tests ReconnectConfig with all value ranges
func TestReconnectConfig_allValues(t *testing.T) {
	t.Run("ReconnectConfig with boundary MaxAttempts", func(t *testing.T) {
		maxAttempts := []int{
			-100, -10, -1, 0, 1, 2, 3, 5, 10, 100, 1000, 1000000,
		}

		for _, attempts := range maxAttempts {
			config := &ReconnectConfig{
				Enabled:     true,
				MaxAttempts: attempts,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Reconnect: config,
			})

			if invoker == nil {
				t.Errorf("Invoker with Reconnect MaxAttempts=%d should not be nil", attempts)
			}

			_ = invoker.Close()
		}
	})

	t.Run("ReconnectConfig with boundary delays", func(t *testing.T) {
		delays := []int{
			-1000, -100, -10, -1, 0, 1, 10, 100, 1000, 10000, 100000, 1000000,
		}

		for _, delay := range delays {
			config := &ReconnectConfig{
				Enabled:        true,
				MaxAttempts:    2,
				InitialDelayMs: delay,
				MaxDelayMs:     delay * 10,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Reconnect: config,
			})

			if invoker == nil {
				t.Errorf("Invoker with Reconnect InitialDelayMs=%d should not be nil", delay)
			}

			_ = invoker.Close()
		}
	})

	t.Run("ReconnectConfig with boundary multipliers", func(t *testing.T) {
		multipliers := []float64{
			-100.0, -10.0, -1.0, -0.1, 0.0, 0.001, 0.01, 0.1, 1.0, 1.5, 2.0,
			5.0, 10.0, 100.0, 1000.0,
		}

		for _, mult := range multipliers {
			config := &ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       2,
				BackoffMultiplier: mult,
			}

			invoker := NewInvoker(&InvokerConfig{
				Address:   "localhost:8080",
				Reconnect: config,
			})

			if invoker == nil {
				t.Errorf("Invoker with Reconnect BackoffMultiplier=%.3f should not be nil", mult)
			}

			_ = invoker.Close()
		}
	})
}

// TestInvokeOptions_combinations tests InvokeOptions combinations
func TestConfig_InvokeOptions_combinations(t *testing.T) {
	t.Run("InvokeOptions with all fields", func(t *testing.T) {
		options := InvokeOptions{
			IdempotencyKey: "unique-key-12345",
			Timeout:        30 * 1000 * 1000 * 1000, // 30 seconds
			Headers: map[string]string{
				"X-Request-ID":     "req-123",
				"X-Correlation-ID": "corr-456",
				"X-User-ID":        "user-789",
				"X-Session-ID":     "session-abc",
				"X-Trace-ID":       "trace-def",
				"X-Client-Version": "1.0.0",
			},
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}

		if options.IdempotencyKey != "unique-key-12345" {
			t.Error("IdempotencyKey not set correctly")
		}

		if len(options.Headers) != 6 {
			t.Errorf("Expected 6 headers, got %d", len(options.Headers))
		}

		if options.Retry == nil {
			t.Error("Retry config not set")
		}
	})

	t.Run("InvokeOptions with only IdempotencyKey", func(t *testing.T) {
		options := InvokeOptions{
			IdempotencyKey: "key-only",
		}

		if options.IdempotencyKey != "key-only" {
			t.Error("IdempotencyKey not set correctly")
		}

		if options.Timeout != 0 {
			t.Error("Timeout should be 0 when not set")
		}
	})

	t.Run("InvokeOptions with only Timeout", func(t *testing.T) {
		options := InvokeOptions{
			Timeout: 60 * 1000 * 1000 * 1000,
		}

		if options.Timeout != 60*1000*1000*1000 {
			t.Error("Timeout not set correctly")
		}
	})

	t.Run("InvokeOptions with only Headers", func(t *testing.T) {
		headers := map[string]string{
			"X-Test": "value",
		}

		options := InvokeOptions{
			Headers: headers,
		}

		if options.Headers == nil {
			t.Error("Headers should not be nil")
		}

		if len(options.Headers) != 1 {
			t.Errorf("Expected 1 header, got %d", len(options.Headers))
		}
	})

	t.Run("InvokeOptions with only Retry", func(t *testing.T) {
		retry := &RetryConfig{
			Enabled:     true,
			MaxAttempts: 3,
		}

		options := InvokeOptions{
			Retry: retry,
		}

		if options.Retry == nil {
			t.Error("Retry should not be nil")
		}

		if options.Retry.Enabled != true {
			t.Error("Retry Enabled not set correctly")
		}
	})

	t.Run("InvokeOptions with empty IdempotencyKey", func(t *testing.T) {
		options := InvokeOptions{
			IdempotencyKey: "",
		}

		if options.IdempotencyKey != "" {
			t.Error("IdempotencyKey should be empty")
		}
	})

	t.Run("InvokeOptions with empty Headers", func(t *testing.T) {
		options := InvokeOptions{
			Headers: map[string]string{},
		}

		if options.Headers == nil {
			t.Error("Headers should not be nil")
		}

		if len(options.Headers) != 0 {
			t.Errorf("Expected 0 headers, got %d", len(options.Headers))
		}
	})
}

// TestDefaultConfigurations tests default configurations
func TestDefaultConfigurations_extended(t *testing.T) {
	t.Run("DefaultClientConfig returns valid config", func(t *testing.T) {
		config := DefaultClientConfig()

		if config.AgentAddr == "" {
			t.Error("DefaultClientConfig AgentAddr should not be empty")
		}

		if config.TimeoutSeconds == 0 {
			t.Error("DefaultClientConfig TimeoutSeconds should not be 0")
		}

		t.Logf("DefaultClientConfig: AgentAddr=%s, Timeout=%d, Insecure=%v",
			config.AgentAddr, config.TimeoutSeconds, config.Insecure)
	})

	t.Run("DefaultRetryConfig returns valid config", func(t *testing.T) {
		config := DefaultRetryConfig()

		if config == nil {
			t.Fatal("DefaultRetryConfig should not return nil")
		}

		if !config.Enabled {
			t.Error("DefaultRetryConfig should be enabled")
		}

		if config.MaxAttempts <= 0 {
			t.Error("DefaultRetryConfig MaxAttempts should be positive")
		}

		t.Logf("DefaultRetryConfig: Enabled=%v, MaxAttempts=%d, InitialDelayMs=%d",
			config.Enabled, config.MaxAttempts, config.InitialDelayMs)
	})

	t.Run("DefaultReconnectConfig returns valid config", func(t *testing.T) {
		config := DefaultReconnectConfig()

		if config == nil {
			t.Fatal("DefaultReconnectConfig should not return nil")
		}

		if !config.Enabled {
			t.Error("DefaultReconnectConfig should be enabled")
		}

		// MaxAttempts can be 0 for infinite retries
		if config.MaxAttempts < 0 {
			t.Error("DefaultReconnectConfig MaxAttempts should be non-negative")
		}

		t.Logf("DefaultReconnectConfig: Enabled=%v, MaxAttempts=%d, InitialDelayMs=%d",
			config.Enabled, config.MaxAttempts, config.InitialDelayMs)
	})
}

// TestAddressFormats_allVariations tests all address format variations
func TestAddressFormats_allVariations(t *testing.T) {
	t.Run("Invoker with various address formats", func(t *testing.T) {
		addresses := []string{
			// Simple formats
			"localhost:8080",
			"127.0.0.1:8080",
			"0.0.0.0:8080",
			"192.168.1.1:8080",
			"example.com:8080",
			// Protocol prefixes
			"tcp://localhost:8080",
			"tcp://127.0.0.1:8080",
			"ipc:///tmp/croupier",
			"ipc://croupier",
			// WebSocket
			"ws://localhost:8080",
			"wss://localhost:8443",
			// HTTP
			"http://localhost:8080",
			"https://localhost:8443",
			// IPv6
			"[::1]:8080",
			"[2001:db8::1]:8080",
			"tcp://[::1]:8080",
			// With paths
			"http://localhost:8080/api",
			"http://localhost:8080/api/v1",
			"tcp://localhost:8080/path",
		}

		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: addr,
			})

			if invoker == nil {
				t.Errorf("Invoker with address '%s' should not be nil", addr)
			}

			_ = invoker.Close()
		}
	})

	t.Run("Client with various address formats", func(t *testing.T) {
		addresses := []string{
			"localhost:19090",
			"127.0.0.1:19090",
			"tcp://localhost:19090",
			"ipc:///tmp/croupier-agent",
			"ipc://croupier-agent",
		}

		for _, addr := range addresses {
			config := &ClientConfig{
				AgentAddr: addr,
			}

			client := NewClient(config)

			if client == nil {
				t.Errorf("Client with address '%s' should not be nil", addr)
			}

			_ = client.Close()
		}
	})
}

// TestZeroValue_structs tests zero value structs
func TestZeroValue_structs(t *testing.T) {
	t.Run("zero FunctionDescriptor", func(t *testing.T) {
		var desc FunctionDescriptor

		if desc.ID != "" {
			t.Error("Zero FunctionDescriptor ID should be empty")
		}
		if desc.Version != "" {
			t.Error("Zero FunctionDescriptor Version should be empty")
		}
		if desc.Enabled != false {
			t.Error("Zero FunctionDescriptor Enabled should be false")
		}

		t.Logf("Zero FunctionDescriptor: ID='%s', Version='%s', Enabled=%v",
			desc.ID, desc.Version, desc.Enabled)
	})

	t.Run("zero JobEvent", func(t *testing.T) {
		var event JobEvent

		if event.EventType != "" {
			t.Error("Zero JobEvent EventType should be empty")
		}
		if event.JobID != "" {
			t.Error("Zero JobEvent JobID should be empty")
		}
		if event.Done != false {
			t.Error("Zero JobEvent Done should be false")
		}

		t.Logf("Zero JobEvent: EventType='%s', JobID='%s', Done=%v",
			event.EventType, event.JobID, event.Done)
	})

	t.Run("zero InvokeOptions", func(t *testing.T) {
		var options InvokeOptions

		if options.IdempotencyKey != "" {
			t.Error("Zero InvokeOptions IdempotencyKey should be empty")
		}
		if options.Timeout != 0 {
			t.Error("Zero InvokeOptions Timeout should be 0")
		}
		if options.Headers != nil {
			t.Error("Zero InvokeOptions Headers should be nil")
		}
		if options.Retry != nil {
			t.Error("Zero InvokeOptions Retry should be nil")
		}

		t.Logf("Zero InvokeOptions: IdempotencyKey='%s', Timeout=%v, Headers=%v, Retry=%v",
			options.IdempotencyKey, options.Timeout, options.Headers, options.Retry)
	})

	t.Run("zero ClientConfig", func(t *testing.T) {
		var config ClientConfig

		t.Logf("Zero ClientConfig: AgentAddr='%s', TimeoutSeconds=%d, Insecure=%v",
			config.AgentAddr, config.TimeoutSeconds, config.Insecure)
	})

	t.Run("zero InvokerConfig", func(t *testing.T) {
		var config InvokerConfig

		t.Logf("Zero InvokerConfig: Address='%s', TimeoutSeconds=%d, Insecure=%v",
			config.Address, config.TimeoutSeconds, config.Insecure)
	})

	t.Run("zero RetryConfig", func(t *testing.T) {
		var config RetryConfig

		t.Logf("Zero RetryConfig: Enabled=%v, MaxAttempts=%d",
			config.Enabled, config.MaxAttempts)
	})

	t.Run("zero ReconnectConfig", func(t *testing.T) {
		var config ReconnectConfig

		t.Logf("Zero ReconnectConfig: Enabled=%v, MaxAttempts=%d",
			config.Enabled, config.MaxAttempts)
	})
}
