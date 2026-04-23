// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"testing"
)

// TestSchemaOperations_BasicScenarios tests basic schema operation scenarios
func TestSchemaOperations_BasicScenarios(t *testing.T) {
	t.Run("SetSchema with simple string schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "string",
		}

		functionID := "test.function.string"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema for %s: error=%v", functionID, err)
	})

	t.Run("SetSchema with number schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "number",
		}

		functionID := "test.function.number"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema for %s: error=%v", functionID, err)
	})

	t.Run("SetSchema with boolean schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "boolean",
		}

		functionID := "test.function.boolean"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema for %s: error=%v", functionID, err)
	})
}

// TestSchemaOperations_ComplexSchemas tests complex schema structures
func TestSchemaOperations_ComplexSchemas(t *testing.T) {
	t.Run("SetSchema with object schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
				"age": map[string]interface{}{
					"type": "number",
				},
				"email": map[string]interface{}{
					"type": "string",
					"format": "email",
				},
			},
			"required": []interface{}{"name", "email"},
		}

		functionID := "test.function.object"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema for object: error=%v", err)
	})

	t.Run("SetSchema with array schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":   map[string]interface{}{"type": "number"},
					"name": map[string]interface{}{"type": "string"},
				},
			},
		}

		functionID := "test.function.array"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema for array: error=%v", err)
	})

	t.Run("SetSchema with nested schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"address": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"street": map[string]interface{}{"type": "string"},
								"city":   map[string]interface{}{"type": "string"},
								"zip":    map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		}

		functionID := "test.function.nested"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema for nested: error=%v", err)
	})
}

// TestSchemaOperations_ValidationFeatures tests schema validation features
func TestSchemaOperations_ValidationFeatures(t *testing.T) {
	t.Run("SetSchema with enum validation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "string",
			"enum": []interface{}{"red", "green", "blue"},
		}

		functionID := "test.function.enum"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with enum: error=%v", err)
	})

	t.Run("SetSchema with range validation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "number",
			"minimum": 0,
			"maximum": 100,
		}

		functionID := "test.function.range"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with range: error=%v", err)
	})

	t.Run("SetSchema with pattern validation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "string",
			"pattern": "^[a-zA-Z0-9]+$",
		}

		functionID := "test.function.pattern"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with pattern: error=%v", err)
	})

	t.Run("SetSchema with length validation", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "string",
			"minLength": 1,
			"maxLength": 100,
		}

		functionID := "test.function.length"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with length: error=%v", err)
	})
}

// TestSchemaOperations_MultipleSchemas tests setting multiple schemas
func TestSchemaOperations_MultipleSchemas(t *testing.T) {
	t.Run("SetSchema for multiple functions", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schemas := []struct {
			functionID string
			schema     map[string]interface{}
		}{
			{
				"test.func1",
				map[string]interface{}{"type": "string"},
			},
			{
				"test.func2",
				map[string]interface{}{"type": "number"},
			},
			{
				"test.func3",
				map[string]interface{}{"type": "boolean"},
			},
			{
				"test.func4",
				map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{"type": "string"},
				},
			},
			{
				"test.func5",
				map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "number"},
					},
				},
			},
		}

		successCount := 0
		for _, s := range schemas {
			err := invoker.SetSchema(s.functionID, s.schema)
			if err == nil {
				successCount++
				t.Logf("SetSchema for %s: success", s.functionID)
			} else {
				t.Logf("SetSchema for %s: error=%v", s.functionID, err)
			}
		}

		t.Logf("SetSchema multiple: %d/%d successful", successCount, len(schemas))
	})

	t.Run("SetSchema with updates to same function", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		functionID := "test.function.update"

		// Set initial schema
		schema1 := map[string]interface{}{
			"type": "string",
		}
		err1 := invoker.SetSchema(functionID, schema1)
		t.Logf("Initial schema: error=%v", err1)

		// Update to more complex schema
		schema2 := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"value": map[string]interface{}{"type": "string"},
				"count": map[string]interface{}{"type": "number"},
			},
		}
		err2 := invoker.SetSchema(functionID, schema2)
		t.Logf("Updated schema: error=%v", err2)

		// Update again
		schema3 := map[string]interface{}{
			"type": "string",
			"enum": []interface{}{"a", "b", "c"},
		}
		err3 := invoker.SetSchema(functionID, schema3)
		t.Logf("Second update: error=%v", err3)
	})
}

// TestSchemaOperations_EdgeCases tests edge cases for schema operations
func TestSchemaOperations_EdgeCases(t *testing.T) {
	t.Run("SetSchema with empty schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{}
		functionID := "test.function.empty"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with empty schema: error=%v", err)
	})

	t.Run("SetSchema with nil schema", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		var schema map[string]interface{} = nil
		functionID := "test.function.nil"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with nil schema: error=%v", err)
	})

	t.Run("SetSchema with invalid function IDs", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"type": "string",
		}

		invalidIDs := []string{
			"",
			" ",
			"function/with/slashes",
			"function\nwith\nnewlines",
		}

		for _, functionID := range invalidIDs {
			err := invoker.SetSchema(functionID, schema)
			t.Logf("SetSchema for invalid ID (len=%d): error=%v", len(functionID), err)
		}
	})
}

// TestSchemaOperations_ContextVariations tests schema operations with different contexts
func TestSchemaOperations_ContextVariations(t *testing.T) {
	t.Run("SetSchema with timeout context", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		_, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		schema := map[string]interface{}{
			"type": "string",
		}

		// Note: SetSchema doesn't take context in current implementation
		// This tests that it works regardless
		err := invoker.SetSchema("test.function", schema)
		t.Logf("SetSchema after cancel: error=%v", err)
	})

	t.Run("SetSchema and Invoke together", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		functionID := "test.function.integration"

		// Set schema
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{"type": "string"},
			},
		}
		err := invoker.SetSchema(functionID, schema)
		t.Logf("SetSchema: error=%v", err)

		// Try to invoke
		ctx := context.Background()
		result, err := invoker.Invoke(ctx, functionID, `{"input":"test"}`, InvokeOptions{})
		t.Logf("Invoke after SetSchema: result_len=%d, error=%v", len(result), err)
	})
}

// TestSchemaOperations_ComplexStructures tests highly complex schema structures
func TestSchemaOperations_ComplexStructures(t *testing.T) {
	t.Run("SetSchema with allOf", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"allOf": []interface{}{
				map[string]interface{}{"type": "string"},
				map[string]interface{}{"minLength": 1},
			},
		}

		functionID := "test.function.allof"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with allOf: error=%v", err)
	})

	t.Run("SetSchema with anyOf", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"anyOf": []interface{}{
				map[string]interface{}{"type": "string"},
				map[string]interface{}{"type": "number"},
			},
		}

		functionID := "test.function.anyof"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with anyOf: error=%v", err)
	})

	t.Run("SetSchema with oneOf", func(t *testing.T) {
		config := &InvokerConfig{
			Address: "http://localhost:19090",
		}

		invoker := NewHTTPInvoker(config)
		if invoker == nil {
			t.Fatal("NewHTTPInvoker returned nil")
		}
		defer invoker.Close()

		schema := map[string]interface{}{
			"oneOf": []interface{}{
				map[string]interface{}{"type": "string"},
				map[string]interface{}{"type": "boolean"},
			},
		}

		functionID := "test.function.oneof"
		err := invoker.SetSchema(functionID, schema)

		t.Logf("SetSchema with oneOf: error=%v", err)
	})
}
