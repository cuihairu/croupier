package croupier

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	config := ManagerConfig{
		AgentAddr:    "127.0.0.1:19090",
		ControlAddr:  "localhost:19100",
		ProviderLang: "golang",
		ProviderSDK:  "custom-go-sdk",
	}

	handlers := map[string]FunctionHandler{
		"test.fn": func(ctx context.Context, payload []byte) ([]byte, error) {
			return payload, nil
		},
	}

	mgr, err := NewManager(config, handlers)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_GetDispatcher(t *testing.T) {
	t.Parallel()

	dispatcher := GetDispatcher()
	if dispatcher == nil {
		t.Error("GetDispatcher returned nil")
	}
}

func TestNewTCPManager(t *testing.T) {
	t.Parallel()

	config := ClientConfig{
		ServiceID:         "test-service",
		ServiceVersion:    "1.0.0",
		AgentAddr:         "127.0.0.1:19090",
		ControlAddr:       "localhost:19100",
		HeartbeatInterval: 15,
		ProviderLang:      "golang",
		ProviderSDK:       "custom-go-sdk",
		Insecure:          true,
	}

	handlers := map[string]FunctionHandler{
		"test.fn": func(ctx context.Context, payload []byte) ([]byte, error) {
			return payload, nil
		},
	}

	mgr, err := NewTCPManager(config, handlers)
	if err != nil {
		t.Fatalf("NewTCPManager returned error: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewTCPManager returned nil")
	}

	tcpMgr, ok := mgr.(*TCPManager)
	if !ok {
		t.Fatal("NewTCPManager did not return *TCPManager")
	}
	if tcpMgr.config.HeartbeatInterval != 15 {
		t.Fatalf("expected HeartbeatInterval 15, got %d", tcpMgr.config.HeartbeatInterval)
	}
	if tcpMgr.config.ControlAddr != "localhost:19100" {
		t.Fatalf("expected ControlAddr localhost:19100, got %q", tcpMgr.config.ControlAddr)
	}
	if tcpMgr.config.ProviderLang != "golang" {
		t.Fatalf("expected ProviderLang golang, got %q", tcpMgr.config.ProviderLang)
	}
	if tcpMgr.config.ProviderSDK != "custom-go-sdk" {
		t.Fatalf("expected ProviderSDK custom-go-sdk, got %q", tcpMgr.config.ProviderSDK)
	}
}

func TestNewTCPManager_EmptyHandlers(t *testing.T) {
	t.Parallel()

	config := ClientConfig{
		ServiceID:      "test-service",
		ServiceVersion: "1.0.0",
		AgentAddr:      "127.0.0.1:19090",
		Insecure:       true,
	}

	// Empty handlers should be valid
	handlers := map[string]FunctionHandler{}

	mgr, err := NewTCPManager(config, handlers)
	if err != nil {
		t.Fatalf("NewTCPManager returned error: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewTCPManager returned nil")
	}
}
