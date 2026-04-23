// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"testing"
)

// TestLoggerConfiguration tests various logger configurations
func TestLoggerConfiguration(t *testing.T) {
	t.Run("config with disable logging", func(t *testing.T) {
		config := &ClientConfig{
			DisableLogging: true,
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with DisableLogging should return valid client")
		}
	})
	
	t.Run("config with debug logging", func(t *testing.T) {
		config := &ClientConfig{
			DebugLogging: true,
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with DebugLogging should return valid client")
		}
	})
	
	t.Run("config with both disable and debug logging", func(t *testing.T) {
		config := &ClientConfig{
			DisableLogging: true,
			DebugLogging:   true,
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with both logging flags should return valid client")
		}
	})
	
	t.Run("config with neither disable nor debug logging", func(t *testing.T) {
		config := &ClientConfig{
			DisableLogging: false,
			DebugLogging:   false,
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with default logging should return valid client")
		}
	})
}

// TestInvokerLogging tests invoker with logging configurations
func TestInvokerLogging(t *testing.T) {
	t.Run("invoker with disabled logging", func(t *testing.T) {
		config := DefaultClientConfig()
		config.DisableLogging = true
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with disabled logging should return valid client")
		}
	})
	
	t.Run("invoker with debug logging", func(t *testing.T) {
		config := DefaultClientConfig()
		config.DebugLogging = true
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with debug logging should return valid client")
		}
	})
	
	t.Run("invoker with normal logging", func(t *testing.T) {
		config := DefaultClientConfig()
		config.DisableLogging = false
		config.DebugLogging = false
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with normal logging should return valid client")
		}
	})
}

// TestInvoker_IdentifierFields tests invoker with various identifier fields
func TestInvoker_IdentifierFields(t *testing.T) {
	t.Run("config with all identifiers", func(t *testing.T) {
		config := &ClientConfig{
			GameID:         "game-123",
			Env:            "production",
			ServiceID:      "service-456",
			ServiceVersion: "2.0.0",
			AgentID:        "agent-789",
			ProviderLang:   "go",
			ProviderSDK:    "croupier-go-sdk",
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with all identifiers should return valid client")
		}
	})
	
	t.Run("config with empty identifiers", func(t *testing.T) {
		config := &ClientConfig{
			GameID:         "",
			Env:            "",
			ServiceID:      "",
			ServiceVersion: "",
			AgentID:        "",
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with empty identifiers should return valid client")
		}
	})
	
	t.Run("config with special characters in identifiers", func(t *testing.T) {
		config := &ClientConfig{
			GameID:    "game_test-123",
			ServiceID: "service.test@456",
			AgentID:   "agent#dev$789",
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with special characters should return valid client")
		}
	})
	
	t.Run("config with very long identifiers", func(t *testing.T) {
		longID := "very-long-identifier-" + string(make([]byte, 100))
		
		config := &ClientConfig{
			ServiceID: longID,
		}
		
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with long identifier should return valid client")
		}
	})
}
