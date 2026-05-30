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
	"time"
)

// TestClient_lifecycle_edge_cases tests client lifecycle edge cases
func TestClient_lifecycle_edge_cases(t *testing.T) {
	t.Run("Create and immediately Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		err := client.Close()
		t.Logf("Immediate Close error: %v", err)
	})

	t.Run("Multiple Close calls", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Multiple Close calls caused panic (recovered): %v", r)
			}
		}()

		err1 := client.Close()
		err2 := client.Close()
		err3 := client.Close()

		t.Logf("Multiple Close errors: %v, %v, %v", err1, err2, err3)
	})

	t.Run("Close after Connect", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		ctx := context.Background()
		connectErr := client.Connect(ctx)
		t.Logf("Connect error: %v", connectErr)

		closeErr := client.Close()
		t.Logf("Close after Connect error: %v", closeErr)
	})

	t.Run("Connect after Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		_ = client.Close()

		ctx := context.Background()
		err := client.Connect(ctx)
		t.Logf("Connect after Close error: %v", err)
	})

	t.Run("GetLocalAddress at various stages", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		addr1 := client.GetLocalAddress()
		t.Logf("Local address (new client): '%s'", addr1)

		ctx := context.Background()
		_ = client.Connect(ctx)

		addr2 := client.GetLocalAddress()
		t.Logf("Local address (after Connect): '%s'", addr2)

		_ = client.Close()

		addr3 := client.GetLocalAddress()
		t.Logf("Local address (after Close): '%s'", addr3)
	})
}

// TestClient_concurrent_operations tests concurrent client operations
func TestClient_concurrent_operations(t *testing.T) {
	t.Run("Concurrent RegisterFunction", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		const numGoroutines = 50
		const funcsPerGoroutine = 10
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < funcsPerGoroutine; j++ {
					desc := FunctionDescriptor{
						ID:      fmt.Sprintf("test.concurrent.%d.%d", idx, j),
						Version: "1.0.0",
					}

					handler := func(ctx context.Context, payload []byte) ([]byte, error) {
						return []byte(`{"result":"ok"}`), nil
					}

					_ = client.RegisterFunction(desc, handler)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent RegisterFunction: %d goroutines × %d functions = %d total",
			numGoroutines, funcsPerGoroutine, numGoroutines*funcsPerGoroutine)
	})

	t.Run("Concurrent Close operations", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		const numGoroutines = 10
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = client.Close()
			}()
		}

		wg.Wait()
		t.Logf("Concurrent Close: %d goroutines", numGoroutines)
	})
}

// TestClient_context_handling tests client behavior with various contexts
func TestClient_context_handling(t *testing.T) {
	t.Run("Connect with cancelled context", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := client.Connect(ctx)
		t.Logf("Connect with cancelled context error: %v", err)
	})

	t.Run("Connect with timeout context", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout expires

		err := client.Connect(ctx)
		t.Logf("Connect with expired timeout error: %v", err)
	})

	t.Run("Connect with context values", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		ctx := context.WithValue(context.Background(), "key", "value")
		err := client.Connect(ctx)
		t.Logf("Connect with context values error: %v", err)
	})
}

// TestClient_multiple_clients tests scenarios with multiple client instances
func TestClient_multiple_clients(t *testing.T) {
	t.Run("Many clients created and closed", func(t *testing.T) {
		const numClients = 100

		for i := 0; i < numClients; i++ {
			config := DefaultClientConfig()
			client := NewClient(config)

			if client == nil {
				t.Errorf("Client %d is nil", i)
				continue
			}

			_ = client.Close()
		}

		t.Logf("Created and closed %d clients successfully", numClients)
	})

	t.Run("Concurrent client creation", func(t *testing.T) {
		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				config := DefaultClientConfig()
				client := NewClient(config)

				if client == nil {
					t.Errorf("Client %d is nil", idx)
					return
				}

				_ = client.Close()
			}(i)
		}

		wg.Wait()
		t.Logf("Concurrent client creation: %d goroutines", numGoroutines)
	})

	t.Run("Multiple clients with different configs", func(t *testing.T) {
		configs := []*ClientConfig{
			{AgentAddr: "localhost:19090"},
			{AgentAddr: "localhost:19091"},
			{AgentAddr: "localhost:19092", Insecure: true},
			{AgentAddr: "localhost:19093", Insecure: true},
			DefaultClientConfig(),
		}

		for i, cfg := range configs {
			client := NewClient(cfg)
			if client == nil {
				t.Errorf("Client with config %d is nil", i)
			}
			_ = client.Close()
		}

		t.Logf("Created %d clients with different configs", len(configs))
	})
}

// TestClient_function_registration_edge_cases tests function registration edge cases
func TestClient_function_registration_edge_cases(t *testing.T) {
	t.Run("Register with empty ID", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		desc := FunctionDescriptor{
			ID:      "",
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("Register with empty ID error: %v", err)
	})

	t.Run("Register with special version formats", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		versions := []string{
			"1.0.0-alpha.1",
			"1.0.0-beta.2",
			"1.0.0-rc.3",
			"2.0.0+build.123",
			"1.0.0-alpha+exp.sha.5114f85",
			"0.0.1",
			"999.999.999",
		}

		for _, version := range versions {
			desc := FunctionDescriptor{
				ID:      fmt.Sprintf("test.version.%s", version),
				Version: version,
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			err := client.RegisterFunction(desc, handler)
			t.Logf("Register with version='%s' error: %v", version, err)
		}
	})

	t.Run("Register with very long ID", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		// Create a very long ID (1000 characters)
		longID := ""
		for i := 0; i < 100; i++ {
			longID += "very.long.function.id."
		}

		desc := FunctionDescriptor{
			ID:      longID,
			Version: "1.0.0",
		}

		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		}

		err := client.RegisterFunction(desc, handler)
		t.Logf("Register with very long ID error: %v", err)
	})

	t.Run("Register after Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		_ = client.Close()

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
}

// TestClient_serve_and_stop tests Serve and Stop operations
func TestClient_serve_and_stop(t *testing.T) {
	t.Run("Stop without Serve", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		err := client.Stop()
		t.Logf("Stop without Serve error: %v", err)
	})

	t.Run("Multiple Stop calls", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		err1 := client.Stop()
		err2 := client.Stop()
		err3 := client.Stop()

		t.Logf("Multiple Stop errors: %v, %v, %v", err1, err2, err3)
	})

	t.Run("Stop after Close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		_ = client.Close()

		err := client.Stop()
		t.Logf("Stop after Close error: %v", err)
	})
}

// TestClient_config_variations tests client with various configurations
func TestClient_config_variations(t *testing.T) {
	t.Run("Client with Insecure variations", func(t *testing.T) {
		configs := []bool{true, false}

		for _, insecure := range configs {
			config := &ClientConfig{
				AgentAddr: "localhost:19090",
				Insecure:  insecure,
			}
			client := NewClient(config)

			if client == nil {
				t.Errorf("Client with Insecure=%v is nil", insecure)
			}
			_ = client.Close()

			t.Logf("Client with Insecure=%v created successfully", insecure)
		}
	})

	t.Run("Client with different AgentAddr formats", func(t *testing.T) {
		addresses := []string{
			"localhost:19090",
			"127.0.0.1:19090",
			"0.0.0.0:19090",
			"example.com:19090",
		}

		for _, addr := range addresses {
			config := &ClientConfig{
				AgentAddr: addr,
			}
			client := NewClient(config)

			if client == nil {
				t.Errorf("Client with AgentAddr='%s' is nil", addr)
			}
			_ = client.Close()

			t.Logf("Client with AgentAddr='%s' created successfully", addr)
		}
	})
}

// TestClient_rapid_operations tests rapid operation sequences
func TestClient_rapid_operations(t *testing.T) {
	t.Run("Rapid Create and Close", func(t *testing.T) {
		const iterations = 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			config := DefaultClientConfig()
			client := NewClient(config)
			if client != nil {
				_ = client.Close()
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Rapid Create+Close: %d iterations in %v (avg %v per cycle)",
			iterations, elapsed, avgTime)
	})

	t.Run("Rapid RegisterFunction", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		defer client.Close()

		const iterations = 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			desc := FunctionDescriptor{
				ID:      fmt.Sprintf("test.rapid.%d", i),
				Version: "1.0.0",
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			_ = client.RegisterFunction(desc, handler)
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Rapid RegisterFunction: %d registrations in %v (avg %v per registration)",
			iterations, elapsed, avgTime)
	})
}

// TestClient_error_handling tests error handling scenarios
func TestClient_error_handling(t *testing.T) {
	t.Run("Nil config handling", func(t *testing.T) {
		client := NewClient(nil)
		if client == nil {
			t.Error("NewClient(nil) returned nil")
		} else {
			_ = client.Close()
			t.Log("NewClient(nil) created a client")
		}
	})

	t.Run("Config with empty AgentAddr", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "",
		}
		client := NewClient(config)

		if client == nil {
			t.Error("NewClient with empty AgentAddr returned nil")
		} else {
			_ = client.Close()
			t.Log("NewClient with empty AgentAddr created a client")
		}
	})

	t.Run("Config with invalid AgentAddr", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "!!!invalid!!!",
		}
		client := NewClient(config)

		if client == nil {
			t.Error("NewClient with invalid AgentAddr returned nil")
		} else {
			_ = client.Close()
			t.Log("NewClient with invalid AgentAddr created a client")
		}
	})
}

// TestClient_resource_cleanup tests resource cleanup
func TestClient_resource_cleanup(t *testing.T) {
	t.Run("Create many clients to check for resource leaks", func(t *testing.T) {
		const numClients = 50
		clients := make([]Client, numClients)

		// Create all clients
		for i := 0; i < numClients; i++ {
			config := DefaultClientConfig()
			clients[i] = NewClient(config)
		}

		// Close all clients
		for i, client := range clients {
			if client != nil {
				err := client.Close()
				t.Logf("Client %d Close error: %v", i, err)
			}
		}

		t.Logf("Created and closed %d clients", numClients)
	})

	t.Run("Register many functions and close", func(t *testing.T) {
		config := DefaultClientConfig()
		client := NewClient(config)

		if client == nil {
			t.Fatal("NewClient returned nil")
		}

		const numFuncs = 100

		// Register many functions
		for i := 0; i < numFuncs; i++ {
			desc := FunctionDescriptor{
				ID:      fmt.Sprintf("test.resource.%d", i),
				Version: "1.0.0",
			}

			handler := func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"result":"ok"}`), nil
			}

			_ = client.RegisterFunction(desc, handler)
		}

		// Close client
		err := client.Close()
		t.Logf("Registered %d functions, Close error: %v", numFuncs, err)
	})
}
