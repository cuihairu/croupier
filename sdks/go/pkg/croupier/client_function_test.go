// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestClient_functionRegistration tests function registration scenarios
func TestClient_functionRegistration(t *testing.T) {
	t.Run("RegisterFunction with minimal descriptor", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "test.minimal",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with minimal descriptor error: %v", err)
	})

	t.Run("RegisterFunction with full descriptor", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:        "test.full",
			Version:   "2.5.0-beta",
			Category:  "test",
			Risk:      "low",
			Entity:    "TestEntity",
			Operation: "create",
			Enabled:   true,
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with full descriptor error: %v", err)
	})

	t.Run("RegisterFunction multiple times same ID", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "test.duplicate",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		// Register same function multiple times
		err1 := client.RegisterFunction(desc, handler)
		err2 := client.RegisterFunction(desc, handler)
		err3 := client.RegisterFunction(desc, handler)

		t.Logf("RegisterFunction 1st time error: %v", err1)
		t.Logf("RegisterFunction 2nd time error: %v", err2)
		t.Logf("RegisterFunction 3rd time error: %v", err3)
	})

	t.Run("RegisterFunction with nil handler", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "test.nilhandler",
			Version: "1.0.0",
		}

		var handler FunctionHandler = nil

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with nil handler error: %v", err)
	})

	t.Run("RegisterFunction with nil payload handling", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "test.nilpayload",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			// Handle nil payload
			if payload == nil {
				return []byte(`{"result":"handled nil"}`), nil
			}
			return payload, nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with nil payload handler error: %v", err)
	})

	t.Run("RegisterFunction with error returning handler", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "test.errorhandler",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, fmt.Errorf("intentional test error")
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with error handler error: %v", err)
	})

	t.Run("RegisterFunction with context cancellation handling", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "test.context",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return []byte(`{"result":"ok"}`), nil
			}
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with context handler error: %v", err)
	})
}

// TestClient_functionVariations tests various function descriptor variations
func TestClient_functionVariations(t *testing.T) {
	t.Run("RegisterFunction with different risk levels", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		risks := []string{"low", "medium", "high", "critical"}

		for _, risk := range risks {
			desc := FunctionDescriptor{
				ID:      "test." + risk,
				Version: "1.0.0",
				Risk:    risk,
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(desc, handler)
			t.Logf("RegisterFunction with risk='%s' error: %v", risk, err)
		}
	})

	t.Run("RegisterFunction with different operations", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		operations := []string{"create", "read", "update", "delete", "custom"}

		for _, op := range operations {
			desc := FunctionDescriptor{
				ID:        "test." + op,
				Version:   "1.0.0",
				Operation: op,
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(desc, handler)
			t.Logf("RegisterFunction with operation='%s' error: %v", op, err)
		}
	})

	t.Run("RegisterFunction with different categories", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		categories := []string{"player", "item", "guild", "chat", "system", "admin"}

		for _, cat := range categories {
			desc := FunctionDescriptor{
				ID:       "test." + cat,
				Version:  "1.0.0",
				Category: cat,
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(desc, handler)
			t.Logf("RegisterFunction with category='%s' error: %v", cat, err)
		}
	})

	t.Run("RegisterFunction with enabled and disabled", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		// Register enabled function
		enabledDesc := FunctionDescriptor{
			ID:      "test.enabled",
			Version: "1.0.0",
			Enabled: true,
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err1 := client.RegisterFunction(enabledDesc, handler)
		t.Logf("RegisterFunction enabled error: %v", err1)

		// Register disabled function
		disabledDesc := FunctionDescriptor{
			ID:      "test.disabled",
			Version: "1.0.0",
			Enabled: false,
		}

		err2 := client.RegisterFunction(disabledDesc, handler)
		t.Logf("RegisterFunction disabled error: %v", err2)
	})

	t.Run("RegisterFunction with special characters in ID", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		ids := []string{
			"test.with-dash",
			"test.with_underscore",
			"test.with.dot",
			"test.with:colon",
			"test.with/slash",
			"test.MixedCase",
			"test.with123numbers",
		}

		for _, id := range ids {
			desc := FunctionDescriptor{
				ID:      id,
				Version: "1.0.0",
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(desc, handler)
			t.Logf("RegisterFunction with ID='%s' error: %v", id, err)
		}
	})
}

// TestClient_versionTests tests version handling
func TestClient_versionTests(t *testing.T) {
	t.Run("RegisterFunction with various version formats", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		versions := []string{
			"1.0.0",
			"2.0.0",
			"0.1.0",
			"1.2.3",
			"10.20.30",
			"1.0.0-alpha",
			"1.0.0-beta",
			"1.0.0-rc.1",
			"2.0.0-beta.2",
			"1.0.0+build",
			"1.0.0-alpha+build",
		}

		for _, version := range versions {
			desc := FunctionDescriptor{
				ID:      "test.version",
				Version: version,
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(desc, handler)
			t.Logf("RegisterFunction with version='%s' error: %v", version, err)
		}
	})
}

// TestClient_multipleClients tests multiple client instances
func TestClient_multipleClients(t *testing.T) {
	t.Run("multiple clients with same config", func(t *testing.T) {
		config := DefaultClientConfig()

		clients := make([]Client, 5)
		for i := 0; i < 5; i++ {
			clients[i] = NewClient(config)
			if clients[i] == nil {
				t.Errorf("Client %d is nil", i)
			}
		}

		// Close all clients
		for i, client := range clients {
			if client != nil {
				err := client.Close()
				t.Logf("Client %d Close error: %v", i, err)
			}
		}
	})

	t.Run("multiple clients with different configs", func(t *testing.T) {
		configs := []*ClientConfig{
			{AgentAddr: "localhost:19090"},
			{AgentAddr: "localhost:19091"},
			{AgentAddr: "localhost:19092"},
			{AgentAddr: "localhost:19093"},
			{AgentAddr: "localhost:19094"},
		}

		for i, cfg := range configs {
			client := NewClient(cfg)
			if client == nil {
				t.Errorf("Client with config %d is nil", i)
			}

			err := client.Close()
			t.Logf("Client %d Close error: %v", i, err)
		}
	})

	t.Run("concurrent client creation", func(t *testing.T) {
		const numClients = 20
		clients := make([]Client, numClients)
		var wg sync.WaitGroup

		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				clients[idx] = NewClient(DefaultClientConfig())
			}(i)
		}

		wg.Wait()

		// Verify all clients created
		for i, client := range clients {
			if client == nil {
				t.Errorf("Client %d is nil", i)
			} else {
				_ = client.Close()
			}
		}

		t.Logf("Created %d clients concurrently", numClients)
	})
}

// TestClient_lifecycleOrdering tests lifecycle operation ordering
func TestClient_lifecycleOrdering(t *testing.T) {
	t.Run("Register before Connect", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		// Register before connect
		desc := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("Register before Connect error: %v", err)

		// Then connect
		ctx := context.Background()
		err = client.Connect(ctx)
		t.Logf("Connect after Register error: %v", err)
	})

	t.Run("Register then Close without Connect", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		desc := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("Register error: %v", err)

		// Close without connect
		err = client.Close()
		t.Logf("Close without Connect error: %v", err)
	})

	t.Run("GetLocalAddress at various stages", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Get address before any operations
		addr1 := client.GetLocalAddress()
		t.Logf("Local address (new): '%s'", addr1)

		// Register function
		desc := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		_ = client.RegisterFunction(desc, handler)

		// Get address after register
		addr2 := client.GetLocalAddress()
		t.Logf("Local address (after register): '%s'", addr2)

		_ = client.Close()

		// Get address after close
		addr3 := client.GetLocalAddress()
		t.Logf("Local address (after close): '%s'", addr3)
	})

	t.Run("Stop without Serve", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		// Stop without Serve
		err := client.Stop()
		t.Logf("Stop without Serve error: %v", err)
	})

	t.Run("Stop after Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Close first
		err := client.Close()
		t.Logf("Close error: %v", err)

		// Then stop
		err = client.Stop()
		t.Logf("Stop after Close error: %v", err)
	})
}

// TestClient_descriptionTests tests descriptor fields
func TestClient_descriptionTests(t *testing.T) {
	t.Run("RegisterFunction with various descriptors", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		ids := []string{
			"test.desc0",
			"test.desc1",
			"test.desc2",
			"test.desc3",
			"test.desc4",
			"test.desc5",
		}

		for _, id := range ids {
			functionDesc := FunctionDescriptor{
				ID:      id,
				Version: "1.0.0",
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(functionDesc, handler)
			t.Logf("RegisterFunction with ID '%s' error: %v", id, err)
		}
	})

	t.Run("RegisterFunction with all descriptor fields", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:        "test.complete",
			Version:   "3.5.0-beta",
			Category:  "comprehensive",
			Risk:      "medium",
			Entity:    "TestEntity",
			Operation: "update",
			Enabled:   true,
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction with complete descriptor error: %v", err)
	})
}

// TestClient_errorPaths tests client error paths
func TestClient_errorPaths(t *testing.T) {
	t.Run("Close before any operations", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Close immediately
		err := client.Close()
		t.Logf("Close immediately error: %v", err)
	})

	t.Run("Register after Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Close first
		_ = client.Close()

		// Try to register
		desc := FunctionDescriptor{
			ID:      "test.afterclose",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("Register after Close error: %v", err)
	})

	t.Run("Connect after Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Close first
		_ = client.Close()

		// Try to connect
		ctx := context.Background()
		err := client.Connect(ctx)
		t.Logf("Connect after Close error: %v", err)
	})

	t.Run("multiple Close calls", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		// Close multiple times
		err1 := client.Close()
		err2 := client.Close()
		err3 := client.Close()

		t.Logf("First Close error: %v", err1)
		t.Logf("Second Close error: %v", err2)
		t.Logf("Third Close error: %v", err3)
	})

	t.Run("multiple Stop calls", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		// Stop multiple times
		err1 := client.Stop()
		err2 := client.Stop()
		err3 := client.Stop()

		t.Logf("First Stop error: %v", err1)
		t.Logf("Second Stop error: %v", err2)
		t.Logf("Third Stop error: %v", err3)
	})
}
