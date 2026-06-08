package croupier

import (
	"context"
	"testing"
)

func TestTCPManager_New(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}
	handlers := map[string]FunctionHandler{
		"test.func": func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("ok"), nil
		},
	}

	m, err := NewTCPManager(config, handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("NewTCPManager returned nil")
	}
}

func TestTCPManager_New_EmptyHandlers(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, err := NewTCPManager(config, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("NewTCPManager returned nil")
	}
}

func TestTCPManager_Connect_Failure(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "127.0.0.1:19999",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	err := m.Connect(ctx)
	if err == nil {
		t.Error("expected error when connecting to non-existent server")
	}
}

func TestTCPManager_Disconnect_WhenNotConnected(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)

	// Should not panic
	m.Disconnect()
}

func TestTCPManager_IsConnected_Initially(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)

	if m.IsConnected() {
		t.Error("should not be connected initially")
	}
}

func TestTCPManager_RegisterWithAgent_NotConnected(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)

	ctx := context.Background()
	_, err := m.RegisterWithAgent(ctx, "test-service", "1.0.0", nil)
	if err == nil {
		t.Error("expected error when registering without connection")
	}
	if err.Error() != "not connected to agent" {
		t.Errorf("expected 'not connected to agent', got: %v", err)
	}
}

func TestTCPRPCHandler_Handle_UnknownMessage(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	handler := tcpMgr.rpcHandler

	ctx := context.Background()
	_, err := handler.Handle(ctx, 99999, 1, []byte{})
	if err == nil {
		t.Error("expected error for unknown message ID")
	}
}

func TestTCPRPCHandler_invoke_FunctionNotFound(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	handler := tcpMgr.rpcHandler

	// Build an InvokeRequest for a non-existent function
	ctx := context.Background()
	_, err := handler.invoke(ctx, 0, 1, []byte(`{"functionId":"nonexistent"}`))
	if err == nil {
		t.Error("expected error for non-existent function")
	}
}

func TestTCPRPCHandler_invoke_InvalidPayload(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	handler := tcpMgr.rpcHandler

	ctx := context.Background()
	_, err := handler.invoke(ctx, 0, 1, []byte("invalid protobuf"))
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}

func TestTCPRPCHandler_cancelTask_InvalidPayload(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	handler := tcpMgr.rpcHandler

	ctx := context.Background()
	_, err := handler.cancelTask(ctx, 0, 1, []byte("invalid protobuf"))
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}

func TestTCPRPCHandler_streamTask_InvalidPayload(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	handler := tcpMgr.rpcHandler

	ctx := context.Background()
	_, err := handler.streamTask(ctx, 0, 1, []byte("invalid protobuf"))
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}

func TestTCPRPCHandler_streamTask_TaskNotFound(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	handler := tcpMgr.rpcHandler

	// Build a TaskStreamRequest for a non-existent task
	ctx := context.Background()
	_, err := handler.streamTask(ctx, 0, 1, []byte(`\x0a\x09nonexistent`))
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestTCPManager_sendHeartbeat_NotConnected(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	ctx := context.Background()
	err := tcpMgr.sendHeartbeat(ctx)
	if err != nil {
		t.Errorf("expected nil error when not connected, got: %v", err)
	}
}

func TestTCPManager_heartbeatLoop_ContextCancel(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
	}

	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should return immediately without panic
	tcpMgr.heartbeatLoop(ctx)
}
