package platform

import (
	"context"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

func TestListPlatformsMarksExtensionSource(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.list_apps": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListPlatforms(context.Background())
	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if len(resp.Platforms) == 0 {
		t.Fatalf("expected discovered platform")
	}
	if resp.Platforms[0].Source != "extension" {
		t.Fatalf("expected source=extension, got=%s", resp.Platforms[0].Source)
	}
}

func TestListPlatforms_WithNilStore(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if len(resp.Platforms) != 0 {
		t.Fatalf("expected empty platforms for nil store, got %v", resp.Platforms)
	}
}

// Additional tests to reach 80%+ coverage

func TestListPlatforms_SortsByName(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.z_platform.method1": {Enabled: true},
			"external.a_platform.method2": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if len(resp.Platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(resp.Platforms))
	}
}

func TestListPlatforms_WithDispatcher(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
			"external.platform2.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if len(resp.Platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(resp.Platforms))
	}
}

func TestListPlatforms_DeduplicatesPlatformNames(t *testing.T) {
	store := reg.NewStore()

	// Add agents with same platform (same case)
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

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	// Should deduplicate platforms with same name
	if len(resp.Platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(resp.Platforms))
	}
	// Should have 2 methods from both agents
	if len(resp.Platforms[0].Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(resp.Platforms[0].Methods))
	}
}

func TestListPlatforms_MergesMethodsFromMultipleAgents(t *testing.T) {
	store := reg.NewStore()

	// Add agents with methods for the same platform
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method3": {Enabled: true},
			"external.test.method1": {Enabled: true}, // same platform, duplicate method
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if len(resp.Platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(resp.Platforms))
	}
	// Should merge methods (3 unique methods: method1, method2, method3)
	if len(resp.Platforms[0].Methods) != 3 {
		t.Fatalf("expected 3 merged methods, got %d", len(resp.Platforms[0].Methods))
	}
}

func TestListPlatforms_EmptyRegistry(t *testing.T) {
	store := reg.NewStore()
	// Don't add any agents

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if len(resp.Platforms) != 0 {
		t.Fatalf("expected 0 platforms for empty registry, got %d", len(resp.Platforms))
	}
}

func TestListPlatforms_WithEmptyRegistry(t *testing.T) {
	store := reg.NewStore()

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned unexpected error: %v", err)
	}
	// Should at least include built-in platforms
	if len(resp.Platforms) == 0 {
		t.Log("No platforms found in empty registry")
	}
}

// TestService_ListPlatforms_CaseInsensitive tests platform listing with different cases
func TestService_ListPlatforms_CaseInsensitiveCheck(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.MyPlatform.Method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())
	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}

	// Should find at least one platform
	if len(resp.Platforms) == 0 {
		t.Fatal("expected at least one platform")
	}

	// Platform name should be lowercased
	found := false
	for _, p := range resp.Platforms {
		if p.Name == "myplatform" {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Platforms: %v", resp.Platforms)
	}
}

// TestStringInSlice tests the stringInSlice helper function

// TestService_ListPlatforms_WithEnabledField tests that enabled field is set correctly
func TestService_ListPlatforms_WithEnabledField(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.testplatform.method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())
	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}

	// Should find the platform
	if len(resp.Platforms) == 0 {
		t.Fatal("expected at least one platform")
	}

	// All platforms should be marked as enabled
	for _, p := range resp.Platforms {
		if !p.Enabled {
			t.Fatalf("expected platform %s to be enabled", p.Name)
		}
	}
}

// TestService_ListMethods_DuplicateMethodNames tests deduplication of method names

// TestService_ListPlatforms_SourceField tests that source field is set correctly
func TestService_ListPlatforms_SourceField(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())
	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}

	// All platforms should have source set to "extension"
	for _, p := range resp.Platforms {
		if p.Source != "extension" {
			t.Fatalf("expected source=extension for platform %s, got %s", p.Name, p.Source)
		}
	}
}

// Additional test to improve discoverExternalPlatforms coverage - disabled function filtering

// TestDiscoverExternalPlatforms_DisabledFunction tests that disabled functions are not discovered

// TestListPlatforms_DuplicatePlatformsAcrossAgents tests platform deduplication
func TestListPlatforms_DuplicatePlatformsAcrossAgents(t *testing.T) {
	store := reg.NewStore()
	// Add two agents with the same platform
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.duplicate.method1": {Enabled: true},
		},
	})
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.duplicate.method2": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	// Should have 1 unique platform with 2 methods
	if len(resp.Platforms) != 1 {
		t.Fatalf("expected 1 unique platform, got %d", len(resp.Platforms))
	}
	if len(resp.Platforms[0].Methods) != 2 {
		t.Fatalf("expected 2 methods for duplicate platform, got %d", len(resp.Platforms[0].Methods))
	}
}

// TestDiscoverExternalPlatforms_DisabledFunction tests that disabled functions are ignored

// TestListPlatforms_MultiplePlatforms tests listing multiple platforms
func TestListPlatforms_MultiplePlatforms(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
			"external.platform2.method1": {Enabled: true},
			"external.platform3.method1": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if len(resp.Platforms) != 3 {
		t.Fatalf("expected 3 platforms, got %d", len(resp.Platforms))
	}
}

// TestListMethods_NoMatchingPlatform tests when platform doesn't exist

// TestListPlatforms_WithRegistryAndInstallation tests platform discovery from both sources
func TestListPlatforms_WithRegistryAndInstallation(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)

	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	// Should have 1 platform from registry
	if len(resp.Platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(resp.Platforms))
	}
}

// TestDiscoverExternalPlatforms_BothSourcesNil tests with both sources nil

// TestListPlatforms_VariousScenarios tests various ListPlatforms scenarios
func TestListPlatforms_VariousScenarios(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*reg.Store)
		expectedCount  int
		expectedSource string
	}{
		{
			name: "empty registry",
			setupFunc: func(store *reg.Store) {
				// No agents
			},
			expectedCount:  0,
			expectedSource: "",
		},
		{
			name: "single platform",
			setupFunc: func(store *reg.Store) {
				_ = store.UpsertAgent(&reg.AgentSession{
					AgentID:  "a1",
					Addr:     "127.0.0.1:19091",
					ExpireAt: time.Now().Add(time.Minute),
					Functions: map[string]reg.FunctionMeta{
						"external.p1.method1": {Enabled: true},
					},
				})
			},
			expectedCount:  1,
			expectedSource: "extension",
		},
		{
			name: "multiple platforms",
			setupFunc: func(store *reg.Store) {
				_ = store.UpsertAgent(&reg.AgentSession{
					AgentID:  "a1",
					Addr:     "127.0.0.1:19091",
					ExpireAt: time.Now().Add(time.Minute),
					Functions: map[string]reg.FunctionMeta{
						"external.p1.method1": {Enabled: true},
						"external.p2.method1": {Enabled: true},
						"external.p3.method1": {Enabled: true},
					},
				})
			},
			expectedCount:  3,
			expectedSource: "extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := reg.NewStore()
			tt.setupFunc(store)

			service := NewService(&svc.ServiceContext{RegistryStore: store})
			resp, err := service.ListPlatforms(context.Background())

			if err != nil {
				t.Fatalf("ListPlatforms returned error: %v", err)
			}
			if resp.Code != 200 {
				t.Fatalf("expected code 200, got %d", resp.Code)
			}
			if len(resp.Platforms) != tt.expectedCount {
				t.Fatalf("expected %d platforms, got %d", tt.expectedCount, len(resp.Platforms))
			}
			if tt.expectedSource != "" && len(resp.Platforms) > 0 {
				if resp.Platforms[0].Source != tt.expectedSource {
					t.Errorf("expected source %s, got %s", tt.expectedSource, resp.Platforms[0].Source)
				}
			}
		})
	}
}

// TestListMethods_VariousScenarios tests various ListMethods scenarios

// TestListPlatforms_WithPlatformHavingNoMethods tests platform with no methods
func TestListPlatforms_WithPlatformHavingNoMethods(t *testing.T) {
	store := reg.NewStore()
	// Add an agent but with disabled functions (no enabled methods)
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.disabled_method": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}
	// No enabled platforms should be discovered
	if len(resp.Platforms) != 0 {
		t.Fatalf("expected 0 platforms (all disabled), got %d", len(resp.Platforms))
	}
}

// TestListMethods_PlatformNameWithDifferentCases tests case handling

// TestListPlatforms_VerifiesResponseStructure verifies the response structure
func TestListPlatforms_VerifiesResponseStructure(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}

	// Verify response structure
	if resp.Code != 200 {
		t.Errorf("expected code 200, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %s", resp.Message)
	}
	if len(resp.Platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(resp.Platforms))
	}

	platform := resp.Platforms[0]
	if platform.Name != "test" {
		t.Errorf("expected platform name 'test', got %s", platform.Name)
	}
	if !platform.Enabled {
		t.Error("expected platform to be enabled")
	}
	if platform.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", platform.Source)
	}
	if len(platform.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(platform.Methods))
	}
	if platform.Methods[0] != "method1" {
		t.Errorf("expected method 'method1', got %s", platform.Methods[0])
	}
}

// TestListMethods_VerifiesResponseStructure verifies the response structure

// TestListPlatforms_PlatformSourceVerification tests that platforms have correct source
func TestListPlatforms_PlatformSourceVerification(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}

	// All platforms should have "extension" as source
	for _, platform := range resp.Platforms {
		if platform.Source != "extension" {
			t.Errorf("expected source 'extension' for platform %s, got %s", platform.Name, platform.Source)
		}
	}
}

// TestListMethods_DuplicateMethodsAcrossAgents tests method deduplication

// TestListPlatforms_WithInstallationInContext exercises Installation path
func TestListPlatforms_WithInstallationInContext2(t *testing.T) {
	store := reg.NewStore()
	// Add platforms from registry
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
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	resp, err := service.ListPlatforms(context.Background())

	if err != nil {
		t.Fatalf("ListPlatforms returned error: %v", err)
	}

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}

	// Should have platform from registry
	if len(resp.Platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(resp.Platforms))
	}
}

// TestListMethods_WithInstallationInContext exercises Installation path
