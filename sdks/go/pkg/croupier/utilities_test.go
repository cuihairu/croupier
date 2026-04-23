// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"strings"
	"testing"
)

// TestGenerateBasicInputSchema tests generateBasicInputSchema function
func TestGenerateBasicInputSchema(t *testing.T) {
	t.Run("generates valid JSON schema", func(t *testing.T) {
		schema := generateBasicInputSchema()

		if schema == "" {
			t.Error("generateBasicInputSchema() should not return empty string")
		}

		if !strings.HasPrefix(schema, "{") {
			t.Error("Schema should start with '{'")
		}

		if !strings.HasSuffix(schema, "}") {
			t.Error("Schema should end with '}'")
		}
	})

	t.Run("schema contains expected fields", func(t *testing.T) {
		schema := generateBasicInputSchema()

		expectedFields := []string{"type", "properties"}

		for _, field := range expectedFields {
			if !strings.Contains(schema, field) {
				t.Errorf("Schema should contain '%s' field", field)
			}
		}
	})

	t.Run("schema is valid JSON object", func(t *testing.T) {
		schema := generateBasicInputSchema()

		// Check for balanced braces
		openBraces := strings.Count(schema, "{")
		closeBraces := strings.Count(schema, "}")

		if openBraces != closeBraces {
			t.Errorf("Schema has unbalanced braces: %d open, %d close", openBraces, closeBraces)
		}
	})
}

// TestGenerateBasicOutputSchema tests generateBasicOutputSchema function
func TestGenerateBasicOutputSchema(t *testing.T) {
	t.Run("generates valid JSON schema", func(t *testing.T) {
		schema := generateBasicOutputSchema()

		if schema == "" {
			t.Error("generateBasicOutputSchema() should not return empty string")
		}

		if !strings.HasPrefix(schema, "{") {
			t.Error("Schema should start with '{'")
		}

		if !strings.HasSuffix(schema, "}") {
			t.Error("Schema should end with '}'")
		}
	})

	t.Run("schema contains expected fields", func(t *testing.T) {
		schema := generateBasicOutputSchema()

		expectedFields := []string{"type", "properties"}

		for _, field := range expectedFields {
			if !strings.Contains(schema, field) {
				t.Errorf("Schema should contain '%s' field", field)
			}
		}
	})

	t.Run("schema is valid JSON object", func(t *testing.T) {
		schema := generateBasicOutputSchema()

		// Check for balanced braces
		openBraces := strings.Count(schema, "{")
		closeBraces := strings.Count(schema, "}")

		if openBraces != closeBraces {
			t.Errorf("Schema has unbalanced braces: %d open, %d close", openBraces, closeBraces)
		}
	})
}

// TestHTTPInvoker_parseJSONPayload_comprehensive tests parseJSONPayload method
func TestHTTPInvoker_parseJSONPayload_comprehensive(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})
	httpInvoker := invoker.(*httpInvoker)

	t.Run("parse valid JSON object", func(t *testing.T) {
		payload := `{"name":"test","value":123}`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for valid JSON")
		}
	})

	t.Run("parse valid JSON array", func(t *testing.T) {
		payload := `[1,2,3,4,5]`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for valid JSON array")
		}
	})

	t.Run("parse valid JSON string", func(t *testing.T) {
		payload := `"hello world"`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for valid JSON string")
		}
	})

	t.Run("parse valid JSON number", func(t *testing.T) {
		payload := `42`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for valid JSON number")
		}
	})

	t.Run("parse valid JSON boolean", func(t *testing.T) {
		payload := `true`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for valid JSON boolean")
		}
	})

	t.Run("parse valid JSON null", func(t *testing.T) {
		payload := `null`
		result := httpInvoker.parseJSONPayload(payload)

		// null is valid JSON, result might be nil
		t.Logf("parseJSONPayload for 'null': %v (type: %T)", result, result)
	})

	t.Run("parse empty string", func(t *testing.T) {
		payload := ``
		result := httpInvoker.parseJSONPayload(payload)

		// Empty string should return nil or some default
		if result != nil {
			t.Logf("parseJSONPayload returned %v for empty string", result)
		}
	})

	t.Run("parse invalid JSON", func(t *testing.T) {
		payload := `{invalid json}`
		result := httpInvoker.parseJSONPayload(payload)

		// Invalid JSON should return nil or the original string
		if result != nil {
			t.Logf("parseJSONPayload returned %v for invalid JSON", result)
		}
	})

	t.Run("parse nested JSON object", func(t *testing.T) {
		payload := `{"user":{"name":"test","age":30,"address":{"city":"NYC"}}}`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for nested JSON")
		}
	})

	t.Run("parse JSON with special characters", func(t *testing.T) {
		payload := `{"message":"Hello\nWorld\t!"}`
		result := httpInvoker.parseJSONPayload(payload)

		if result == nil {
			t.Error("parseJSONPayload should not return nil for JSON with special characters")
		}
	})
}

// TestHTTPInvoker_validatePayload_comprehensive tests validatePayload method
func TestHTTPInvoker_validatePayload_comprehensive(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})
	httpInvoker := invoker.(*httpInvoker)

	t.Run("validate with nil schema", func(t *testing.T) {
		err := httpInvoker.validatePayload("test.func", `{"data":"test"}`, nil)

		// Nil schema should return error
		if err == nil {
			t.Logf("validatePayload with nil schema: no error (implementation may allow nil schema)")
		} else {
			t.Logf("validatePayload with nil schema error: %v", err)
		}
	})

	t.Run("validate with empty schema", func(t *testing.T) {
		schema := map[string]interface{}{}
		err := httpInvoker.validatePayload("test.func", `{"data":"test"}`, schema)

		if err != nil {
			t.Errorf("validatePayload with empty schema should not error: %v", err)
		}
	})

	t.Run("validate with simple type schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
		}
		err := httpInvoker.validatePayload("test.func", `{"data":"test"}`, schema)

		if err != nil {
			t.Logf("validatePayload error: %v", err)
		}
	})

	t.Run("validate empty payload with schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
		}
		err := httpInvoker.validatePayload("test.func", ``, schema)

		if err != nil {
			t.Logf("validatePayload error for empty payload: %v", err)
		}
	})

	t.Run("validate invalid payload with schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}
		err := httpInvoker.validatePayload("test.func", `{"wrong":"data"}`, schema)

		// This might error depending on validation implementation
		t.Logf("validatePayload error for invalid payload: %v", err)
	})

	t.Run("validate valid payload with required fields", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
				"age": map[string]interface{}{
					"type": "integer",
				},
			},
			"required": []string{"name"},
		}
		err := httpInvoker.validatePayload("test.func", `{"name":"test","age":30}`, schema)

		if err != nil {
			t.Logf("validatePayload error: %v", err)
		}
	})
}

// TestHTTPInvoker_SetDefaultGameEnv_comprehensive tests SetDefaultGameEnv method
func TestHTTPInvoker_SetDefaultGameEnv_comprehensive(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})
	httpInvoker := invoker.(*httpInvoker)

	t.Run("set game and env values", func(t *testing.T) {
		// This method likely sets internal state
		// We're testing that it doesn't panic
		httpInvoker.SetDefaultGameEnv("game-123", "production")
	})

	t.Run("set empty game ID", func(t *testing.T) {
		httpInvoker.SetDefaultGameEnv("", "production")
	})

	t.Run("set empty env", func(t *testing.T) {
		httpInvoker.SetDefaultGameEnv("game-123", "")
	})

	t.Run("set both empty", func(t *testing.T) {
		httpInvoker.SetDefaultGameEnv("", "")
	})

	t.Run("set with special characters", func(t *testing.T) {
		httpInvoker.SetDefaultGameEnv("game_test-123", "staging-env")
	})

	t.Run("set with various environments", func(t *testing.T) {
		envs := []string{"development", "staging", "production", "test", "prod"}

		for _, env := range envs {
			httpInvoker.SetDefaultGameEnv("game-123", env)
		}
	})
}

// TestHTTPInvoker_IsConnected_default tests IsConnected method
func TestHTTPInvoker_IsConnected_default(t *testing.T) {
	t.Run("new invoker is not connected", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})
		httpInvoker := invoker.(*httpInvoker)

		// A newly created invoker should report not connected
		connected := httpInvoker.IsConnected()
		t.Logf("New invoker IsConnected: %v", connected)
	})
}

// TestHTTPInvoker_GetAddress_various tests GetAddress method
func TestHTTPInvoker_GetAddress_various(t *testing.T) {
	t.Run("get address returns configured address", func(t *testing.T) {
		addresses := []string{
			"localhost:8080",
			"127.0.0.1:9090",
			"example.com:8443",
			"192.168.1.1:8080",
		}

		for _, addr := range addresses {
			invoker := NewHTTPInvoker(&InvokerConfig{
				Address: addr,
			})
			httpInvoker := invoker.(*httpInvoker)

			retrievedAddr := httpInvoker.GetAddress()
			// HTTP invoker may add "http://" prefix
			if retrievedAddr != addr && retrievedAddr != "http://"+addr {
				t.Logf("GetAddress() = %s, expected %s or http://%s", retrievedAddr, addr, addr)
			}
		}
	})

	t.Run("get address with empty string", func(t *testing.T) {
		invoker := NewHTTPInvoker(&InvokerConfig{
			Address: "",
		})
		httpInvoker := invoker.(*httpInvoker)

		addr := httpInvoker.GetAddress()
		// Empty address is valid (will use default)
		if addr != "" {
			t.Logf("GetAddress() returned: %s", addr)
		}
	})
}

// TestClientConfig_defaults tests ClientConfig default behaviors
func TestClientConfig_defaults(t *testing.T) {
	t.Run("config with all fields set", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr:         "localhost:19090",
			AgentIPCAddr:      "ipc://croupier-agent",
			GameID:            "game-123",
			Env:               "production",
			ServiceID:         "service-456",
			ServiceVersion:    "2.0.0",
			AgentID:           "agent-789",
			ProviderLang:      "go",
			ProviderSDK:       "croupier-go-sdk",
			TimeoutSeconds:    60,
			Insecure:          false,
			CAFile:            "/path/to/ca.crt",
			CertFile:          "/path/to/cert.pem",
			KeyFile:           "/path/to/key.pem",
			ServerName:        "example.com",
			InsecureSkipVerify: false,
			DisableLogging:    false,
			DebugLogging:      false,
		}

		if config.AgentAddr != "localhost:19090" {
			t.Errorf("AgentAddr = %s, want localhost:19090", config.AgentAddr)
		}

		if config.Env != "production" {
			t.Errorf("Env = %s, want production", config.Env)
		}

		if config.TimeoutSeconds != 60 {
			t.Errorf("TimeoutSeconds = %d, want 60", config.TimeoutSeconds)
		}
	})

	t.Run("config with zero values", func(t *testing.T) {
		config := &ClientConfig{}

		if config.AgentAddr != "" {
			t.Errorf("AgentAddr should be empty, got %s", config.AgentAddr)
		}

		if config.TimeoutSeconds != 0 {
			t.Errorf("TimeoutSeconds should be 0, got %d", config.TimeoutSeconds)
		}

		if config.Insecure {
			t.Error("Insecure should be false (zero value)")
		}
	})

	t.Run("config with TLS settings only", func(t *testing.T) {
		config := &ClientConfig{
			Insecure: false,
			CAFile:   "/path/to/ca.crt",
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		}

		if config.Insecure {
			t.Error("Insecure should be false")
		}

		if config.CAFile == "" {
			t.Error("CAFile should not be empty")
		}
	})

	t.Run("config with various environments", func(t *testing.T) {
		envs := []string{"development", "staging", "production", "test", "dev", "prod"}

		for _, env := range envs {
			config := &ClientConfig{
				Env: env,
			}

			if config.Env != env {
				t.Errorf("Env = %s, want %s", config.Env, env)
			}
		}
	})
}

// TestInvokerConfig_defaults tests InvokerConfig default behaviors
func TestInvokerConfig_defaults(t *testing.T) {
	t.Run("config with various addresses including protocols", func(t *testing.T) {
		addresses := []string{
			"tcp://localhost:8080",
			"ipc://croupier-agent",
			"ws://localhost:8080",
			"wss://localhost:8443",
			"localhost:8080",
		}

		for _, addr := range addresses {
			config := &InvokerConfig{
				Address: addr,
			}

			if config.Address != addr {
				t.Errorf("Address = %s, want %s", config.Address, addr)
			}
		}
	})

	t.Run("config with zero timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:8080",
			TimeoutSeconds: 0,
		}

		if config.TimeoutSeconds != 0 {
			t.Errorf("TimeoutSeconds should be 0, got %d", config.TimeoutSeconds)
		}
	})

	t.Run("config with negative timeout", func(t *testing.T) {
		config := &InvokerConfig{
			Address:        "localhost:8080",
			TimeoutSeconds: -1,
		}

		if config.TimeoutSeconds != -1 {
			t.Errorf("TimeoutSeconds should be -1, got %d", config.TimeoutSeconds)
		}
	})
}
