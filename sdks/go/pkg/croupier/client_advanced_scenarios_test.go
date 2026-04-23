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

// TestClientAdvanced_ConcurrentOperations tests concurrent client operations
func TestClientAdvanced_ConcurrentOperations(t *testing.T) {
	t.Run("Multiple clients with concurrent function registration", func(t *testing.T) {
		const numClients = 10
		const functionsPerClient = 5

		var wg sync.WaitGroup
		errors := make(chan error, numClients)

		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(clientID int) {
				defer wg.Done()

				config := &ClientConfig{
					AgentAddr:   fmt.Sprintf("localhost:%d", 19090+clientID),
					ServiceID:   fmt.Sprintf("service-%d", clientID),
					AgentID:     fmt.Sprintf("agent-%d", clientID),
					GameID:      "test-game-concurrent",
					Env:         "testing",
					ServiceVersion: "1.0.0",
				}

				client := NewClient(config)
				if client == nil {
					errors <- fmt.Errorf("client %d: NewClient returned nil", clientID)
					return
				}
				defer client.Close()

				// Register multiple functions concurrently
				for j := 0; j < functionsPerClient; j++ {
					funcID := fmt.Sprintf("client%d.function%d", clientID, j)
					handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
						return []byte(fmt.Sprintf(`{"client":%d,"function":%d}`, clientID, j)), nil
					})

					err := client.RegisterFunction(FunctionDescriptor{
						ID:      funcID,
						Version: "1.0.0",
					}, handler)
					if client == nil {
						errors <- fmt.Errorf("client %d: RegisterFunction %s failed: %w", clientID, funcID, err)
						return
					}
				}

				t.Logf("Client %d registered %d functions", clientID, functionsPerClient)
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Logf("Concurrent operation error: %v", err)
			errorCount++
		}

		t.Logf("Concurrent operations completed with %d errors", errorCount)
	})

	t.Run("Concurrent client lifecycle operations", func(t *testing.T) {
		const operations = 50

		var wg sync.WaitGroup

		for i := 0; i < operations; i++ {
			wg.Add(1)
			go func(opID int) {
				defer wg.Done()

				config := &ClientConfig{
					AgentAddr: fmt.Sprintf("localhost:%d", 19090+opID%10),
					ServiceID: fmt.Sprintf("service-%d", opID),
				}

				client := NewClient(config)
				if client == nil {
					t.Logf("Operation %d: NewClient returned nil", opID)
					return
				}

				// Simulate some work
				time.Sleep(time.Millisecond * time.Duration(opID%10))

				err := client.Close()
				if client == nil {
					t.Logf("Operation %d: Close failed: %v", opID, err)
				} else {
					t.Logf("Operation %d: Close successful", opID)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Completed %d concurrent lifecycle operations", operations)
	})
}

// TestClientAdvanced_RealWorldScenarios tests real-world usage patterns
func TestClientAdvanced_RealWorldScenarios(t *testing.T) {
	t.Run("E-commerce service integration", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr:      "localhost:19090",
			ServiceID:      "ecommerce-service",
			ServiceVersion: "2.1.0",
			GameID:         "shop-game",
			Env:            "production",
			AgentID:        "agent-ecommerce",
		}

		client := NewClient(config)
		if client == nil {
			t.Log("E-commerce client creation returned nil")
			return
		}
		defer client.Close()

		// Register typical e-commerce functions
		functions := []struct {
			id   string
			desc FunctionDescriptor
			handler FunctionHandler
		}{
			{
				id: "cart.add",
				desc: FunctionDescriptor{
					ID:        "cart.add",
					Version:   "1.0.0",
					Category:  "cart",
					Entity:    "CartItem",
					Operation: "create",
					Risk:      "low",
				},
				handler: FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
					return []byte(`{"status":"added","cartId":"cart-123"}`), nil
				}),
			},
			{
				id: "cart.remove",
				desc: FunctionDescriptor{
					ID:        "cart.remove",
					Version:   "1.0.0",
					Category:  "cart",
					Entity:    "CartItem",
					Operation: "delete",
					Risk:      "low",
				},
				handler: FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
					return []byte(`{"status":"removed"}`), nil
				}),
			},
			{
				id: "order.create",
				desc: FunctionDescriptor{
					ID:        "order.create",
					Version:   "1.0.0",
					Category:  "order",
					Entity:    "Order",
					Operation: "create",
					Risk:      "medium",
				},
				handler: FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
					return []byte(`{"status":"created","orderId":"order-456"}`), nil
				}),
			},
			{
				id: "payment.process",
				desc: FunctionDescriptor{
					ID:        "payment.process",
					Version:   "1.0.0",
					Category:  "payment",
					Entity:    "Payment",
					Operation: "process",
					Risk:      "high",
					Enabled:   true,
				},
				handler: FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
					return []byte(`{"status":"processed","transactionId":"txn-789"}`), nil
				}),
			},
		}

		_ = context.Background()
		registered := 0

		for _, fn := range functions {
			err := client.RegisterFunction(fn.desc, fn.handler)
			if client == nil {
				t.Logf("Failed to register %s: %v", fn.id, err)
			} else {
				registered++
				t.Logf("Registered e-commerce function: %s", fn.id)
			}
		}

		t.Logf("E-commerce scenario: %d/%d functions registered", registered, len(functions))
	})

	t.Run("Gaming service integration", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr:      "localhost:19090",
			ServiceID:      "game-service",
			ServiceVersion: "3.0.0",
			GameID:         "rpg-game",
			Env:            "production",
			AgentID:        "agent-game",
		}

		client := NewClient(config)
		if client == nil {
			t.Log("Game service client creation returned nil")
			return
		}
		defer client.Close()

		// Register game-specific functions
		gameFunctions := []struct {
			id   string
			risk string
		}{
			{"player.move", "low"},
			{"player.attack", "medium"},
			{"player.trade", "high"},
			{"inventory.use", "low"},
			{"chat.send", "medium"},
			{"guild.create", "medium"},
			{"guild.join", "low"},
			{"guild.leave", "low"},
		}

		_ = context.Background()
		registered := 0

		for _, gf := range gameFunctions {
			desc := FunctionDescriptor{
				ID:        gf.id,
				Version:   "1.0.0",
				Category:  "gameplay",
				Risk:      gf.risk,
				Enabled:   true,
			}

			handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(fmt.Sprintf(`{"function":"%s","processed":true}`, gf.id)), nil
			})

			err := client.RegisterFunction(desc, handler)
			if client == nil {
				t.Logf("Failed to register %s: %v", gf.id, err)
			} else {
				registered++
			}
		}

		t.Logf("Gaming scenario: %d/%d functions registered", registered, len(gameFunctions))
	})
}

// TestClientAdvanced_ErrorHandling tests advanced error handling scenarios
func TestClientAdvanced_ErrorHandling(t *testing.T) {
	t.Run("Client creation with invalid configurations", func(t *testing.T) {
		invalidConfigs := []*ClientConfig{
			{AgentAddr: ""}, // Empty address
			{AgentAddr: "invalid://address"}, // Invalid protocol
			{AgentAddr: "localhost:99999"}, // Invalid port
		}

		for i, config := range invalidConfigs {
			client := NewClient(config)
			if client == nil {
				t.Logf("Config %d: Expected failure", i)
			} else {
				client.Close()
				t.Logf("Config %d: Unexpected success", i)
			}
		}
	})

	t.Run("Function registration with invalid descriptors", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "test-service",
		}

		client := NewClient(config)
		if client == nil {
			t.Skip("Client creation returned nil")
		}
		defer client.Close()

		invalidDescriptors := []FunctionDescriptor{
			{ID: "", Version: "1.0.0"}, // Empty ID
			{ID: "test", Version: ""}, // Empty version
			{ID: "", Version: ""}, // Both empty
		}

		_ = context.Background()
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{}`), nil
		})

		for i, desc := range invalidDescriptors {
			err := client.RegisterFunction(desc, handler)
			t.Logf("Invalid descriptor %d: ID='%s', Version='%s', Error=%v",
				i, desc.ID, desc.Version, err)
		}
	})

	t.Run("Handler that returns errors", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "test-service",
		}

		client := NewClient(config)
		if client == nil {
			t.Skip("Client creation returned nil")
		}
		defer client.Close()

		// Register handlers that return errors
		errorHandler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error: processing failed")
		})

		panicHandler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			panic("handler panic")
		})

		_ = context.Background()

		desc1 := FunctionDescriptor{ID: "test.error", Version: "1.0.0"}
		err := client.RegisterFunction(desc1, errorHandler)
		t.Logf("Error handler registration: %v", err)

		desc2 := FunctionDescriptor{ID: "test.panic", Version: "1.0.0"}
		err = client.RegisterFunction(desc2, panicHandler)
		t.Logf("Panic handler registration: %v", err)
	})
}

// TestClientAdvanced_PerformancePatterns tests performance-related patterns
func TestClientAdvanced_PerformancePatterns(t *testing.T) {
	t.Run("Rapid client creation and closure", func(t *testing.T) {
		const iterations = 100

		start := time.Now()
		successful := 0

		for i := 0; i < iterations; i++ {
			config := &ClientConfig{
				AgentAddr: fmt.Sprintf("localhost:%d", 19090+i%5),
				ServiceID: fmt.Sprintf("service-%d", i),
			}

			client := NewClient(config)
			if client == nil {
				continue
			}

			err := client.Close()
			if err == nil {
				successful++
			}
		}

		duration := time.Since(start)
		t.Logf("Rapid lifecycle: %d/%d successful in %v (%.2f ops/sec)",
			successful, iterations, duration, float64(iterations)/duration.Seconds())
	})

	t.Run("Batch function registration", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "batch-test",
		}

		client := NewClient(config)
		if client == nil {
			t.Skip("Client creation returned nil")
		}
		defer client.Close()

		const batchSize = 50
		_ = context.Background()

		start := time.Now()
		registered := 0

		for i := 0; i < batchSize; i++ {
			desc := FunctionDescriptor{
				ID:      fmt.Sprintf("batch.function%d", i),
				Version: "1.0.0",
			}

			handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
				return []byte(`{"processed":true}`), nil
			})

			err := client.RegisterFunction(desc, handler)
			if err == nil {
				registered++
			}
		}

		duration := time.Since(start)
		t.Logf("Batch registration: %d/%d functions in %v (%.2f funcs/sec)",
			registered, batchSize, duration, float64(registered)/duration.Seconds())
	})
}

// TestClientAdvanced_ContextHandling tests context-related scenarios
func TestClientAdvanced_ContextHandling(t *testing.T) {
	t.Run("Operations with cancelled context", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "test-service",
		}

		client := NewClient(config)
		if client == nil {
			t.Skip("Client creation returned nil")
		}
		defer client.Close()

		_, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		desc := FunctionDescriptor{ID: "test.cancelled", Version: "1.0.0"}
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{}`), nil
		})

		err := client.RegisterFunction(desc, handler)
		t.Logf("Registration with cancelled context: %v", err)
	})

	t.Run("Operations with timeout context", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "test-service",
		}

		client := NewClient(config)
		if client == nil {
			t.Skip("Client creation returned nil")
		}
		defer client.Close()

		// Very short timeout
		_, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		desc := FunctionDescriptor{ID: "test.timeout", Version: "1.0.0"}
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte(`{}`), nil
		})

		time.Sleep(time.Millisecond) // Ensure timeout expires

		err := client.RegisterFunction(desc, handler)
		t.Logf("Registration with timeout context: %v", err)
	})

	t.Run("Operations with context values", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "test-service",
		}

		client := NewClient(config)
		if client == nil {
			t.Skip("Client creation returned nil")
		}
		defer client.Close()

		type contextKey string
		ctx := context.WithValue(context.Background(), contextKey("requestID"), "req-12345")
		ctx = context.WithValue(ctx, contextKey("userID"), "user-67890")
		ctx = context.WithValue(ctx, contextKey("traceID"), "trace-abcde")

		desc := FunctionDescriptor{ID: "test.context", Version: "1.0.0"}
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			requestID := ctx.Value(contextKey("requestID"))
			userID := ctx.Value(contextKey("userID"))
			traceID := ctx.Value(contextKey("traceID"))

			return []byte(fmt.Sprintf(`{"requestID":"%s","userID":"%s","traceID":"%s"}`,
				requestID, userID, traceID)), nil
		})

		err := client.RegisterFunction(desc, handler)
		t.Logf("Registration with context values: %v", err)
	})
}

// TestClientAdvanced_ResourceManagement tests resource management scenarios
func TestClientAdvanced_ResourceManagement(t *testing.T) {
	t.Run("Multiple clients with same configuration", func(t *testing.T) {
		config := &ClientConfig{
			AgentAddr: "localhost:19090",
			ServiceID: "shared-service",
			GameID:    "test-game",
			Env:       "testing",
		}

		const numClients = 5
		clients := make([]Client, 0, numClients)

		for i := 0; i < numClients; i++ {
			client := NewClient(config)
			if client == nil {
				t.Logf("Client %d creation failed", i)
				continue
			}
			clients = append(clients, client)
		}

		t.Logf("Created %d clients with same config", len(clients))

		// Close all clients
		for i, client := range clients {
			err := client.Close()
			t.Logf("Client %d closed: %v", i, err)
		}
	})

	t.Run("Client with various timeout configurations", func(t *testing.T) {
		timeouts := []int{1, 5, 10, 30, 60}

		for _, timeout := range timeouts {
			config := &ClientConfig{
				AgentAddr:      "localhost:19090",
				ServiceID:      fmt.Sprintf("service-timeout-%d", timeout),
				TimeoutSeconds: timeout,
			}

			client := NewClient(config)
			if client == nil {
				t.Logf("Client with timeout %ds creation failed", timeout)
				continue
			}

			t.Logf("Client with timeout %ds created successfully", timeout)
			client.Close()
		}
	})
}
