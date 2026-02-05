package agent

import (
	"context"
	"testing"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// TestUpstreamClient_NewUpstreamClient tests client creation
func TestUpstreamClient_NewUpstreamClient(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, &UpstreamMetadata{
		GameID:  "game-1",
		Env:     "staging",
		Version: "agent-ver",
		RPCAddr: "127.0.0.1:19090",
		Region:  "us-west-1",
		Zone:    "us-west-1a",
	})

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.agentID != "agent-1" {
		t.Errorf("expected agentID 'agent-1', got '%s'", client.agentID)
	}
	if client.gameID != "game-1" {
		t.Errorf("expected gameID 'game-1', got '%s'", client.gameID)
	}
	if client.env != "staging" {
		t.Errorf("expected env 'staging', got '%s'", client.env)
	}
	if client.version != "agent-ver" {
		t.Errorf("expected version 'agent-ver', got '%s'", client.version)
	}
	if client.rpcAddr != "127.0.0.1:19090" {
		t.Errorf("expected rpcAddr '127.0.0.1:19090', got '%s'", client.rpcAddr)
	}
	if client.region != "us-west-1" {
		t.Errorf("expected region 'us-west-1', got '%s'", client.region)
	}
	if client.zone != "us-west-1a" {
		t.Errorf("expected zone 'us-west-1a', got '%s'", client.zone)
	}
}

// TestUpstreamClient_NewUpstreamClient_NilMetadata tests client creation with nil metadata
func TestUpstreamClient_NewUpstreamClient_NilMetadata(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, nil)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.agentID != "agent-1" {
		t.Errorf("expected agentID 'agent-1', got '%s'", client.agentID)
	}
}

// TestUpstreamClient_WithMetadata tests metadata updates
func TestUpstreamClient_WithMetadata(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, &UpstreamMetadata{
		GameID: "game-1",
		Env:    "staging",
	})

	// Update metadata
	client.WithMetadata(UpstreamMetadata{
		GameID:  "game-2",
		Env:     "production",
		Version: "v2.0.0",
		RPCAddr: "127.0.0.1:19091",
	})

	if client.gameID != "game-2" {
		t.Errorf("expected gameID 'game-2', got '%s'", client.gameID)
	}
	if client.env != "production" {
		t.Errorf("expected env 'production', got '%s'", client.env)
	}
	if client.version != "v2.0.0" {
		t.Errorf("expected version 'v2.0.0', got '%s'", client.version)
	}
	if client.rpcAddr != "127.0.0.1:19091" {
		t.Errorf("expected rpcAddr '127.0.0.1:19091', got '%s'", client.rpcAddr)
	}
}

// TestUpstreamClient_Connected tests Connected method
func TestUpstreamClient_Connected(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, nil)

	// Not connected before dial
	if client.Connected() {
		t.Error("expected false when not connected")
	}
}

// TestUpstreamClient_Sync_NotConnected tests Sync when not connected
func TestUpstreamClient_Sync_NotConnected(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, nil)

	err := client.Sync(context.Background())
	if err == nil {
		t.Error("expected error when not connected")
	}
}

// TestUpstreamClient_Sync_NilClient tests Sync with nil client
func TestUpstreamClient_Sync_NilClient(t *testing.T) {
	t.Parallel()

	var client *UpstreamClient

	err := client.Sync(context.Background())
	if err == nil {
		t.Error("expected error with nil client")
	}
}

// TestUpstreamClient_Heartbeat_NotConnected tests Heartbeat when not connected
func TestUpstreamClient_Heartbeat_NotConnected(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, nil)

	err := client.Heartbeat(context.Background())
	if err == nil {
		t.Error("expected error when not connected")
	}
}

// TestUpstreamClient_StoreDataCollection tests that store data is correctly structured for sync
func TestUpstreamClient_StoreDataCollection(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()
	store.Register("svc-1", "127.0.0.1:10001", "v1", []*sdkv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
		{Id: "f2", Version: "2.0.0"},
	})

	// Verify store has the data
	localData := store.List()
	if len(localData) != 2 {
		t.Fatalf("expected 2 functions in store, got %d", len(localData))
	}

	// Verify function IDs
	funcIDs := []string{}
	for fid := range localData {
		funcIDs = append(funcIDs, fid)
	}

	if len(funcIDs) != 2 {
		t.Fatalf("expected 2 function IDs, got %d", len(funcIDs))
	}

	// Check f1 and f2 exist
	hasF1, hasF2 := false, false
	for _, fid := range funcIDs {
		if fid == "f1" {
			hasF1 = true
		}
		if fid == "f2" {
			hasF2 = true
		}
	}

	if !hasF1 {
		t.Error("expected function f1 in store")
	}
	if !hasF2 {
		t.Error("expected function f2 in store")
	}
}

// TestUpstreamClient_Stop tests Stop method
func TestUpstreamClient_Stop(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, nil)

	// Should not panic
	client.Stop()
}

// TestPickVersion tests the pickVersion helper function
func TestPickVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		versions map[string]string
		want     string
	}{
		{
			name:     "nil map",
			versions: nil,
			want:     "",
		},
		{
			name:     "empty map",
			versions: map[string]string{},
			want:     "",
		},
		{
			name:     "single version",
			versions: map[string]string{"svc-1": "1.0.0"},
			want:     "1.0.0",
		},
		{
			name:     "multiple versions",
			versions: map[string]string{"svc-1": "1.0.0", "svc-2": "2.0.0"},
			want:     "2.0.0", // highest
		},
		{
			name:     "versions with empty strings",
			versions: map[string]string{"svc-1": "", "svc-2": "1.5.0"},
			want:     "1.5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickVersion(tt.versions); got != tt.want {
				t.Errorf("pickVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFirstNonEmpty tests the firstNonEmpty helper function
func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "no values",
			values: []string{},
			want:   "",
		},
		{
			name:   "all empty",
			values: []string{"", "", ""},
			want:   "",
		},
		{
			name:   "first non-empty",
			values: []string{"", "value1", "value2"},
			want:   "value1",
		},
		{
			name:   "first value non-empty",
			values: []string{"value1", "value2"},
			want:   "value1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.values...); got != tt.want {
				t.Errorf("firstNonEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHostFromTarget tests the hostFromTarget helper function
func TestHostFromTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "empty string",
			target: "",
			want:   "",
		},
		{
			name:   "host:port",
			target: "example.com:8080",
			want:   "example.com",
		},
		{
			name:   "ipv6:port",
			target: "[::1]:8080",
			want:   "::1",
		},
		{
			name:   "ipv6 without port",
			target: "[::1]",
			want:   "::1",
		},
		{
			name:   "host only",
			target: "example.com",
			want:   "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostFromTarget(tt.target); got != tt.want {
				t.Errorf("hostFromTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}
