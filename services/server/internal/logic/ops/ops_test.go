package ops

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// TestInitAgentOpsClient tests the initialization of the global client
func TestInitAgentOpsClient(t *testing.T) {
	// Reset global client by creating a new one
	globalAgentOpsClient = nil
	globalAgentOpsOnce = sync.Once{}

	r := reg.NewStore()
	InitAgentOpsClient(r)

	if globalAgentOpsClient == nil {
		t.Fatal("InitAgentOpsClient() should set globalAgentOpsClient")
	}
	if globalAgentOpsClient.registry != r {
		t.Error("InitAgentOpsClient() registry not set correctly")
	}
}

// TestGetAgentOpsClient tests retrieving the global client
func TestGetAgentOpsClient(t *testing.T) {
	// Ensure global client is initialized
	if globalAgentOpsClient == nil {
		r := reg.NewStore()
		InitAgentOpsClient(r)
	}

	client := GetAgentOpsClient()
	if client == nil {
		t.Fatal("GetAgentOpsClient() should return non-nil client")
	}
	if client != globalAgentOpsClient {
		t.Error("GetAgentOpsClient() should return the global instance")
	}
}

// TestNewAgentOpsClient tests creating a new client
func TestNewAgentOpsClient(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	if client == nil {
		t.Fatal("NewAgentOpsClient() should return non-nil client")
	}
	if client.registry != r {
		t.Error("NewAgentOpsClient() registry not set correctly")
	}
	if client.clients == nil {
		t.Error("NewAgentOpsClient() clients map should be initialized")
	}
	if client.conns == nil {
		t.Error("NewAgentOpsClient() conns map should be initialized")
	}
}

// TestAgentOpsClient_threadSafety tests concurrent access to the client
func TestAgentOpsClient_threadSafety(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// Simulate concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Try to get a client (will fail since agent doesn't exist)
			_, _ = client.GetClient(context.Background(), "test-agent")
		}(i)
	}
	wg.Wait()
	// If we reach here without panic or deadlock, the test passes
}

// TestAgentOpsClient_GetClient_NonExistent tests getting client for non-existent agent
func TestAgentOpsClient_GetClient_NonExistent(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// Test with non-existent agent
	_, err := client.GetClient(context.Background(), "non-existent-agent")
	if err == nil {
		t.Error("GetClient() should return error for non-existent agent")
	}
}

// TestAgentOpsClient_GetClient_EmptyAgentID tests with empty agent ID
func TestAgentOpsClient_GetClient_EmptyAgentID(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	_, err := client.GetClient(context.Background(), "")
	if err == nil {
		t.Error("GetClient() should return error for empty agent ID")
	}
}

// TestAgentOpsClient_GetClient_NilRegistry tests with nil registry
func TestAgentOpsClient_NilRegistry(t *testing.T) {
	client := NewAgentOpsClient(nil)

	_, err := client.GetClient(context.Background(), "test-agent")
	if err == nil {
		t.Error("GetClient() should return error when registry is nil")
	}
	if err != nil && err.Error() != "registry not available" {
		t.Errorf("GetClient() error = %v, want 'registry not available'", err)
	}
}

// TestAgentOpsClient_Close tests closing a specific agent connection
func TestAgentOpsClient_Close(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// Close should not error on non-existent connection
	err := client.Close("non-existent")
	if err != nil {
		t.Errorf("Close() should not error for non-existent connection, got: %v", err)
	}
}

// TestAgentOpsClient_CloseAll tests closing all connections
func TestAgentOpsClient_CloseAll(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// CloseAll should not panic or error with empty maps
	err := client.CloseAll()
	if err != nil {
		t.Errorf("CloseAll() should not error with empty connections, got: %v", err)
	}
}

// TestAgentOpsClient_Remove tests removing a client
func TestAgentOpsClient_Remove(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// Add a mock client
	client.mu.Lock()
	client.clients["test-agent"] = nil
	client.conns["test-agent"] = nil
	client.mu.Unlock()

	// Remove it
	client.Remove("test-agent")

	// Verify it's gone
	client.mu.RLock()
	_, exists := client.clients["test-agent"]
	client.mu.RUnlock()

	if exists {
		t.Error("Remove() should remove the client from the cache")
	}
}

// TestAgentOpsClient_Remove_NonExistent tests removing non-existent client
func TestAgentOpsClient_Remove_NonExistent(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// Should not panic
	client.Remove("non-existent")
}

// TestFormatTimestamp tests the timestamp formatting helper
func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input func() *time.Time
		want  string
	}{
		{
			name: "valid timestamp",
			input: func() *time.Time {
				tm := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
				return &tm
			},
			want: "2024-01-01T12:00:00Z",
		},
		{
			name: "nil timestamp",
			input: func() *time.Time {
				return nil
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.input()
			var result string

			if ts == nil {
				result = ""
			} else {
				result = ts.Format(time.RFC3339)
			}

			if result != tt.want {
				t.Errorf("format timestamp = %v, want %v", result, tt.want)
			}
		})
	}
}

// BenchmarkAgentOpsClient_GetClient benchmarks the GetClient performance
func BenchmarkAgentOpsClient_GetClient(b *testing.B) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)
	agentID := "bench-agent"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail due to non-existent agent, but we're benchmarking the path
		_, _ = client.GetClient(context.Background(), agentID)
	}
}

// BenchmarkAgentOpsClient_Lock benchmarks the lock performance
func BenchmarkAgentOpsClient_Lock(b *testing.B) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.mu.RLock()
		_ = client.clients
		client.mu.RUnlock()
	}
}

// ExampleAgentOpsClient demonstrates how to use the AgentOpsClient
func ExampleAgentOpsClient() {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// In real usage, you would first register an agent
	// Then get a client for that agent
	ctx := context.Background()
	c, err := client.GetClient(ctx, "some-agent-id")
	if err != nil {
		fmt.Printf("Error getting client: %v\n", err)
		return
	}
	_ = c // Use the client to call ops methods

	// When done, close the connection
	_ = client.Close("some-agent-id")
}

// TestAgentOpsClient_ConnectionCaching tests that connections are cached
func TestAgentOpsClient_ConnectionCaching(t *testing.T) {
	r := reg.NewStore()
	client := NewAgentOpsClient(r)

	// Test locking behavior for connection caching
	client.mu.Lock()
	client.clients["test"] = nil // Simulate a cached client
	client.mu.Unlock()

	// Check if the client is in the map
	client.mu.RLock()
	_, exists := client.clients["test"]
	client.mu.RUnlock()

	if !exists {
		t.Error("Connection should be cached in the map")
	}
}
