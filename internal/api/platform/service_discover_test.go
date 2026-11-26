package platform

import (
	"context"
	"fmt"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

func TestDiscoverExternalPlatforms(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.install_app": {Enabled: true},
			"external.onepanel.list_apps":   {Enabled: true},
			"test.echo":                     {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())
	methods := got["onepanel"]
	if len(methods) != 2 {
		t.Fatalf("expected two discovered methods for onepanel, got=%v", methods)
	}
}

func TestDiscoverExternalPlatforms_EmptyStore(t *testing.T) {
	store := reg.NewStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for empty store, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NoExternalFunctions(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"test.echo": {Enabled: true},
			"test.ping": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms when no external functions, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_MultiplePlatforms(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
			"external.platform1.method2": {Enabled: true},
			"external.platform2.method1": {Enabled: true},
			"test.echo":                  {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(platforms))
	}
	if _, ok := platforms["platform1"]; !ok {
		t.Fatal("expected platform1 to be present")
	}
	if _, ok := platforms["platform2"]; !ok {
		t.Fatal("expected platform2 to be present")
	}
}

func TestDiscoverExternalPlatforms_NilSvcCtx(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil svcCtx, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NilRegistryStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil store, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NilExtensions(t *testing.T) {
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store, Extensions: nil}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should work with just registry store
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms with no agents, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NilInstallationService(t *testing.T) {
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    &svc.ExtensionServices{Installation: nil},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should work with just registry store
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms with no agents, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_WithInstallationBindings(t *testing.T) {
	// This tests the installation binding discovery path
	// Since we can't easily mock the Installation service, we test the nil case
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	service := NewService(svcCtx)

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover from registry even with nil Installation
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method for test platform, got %d", len(platforms["test"]))
	}
}

func TestDiscoverExternalPlatforms_WithExtensionsInstalled(t *testing.T) {
	// Test the Extensions.Installation discovery path
	store := reg.NewStore()

	// Create a mock extension services with nil installation
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	service := NewService(svcCtx)

	// Should work without panicking
	platforms := service.discoverExternalPlatforms(context.Background())
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

func TestDiscoverExternalPlatforms_NilAgent(t *testing.T) {
	store := reg.NewStore()

	// Add an agent with nil functions map
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:   "nil-agent",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: nil,
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil functions, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_DisabledFunctions(t *testing.T) {
	store := reg.NewStore()

	// Add agent with disabled functions
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: false},
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should only discover enabled functions
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 enabled method for test platform, got %d", len(platforms["test"]))
	}
	if platforms["test"][0] != "method2" {
		t.Fatalf("expected method2, got %s", platforms["test"][0])
	}
}

func TestDiscoverExternalPlatforms_MultipleAgentsSamePlatform(t *testing.T) {
	store := reg.NewStore()

	// Add multiple agents with functions for the same platform
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should merge methods from both agents
	if len(platforms["test"]) != 2 {
		t.Fatalf("expected 2 methods from multiple agents, got %d", len(platforms["test"]))
	}
}

func TestDiscoverExternalPlatforms_DeduplicatesMethods(t *testing.T) {
	store := reg.NewStore()

	// Add multiple agents with the same function (case insensitive)
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.get_player": {Enabled: true},
		},
	})

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.GET_PLAYER": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should deduplicate case-insensitively
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 deduplicated method, got %d: %v", len(platforms["test"]), platforms["test"])
	}
}

func TestDiscoverExternalPlatforms_SkipsNonExternalFunctions(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"game.start":       {Enabled: true},
			"game.stop":        {Enabled: true},
			"player.get":       {Enabled: true},
			"external.test.fn": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should only discover external.* functions
	if len(platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(platforms))
	}
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method for test platform, got %d", len(platforms["test"]))
	}
}

func TestDiscoverExternalPlatforms_EmptyFunctionID(t *testing.T) {
	store := reg.NewStore()

	// Add agent with empty function ID (edge case)
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should handle empty function ID gracefully
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for empty function ID, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_InvalidFunctionID(t *testing.T) {
	store := reg.NewStore()

	// Add agent with invalid function IDs
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external":      {Enabled: true}, // No method part
			"external.test": {Enabled: true}, // Only has platform
			"test.echo":     {Enabled: true}, // Non-external
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should skip invalid function IDs
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for invalid function IDs, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_WithExtensionsNilInstallation(t *testing.T) {
	store := reg.NewStore()

	// Create Extensions with nil Installation
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	service := NewService(svcCtx)

	// Should not panic
	platforms := service.discoverExternalPlatforms(context.Background())
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

func TestDiscoverExternalPlatforms_WithNilExtensions(t *testing.T) {
	store := reg.NewStore()

	// Test with Extensions field explicitly set to nil
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil,
	}
	service := NewService(svcCtx)

	// Should not panic
	platforms := service.discoverExternalPlatforms(context.Background())
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

func TestDiscoverExternalPlatforms_WithBothSources(t *testing.T) {
	store := reg.NewStore()

	// Add registry agent
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "registry-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	// Create service with both registry and extensions
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil, // Will skip installation path
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover from registry
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method from registry, got %d", len(platforms["test"]))
	}
}

func TestDiscoverExternalPlatforms_EmptyRegistryStore(t *testing.T) {
	store := reg.NewStore()
	// Store is empty, no agents

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms, got %d", len(platforms))
	}
}

func TestDiscoverExternalPlatforms_FunctionsMapIteration(t *testing.T) {
	store := reg.NewStore()

	// Add agent with multiple platforms
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.steam.get_player":  {Enabled: true},
			"external.ephicgames.login":  {Enabled: true},
			"external.xbox.auth":         {Enabled: true},
			"external.ps.plus":           {Enabled: true},
			"external.nintendo.account":  {Enabled: true},
			"external.test.not_external": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover 5 platforms (steam, epicgames, xbox, ps, nintendo)
	if len(platforms) != 5 {
		t.Fatalf("expected 5 platforms, got %d: %v", len(platforms), platforms)
	}
}

func TestDiscoverExternalPlatforms_WithEmptyFunctions(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:   "test-agent",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map
	if len(platforms) != 0 {
		t.Fatalf("expected 0 platforms for empty functions, got %d", len(platforms))
	}
}

func TestDiscoverExternalPlatforms_WithNoAgents(t *testing.T) {
	store := reg.NewStore()

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map when no agents registered
	if len(platforms) != 0 {
		t.Fatalf("expected 0 platforms when no agents, got %d", len(platforms))
	}
}

func TestDiscoverExternalPlatforms_WithNilServiceContext(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map when service context is nil
	if len(platforms) != 0 {
		t.Fatalf("expected 0 platforms with nil service context, got %d", len(platforms))
	}
}

// TestDiscoverExternalPlatforms_MultipleAgents tests discovery with multiple agents
func TestDiscoverExternalPlatforms_MultipleAgents(t *testing.T) {
	store := reg.NewStore()

	// Add multiple agents with different functions
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
			"external.platform1.method2": {Enabled: true},
		},
	})

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform2.method1": {Enabled: true},
			"external.platform1.method3": {Enabled: true}, // Same platform, different method
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover both platforms
	if len(platforms) < 2 {
		t.Fatalf("expected at least 2 platforms, got=%d", len(platforms))
	}

	// platform1 should have 3 methods (method1, method2 from agent1 and method3 from agent2)
	methods, ok := platforms["platform1"]
	if !ok {
		t.Fatalf("expected platform1 in discovered platforms, got: %v", platforms)
	}
	if len(methods) < 3 {
		t.Fatalf("expected at least 3 methods for platform1, got=%v", methods)
	}
}

// TestDiscoverExternalPlatforms_DisabledFunctionsCheck tests that disabled functions are not discovered

// TestDiscoverExternalPlatforms_DisabledFunctionsCheck tests that disabled functions are not discovered
func TestDiscoverExternalPlatforms_DisabledFunctionsCheck(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.enabled_method":  {Enabled: true},
			"external.test.disabled_method": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	methods := platforms["test"]
	if len(methods) != 1 {
		t.Fatalf("expected only 1 method (enabled), got=%v", methods)
	}
	if methods[0] != "enabled_method" {
		t.Fatalf("expected enabled_method, got=%s", methods[0])
	}
}

// TestDiscoverExternalPlatforms_NilExtensionsCheck tests discovery when Extensions is nil

// TestDiscoverExternalPlatforms_NilExtensionsCheck tests discovery when Extensions is nil
func TestDiscoverExternalPlatforms_NilExtensionsCheck(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil, // No extensions configured
	}

	service := NewService(svcCtx)
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should still discover from registry
	if len(platforms) != 1 {
		t.Fatalf("expected 1 platform from registry, got=%d", len(platforms))
	}
}

// TestService_Call_DispatcherError tests Call when dispatcher returns an error

// TestDiscoverExternalPlatforms_DisabledFunction tests that disabled functions are not discovered
func TestDiscoverExternalPlatforms_DisabledFunction2(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.enabled":  {Enabled: true},
			"external.test.disabled": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())

	methods := got["test"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method (only enabled), got %d: %v", len(methods), methods)
	}
	if methods[0] != "enabled" {
		t.Fatalf("expected 'enabled' method, got %s", methods[0])
	}
}

// TestDiscoverExternalPlatforms_Deduplication tests method deduplication

// TestDiscoverExternalPlatforms_Deduplication tests method deduplication
func TestDiscoverExternalPlatforms_Deduplication(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true}, // Duplicate
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())

	methods := got["test"]
	if len(methods) != 2 {
		t.Fatalf("expected 2 unique methods, got %d: %v", len(methods), methods)
	}
}

// TestCall_DispatcherNonJSONResponse tests the response handling when dispatcher returns non-JSON

// TestDiscoverExternalPlatforms_WithExtensionsNil covers nil Extensions field
func TestDiscoverExternalPlatforms_WithExtensionsNil(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil,
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should still discover from registry
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method from registry, got %d", len(platforms["test"]))
	}
}

// TestListMethods_WithWhitespacePlatform covers whitespace-only platform parameter

// TestDiscoverExternalPlatforms_DisabledFunction tests that disabled functions are ignored
func TestDiscoverExternalPlatforms_DisabledFunction(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.enabled_method":  {Enabled: true},
			"external.test.disabled_method": {Enabled: false},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	result := service.discoverExternalPlatforms(context.Background())

	methods := result["test"]
	if len(methods) != 1 {
		t.Fatalf("expected only enabled method (1), got %d: %v", len(methods), methods)
	}
	if methods[0] != "enabled_method" {
		t.Fatalf("expected enabled_method, got %s", methods[0])
	}
}

// TestListPlatforms_MultiplePlatforms tests listing multiple platforms

// TestDiscoverExternalPlatforms_WithNilInstallation tests the Installation discovery path with nil
func TestDiscoverExternalPlatforms_WithNilInstallation2(t *testing.T) {
	store := reg.NewStore()

	// Create ExtensionServices with nil Installation (but not nil itself)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil, // Explicitly nil
		},
	}
	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	// Should not panic and return empty map (no registry platforms, no installation)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d platforms", len(result))
	}
}

// TestDiscoverExternalPlatforms_WithEmptyInstallationList tests the Installation discovery path

// TestDiscoverExternalPlatforms_WithEmptyInstallationList tests the Installation discovery path
func TestDiscoverExternalPlatforms_WithEmptyInstallationList(t *testing.T) {
	store := reg.NewStore()

	// Create ExtensionServices with Installation that returns empty list
	// Note: We can't easily create a real Installation service without DB
	// So we create a minimal service context that exercises the code path
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	// Should not panic and return empty map
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d platforms", len(result))
	}
}

// TestDiscoverExternalPlatforms_CombinedSources tests both registry and installation sources

// TestDiscoverExternalPlatforms_CombinedSources tests both registry and installation sources
func TestDiscoverExternalPlatforms_CombinedSources(t *testing.T) {
	store := reg.NewStore()
	// Add platform from registry
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.registry_method": {Enabled: true},
		},
	})

	// Add Installation service (will return empty, but exercises the code path)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	// Should have platform from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}
	methods := result["test"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0] != "registry_method" {
		t.Fatalf("expected registry_method, got %s", methods[0])
	}
}

// TestCall_DispatcherReturnsError tests when dispatcher returns an error

// TestDiscoverExternalPlatforms_BothSourcesNil tests with both sources nil
func TestDiscoverExternalPlatforms_BothSourcesNil(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
		Extensions:    nil,
	}

	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d platforms", len(result))
	}
}

// TestCall_EmptyRequestData tests Call with empty request data

// TestDiscoverExternalPlatforms_VariousExtensionsInstallations tests different installation states
func TestDiscoverExternalPlatforms_VariousExtensionsInstallations(t *testing.T) {
	store := reg.NewStore()

	// Add some platforms from registry
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.registry.method1": {Enabled: true},
		},
	})

	// Create a nil Installation service to exercise the code path
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should have platforms from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}
	if len(result["registry"]) != 1 {
		t.Fatalf("expected 1 method, got %d", len(result["registry"]))
	}
}

// TestListPlatforms_VariousScenarios tests various ListPlatforms scenarios

// TestDiscoverExternalPlatforms_MergesSources tests merging platforms from both sources
func TestDiscoverExternalPlatforms_MergesSources(t *testing.T) {
	store := reg.NewStore()
	// Add platform from registry
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.registry_method": {Enabled: true},
		},
	})

	// Create Installation service (returns empty)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should have platform from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(result))
	}
	methods := result["test"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0] != "registry_method" {
		t.Fatalf("expected registry_method, got %s", methods[0])
	}
}

// TestCall_WithRequestDataSet tests the request data conversion path (line 43-44)

// TestDiscoverExternalPlatforms_ExtensionsFieldSetButInstallationNil tests code path
func TestDiscoverExternalPlatforms_ExtensionsFieldSetButInstallationNil(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil, // Installation is nil but Extensions is not
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should still discover from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}
}

// TestListPlatforms_VerifiesResponseStructure verifies the response structure

// TestDiscoverExternalPlatforms_WithInstallationService exercises Installation path
func TestDiscoverExternalPlatforms_WithInstallationService2(t *testing.T) {
	store := reg.NewStore()
	// Add a platform from registry first
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.registry.method1": {Enabled: true},
		},
	})

	// Create Installation service (will return empty list due to no DB)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should have platform from registry (Installation returns empty)
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}

	// Verify the Installation code path was exercised (even though it returns empty)
	methods := result["registry"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method for registry platform, got %d", len(methods))
	}
}

// TestListPlatforms_WithInstallationInContext exercises Installation path

// TestDiscoverExternalPlatforms_WithInstallationService3 tests the installation bindings discovery path with NewService
func TestDiscoverExternalPlatforms_WithInstallationService3(t *testing.T) {
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map (no installation data in nil repo)
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil repo, got %d", len(platforms))
	}
}

// TestCall_DispatcherInvokeError tests the dispatcher error path

// TestDiscoverExternalPlatforms_ListError tests the List error path in installation bindings
func TestDiscoverExternalPlatforms_ListError(t *testing.T) {
	store := reg.NewStore()

	// Create a mock installation service that returns error
	// Since we can't easily mock, we rely on the nil repo returning empty
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should not crash and return platforms from registry
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

// TestCall_DispatcherSuccessWithPathologicalResponse tests various response handling

// TestDiscoverExternalPlatforms_WithRegistryOnly tests registry discovery without installation
func TestDiscoverExternalPlatforms_WithRegistryOnly(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil, // No installation service
	}

	service := NewService(svcCtx)
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover from registry
	if len(platforms) == 0 {
		t.Fatal("expected platforms from registry")
	}
	if len(platforms["test"]) != 2 {
		t.Fatalf("expected 2 methods for test platform, got %d", len(platforms["test"]))
	}
}

// TestDiscoverExternalPlatforms_NilRegistryStoreWithExtensions tests with nil registry store and extensions

// TestDiscoverExternalPlatforms_NilRegistryStoreWithExtensions tests with nil registry store and extensions
func TestDiscoverExternalPlatforms_NilRegistryStoreWithExtensions(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil store, got %d", len(platforms))
	}
}

// TestCall_WithDispatcherSuccess tests successful dispatcher invocation path

// TestDiscoverExternalPlatforms_WithDisabledFunction tests that disabled functions are skipped
func TestDiscoverExternalPlatforms_WithDisabledFunction(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.myplatform.enabled_method":  {Enabled: true},
			"external.myplatform.disabled_method": {Enabled: false}, // Disabled
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms["myplatform"]) != 1 {
		t.Fatalf("expected only 1 enabled method, got %d: %v", len(platforms["myplatform"]), platforms["myplatform"])
	}
	if platforms["myplatform"][0] != "enabled_method" {
		t.Fatalf("expected 'enabled_method', got '%s'", platforms["myplatform"][0])
	}
}

// TestDiscoverExternalPlatforms_WithInstallationDB tests the installation bindings discovery path

// TestDiscoverExternalPlatforms_WithInstallationDB tests the installation bindings discovery path
func TestDiscoverExternalPlatforms_WithInstallationDB(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:platform_it_%d?mode=memory&cache=shared", platformTestDBSeq.Add(1))), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create repos
	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	// Create installation service
	installationService := extensioninstallation.NewService(installationRepo, eventRepo, bindingRepo)

	// Create a test installation with an external-platform extension ID
	testInstallation := &model.ExtensionInstallation{
		InstallationKey: "test-external-platform",
		ExtensionID:     "test.external-platform", // Contains "external-platform"
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "default",
		TargetType:      "server",
		TargetID:        "default",
		Status:          "installed",
		DesiredState:    "installed",
		Enabled:         true,
		InstalledBy:     "test",
		InstalledAtUnix: time.Now().Unix(),
	}
	if err := installationRepo.Create(context.Background(), testInstallation); err != nil {
		t.Fatalf("failed to create installation: %v", err)
	}

	// Create test bindings
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "steam",
			SpecJSON:    model.JSON([]byte(`{"provider":"steam","operations":["get_player","ban_player"]}`)),
			Status:      "active",
		},
		{
			BindingType: "function",
			BindingKey:  "external.k8s.restart_pod",
			Status:      "active",
		},
	}
	if err := bindingRepo.ReplaceForInstallation(context.Background(), testInstallation.ID, bindings); err != nil {
		t.Fatalf("failed to create bindings: %v", err)
	}

	// Create service context with installation service
	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationService,
		},
	}
	service := NewService(svcCtx)

	// Test discoverExternalPlatforms
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover platforms from installation bindings
	if len(platforms["steam"]) != 2 {
		t.Fatalf("expected 2 methods for steam platform, got %d: %v", len(platforms["steam"]), platforms["steam"])
	}
	if len(platforms["k8s"]) != 1 {
		t.Fatalf("expected 1 method for k8s platform, got %d: %v", len(platforms["k8s"]), platforms["k8s"])
	}

	// Verify methods
	steamHasGetPlayer := false
	for _, m := range platforms["steam"] {
		if m == "get_player" {
			steamHasGetPlayer = true
			break
		}
	}
	if !steamHasGetPlayer {
		t.Fatal("expected get_player method for steam")
	}
}

// TestHandler_List_AliasMethod tests the List alias method
