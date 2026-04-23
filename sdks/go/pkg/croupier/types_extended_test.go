// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"testing"
)

// TestTypes_FunctionDescriptor tests FunctionDescriptor variations
func TestTypes_FunctionDescriptor(t *testing.T) {
	t.Run("FunctionDescriptor with minimal fields", func(t *testing.T) {
		desc := FunctionDescriptor{
			ID:      "test.minimal",
			Version: "1.0.0",
		}

		if desc.ID != "test.minimal" {
			t.Errorf("Expected ID 'test.minimal', got '%s'", desc.ID)
		}
		if desc.Version != "1.0.0" {
			t.Errorf("Expected Version '1.0.0', got '%s'", desc.Version)
		}
	})

	t.Run("FunctionDescriptor with all fields", func(t *testing.T) {
		desc := FunctionDescriptor{
			ID:        "test.full",
			Version:   "2.0.0",
			Category:  "test",
			Risk:      "low",
			Entity:    "TestEntity",
			Operation: "create",
			Enabled:   true,
		}

		t.Logf("Full descriptor: ID=%s, Version=%s, Category=%s, Risk=%s",
			desc.ID, desc.Version, desc.Category, desc.Risk)
	})

	t.Run("FunctionDescriptor with special characters in ID", func(t *testing.T) {
		ids := []string{
			"test.with-dash",
			"test.with_underscore",
			"test.with.dot",
			"test.with:colon",
			"test.MixedCase",
			"test123",
		}

		for _, id := range ids {
			desc := FunctionDescriptor{
				ID:      id,
				Version: "1.0.0",
			}
			t.Logf("ID with special characters: '%s'", desc.ID)
		}
	})

	t.Run("FunctionDescriptor Enabled variations", func(t *testing.T) {
		enabledDesc := FunctionDescriptor{
			ID:      "test.enabled",
			Version: "1.0.0",
			Enabled: true,
		}

		disabledDesc := FunctionDescriptor{
			ID:      "test.disabled",
			Version: "1.0.0",
			Enabled: false,
		}

		t.Logf("Enabled: %v, Disabled: %v", enabledDesc.Enabled, disabledDesc.Enabled)
	})
}

// TestTypes_InvokeOptions tests InvokeOptions variations
func TestTypes_InvokeOptions(t *testing.T) {
	t.Run("InvokeOptions with all fields", func(t *testing.T) {
		opts := InvokeOptions{
			IdempotencyKey: "test-key-123",
			Timeout:        30 * 1000 * 1000 * 1000, // 30s in nanoseconds
			Headers: map[string]string{
				"X-Custom-Header": "value",
				"X-Another":      "another-value",
			},
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}

		t.Logf("InvokeOptions: IdempotencyKey=%s, Timeout=%v",
			opts.IdempotencyKey, opts.Timeout)
	})

	t.Run("InvokeOptions with empty IdempotencyKey", func(t *testing.T) {
		opts := InvokeOptions{
			IdempotencyKey: "",
		}

		t.Logf("Empty IdempotencyKey: '%s'", opts.IdempotencyKey)
	})

	t.Run("InvokeOptions with nil headers map", func(t *testing.T) {
		opts := InvokeOptions{}

		if opts.Headers == nil {
			t.Log("Headers map is nil")
		}
	})

	t.Run("InvokeOptions with nil Retry", func(t *testing.T) {
		opts := InvokeOptions{
			Retry: nil,
		}

		if opts.Retry == nil {
			t.Log("Retry config is nil")
		}
	})
}

// TestTypes_RetryConfig tests RetryConfig variations
func TestTypes_RetryConfig(t *testing.T) {
	t.Run("RetryConfig with all fields", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:           true,
			MaxAttempts:       5,
			InitialDelayMs:    100,
			MaxDelayMs:        5000,
			BackoffMultiplier: 2.0,
			JitterFactor:      0.1,
		}

		t.Logf("RetryConfig: Enabled=%v, MaxAttempts=%d, Multiplier=%.2f",
			config.Enabled, config.MaxAttempts, config.BackoffMultiplier)
	})

	t.Run("RetryConfig boundary values", func(t *testing.T) {
		configs := []*RetryConfig{
			{Enabled: true, MaxAttempts: 0},
			{Enabled: true, MaxAttempts: 1},
			{Enabled: true, MaxAttempts: 100},
			{Enabled: false, MaxAttempts: -1},
			{Enabled: true, InitialDelayMs: -100},
			{Enabled: true, InitialDelayMs: 0},
			{Enabled: true, BackoffMultiplier: 0.0},
			{Enabled: true, BackoffMultiplier: 10.0},
		}

		for i, config := range configs {
			t.Logf("Config %d: MaxAttempts=%d, InitialDelayMs=%d, Multiplier=%.2f",
				i, config.MaxAttempts, config.InitialDelayMs, config.BackoffMultiplier)
		}
	})
}

// TestTypes_ReconnectConfig tests ReconnectConfig variations
func TestTypes_ReconnectConfig(t *testing.T) {
	t.Run("ReconnectConfig with all fields", func(t *testing.T) {
		config := &ReconnectConfig{
			Enabled:           true,
			MaxAttempts:       3,
			InitialDelayMs:    1000,
			MaxDelayMs:        10000,
			BackoffMultiplier: 1.5,
			JitterFactor:      0.2,
		}

		t.Logf("ReconnectConfig: Enabled=%v, MaxAttempts=%d",
			config.Enabled, config.MaxAttempts)
	})

	t.Run("ReconnectConfig boundary values", func(t *testing.T) {
		configs := []*ReconnectConfig{
			{Enabled: true, MaxAttempts: 0},
			{Enabled: true, MaxAttempts: 1},
			{Enabled: true, MaxAttempts: 100},
			{Enabled: false},
			{Enabled: true, InitialDelayMs: 0},
			{Enabled: true, BackoffMultiplier: 1.0},
		}

		for i, config := range configs {
			t.Logf("Config %d: MaxAttempts=%d, InitialDelayMs=%d",
				i, config.MaxAttempts, config.InitialDelayMs)
		}
	})
}

// TestTypes_InvokerConfig tests InvokerConfig variations
func TestTypes_InvokerConfig(t *testing.T) {
	t.Run("InvokerConfig minimal", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		t.Logf("Minimal config: Address=%s", config.Address)
	})

	t.Run("InvokerConfig with all fields", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "http://localhost:8080",
			TimeoutSeconds: 30,
			Insecure:       true,
			CAFile:         "/path/to/ca.crt",
			CertFile:       "/path/to/cert.pem",
			KeyFile:        "/path/to/key.pem",
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
			Reconnect: &ReconnectConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}

		t.Logf("Full config: Address=%s, TimeoutSeconds=%d, Insecure=%v",
			config.Address, config.TimeoutSeconds, config.Insecure)
	})

	t.Run("InvokerConfig with nil Retry and Reconnect", func(t *testing.T) {
		config := &InvokerConfig{
			Address:   "localhost:8080",
			Retry:     nil,
			Reconnect: nil,
		}

		t.Logf("Config with nil configs: Retry=%v, Reconnect=%v",
			config.Retry, config.Reconnect)
	})
}

// TestTypes_ClientConfig tests ClientConfig variations
func TestTypes_ClientConfig(t *testing.T) {
	t.Run("ClientConfig with all fields", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr:      "localhost:19090",
			AgentIPCAddr:   "ipc://croupier-agent",
			GameID:         "test-game",
			Env:            "development",
			ServiceID:      "service-123",
			ServiceVersion: "1.0.0",
			AgentID:        "agent-1",
			TimeoutSeconds: 30,
			Insecure:       true,
			CAFile:         "/path/to/ca.crt",
			CertFile:       "/path/to/cert.pem",
			KeyFile:        "/path/to/key.pem",
			ServerName:     "example.com",
		}

		t.Logf("Full config: GameID=%s, Env=%s, ServiceID=%s",
			config.GameID, config.Env, config.ServiceID)
	})

	t.Run("ClientConfig minimal", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
		}

		_ = config
		t.Log("Minimal config created")
	})

	t.Run("ClientConfig with various Env values", func(t *testing.T) {
		envs := []string{"development", "staging", "production", "test", ""}

		for _, env := range envs {
			config := &ClientConfig{
				AgentAddr: "localhost:19090",
				Env:       env,
			}
			_ = config
			t.Logf("Config with Env='%s'", env)
		}
	})
}

// TestTypes_JobEvent tests JobEvent variations
func TestTypes_JobEvent(t *testing.T) {
	t.Run("JobEvent with all event types", func(t *testing.T) {
		eventTypes := []string{"started", "progress", "completed", "error"}

		for _, eventType := range eventTypes {
			event := JobEvent{
				EventType: eventType,
				JobID:     "job-123",
				Payload:   `{"status":"processing"}`,
				Error:     "",
			}

			t.Logf("JobEvent: Type=%s, JobID=%s", event.EventType, event.JobID)
		}
	})

	t.Run("JobEvent with error", func(t *testing.T) {
		event := JobEvent{
			EventType: "error",
			JobID:     "job-456",
			Payload:   "",
			Error:     "processing failed",
		}

		t.Logf("Error event: Error='%s'", event.Error)
	})

	t.Run("JobEvent with progress", func(t *testing.T) {
		event := JobEvent{
			EventType: "progress",
			JobID:     "job-789",
			Payload:   `{"progress":50,"message":"Half done"}`,
			Error:     "",
		}

		t.Logf("Progress event: Payload='%s'", event.Payload)
	})
}

// TestTypes_FunctionHandler tests FunctionHandler scenarios
func TestTypes_FunctionHandler(t *testing.T) {
	t.Run("FunctionHandler returning success", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"success"}`), nil
		})

		result, err := handler(context.Background(), []byte(`{}`))
		t.Logf("Handler result: %s, error: %v", string(result), err)
	})

	t.Run("FunctionHandler returning error", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		})

		result, err := handler(context.Background(), []byte(`{}`))
		t.Logf("Handler with error: result=%v, error=%v", result, err)
	})

	t.Run("FunctionHandler with nil payload", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			if payload == nil {
				return []byte(`{"result":"handled nil"}`), nil
			}
			return payload, nil
		})

		result, err := handler(context.Background(), nil)
		t.Logf("Handler with nil payload: %s, error: %v", string(result), err)
	})

	t.Run("FunctionHandler with empty payload", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			if len(payload) == 0 {
				return []byte(`{"result":"handled empty"}`), nil
			}
			return payload, nil
		})

		result, err := handler(context.Background(), []byte{})
		t.Logf("Handler with empty payload: %s, error: %v", string(result), err)
	})

	t.Run("FunctionHandler with context cancellation", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return []byte(`{"result":"ok"}`), nil
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := handler(ctx, []byte(`{}`))
		t.Logf("Handler with cancelled context: result=%v, error=%v", result, err)
	})
}

// TestTypes_zero_values tests zero value handling
func TestTypes_zero_values(t *testing.T) {
	t.Run("FunctionDescriptor zero values", func(t *testing.T) {
		var desc FunctionDescriptor
		t.Logf("Zero descriptor: ID='%s', Version='%s', Enabled=%v",
			desc.ID, desc.Version, desc.Enabled)
	})

	t.Run("InvokeOptions zero values", func(t *testing.T) {
		var opts InvokeOptions
		t.Logf("Zero options: Timeout=%v, Headers=%v",
			opts.Timeout, opts.Headers)
	})

	t.Run("RetryConfig zero values", func(t *testing.T) {
		var config RetryConfig
		t.Logf("Zero retry config: Enabled=%v, MaxAttempts=%d",
			config.Enabled, config.MaxAttempts)
	})

	t.Run("ReconnectConfig zero values", func(t *testing.T) {
		var config ReconnectConfig
		t.Logf("Zero reconnect config: Enabled=%v, MaxAttempts=%d",
			config.Enabled, config.MaxAttempts)
	})

	t.Run("InvokerConfig zero values", func(t *testing.T) {
		var config InvokerConfig
		t.Logf("Zero invoker config: Address='%s', TimeoutSeconds=%d",
			config.Address, config.TimeoutSeconds)
	})

	t.Run("ClientConfig zero values", func(t *testing.T) {
		var config ClientConfig
		t.Logf("Zero client config: AgentAddr='%s', TimeoutSeconds=%d",
			config.AgentAddr, config.TimeoutSeconds)
	})
}
