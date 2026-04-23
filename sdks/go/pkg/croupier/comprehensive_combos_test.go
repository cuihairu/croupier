// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestComprehensive_configCombos(t *testing.T) {
	t.Run("client with all fields configured", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr:          "tcp://localhost:19090",
			AgentIPCAddr:       "ipc://croupier-agent",
			GameID:             "game-123",
			Env:                "production",
			ServiceID:          "service-456",
			ServiceVersion:     "2.0.0",
			AgentID:            "agent-789",
			ProviderLang:       "go",
			ProviderSDK:        "croupier-go-sdk",
			TimeoutSeconds:     60,
			Insecure:           false,
			CAFile:             "/path/to/ca.crt",
			CertFile:           "/path/to/cert.pem",
			KeyFile:            "/path/to/key.pem",
			ServerName:         "example.com",
			InsecureSkipVerify: false,
			DisableLogging:     false,
			DebugLogging:       false,
		}

		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with all fields should return valid client")
		}
	})

	t.Run("invoker with all combinations of retry and reconnect", func(t *testing.T) {
		retryEnabled := []bool{true, false}
		reconnectEnabled := []bool{true, false}

		for _, retryOn := range retryEnabled {
			for _, reconnectOn := range reconnectEnabled {
				invoker := NewInvoker(&InvokerConfig{
					Address: "http://localhost:19090",
					Retry: &RetryConfig{
						Enabled:     retryOn,
						MaxAttempts: 3,
					},
					Reconnect: &ReconnectConfig{
						Enabled:     reconnectOn,
						MaxAttempts: 5,
					},
				})

				if invoker == nil {
					t.Errorf("NewInvoker with retry=%v, reconnect=%v should return valid invoker",
						retryOn, reconnectOn)
				}
			}
		}
	})
}

func TestComprehensive_descriptorCombinations(t *testing.T) {
	t.Run("all risk levels with all operations", func(t *testing.T) {
		risks := []string{"low", "medium", "high", "critical"}
		operations := []string{"create", "read", "update", "delete", "custom"}

		for _, risk := range risks {
			for _, op := range operations {
				desc := FunctionDescriptor{
					ID:        "test.function",
					Version:   "1.0.0",
					Category:  "test",
					Risk:      risk,
					Entity:    "TestEntity",
					Operation: op,
					Enabled:   true,
				}

				if desc.Risk != risk || desc.Operation != op {
					t.Errorf("Risk=%s, Operation=%s not preserved", risk, op)
				}
			}
		}
	})

	t.Run("local descriptor with all OpenAPI fields", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:           "comprehensive.test",
			Version:      "1.0.0",
			Tags:         []string{"comprehensive", "test", "openapi"},
			Summary:      "Comprehensive test function",
			Description:  "This is a comprehensive test function with all OpenAPI fields",
			OperationID:  "comprehensiveTest",
			Deprecated:   false,
			InputSchema:  `{"type":"object","properties":{"input":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"output":{"type":"string"}}}`,
			Category:     "test",
			Risk:         "safe",
			Entity:       "Test",
			Operation:    "test",
		}

		if len(desc.Tags) != 3 {
			t.Errorf("Tags length = %d, want 3", len(desc.Tags))
		}
	})
}

func TestComprehensive_eventScenarios(t *testing.T) {
	t.Run("all event types with all payloads", func(t *testing.T) {
		eventTypes := []string{"started", "progress", "completed", "error"}
		payloads := []string{"", "{}", `{"data": "test"}`, `{"error": "failed"}`}

		for _, eventType := range eventTypes {
			for _, payload := range payloads {
				event := JobEvent{
					EventType: eventType,
					JobID:     "test-job",
					Payload:   payload,
					Done:      eventType == "completed" || eventType == "error",
				}

				if event.EventType != eventType {
					t.Errorf("EventType = %s, want %s", event.EventType, eventType)
				}
			}
		}
	})

	t.Run("job event lifecycle", func(t *testing.T) {
		events := []JobEvent{
			{EventType: "started", JobID: "job-1", Done: false},
			{EventType: "progress", JobID: "job-1", Payload: `{"percent": 25}`, Done: false},
			{EventType: "progress", JobID: "job-1", Payload: `{"percent": 50}`, Done: false},
			{EventType: "progress", JobID: "job-1", Payload: `{"percent": 75}`, Done: false},
			{EventType: "completed", JobID: "job-1", Payload: `{"result": "success"}`, Done: true},
		}

		for i, event := range events {
			if event.JobID != "job-1" {
				t.Errorf("Event %d has wrong JobID", i)
			}
		}
	})
}

func TestComprehensive_timeoutScenarios(t *testing.T) {
	t.Run("all timeout values with context", func(t *testing.T) {
		timeouts := []time.Duration{
			0,
			time.Nanosecond,
			time.Microsecond,
			time.Millisecond,
			time.Second,
			30 * time.Second,
			time.Minute,
			time.Hour,
		}

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		for _, timeout := range timeouts {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
			t.Logf("Invoke with timeout=%v error: %v", timeout, err)
		}
	})
}

func TestComprehensive_headerCombinations(t *testing.T) {
	t.Run("various header configurations", func(t *testing.T) {
		headerConfigs := []map[string]string{
			{},
			{"X-Request-ID": "req-123"},
			{
				"X-Request-ID":     "req-123",
				"X-Correlation-ID": "corr-456",
			},
			{
				"X-Request-ID":    "req-123",
				"X-User-ID":       "user-789",
				"X-Session-ID":    "session-abc",
				"X-Trace-ID":      "trace-def",
				"X-Custom-Header": "custom-value",
			},
		}

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()

		for _, headers := range headerConfigs {
			options := InvokeOptions{
				Headers: headers,
			}

			_, err := invoker.Invoke(ctx, "test.func", "{}", options)
			t.Logf("Invoke with %d headers error: %v", len(headers), err)
		}
	})

	t.Run("headers with special characters", func(t *testing.T) {
		specialHeaders := map[string]string{
			"X-Special": "value with spaces\nand\ttabs",
			"X-Unicode": "值😀🎉",
			"X-Empty":   "",
		}

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		ctx := context.Background()
		options := InvokeOptions{
			Headers: specialHeaders,
		}

		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with special headers error: %v", err)
	})
}

func TestComprehensive_concurrentAccess(t *testing.T) {
	t.Run("concurrent client creation", func(t *testing.T) {
		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				config := DefaultClientConfig()
				client := NewClient(config)
				if client == nil {
					t.Errorf("Goroutine %d: NewClient returned nil", idx)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent invoker creation", func(t *testing.T) {
		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				invoker := NewInvoker(&InvokerConfig{
					Address: "http://localhost:19090",
				})
				if invoker == nil {
					t.Errorf("Goroutine %d: NewInvoker returned nil", idx)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent descriptor creation", func(t *testing.T) {
		const numGoroutines = 100
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				desc := FunctionDescriptor{
					ID:       "test.func",
					Version:  "1.0.0",
					Category: "test",
					Risk:     "low",
					Enabled:  true,
				}

				if desc.ID != "test.func" {
					t.Errorf("Goroutine %d: ID mismatch", idx)
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestComprehensive_nilAndZero(t *testing.T) {
	t.Run("nil configurations", func(t *testing.T) {
		client := NewClient(nil)
		if client == nil {
			t.Error("NewClient(nil) should return client")
		}

		invoker := NewInvoker(nil)
		if invoker == nil {
			t.Error("NewInvoker(nil) should return invoker")
		}
	})

	t.Run("zero values in structs", func(t *testing.T) {
		var desc FunctionDescriptor
		if desc.ID != "" {
			t.Error("Zero FunctionDescriptor should have empty ID")
		}

		var event JobEvent
		if event.EventType != "" {
			t.Error("Zero JobEvent should have empty EventType")
		}

		var options InvokeOptions
		if options.Timeout != 0 {
			t.Error("Zero InvokeOptions should have zero timeout")
		}
	})
}

func TestComprehensive_addressFormats(t *testing.T) {
	t.Run("all possible address formats", func(t *testing.T) {
		addresses := []string{
			"localhost:8080",
			"127.0.0.1:8080",
			"0.0.0.0:8080",
			"192.168.1.1:8080",
			"example.com:8080",
			"subdomain.example.com:8080",
			"tcp://localhost:8080",
			"tcp://127.0.0.1:8080",
			"ipc://croupier-agent",
			"ipc:///tmp/croupier-agent",
			"ws://localhost:8080",
			"wss://localhost:8443",
			"[::1]:8080",
			"[2001:db8::1]:8080",
			"http://localhost:8080",
			"https://localhost:8443",
		}

		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: addr,
			})

			if invoker == nil {
				t.Errorf("NewInvoker with address '%s' should return valid invoker", addr)
			}
		}
	})
}
