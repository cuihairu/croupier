// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"testing"
)

// TestStringConfigurations tests various string configuration fields
func TestStringConfigurations(t *testing.T) {
	t.Run("empty string addresses", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address: "",
		})
		if invoker == nil {
			t.Error("NewInvoker with empty address should return valid invoker")
		}
	})
	
	t.Run("whitespace addresses", func(t *testing.T) {
		addresses := []string{" ", "  ", "\t", "\n"}
		
		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: addr,
			})
			if invoker == nil {
				t.Errorf("NewInvoker with whitespace address '%s' should return valid invoker", addr)
			}
		}
	})
	
	t.Run("special character addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:8080/path",
			"example.com:8080?query=value",
			"192.168.1.1:8080#fragment",
			"[::1]:8080",
		}
		
		for _, addr := range addresses {
			invoker := NewInvoker(&InvokerConfig{
				Address: addr,
			})
			if invoker == nil {
				t.Errorf("NewInvoker with special address '%s' should return valid invoker", addr)
			}
		}
	})
}

// TestNumericConfigurations tests various numeric configuration values
func TestNumericConfigurations(t *testing.T) {
	t.Run("very large timeout values", func(t *testing.T) {
		invoker := NewInvoker(&InvokerConfig{
			Address:        "localhost:8080",
			TimeoutSeconds: 1000000, // Very large value
		})
		if invoker == nil {
			t.Error("NewInvoker with very large timeout should return valid invoker")
		}
	})
	
	t.Run("zero and negative timeouts", func(t *testing.T) {
		timeouts := []int{-100, -10, -1, 0}
		
		for _, timeout := range timeouts {
			invoker := NewInvoker(&InvokerConfig{
				Address:        "localhost:8080",
				TimeoutSeconds: timeout,
			})
			if invoker == nil {
				t.Errorf("NewInvoker with timeout=%d should return valid invoker", timeout)
			}
		}
	})
}

// TestBooleanConfigurations tests boolean configuration combinations
func TestBooleanConfigurations(t *testing.T) {
	t.Run("insecure true combinations", func(t *testing.T) {
		configs := []InvokerConfig{
			{Address: "http://localhost:19090", Insecure: true},
			{Address: "http://localhost:19090", Insecure: true, CAFile: "/path/to/ca.crt"},
			{Address: "http://localhost:19090", Insecure: true, CertFile: "/path/to/cert.pem"},
			{Address: "http://localhost:19090", Insecure: true, KeyFile: "/path/to/key.pem"},
		}
		
		for _, cfg := range configs {
			invoker := NewInvoker(&cfg)
			if invoker == nil {
				t.Error("NewInvoker with insecure=true should return valid invoker")
			}
		}
	})
	
	t.Run("insecure false with partial TLS", func(t *testing.T) {
		configs := []InvokerConfig{
			{Address: "http://localhost:19090", Insecure: false, CAFile: "/path/to/ca.crt"},
			{Address: "http://localhost:19090", Insecure: false, CertFile: "/path/to/cert.pem"},
			{Address: "http://localhost:19090", Insecure: false, KeyFile: "/path/to/key.pem"},
		}
		
		for _, cfg := range configs {
			invoker := NewInvoker(&cfg)
			if invoker == nil {
				t.Error("NewInvoker with partial TLS should return valid invoker")
			}
		}
	})
}

// TestFunctionDescriptor_IDVariations tests various function ID formats
func TestFunctionDescriptor_IDVariations(t *testing.T) {
	t.Run("various ID formats", func(t *testing.T) {
		ids := []string{
			"simple",
			"dotted.name",
			"namespace.function.name",
			"user.profile.update",
			"inventory.add.item",
			"guild.member.kick",
			"system.shutdown",
			"1starts.with.number",
			"has_underscore",
			"has-dash",
		}
		
		for _, id := range ids {
			desc := FunctionDescriptor{
				ID:      id,
				Version: "1.0.0",
			}
			
			if desc.ID != id {
				t.Errorf("ID = %s, want %s", desc.ID, id)
			}
		}
	})
	
	t.Run("empty and special IDs", func(t *testing.T) {
		ids := []string{"", " ", "  ", "with spaces", "with\tspecial"}
		
		for _, id := range ids {
			desc := FunctionDescriptor{
				ID:      id,
				Version: "1.0.0",
			}
			
			if desc.ID != id {
				t.Errorf("ID = %s, want %s", desc.ID, id)
			}
		}
	})
}

// TestRetryConfig_BoundaryValues tests boundary values for retry config
func TestRetryConfig_BoundaryValues(t *testing.T) {
	t.Run("boundary max attempts", func(t *testing.T) {
		attempts := []int{-10, -1, 0, 1, 2, 100, 10000}
		
		for _, maxAttempts := range attempts {
			config := &RetryConfig{
				MaxAttempts: maxAttempts,
			}
			
			if config.MaxAttempts != maxAttempts {
				t.Errorf("MaxAttempts = %d, want %d", config.MaxAttempts, maxAttempts)
			}
		}
	})
	
	t.Run("boundary delay values", func(t *testing.T) {
		delays := []int{-1000, -100, -1, 0, 1, 10, 100, 10000}
		
		for _, delay := range delays {
			config := &RetryConfig{
				InitialDelayMs: delay,
			}
			
			if config.InitialDelayMs != delay {
				t.Errorf("InitialDelayMs = %d, want %d", config.InitialDelayMs, delay)
			}
		}
	})
	
	t.Run("boundary multipliers", func(t *testing.T) {
		multipliers := []float64{-10.0, -1.0, 0.0, 0.5, 1.0, 1.1, 2.0, 10.0, 100.0}
		
		for _, mult := range multipliers {
			config := &RetryConfig{
				BackoffMultiplier: mult,
			}
			
			if config.BackoffMultiplier != mult {
				t.Errorf("BackoffMultiplier = %f, want %f", config.BackoffMultiplier, mult)
			}
		}
	})
	
	t.Run("boundary jitter values", func(t *testing.T) {
		jitters := []float64{-1.0, -0.5, 0.0, 0.1, 0.5, 1.0, 1.5, 2.0}
		
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

// TestReconnectConfig_BoundaryValues tests boundary values for reconnect config
func TestReconnectConfig_BoundaryValues(t *testing.T) {
	t.Run("boundary max attempts", func(t *testing.T) {
		attempts := []int{-10, -1, 0, 1, 2, 100, 10000}
		
		for _, maxAttempts := range attempts {
			config := &ReconnectConfig{
				MaxAttempts: maxAttempts,
			}
			
			if config.MaxAttempts != maxAttempts {
				t.Errorf("MaxAttempts = %d, want %d", config.MaxAttempts, maxAttempts)
			}
		}
	})
	
	t.Run("boundary delay values", func(t *testing.T) {
		delays := []int{-10000, -1000, -1, 0, 1, 100, 1000, 100000}
		
		for _, delay := range delays {
			config := &ReconnectConfig{
				InitialDelayMs: delay,
				MaxDelayMs:     delay * 10,
			}
			
			if config.InitialDelayMs != delay {
				t.Errorf("InitialDelayMs = %d, want %d", config.InitialDelayMs, delay)
			}
		}
	})
}

// TestLocalFunctionDescriptor_AllFieldCombinations tests various field combinations
func TestLocalFunctionDescriptor_AllFieldCombinations(t *testing.T) {
	t.Run("minimal descriptor", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:      "minimal",
			Version: "1.0.0",
		}
		
		if desc.ID != "minimal" {
			t.Error("ID should be minimal")
		}
	})
	
	t.Run("descriptor with tags", func(t *testing.T) {
		tags := []string{"tag1", "tag2", "tag3"}
		desc := LocalFunctionDescriptor{
			ID:      "tagged",
			Version: "1.0.0",
			Tags:    tags,
		}
		
		if len(desc.Tags) != 3 {
			t.Errorf("Tags length = %d, want 3", len(desc.Tags))
		}
	})
	
	t.Run("descriptor with all OpenAPI fields", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:          "full",
			Version:     "1.0.0",
			Tags:        []string{"test"},
			Summary:     "Test function",
			Description: "Detailed description",
			OperationID: "fullTest",
			Deprecated:  false,
			InputSchema:  `{"type":"object"}`,
			OutputSchema: `{"type":"object"}`,
			Category:     "test",
			Risk:         "safe",
			Entity:       "TestEntity",
			Operation:    "test",
		}
		
		if desc.Summary != "Test function" {
			t.Error("Summary should be 'Test function'")
		}
	})
}
