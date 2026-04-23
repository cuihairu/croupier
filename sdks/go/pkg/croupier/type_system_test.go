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

// TestTypeSystem_PrimitiveTypes tests primitive type handling
func TestTypeSystem_PrimitiveTypes(t *testing.T) {
	t.Run("String type", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		stringPayloads := []string{
			`{"value":"simple string"}`,
			`{"value":"string with spaces"}`,
			`{"value":"string-with-special-chars:!@#$%"}`,
			`{"value":"unicode:中文测试"}`,
			`{"value":"emoji:🎉🚀"}`,
		}

		for i, payload := range stringPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("String payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Number types", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		numberPayloads := []string{
			`{"value":42}`,
			`{"value":-123}`,
			`{"value":0}`,
			`{"value":3.14159}`,
			`{"value":-0.001}`,
			`{"value":1.7976931348623157e+308}`, // Max float64
			`{"value":-1.7976931348623157e+308}`, // Min float64
		}

		for i, payload := range numberPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Number payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Boolean type", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		boolPayloads := []string{
			`{"value":true}`,
			`{"value":false}`,
		}

		for i, payload := range boolPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Boolean payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Null type", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		nullPayloads := []string{
			`{"value":null}`,
			`{"value":null, "other":"value"}`,
		}

		for i, payload := range nullPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Null payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_CompositeTypes tests composite type handling
func TestTypeSystem_CompositeTypes(t *testing.T) {
	t.Run("Array types", func(t *testing.T) {
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
			`{"array":[]}`,
			`{"array":[1]}`,
			`{"array":[1,2,3]}`,
			`{"array":["a","b","c"]}`,
			`{"array":[1,"two",3.0,true,null]}`,
			`{"array":[[1,2],[3,4]]}`,
			`{"array":[{"key":"value"}]}`,
		}

		for i, payload := range arrayPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Array payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Object types", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		objectPayloads := []string{
			`{}`,
			`{"key":"value"}`,
			`{"key1":"value1","key2":"value2"}`,
			`{"nested":{"key":"value"}}`,
			`{"array":[1,2,3],"object":{"key":"value"}}`,
			`{"string":"text","number":123,"bool":true,"null":null}`,
		}

		for i, payload := range objectPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Object payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Nested structures", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		nestedPayloads := []string{
			`{"level1":{"level2":{"level3":"deep"}}}`,
			`{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}]}`,
			`{"config":{"timeout":30,"retry":{"enabled":true,"max":3}}}`,
		}

		for i, payload := range nestedPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Nested payload %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_SerializationTests tests serialization/deserialization
func TestTypeSystem_SerializationTests(t *testing.T) {
	t.Run("JSON marshal/unmarshal", func(t *testing.T) {
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
			Name    string   `json:"name"`
			Value   int      `json:"value"`
			Enabled bool     `json:"enabled"`
			Tags    []string `json:"tags"`
		}

		testData := TestData{
			Name:    "test",
			Value:   123,
			Enabled: true,
			Tags:    []string{"tag1", "tag2"},
		}

		// Marshal to JSON
		jsonBytes, err := json.Marshal(testData)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Invoke with marshaled data
		result, err := invoker.Invoke(ctx, "test.function", string(jsonBytes), InvokeOptions{})
		t.Logf("Marshal/unmarshal: error=%v, result_len=%d", err, len(result))

		// Try to unmarshal result
		var resultData TestData
		unmarshalErr := json.Unmarshal([]byte(result), &resultData)
		t.Logf("Unmarshal result: error=%v", unmarshalErr)
	})

	t.Run("Special JSON characters", func(t *testing.T) {
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
			`{"text":"Line 1\nLine 2\tTabbed"}`,
			`{"text":"Quote: \"Hello\""}`,
			`{"text":"Backslash: \\path"}`,
			`{"text":"Unicode: \u0048\u0065\u006C\u006C\u006F"}`,
			`{"text":"Emoji: \ud83d\ude00"}`,
		}

		for i, payload := range specialPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Special chars %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_SchemaTypes tests schema type definitions
func TestTypeSystem_SchemaTypes(t *testing.T) {
	t.Run("Basic schema types", func(t *testing.T) {
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
		}

		for i, schema := range schemaTypes {
			err := invoker.SetSchema(fmt.Sprintf("function%d", i), schema)
			t.Logf("Schema type %d: error=%v", i, err)
		}
	})

	t.Run("Complex schema definitions", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		complexSchemas := []map[string]interface{}{
			{
				"type": "object",
				"properties": map[string]interface{}{
					"name":  map[string]string{"type": "string"},
					"age":   map[string]string{"type": "integer"},
					"email": map[string]string{"type": "string"},
				},
				"required": []string{"name", "email"},
			},
			{
				"type": "array",
				"items": map[string]string{
					"type": "object",
				},
			},
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

		for i, schema := range complexSchemas {
			err := invoker.SetSchema(fmt.Sprintf("complex%d", i), schema)
			t.Logf("Complex schema %d: error=%v", i, err)
		}
	})
}

// TestTypeSystem_TypeCoercion tests type coercion behavior
func TestTypeSystem_TypeCoercion(t *testing.T) {
	t.Run("Numeric type coercion", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		numericPayloads := []string{
			`{"int":42,"float":3.14,"stringNum":"123"}`,
			`{"zero":0,"negative":-5,"scientific":1.23e-10}`,
			`{"maxInt":9223372036854775807,"minInt":-9223372036854775808}`,
		}

		for i, payload := range numericPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Numeric coercion %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_DateTimeTypes tests date/time type handling
func TestTypeSystem_DateTimeTypes(t *testing.T) {
	t.Run("ISO 8601 timestamps", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		now := time.Now()
		timestamps := []string{
			now.Format(time.RFC3339),
			now.UTC().Format(time.RFC3339),
			now.Add(24 * time.Hour).Format(time.RFC3339),
			now.Add(-24 * time.Hour).Format(time.RFC3339),
		}

		for i, timestamp := range timestamps {
			payload := fmt.Sprintf(`{"timestamp":"%s"}`, timestamp)
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Timestamp %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Unix timestamps", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		now := time.Now()
		unixTimestamps := []int64{
			now.Unix(),
			now.UnixMilli(),
			0,
			-1,
			9223372036854775807, // Max int64
		}

		for i, ts := range unixTimestamps {
			payload := fmt.Sprintf(`{"unix":%d}`, ts)
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Unix timestamp %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_BinaryData tests binary data handling
func TestTypeSystem_BinaryData(t *testing.T) {
	t.Run("Base64 encoded data", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		base64Data := []string{
			`{"data":"SGVsbG8gV29ybGQ="}`, // "Hello World"
			`{"data":""}`,                // Empty
			`{"data":"YWJj"}`,             // "abc" (incomplete, but valid)
		}

		for i, payload := range base64Data {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Base64 data %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Hex encoded data", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		hexData := []string{
			`{"data":"48656c6c6f"}`, // "Hello"
			`{"data":"{}"}`,
			`{"data":"deadbeef"}`,
		}

		for i, payload := range hexData {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Hex data %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_LargeNumbers tests large number handling
func TestTypeSystem_LargeNumbers(t *testing.T) {
	t.Run("Integer boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		boundaries := []string{
			`{"value":9223372036854775807}`,  // Max int64
			`{"value":-9223372036854775808}`, // Min int64
			`{"value":18446744073709551615}`, // Max uint64
			`{"value":0}`,
		}

		for i, payload := range boundaries {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Boundary %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Floating point boundaries", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		floatBoundaries := []string{
			`{"value":1.7976931348623157e+308}`,  // Max float64
			`{"value":-1.7976931348623157e+308}`, // Min float64
			`{"value":5e-324}`,                  // Min positive float64
			`{"value":0.0}`,
		}

		for i, payload := range floatBoundaries {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Float boundary %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_EmptyValues tests empty value handling
func TestTypeSystem_EmptyValues(t *testing.T) {
	t.Run("Empty string", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		emptyStringPayloads := []string{
			`{"value":""}`,
			`{"value":"","other":"data"}`,
			`{"empty":"","null":null}`,
		}

		for i, payload := range emptyStringPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Empty string %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Empty collections", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		emptyCollectionPayloads := []string{
			`{"array":[]}`,
			`{"object":{}}`,
			`{"array":[],"object":{}}`,
		}

		for i, payload := range emptyCollectionPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Empty collection %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_SpecialCharacters tests special character handling
func TestTypeSystem_SpecialCharacters(t *testing.T) {
	t.Run("Unicode characters", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		unicodePayloads := []string{
			`{"text":"中文测试"}`,
			`{"text":"日本語テスト"}`,
			`{"text":"한국어테스트"}`,
			`{"text":"الاختبار"}`,
			`{"text":"Тест"}`,
			`{"text":"🎉🚀❤️"}`,
			`{"text":"©®™℠€£¥"}`,
		}

		for i, payload := range unicodePayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Unicode %d: error=%v, result_len=%d", i, err, len(result))
		}
	})

	t.Run("Control characters", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		ctx := context.Background()

		controlPayloads := []string{
			`{"text":"Line1\nLine2\rLine3\tTabbed"}`,
			`{"text":"Null\x00Byte"}`,
			`{"text":"Escape sequences: \n\r\t\b\f"}`,
		}

		for i, payload := range controlPayloads {
			result, err := invoker.Invoke(ctx, "test.function", payload, InvokeOptions{})
			t.Logf("Control chars %d: error=%v, result_len=%d", i, err, len(result))
		}
	})
}

// TestTypeSystem_TypeValidationRuntime tests runtime type validation
func TestTypeSystem_TypeValidationRuntime(t *testing.T) {
	t.Run("Schema validation runtime", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		// Set a schema
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":  map[string]string{"type": "string"},
				"age":   map[string]string{"type": "integer"},
				"active": map[string]string{"type": "boolean"},
			},
			"required": []string{"name"},
		}

		err := invoker.SetSchema("validate.function", schema)
		t.Logf("Set validation schema: error=%v", err)

		// Test with valid payload
		ctx := context.Background()
		validPayload := `{"name":"Alice","age":30,"active":true}`
		result, err := invoker.Invoke(ctx, "validate.function", validPayload, InvokeOptions{})
		t.Logf("Valid payload: error=%v, result_len=%d", err, len(result))

		// Test with invalid payload
		invalidPayload := `{"name":"Bob","age":"not_a_number","active":true}`
		result, err = invoker.Invoke(ctx, "validate.function", invalidPayload, InvokeOptions{})
		t.Logf("Invalid payload: error=%v, result_len=%d", err, len(result))
	})
}
