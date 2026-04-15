package ops

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/transport"
)

// TestInitAgentOpsClient tests the initialization of the global client
func TestInitAgentOpsClient(t *testing.T) {
	// Initialize without parameters
	InitAgentOpsClient()

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

// TestAgentOpsClient_SetSessionResolver tests setting session resolver
func TestAgentOpsClient_SetSessionResolver(t *testing.T) {
	client := GetAgentOpsClient()

	// Set a nil resolver (for testing purposes)
	client.SetSessionResolver(nil)

	// GetClient should fail with nil resolver
	_, err := client.GetClient(context.Background(), "test-agent")
	if err == nil {
		t.Error("GetClient() should return error when resolver is nil")
	}
}

// TestAgentOpsClient_SetSessionResolver_ThenGetClient tests setting resolver and getting client
func TestAgentOpsClient_SetSessionResolver_ThenGetClient(t *testing.T) {
	client := GetAgentOpsClient()

	// Create a mock resolver
	mockResolver := &mockSessionResolver{
		agentExists: true,
	}

	client.SetSessionResolver(mockResolver)

	// GetClient should succeed with mock resolver
	wrapper, err := client.GetClient(context.Background(), "test-agent")
	if err != nil {
		t.Errorf("GetClient() should not error with mock resolver, got: %v", err)
	}
	if wrapper == nil {
		t.Error("GetClient() should return non-nil wrapper")
	}
}

// mockSessionResolver is a mock implementation of AgentSessionResolver
type mockSessionResolver struct {
	agentExists bool
}

func (m *mockSessionResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	if !m.agentExists {
		return nil, false
	}
	return &mockSessionCaller{}, true
}

// mockSessionCaller is a mock implementation of SessionCaller
type mockSessionCaller struct{}

func (m *mockSessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	return msgID, []byte("ok"), nil
}
