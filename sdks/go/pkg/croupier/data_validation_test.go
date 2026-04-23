// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestDataValidation_InputValidation tests input data validation
func TestDataValidation_InputValidation(t *testing.T) {
	t.Run("Empty payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		emptyPayloads := []string{
			"",
			"{}",
			"null",
			"[]",
		}

		for i, payload := range emptyPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Empty payload %d (len=%d): error=%v, result_len=%d", i, len(payload), err, len(result))
		}
	})

	t.Run("Malformed JSON payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		malformedPayloads := []string{
			"{",
			"}",
			"{{}",
			"}{",
			"{invalid}",
			"{'key': 'value'}", // Single quotes instead of double
			"{key: value}",      // Unquoted keys
			"{{\"key\": \"value\"}",
			"{\"key\": \"value\"",
			"not json at all",
		}

		for i, payload := range malformedPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Malformed JSON %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Special character payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		specialPayloads := []string{
			`{"data":"\n\t\r"}`,
			`{"data":"\x00\x01\x02"}`,
			`{"data":"éàüöä"}`,
			`{"data":"中文"}`,
			`{"data":"🎉🚀"}`,
			`{"data":"<>\"'&"}`,
			`{"data":"\u0000\u0001"}`,
		}

		for i, payload := range specialPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Special chars %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Large payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		sizes := []int{
			1024,          // 1 KB
			10 * 1024,     // 10 KB
			100 * 1024,    // 100 KB
			1024 * 1024,   // 1 MB
		}

		for _, size := range sizes {
			largeData := make([]byte, size)
			for i := range largeData {
				largeData[i] = byte('a' + (i % 26))
			}

			payload := fmt.Sprintf(`{"data":"%s"}`, string(largeData))
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Large payload (%d bytes): error=%v, result_len=%d", size, err, len(result))
		}
	})
}

// TestDataValidation_OutputValidation tests output data validation
func TestDataValidation_OutputValidation(t *testing.T) {
	t.Run("Empty result handling", func(t *testing.T) {
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
		t.Logf("Empty result: len=%d, error=%v", len(result), err)

		// Try to parse as JSON
		var parsed interface{}
		parseErr := json.Unmarshal([]byte(result), &parsed)
		t.Logf("JSON parse: error=%v, parsed=%v", parseErr, parsed)
	})

	t.Run("Malformed result handling", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Multiple invocations to test various result scenarios
		for i := 0; i < 10; i++ {
			result, err := invoker.Invoke(ctx, "test.function", fmt.Sprintf(`{"req":%d}`, i), InvokeOptions{})
			t.Logf("Result %d: len=%d, error=%v", i, len(result), err)

			// Try to validate JSON structure
			if len(result) > 0 {
				var parsed map[string]interface{}
				parseErr := json.Unmarshal([]byte(result), &parsed)
				t.Logf("Parse result %d: error=%v", i, parseErr)
			}
		}
	})
}

// TestDataValidation_TypeValidation tests type validation
func TestDataValidation_TypeValidation(t *testing.T) {
	t.Run("JSON primitive types", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		primitivePayloads := []string{
			`{"string":"value"}`,
			`{"number":123}`,
			`{"float":123.456}`,
			`{"bool":true}`,
			`{"null":null}`,
		}

		for i, payload := range primitivePayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Primitive type %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("JSON complex types", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		complexPayloads := []string{
			`{"array":[1,2,3]}`,
			`{"nested":{"key":"value"}}`,
			`{"mixed":{"arr":[1,2],"str":"text","num":123}}`,
			`{"deep":{"nested":{"structure":{"key":"value"}}}}`,
		}

		for i, payload := range complexPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Complex type %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Array payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		arrayPayloads := []string{
			`[]`,
			`[1]`,
			`[1,2,3]`,
			`[{"a":1},{"b":2}]`,
			`[[1,2],[3,4]]`,
		}

		for i, payload := range arrayPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Array payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestDataValidation_SchemaValidation tests schema validation
func TestDataValidation_SchemaValidation(t *testing.T) {
	t.Run("Set schema with various types", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schemaTypes := []map[string]interface{}{
			{"type": "string"},
			{"type": "number"},
			{"type": "integer"},
			{"type": "boolean"},
			{"type": "array"},
			{"type": "object"},
			{
				"type": "object",
				"properties": map[string]interface{}{
					"name":  map[string]string{"type": "string"},
					"age":   map[string]string{"type": "integer"},
					"email": map[string]string{"type": "string"},
				},
			},
		}

		for i, schema := range schemaTypes {
			err := invoker.SetSchema(fmt.Sprintf("function%d", i), schema)
			t.Logf("Schema type %d: error=%v", i, err)
		}
	})

	t.Run("Schema with nested structures", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		nestedSchemas := []map[string]interface{}{
			{
				"type": "object",
				"properties": map[string]interface{}{
					"user": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]string{"type": "string"},
							"address": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"street": map[string]string{"type": "string"},
									"city":   map[string]string{"type": "string"},
								},
							},
						},
					},
				},
			},
		}

		for i, schema := range nestedSchemas {
			err := invoker.SetSchema(fmt.Sprintf("nested%d", i), schema)
			t.Logf("Nested schema %d: error=%v", i, err)
		}
	})
}

// TestDataValidation_FunctionIDValidation tests function ID validation
func TestDataValidation_FunctionIDValidation(t *testing.T) {
	t.Run("Valid function IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		validFunctionIDs := []string{
			"test.function",
			"my_function",
			"function123",
			"a.b.c",
			"service.v1.function",
			"test",
			"TestFunction",
			"test-function",
		}

		for _, functionID := range validFunctionIDs {
			result, err := invoker.Invoke(ctx, functionID, "{}", InvokeOptions{})
			t.Logf("Valid function ID '%s': error=%v, result_len=%d", functionID, err, len(result))
		}
	})

	t.Run("Special character function IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		specialFunctionIDs := []string{
			"test.function",
			"test.function中文",
			"test.function.🎉",
			"test.function-日本語",
		}

		for _, functionID := range specialFunctionIDs {
			result, err := invoker.Invoke(ctx, functionID, "{}", InvokeOptions{})
			t.Logf("Special char function ID '%s': error=%v, result_len=%d", functionID, err, len(result))
		}
	})

	t.Run("Empty and invalid function IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		invalidFunctionIDs := []string{
			"",
			".",
			"..",
			"   ",
			"test..function",
		}

		for _, functionID := range invalidFunctionIDs {
			result, err := invoker.Invoke(ctx, functionID, "{}", InvokeOptions{})
			t.Logf("Invalid function ID '%s': error=%v, result_len=%d", functionID, err, len(result))
		}
	})
}

// TestDataValidation_HeaderValidation tests header validation
func TestDataValidation_HeaderValidation(t *testing.T) {
	t.Run("Standard headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		standardHeaders := []map[string]string{
			{"Content-Type": "application/json"},
			{"Accept": "application/json"},
			{"Authorization": "Bearer token123"},
			{"User-Agent": "MyApp/1.0"},
		}

		for i, headers := range standardHeaders {
			opts := InvokeOptions{Headers: headers}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Standard headers %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Custom headers", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		customHeaders := []map[string]string{
			{"X-Custom-Header": "custom-value"},
			{"X-Request-ID": "12345"},
			{"X-Trace-ID": "trace-123"},
			{"X-Session-ID": "session-abc"},
		}

		for i, headers := range customHeaders {
			opts := InvokeOptions{Headers: headers}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Custom headers %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Headers with special values", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		specialHeaders := []map[string]string{
			{"X-Unicode": "中文"},
			{"X-Emoji": "🎉"},
			{"X-Special": "!@#$%^&*()"},
			{"X-URL": "http://example.com?param=value"},
		}

		for i, headers := range specialHeaders {
			opts := InvokeOptions{Headers: headers}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Special headers %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestDataValidation_IdempotencyKeyValidation tests idempotency key validation
func TestDataValidation_IdempotencyKeyValidation(t *testing.T) {
	t.Run("Valid idempotency keys", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		validKeys := []string{
			"key-123",
			"uuid-like-12345678-1234-1234-1234-123456789abc",
			"request.12345",
			fmt.Sprintf("key-%d", time.Now().UnixNano()),
		}

		for _, key := range validKeys {
			opts := InvokeOptions{IdempotencyKey: key}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Valid key '%s': error=%v, result_len=%d", key, err, len(result))
		}
	})

	t.Run("Empty and special idempotency keys", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		specialKeys := []string{
			"",
			"   ",
			"中文",
			"🎉",
		}

		for _, key := range specialKeys {
			opts := InvokeOptions{IdempotencyKey: key}
			result, err := invoker.Invoke(ctx, "test.function", "{}", opts)
			t.Logf("Special key '%s': error=%v, result_len=%d", key, err, len(result))
		}
	})
}

// TestDataValidation_DataTransformation tests data transformation scenarios
func TestDataValidation_DataTransformation(t *testing.T) {
	t.Run("JSON marshaling/unmarshaling", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test data structure
		type TestData struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		data := TestData{Name: "test", Value: 123}
		jsonData, _ := json.Marshal(data)

		result, err := invoker.Invoke(ctx, "test.function", string(jsonData), InvokeOptions{})
		t.Logf("JSON marshal/unmarshal: error=%v, result_len=%d", err, len(result))

		// Try to unmarshal result
		var resultData TestData
		unmarshalErr := json.Unmarshal([]byte(result), &resultData)
		t.Logf("Unmarshal result: error=%v", unmarshalErr)
	})

	t.Run("Data encoding scenarios", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test various encodings
		encodings := []string{
			`{"data":"base64:SGVsbG8="}`,
			`{"data":"url:hello%20world"}`,
			`{"data":"hex:48656c6c6f"}`,
		}

		for i, encoding := range encodings {
			result, err := invoker.Invoke(ctx, "test.function", encoding, InvokeOptions{})
			t.Logf("Encoding %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestDataValidation_BoundaryTests tests boundary conditions for data
func TestDataValidation_BoundaryTests(t *testing.T) {
	t.Run("Maximum payload sizes", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		// Test progressively larger payloads
		sizes := []int{
			1024,
			10 * 1024,
			50 * 1024,
			100 * 1024,
		}

		for _, size := range sizes {
			largeData := make([]byte, size)
			payload := fmt.Sprintf(`{"data":"%s"}`, string(largeData))

			start := time.Now()
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			duration := time.Since(start)

			t.Logf("Size %d bytes: duration=%v, error=%v, result_len=%d",
				size, duration, err, len(result))
		}
	})

	t.Run("Minimum and zero-length payloads", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		minPayloads := []string{
			"",
			"{}",
			`{}`,
		}

		for i, payload := range minPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Min payload %d (len=%d): error=%v, result_len=%d", i, len(payload), err, len(result))
		}
	})
}

// TestDataValidation_ConcurrentDataTests tests concurrent data operations
func TestDataValidation_ConcurrentDataTests(t *testing.T) {
	t.Run("Concurrent schema updates", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		const numUpdates = 50

		for i := 0; i < numUpdates; i++ {
			go func(idx int) {
				schema := map[string]interface{}{
					"type": "string",
				}
				err := invoker.SetSchema(fmt.Sprintf("function%d", idx), schema)
				_ = err
			}(i)
		}

		time.Sleep(100 * time.Millisecond)
		t.Logf("Completed %d concurrent schema updates", numUpdates)
	})

	t.Run("Concurrent invokes with different data", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()
		const numInvokes = 50

		for i := 0; i < numInvokes; i++ {
			go func(idx int) {
				payload := fmt.Sprintf(`{"id":%d,"data":"test"}`, idx)
				result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
				_ = result
				_ = err
			}(i)
		}

		time.Sleep(100 * time.Millisecond)
		t.Logf("Completed %d concurrent invokes", numInvokes)
	})
}
