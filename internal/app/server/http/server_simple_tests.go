package httpserver

import (
	"testing"
	"time"
	"context"
	"os"

	auditchain "github.com/cuihairu/croupier/internal/audit/chain"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// Mock function invoker for simple tests
type simpleMockInvoker struct{}

func (simpleMockInvoker) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	return &functionv1.InvokeResponse{Payload: []byte(`{"result":"success"}`)}, nil
}

func (simpleMockInvoker) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	return &functionv1.StartJobResponse{JobId: "job-123"}, nil
}

func (simpleMockInvoker) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
	return nil, nil
}

func (simpleMockInvoker) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	return &functionv1.StartJobResponse{JobId: "job-123"}, nil
}

// Test basic server creation and initialization
func TestNewAPIsBasicSetup(t *testing.T) {
	dir := t.TempDir()

	// Setup minimal audit writer
	_ = os.MkdirAll("logs", 0o755)
	aw, err := auditchain.NewWriter("logs/audit.log")
	if err != nil {
		t.Fatalf("audit writer: %v", err)
	}

	// Create registry store with mock data
	store := registry.NewStore()

	// Add mock agent data
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "test-agent-1",
		GameID:   "test-game",
		Env:      "development",
		RPCAddr:  "127.0.0.1:19090",
		Version:  "1.0.0",
		Functions: map[string]registry.FunctionMeta{
			"test-func": {
				Enabled: true,
			},
		},
		ExpireAt: time.Now().Add(time.Hour),
	})

	// Add mock provider capabilities
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:       "test-provider",
		Version:  "1.0.0",
		Lang:     "go",
		SDK:      "go-sdk",
		Manifest: []byte(`{"functions":[{"id":"provider-func","version":"1.0.0"}]}`),
	})

	// Create server with all dependencies
	srv, err := NewServer(dir, new(simpleMockInvoker), aw, nil, store, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Test basic server initialization
	if srv == nil {
		t.Fatal("Server should not be nil")
	}

	t.Logf("Server created successfully with registry containing %d agents", len(store.AgentsUnsafe()))
}

// Test registry store functionality
func TestRegistryStoreFunctionality(t *testing.T) {
	store := registry.NewStore()

	// Test adding agent session
	agent := &registry.AgentSession{
		AgentID:  "test-agent",
		GameID:   "test-game",
		Env:      "development",
		RPCAddr:  "127.0.0.1:19090",
		Version:  "1.0.0",
		Functions: map[string]registry.FunctionMeta{
			"test-func": {
				Enabled: true,
			},
		},
		ExpireAt: time.Now().Add(time.Hour),
	}

	store.UpsertAgent(agent)

	// Verify agent was stored
	store.Mu().RLock()
	storedAgent := store.AgentsUnsafe()["test-agent"]
	store.Mu().RUnlock()

	if storedAgent == nil {
		t.Fatal("Agent should be stored in registry")
	}

	if storedAgent.AgentID != "test-agent" {
		t.Errorf("Expected agent ID 'test-agent', got '%s'", storedAgent.AgentID)
	}

	// Test adding provider capabilities
	provider := registry.ProviderCaps{
		ID:       "test-provider",
		Version:  "1.0.0",
		Lang:     "go",
		SDK:      "go-sdk",
		Manifest: []byte(`{"functions":[{"id":"provider-func","version":"1.0.0"}]}`),
	}

	store.UpsertProviderCaps(provider)

	// Verify provider was stored
	providers := store.ListProviderCaps()
	if len(providers) == 0 {
		t.Fatal("Provider should be stored in registry")
	}

	found := false
	for _, p := range providers {
		if p.ID == "test-provider" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Provider 'test-provider' not found in registry")
	}
}

// Test unified descriptors build
func TestUnifiedDescriptorsBuild(t *testing.T) {
	store := registry.NewStore()

	// Add multiple providers
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:       "provider-1",
		Version:  "1.0.0",
		Lang:     "go",
		SDK:      "go-sdk",
		Manifest: []byte(`{"functions":[{"id":"func-1","version":"1.0.0"}]}`),
	})

	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:       "provider-2",
		Version:  "2.0.0",
		Lang:     "java",
		SDK:      "java-sdk",
		Manifest: []byte(`{"functions":[{"id":"func-2","version":"2.0.0"}]}`),
	})

	// Build unified descriptors
	unified := store.BuildUnifiedDescriptors()

	// Verify unified structure
	if unified == nil {
		t.Fatal("Unified descriptors should not be nil")
	}

	providers, ok := unified["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("Providers field should be a map")
	}

	if len(providers) != 2 {
		t.Errorf("Expected 2 providers in unified descriptors, got %d", len(providers))
	}
}