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

func TestHTTPInvoker_fullConfig(t *testing.T) {
	t.Run("HTTP invoker with full config", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "http://localhost:8080",
			TimeoutSeconds: 30,
			Insecure:       false,
			Reconnect:      DefaultReconnectConfig(),
			Retry:          DefaultRetryConfig(),
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Error("NewHTTPInvoker should return valid invoker")
		}
	})

	t.Run("HTTP invoker with secure config", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "https://localhost:8443",
			TimeoutSeconds: 60,
			Insecure:       false,
			CAFile:         "/path/to/ca.crt",
			CertFile:       "/path/to/cert.pem",
			KeyFile:        "/path/to/key.pem",
			Reconnect:      DefaultReconnectConfig(),
			Retry:          DefaultRetryConfig(),
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Error("NewHTTPInvoker with secure config should return valid invoker")
		}
	})
}

func TestHTTPInvoker_connectAttempts(t *testing.T) {
	t.Run("connect with context", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx := context.Background()
		err := invoker.Connect(ctx)
		t.Logf("HTTPInvoker Connect error: %v", err)
	})

	t.Run("connect with timeout", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := invoker.Connect(ctx)
		t.Logf("HTTPInvoker Connect with timeout error: %v", err)
	})

	t.Run("connect when already connected", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)
		err := invoker.Connect(ctx)
		t.Logf("HTTPInvoker Connect again error: %v", err)
	})
}

func TestHTTPInvoker_closeOperations(t *testing.T) {
	t.Run("close new invoker", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		err := invoker.Close()
		if err != nil {
			t.Logf("HTTPInvoker Close error: %v", err)
		}
	})

	t.Run("close after connect attempt", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx := context.Background()
		_ = invoker.Connect(ctx)

		err := invoker.Close()
		if err != nil {
			t.Logf("HTTPInvoker Close after connect error: %v", err)
		}
	})

	t.Run("close multiple times", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		err1 := invoker.Close()
		err2 := invoker.Close()
		err3 := invoker.Close()

		t.Logf("Close errors: 1st=%v, 2nd=%v, 3rd=%v", err1, err2, err3)
	})
}

func TestHTTPInvoker_invokeVariations(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: "http://localhost:8080",
	})

	ctx := context.Background()

	t.Run("invoke with empty payload", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.func", "", InvokeOptions{})
		t.Logf("Invoke with empty payload error: %v", err)
	})

	t.Run("invoke with JSON payload", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.func", `{"key":"value"}`, InvokeOptions{})
		t.Logf("Invoke with JSON payload error: %v", err)
	})

	t.Run("invoke with large payload", func(t *testing.T) {
		largePayload := string(make([]byte, 1024*10))
		_, err := invoker.Invoke(ctx, "test.func", largePayload, InvokeOptions{})
		t.Logf("Invoke with large payload error: %v", err)
	})

	t.Run("invoke with special characters in payload", func(t *testing.T) {
		specialPayload := `{"data":"test\n\t\r"}`
		_, err := invoker.Invoke(ctx, "test.func", specialPayload, InvokeOptions{})
		t.Logf("Invoke with special chars error: %v", err)
	})
}

func TestHTTPInvoker_jobOperations(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: "http://localhost:8080",
	})

	ctx := context.Background()

	t.Run("start job with empty options", func(t *testing.T) {
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

func TestHTTPInvoker_setSchema(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: "http://localhost:8080",
	})

	t.Run("set schema for function", func(t *testing.T) {
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

	t.Run("set schema for multiple functions", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
		}

		functions := []string{"func1", "func2", "func3"}
		for _, fn := range functions {
			err := invoker.SetSchema(fn, schema)
			t.Logf("SetSchema for %s error: %v", fn, err)
		}
	})

	t.Run("set empty schema", func(t *testing.T) {
		schema := map[string]interface{}{}
		err := invoker.SetSchema("test.func", schema)
		t.Logf("SetSchema with empty schema error: %v", err)
	})
}

func TestHTTPInvoker_connectionState(t *testing.T) {
	t.Run("check IsConnected initially", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		connected := invoker.(*httpInvoker).IsConnected()
		t.Logf("IsConnected (initial): %v", connected)
	})

	t.Run("check GetAddress", func(t *testing.T) {
		addresses := []string{
			"http://localhost:8080",
			"https://localhost:8443",
			"http://example.com:8080",
		}

		for _, addr := range addresses {
			invoker := NewHTTPInvoker(&InvokerConfig{
				Address: addr,
			})

			retrievedAddr := invoker.(*httpInvoker).GetAddress()
			t.Logf("Configured: %s, Retrieved: %s", addr, retrievedAddr)
		}
	})
}

func TestHTTPInvoker_variousAddresses(t *testing.T) {
	addresses := []struct {
		name    string
		address string
	}{
		{"localhost", "http://localhost:8080"},
		{"127.0.0.1", "http://127.0.0.1:8080"},
		{"example.com", "http://example.com:8080"},
		{"with path", "http://localhost:8080/api"},
		{"HTTPS", "https://localhost:8443"},
	}

	for _, tc := range addresses {
		t.Run(tc.name, func(t *testing.T) {
			invoker := NewHTTPInvoker(&InvokerConfig{
				Address: tc.address,
			})

			if invoker == nil {
				t.Errorf("NewHTTPInvoker with %s should return valid invoker", tc.name)
			}
		})
	}
}

func TestHTTPInvoker_concurrentOperations(t *testing.T) {
	t.Run("concurrent invoke", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx := context.Background()

		for i := 0; i < 10; i++ {
			go func(idx int) {
				_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
				t.Logf("Concurrent invoke %d error: %v", idx, err)
			}(i)
		}
	})

	t.Run("concurrent job operations", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:8080",
		})

		ctx := context.Background()

		for i := 0; i < 5; i++ {
			go func(idx int) {
				jobID, _ := invoker.StartJob(ctx, "test.func", "{}", InvokeOptions{})
				_ = invoker.CancelJob(ctx, jobID)
				t.Logf("Concurrent job operation %d", idx)
			}(i)
		}
	})
}
