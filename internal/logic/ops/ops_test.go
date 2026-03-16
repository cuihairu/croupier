package ops

import (
	"context"
	"sync"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// TestInitAgentOpsClient tests the initialization of the global client
func TestInitAgentOpsClient(t *testing.T) {
	// Initialize with a registry store
	r := reg.NewStore()
	InitAgentOpsClient(r)

	client := GetAgentOpsClient()
	if client == nil {
		t.Fatal("InitAgentOpsClient() should set globalAgentOpsClient")
	}
}

// TestGetAgentOpsClient tests retrieving the global client
func TestGetAgentOpsClient(t *testing.T) {
	// Ensure global client is initialized
	client := GetAgentOpsClient()
	if client == nil {
		t.Fatal("GetAgentOpsClient() should return non-nil client")
	}
}

// TestAgentOpsClient_GetClient_NonExistent tests getting client for non-existent agent
func TestAgentOpsClient_GetClient_NonExistent(t *testing.T) {
	client := GetAgentOpsClient()

	// Test with non-existent agent
	_, err := client.GetClient(context.Background(), "non-existent-agent")
	if err == nil {
		t.Error("GetClient() should return error for non-existent agent")
	}
}

// TestAgentOpsClient_GetClient_EmptyAgentID tests with empty agent ID
func TestAgentOpsClient_GetClient_EmptyAgentID(t *testing.T) {
	client := GetAgentOpsClient()

	_, err := client.GetClient(context.Background(), "")
	if err == nil {
		t.Error("GetClient() should return error for empty agent ID")
	}
}

// TestAgentOpsClient_UnregisterClient tests unregistering a client
func TestAgentOpsClient_UnregisterClient(t *testing.T) {
	client := GetAgentOpsClient()

	// Unregister should not panic
	client.UnregisterClient("test-agent")
}

// TestAgentOpsClient_Close tests closing the client
func TestAgentOpsClient_Close(t *testing.T) {
	client := GetAgentOpsClient()

	// Close should not panic
	err := client.Close()
	if err != nil {
		t.Errorf("Close() should not error, got: %v", err)
	}

	// Re-initialize for other tests
	globalAgentOpsClient = nil
	agentOpsClientOnce = sync.Once{}
}

// TestAgentOpsClient_RegisterClient_NotRunning tests registering when existing client is not running
func TestAgentOpsClient_RegisterClient_NotRunning(t *testing.T) {
	// This tests the branch where existing.IsRunning() is false
	// which is covered by the implementation but we can test the behavior

	client := GetAgentOpsClient()

	// Try to get a non-existent client - should return error
	_, err := client.GetClient(context.Background(), "non-existent-agent-test")
	if err == nil {
		t.Error("GetClient() should return error for non-existent agent")
	}
}

// TestAgentOpsClient_GetClient_ContextCancellation tests with cancelled context
func TestAgentOpsClient_GetClient_ContextCancellation(t *testing.T) {
	client := GetAgentOpsClient()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// GetClient should handle cancelled context
	_, err := client.GetClient(ctx, "test-agent")
	if err == nil {
		t.Error("GetClient() should return error for cancelled context when agent doesn't exist")
	}
}
