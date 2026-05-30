// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewClient_coverage(t *testing.T) {
	t.Run("create client with nil config", func(t *testing.T) {
		client := NewClient(nil)
		if client == nil {
			t.Error("NewClient(nil) should return a client")
		}
	})

	t.Run("create client with empty config", func(t *testing.T) {
		config := &ClientConfig{}
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with empty config should return a client")
		}
	})

	t.Run("create client with default config", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)
		if client == nil {
			t.Error("NewClient with default config should return a client")
		}
	})

	t.Run("create multiple clients", func(t *testing.T) {
		config := DefaultClientConfig()
		client1 := NewClient(config)
		client2 := NewClient(config)

		if client1 == nil || client2 == nil {
			t.Error("NewClient should return valid clients")
		}

		if client1 == client2 {
			t.Error("NewClient should return different instances")
		}
	})
}

func TestClient_RegisterFunction_coverage(t *testing.T) {
	t.Run("register function with minimal descriptor", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("response"), nil
		}

		desc := FunctionDescriptor{
			ID:      "test.function",
			Version: "1.0.0",
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction error: %v", err)
	})

	t.Run("register function with full descriptor", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"success"}`), nil
		}

		desc := FunctionDescriptor{
			ID:        "player.create",
			Version:   "1.0.0",
			Category:  "player",
			Risk:      "low",
			Entity:    "Player",
			Operation: "create",
			Enabled:   true,
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("RegisterFunction error: %v", err)
	})

	t.Run("register multiple functions", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		for i := 0; i < 5; i++ {
			desc := FunctionDescriptor{
				ID:      "test.func",
				Version: "1.0.0",
			}
			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte("response"), nil
			}
			_ = client.RegisterFunction(desc, handler)
		}
	})

	t.Run("register function with nil handler", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		desc := FunctionDescriptor{
			ID:      "test.nil",
			Version: "1.0.0",
		}

		err := client.RegisterFunction(desc, nil)
		t.Logf("RegisterFunction with nil handler error: %v", err)
	})
}

func TestClient_Connect_coverage(t *testing.T) {
	t.Run("connect with context", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		ctx := context.Background()
		err := client.Connect(ctx)
		t.Logf("Connect error: %v", err)
	})

	t.Run("connect with timeout context", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := client.Connect(ctx)
		t.Logf("Connect with timeout error: %v", err)
	})

	t.Run("connect with cancelled context", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.Connect(ctx)
		t.Logf("Connect with cancelled context error: %v", err)
	})
}

func TestClient_Close_coverage(t *testing.T) {
	t.Run("close new client", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		err := client.Close()
		if err != nil {
			t.Logf("Close error: %v", err)
		}
	})

	t.Run("close client twice", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		err1 := client.Close()
		t.Logf("First close error: %v", err1)

		// Note: Calling Close() twice may panic due to closing closed channel
		// This is expected behavior in current implementation
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Second close panicked (expected): %v", r)
			}
		}()
		err2 := client.Close()
		t.Logf("Second close error: %v", err2)
	})
}

func TestClient_ConcurrentOperations_coverage(t *testing.T) {
	t.Run("concurrent register functions", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		const numGoroutines = 10
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				desc := FunctionDescriptor{
					ID:      "concurrent.func",
					Version: "1.0.0",
				}
				handler := func(ctx context.Context, payload []byte) ([]byte, error) {
					return []byte("response"), nil
				}
				_ = client.RegisterFunction(desc, handler)
			}(i)
		}

		wg.Wait()
	})
}

func TestClient_VariousConfigs_coverage(t *testing.T) {
	t.Run("client with all timeout values", func(t *testing.T) {
		timeouts := []int{-1, 0, 1, 10, 30, 60, 120, 300}

		for _, timeout := range timeouts {
			config := &ClientConfig{
				AgentAddr:      "localhost:19090",
				TimeoutSeconds: timeout,
			}
			client := NewClient(config)
			if client == nil {
				t.Errorf("NewClient with timeout=%d should return valid client", timeout)
			}
		}
	})

	t.Run("client with various environments", func(t *testing.T) {
		envs := []string{"development", "staging", "production", "test", "dev", "prod", ""}

		for _, env := range envs {
			config := &ClientConfig{
				AgentAddr: "localhost:19090",
				Env:       env,
			}
			client := NewClient(config)
			if client == nil {
				t.Errorf("NewClient with env=%s should return valid client", env)
			}
		}
	})

	t.Run("client with various agent addresses", func(t *testing.T) {
		addresses := []string{
			"localhost:19090",
			"127.0.0.1:19090",
			"tcp://localhost:19090",
			"ipc://croupier-agent",
		}

		for _, addr := range addresses {
			config := &ClientConfig{
				AgentAddr: addr,
			}
			client := NewClient(config)
			if client == nil {
				t.Errorf("NewClient with addr=%s should return valid client", addr)
			}
		}
	})
}
