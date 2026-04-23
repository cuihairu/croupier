// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

// TestBoundaryValues_Numeric tests numeric boundary values
func TestBoundaryValues_Numeric(t *testing.T) {
	t.Run("InvokeOptions timeout boundary values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		timeouts := []time.Duration{
			0,
			1,
			time.Nanosecond,
			time.Microsecond,
			time.Millisecond,
			time.Second,
			time.Minute,
			time.Hour,
			math.MaxInt64,
		}

		for _, timeout := range timeouts {
			opts := InvokeOptions{Timeout: timeout}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Timeout %v: error=%v, result_len=%d", timeout, err, len(result))
		}
	})

	t.Run("RetryConfig numeric boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		maxAttemptsValues := []int{
			0,
			1,
			2,
			10,
			100,
			1000,
			math.MaxInt32,
			math.MaxInt64,
		}

		for _, maxAttempts := range maxAttemptsValues {
			opts := InvokeOptions{
				Retry: &RetryConfig{
					Enabled:     true,
					MaxAttempts: maxAttempts,
				},
			}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("MaxAttempts %d: error=%v, result_len=%d", maxAttempts, err, len(result))
		}
	})

	t.Run("RetryConfig delay boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		delayValues := []int{
			-1000,
			-1,
			0,
			1,
			100,
			1000,
			10000,
			100000,
			math.MaxInt32,
		}

		for _, delay := range delayValues {
			opts := InvokeOptions{
				Retry: &RetryConfig{
					Enabled:        true,
					MaxAttempts:    2,
					InitialDelayMs: delay,
				},
			}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("InitialDelayMs %d: error=%v, result_len=%d", delay, err, len(result))
		}
	})
}

// TestBoundaryValues_StringLengths tests string length boundary values
func TestBoundaryValues_StringLengths(t *testing.T) {
	t.Run("Function ID length boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		lengths := []int{
			0,
			1,
			10,
			100,
			1000,
			10000,
		}

		for _, length := range lengths {
			functionID := string(make([]byte, length))
			result, err := invoker.Invoke(ctx, functionID, "{}", InvokeOptions{})
			t.Logf("FunctionID length %d: error=%v, result_len=%d", length, err, len(result))
		}
	})

	t.Run("Payload length boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		payloads := []string{
			"",
			"{",
			"{}",
			string(make([]byte, 10)),
			string(make([]byte, 100)),
			string(make([]byte, 1000)),
			string(make([]byte, 10000)),
		}

		for i, payload := range payloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Payload %d (len=%d): error=%v, result_len=%d", i, len(payload), err, len(result))
		}
	})

	t.Run("IdempotencyKey length boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		lengths := []int{
			0,
			1,
			32,
			64,
			128,
			256,
			512,
			1024,
		}

		for _, length := range lengths {
			key := string(make([]byte, length))
			opts := InvokeOptions{IdempotencyKey: key}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("IdempotencyKey length %d: error=%v, result_len=%d", length, err, len(result))
		}
	})
}

// TestBoundaryValues_SliceAndMap tests slice and map boundary values
func TestBoundaryValues_SliceAndMap(t *testing.T) {
	t.Run("Headers map boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Empty headers
		opts1 := InvokeOptions{
			Headers: map[string]string{},
		}

		// Single header
		opts2 := InvokeOptions{
			Headers: map[string]string{
				"X-Header": "value",
			},
		}

		// Many headers
		manyHeaders := make(map[string]string)
		for i := 0; i < 100; i++ {
			manyHeaders[fmt.Sprintf("X-Header-%d", i)] = fmt.Sprintf("value-%d", i)
		}
		opts3 := InvokeOptions{
			Headers: manyHeaders,
		}

		// Nil headers
		opts4 := InvokeOptions{
			Headers: nil,
		}

		opts := []InvokeOptions{opts1, opts2, opts3, opts4}

		for i, opt := range opts {
			result, err := invoker.Invoke(ctx, "test.function", "{}", opt)
			t.Logf("Headers scenario %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Schema complexity boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Empty schema
		schema1 := map[string]interface{}{}

		// Simple schema
		schema2 := map[string]interface{}{
			"type": "string",
		}

		// Complex nested schema
		schema3 := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field1": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"nested1": map[string]interface{}{"type": "string"},
						"nested2": map[string]interface{}{"type": "number"},
					},
				},
				"field2": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"item": map[string]interface{}{"type": "boolean"},
						},
					},
				},
			},
		}

		// Nil schema
		var schema4 map[string]interface{} = nil

		schemas := []struct {
			name   string
			schema map[string]interface{}
		}{
			{"empty", schema1},
			{"simple", schema2},
			{"complex", schema3},
			{"nil", schema4},
		}

		for _, s := range schemas {
			err := invoker.SetSchema("test.function", s.schema)
			t.Logf("Schema %s: error=%v", s.name, err)
		}
	})
}

// TestBoundaryValues_FloatValues tests floating-point boundary values
func TestBoundaryValues_FloatValues(t *testing.T) {
	t.Run("BackoffMultiplier boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		multipliers := []float64{
			0.0,
			0.1,
			0.5,
			1.0,
			1.5,
			2.0,
			5.0,
			10.0,
			100.0,
		}

		for _, multiplier := range multipliers {
			opts := InvokeOptions{
				Retry: &RetryConfig{
					Enabled:           true,
					MaxAttempts:       2,
					BackoffMultiplier: multiplier,
				},
			}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("BackoffMultiplier %.2f: error=%v, result_len=%d", multiplier, err, len(result))
		}
	})

	t.Run("JitterFactor boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		jitters := []float64{
			0.0,
			0.01,
			0.1,
			0.25,
			0.5,
			0.75,
			1.0,
		}

		for _, jitter := range jitters {
			opts := InvokeOptions{
				Retry: &RetryConfig{
					Enabled:      true,
					MaxAttempts:  2,
					JitterFactor: jitter,
				},
			}

			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("JitterFactor %.2f: error=%v, result_len=%d", jitter, err, len(result))
		}
	})
}

// TestBoundaryValues_SpecialCharacters tests special character edge cases
func TestBoundaryValues_SpecialCharacters(t *testing.T) {
	t.Run("Function ID with special characters", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		functionIDs := []string{
			"test.function",
			"test-function",
			"test_function",
			"test.function",
			"test:function",
			"test:function:v1",
			"test.function/sub.function",
			"测试.函数",  // Chinese
			"функция",    // Russian
			"関数",       // Japanese
			"🎉.emoji",   // Emoji
		}

		for _, functionID := range functionIDs {
			result, err := invoker.Invoke(ctx, functionID, "{}", InvokeOptions{})
			t.Logf("FunctionID '%s': error=%v, result_len=%d", functionID, err, len(result))
		}
	})

	t.Run("Payload with special characters", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		payloads := []string{
			`{"text":"Hello\nWorld\t!"}`,
			`{"text":"Quote\"Test\""}`,
			`{"text":"Backslash\\Test"}`,
			`{"text":"Null\x00Character"}`,
			`{"text":"中文测试"}`,
			`{"text":"🎉🚀❤️"}`,
			`{"text":"\u4e2d\u6587"}`,
		}

		for i, payload := range payloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Headers with special characters", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		headers := []map[string]string{
			{"X-Special": "Value\nWith\nNewlines"},
			{"X-Special": "Value\tWith\tTabs"},
			{"X-Special": "Value\"With\"Quotes"},
			{"X-中文": "中文值"},
			{"X-Emoji": "🎉"},
			{"": "EmptyKey"},
			{"X-Space": " "},
		}

		for i, headersMap := range headers {
			opts := InvokeOptions{Headers: headersMap}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Headers %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestBoundaryValues_Combinatorial tests combinatorial boundary scenarios
func TestBoundaryValues_Combinatorial(t *testing.T) {
	t.Run("Min timeout + Max retries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		opts := InvokeOptions{
			Timeout: time.Nanosecond,
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: math.MaxInt32,
			},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Min timeout + Max retries: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Max timeout + Min retries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		opts := InvokeOptions{
			Timeout: math.MaxInt64,
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 1,
			},
		}

		result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
		t.Logf("Max timeout + Min retries: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Large payload + Many headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		largePayload := string(make([]byte, 10000))

		manyHeaders := make(map[string]string)
		for i := 0; i < 50; i++ {
			manyHeaders[fmt.Sprintf("X-Header-%d", i)] = fmt.Sprintf("value-%d", i)
		}

		opts := InvokeOptions{
			Headers: manyHeaders,
		}

		result, err := invoker.Invoke(ctx, "test.function", largePayload, opts)
		t.Logf("Large payload + Many headers: error=%v, result_len=%d", err, len(result))
	})
}

// TestBoundaryValues_BooleanAndNil tests boolean and nil boundary values
func TestBoundaryValues_BooleanAndNil(t *testing.T) {
	t.Run("Boolean flag combinations", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
			Insecure: true,
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test with Insecure = true
		result, err := invoker.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Insecure=true: error=%v, result_len=%d", err, len(result))
	})

	t.Run("Nil vs empty configurations", func(t *testing.T) {
		// Nil retry config
		config1 := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry:   nil,
		}

		invoker1 := NewHTTPInvoker(config1)
		if invoker1 == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker1.Close()

		ctx := context.Background()

		result, err := invoker1.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Nil Retry config: error=%v, result_len=%d", err, len(result))

		// Empty retry config
		config2 := &InvokerConfig{
			Address: "http://localhost:19090",
			Retry:   &RetryConfig{},
		}

		invoker2 := NewHTTPInvoker(config2)
		if invoker2 == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker2.Close()

		result, err = invoker2.Invoke(ctx, "test.function", "{}", InvokeOptions{})
		t.Logf("Empty Retry config: error=%v, result_len=%d", err, len(result))
	})
}
