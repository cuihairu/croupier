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

func TestInvoker_NewInvoker_various(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		invoker := NewInvoker(nil)
		if invoker == nil {
			t.Error("NewInvoker(nil) should return invoker")
		}
	})
	
	t.Run("with empty config", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{})
		if invoker == nil {
			t.Error("NewInvoker(&InvokerConfig{}) should return invoker")
		}
	})
	
	t.Run("with minimal config", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})
		if invoker == nil {
			t.Error("NewInvoker with minimal config should return invoker")
		}
	})
}

func TestInvokerContext_usage(t *testing.T) {
	t.Run("context with timeout", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		_ = invoker.Connect(ctx)
	})
	
	t.Run("cancelled context", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		
		_ = invoker.Connect(ctx)
	})
}

func TestInvoker_InvokeOptions(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})
	
	ctx := context.Background()
	
	t.Run("invoke with empty options", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke error: %v", err)
	})
	
	t.Run("invoke with timeout", func(t *testing.T) {
		options := InvokeOptions{
			Timeout: 5 * time.Second,
		}
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with timeout error: %v", err)
	})
	
	t.Run("invoke with headers", func(t *testing.T) {
		options := InvokeOptions{
			Headers: map[string]string{
				"X-Request-ID": "req-123",
				"X-User-ID":    "user-456",
			},
		}
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with headers error: %v", err)
	})
	
	t.Run("invoke with retry override", func(t *testing.T) {
		options := InvokeOptions{
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with retry override error: %v", err)
	})
	
	t.Run("invoke with idempotency key", func(t *testing.T) {
		options := InvokeOptions{
			IdempotencyKey: "unique-key-12345",
		}
		_, err := invoker.Invoke(ctx, "test.func", "{}", options)
		t.Logf("Invoke with idempotency key error: %v", err)
	})
}

func TestInvoker_JobOperations(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})
	
	ctx := context.Background()
	
	t.Run("start job", func(t *testing.T) {
		jobID, err := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("StartJob jobID: %s, error: %v", jobID, err)
	})
	
	t.Run("start job with timeout", func(t *testing.T) {
		options := InvokeOptions{
			Timeout: 30 * time.Second,
		}
		jobID, err := invoker.StartJob(ctx, "test.func", "{}", options)
		t.Logf("StartJob with timeout jobID: %s, error: %v", jobID, err)
	})
	
	t.Run("stream job", func(t *testing.T) {
		events, err := invoker.StreamJob(ctx, "job-123")
		t.Logf("StreamJob error: %v", err)
		
		if err == nil && events != nil {
			t.Log("StreamJob returned event channel")
		}
	})
	
	t.Run("cancel job", func(t *testing.T) {
		err := invoker.CancelJob(ctx, "job-456")
		t.Logf("CancelJob error: %v", err)
	})
}

func TestInvoker_SetSchema_various(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})
	
	t.Run("set schema with valid JSON schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
		}
		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema error: %v", err)
	})
	
	t.Run("set schema with empty schema", func(t *testing.T) {
		schema := map[string]interface{}{}
		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with empty schema error: %v", err)
	})
	
	t.Run("set schema with nil schema", func(t *testing.T) {
		err := invoker.SetSchema("test.func", nil)
		t.Logf("SetSchema with nil error: %v", err)
	})
	
	t.Run("set schema for empty function ID", func(t *testing.T) {
		schema := map[string]interface{}{"type": "object"}
		err := invoker.SetSchema("", schema)
		t.Logf("SetSchema with empty ID error: %v", err)
	})
}
