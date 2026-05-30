// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"testing"
)

func TestNewManager_configurations(t *testing.T) {
	t.Run("manager with nil handlers", func(t *testing.T) {
		config := ManagerConfig{
			AgentAddr: "localhost:19090",
		}

		_, err := NewManager(config, nil)
		t.Logf("NewManager with nil handlers error: %v", err)
	})

	t.Run("manager with empty handlers", func(t *testing.T) {
		config := ManagerConfig{
			AgentAddr: "localhost:19090",
		}

		handlers := make(map[string]FunctionHandler)
		_, err := NewManager(config, handlers)
		t.Logf("NewManager with empty handlers error: %v", err)
	})

	t.Run("manager with handlers", func(t *testing.T) {
		config := ManagerConfig{
			AgentAddr: "localhost:19090",
		}

		handlers := map[string]FunctionHandler{
			"test.func": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
		}

		_, err := NewManager(config, handlers)
		t.Logf("NewManager with handlers error: %v", err)
	})
}

func TestManager_basic_operations(t *testing.T) {
	t.Run("manager local address", func(t *testing.T) {
		config := ManagerConfig{
			AgentAddr: "localhost:19090",
		}

		manager, err := NewManager(config, nil)
		if err != nil {
			t.Logf("NewManager error: %v", err)
		}

		if manager != nil {
			addr := manager.GetLocalAddress()
			connected := manager.IsConnected()
			t.Logf("Address: %s, Connected: %v", addr, connected)
		}
	})

	t.Run("manager disconnect", func(t *testing.T) {
		config := ManagerConfig{
			AgentAddr: "localhost:19090",
		}

		manager, err := NewManager(config, nil)
		if err != nil {
			t.Logf("NewManager error: %v", err)
		}

		if manager != nil {
			manager.Disconnect()
			manager.Disconnect()
			t.Log("Multiple disconnects completed")
		}
	})
}

func TestFunctionHandler_mapVariations(t *testing.T) {
	t.Run("handlers with single function", func(t *testing.T) {
		handlers := map[string]FunctionHandler{
			"test.func": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("ok"), nil
			},
		}

		if len(handlers) != 1 {
			t.Errorf("Handlers length = %d, want 1", len(handlers))
		}
	})

	t.Run("handlers with multiple functions", func(t *testing.T) {
		handlers := map[string]FunctionHandler{
			"func1": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("func1"), nil
			},
			"func2": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("func2"), nil
			},
			"func3": func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("func3"), nil
			},
		}

		if len(handlers) != 3 {
			t.Errorf("Handlers length = %d, want 3", len(handlers))
		}
	})

	t.Run("handlers with nil handler", func(t *testing.T) {
		handlers := map[string]FunctionHandler{
			"test.func": nil,
		}

		if handlers["test.func"] != nil {
			t.Error("Handler should be nil")
		}
	})
}

func TestHandler_payloadProcessing(t *testing.T) {
	t.Run("handler with empty payload", func(t *testing.T) {
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("handled empty"), nil
		}

		ctx := context.Background()
		resp, err := handler(ctx, []byte{})
		if err != nil {
			t.Errorf("Handler error: %v", err)
		}
		if string(resp) != "handled empty" {
			t.Errorf("Response = %s, want 'handled empty'", string(resp))
		}
	})

	t.Run("handler with nil payload", func(t *testing.T) {
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("handled nil"), nil
		}

		ctx := context.Background()
		resp, err := handler(ctx, nil)
		if err != nil {
			t.Errorf("Handler error: %v", err)
		}
		if string(resp) != "handled nil" {
			t.Errorf("Response = %s, want 'handled nil'", string(resp))
		}
	})

	t.Run("handler with JSON payload", func(t *testing.T) {
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"success"}`), nil
		}

		ctx := context.Background()
		jsonPayload := []byte(`{"input":"test"}`)

		resp, err := handler(ctx, jsonPayload)
		if err != nil {
			t.Errorf("Handler error: %v", err)
		}
		if string(resp) != `{"result":"success"}` {
			t.Errorf("Response = %s", string(resp))
		}
	})

	t.Run("handler returning error", func(t *testing.T) {
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, context.Canceled
		}

		ctx := context.Background()
		_, err := handler(ctx, []byte("test"))
		if err == nil {
			t.Error("Handler should return error")
		}
	})
}

func TestManagerConfig_various(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		config := ManagerConfig{}

		_, err := NewManager(config, nil)
		t.Logf("NewManager with empty config error: %v", err)
	})

	t.Run("config with various addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:19090",
			"127.0.0.1:19090",
			"tcp://localhost:19090",
			"ipc://croupier-agent",
		}

		for _, addr := range addresses {
			config := ManagerConfig{
				AgentAddr: addr,
			}

			_, err := NewManager(config, nil)
			t.Logf("NewManager with address=%s error: %v", addr, err)
		}
	})

	t.Run("config with various timeouts", func(t *testing.T) {
		timeouts := []int{0, 1, 10, 30, 60, 120}

		for _, timeout := range timeouts {
			config := ManagerConfig{
				AgentAddr:      "localhost:19090",
				TimeoutSeconds: timeout,
			}

			_, err := NewManager(config, nil)
			t.Logf("NewManager with timeout=%d error: %v", timeout, err)
		}
	})
}
