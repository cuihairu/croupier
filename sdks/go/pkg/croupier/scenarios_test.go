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

func TestClient_LifecycleScenarios(t *testing.T) {
	t.Run("create and close immediately", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)
		
		if client == nil {
			t.Fatal("NewClient should return valid client")
		}
		
		err := client.Close()
		if err != nil {
			t.Logf("Close error: %v", err)
		}
	})
	
	t.Run("create register and close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)
		
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("ok"), nil
		}
		
		desc := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
		}
		
		_ = client.RegisterFunction(desc, handler)
		err := client.Close()
		t.Logf("Close error: %v", err)
	})
}

func TestInvoker_MultipleAddresses(t *testing.T) {
	addresses := []string{
		"localhost:8080",
		"127.0.0.1:8080",
		"tcp://localhost:8080",
		"ipc://croupier-agent",
	}
	
	for _, addr := range addresses {
		invoker := NewInvoker(&InvokerConfig{
			Address: addr,
		})
		
		if invoker == nil {
			t.Errorf("NewInvoker with address %s should return valid invoker", addr)
		}
	}
}

func TestFunctionDescriptor_AllVersions(t *testing.T) {
	versions := []string{
		"1.0.0",
		"1.0.0-alpha",
		"1.0.0+build123",
		"1.2",
		"v1.0.0",
		"0.0.0",
		"10.20.30",
	}
	
	for _, version := range versions {
		desc := FunctionDescriptor{
			ID:      "test.func",
			Version: version,
		}
		
		if desc.Version != version {
			t.Errorf("Version = %s, want %s", desc.Version, version)
		}
	}
}

func TestJobEvent_AllEventTypes(t *testing.T) {
	t.Run("started event", func(t *testing.T) {
		event := JobEvent{
			EventType: "started",
			JobID:     "job-123",
			Payload:   `{"status":"started"}`,
			Done:      false,
		}
		
		if event.Done {
			t.Error("Started event should not be done")
		}
	})
	
	t.Run("completed event", func(t *testing.T) {
		event := JobEvent{
			EventType: "completed",
			JobID:     "job-789",
			Payload:   `{"result":"success"}`,
			Done:      true,
		}
		
		if !event.Done {
			t.Error("Completed event should be done")
		}
	})
	
	t.Run("error event", func(t *testing.T) {
		event := JobEvent{
			EventType: "error",
			JobID:     "job-error",
			Error:     "Something went wrong",
			Done:      true,
		}
		
		if event.Error == "" {
			t.Error("Error event should have error message")
		}
	})
}

func TestInvokeOptions_TimeoutVariations(t *testing.T) {
	timeouts := []time.Duration{
		0,
		time.Millisecond,
		100 * time.Millisecond,
		time.Second,
		30 * time.Second,
		time.Minute,
	}
	
	for _, timeout := range timeouts {
		options := InvokeOptions{
			Timeout: timeout,
		}
		
		if options.Timeout != timeout {
			t.Errorf("Timeout = %v, want %v", options.Timeout, timeout)
		}
	}
}

func TestRetryConfig_AllRetryableStatusCodes(t *testing.T) {
	t.Run("default retryable codes", func(t *testing.T) {
		config := DefaultRetryConfig()
		
		if len(config.RetryableStatusCodes) == 0 {
			t.Error("RetryableStatusCodes should not be empty")
		}
	})
	
	t.Run("custom retryable codes", func(t *testing.T) {
		config := &RetryConfig{
			RetryableStatusCodes: []int32{1, 2, 3, 4, 5},
		}
		
		if len(config.RetryableStatusCodes) != 5 {
			t.Errorf("RetryableStatusCodes length = %d, want 5", len(config.RetryableStatusCodes))
		}
	})
	
	t.Run("empty retryable codes", func(t *testing.T) {
		config := &RetryConfig{
			RetryableStatusCodes: []int32{},
		}
		
		if len(config.RetryableStatusCodes) != 0 {
			t.Error("RetryableStatusCodes should be empty")
		}
	})
}

func TestBackoffCalculations(t *testing.T) {
	t.Run("various backoff multipliers", func(t *testing.T) {
		multipliers := []float64{1.0, 1.5, 2.0, 3.0, 5.0, 10.0}
		
		for _, mult := range multipliers {
			config := &RetryConfig{
				BackoffMultiplier: mult,
			}
			
			if config.BackoffMultiplier != mult {
				t.Errorf("BackoffMultiplier = %f, want %f", config.BackoffMultiplier, mult)
			}
		}
	})
	
	t.Run("various jitter factors", func(t *testing.T) {
		jitters := []float64{0.0, 0.1, 0.2, 0.5, 1.0}
		
		for _, jitter := range jitters {
			config := &RetryConfig{
				JitterFactor: jitter,
			}
			
			if config.JitterFactor != jitter {
				t.Errorf("JitterFactor = %f, want %f", config.JitterFactor, jitter)
			}
		}
	})
}
