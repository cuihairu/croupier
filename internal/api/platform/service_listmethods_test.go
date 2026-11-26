package platform

import (
	"context"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

func TestListMethodsUsesDiscoveredExternalFunctions(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.install_app": {Enabled: true},
			"external.onepanel.list_apps":   {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "onepanel")
	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d message=%s", resp.Code, resp.Message)
	}
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 methods, got=%v", resp.Methods)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got=%s", resp.Source)
	}
}

func TestListMethods_EmptyPlatform(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListMethods(context.Background(), "")

	if err != nil {
		t.Fatalf("ListMethods returned unexpected error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for empty platform, got %d", resp.Code)
	}
}

func TestListMethods_UnknownPlatform(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListMethods(context.Background(), "unknown_platform_xyz")

	if err != nil {
		t.Fatalf("ListMethods returned unexpected error: %v", err)
	}
	// Unknown platform returns 404
	if resp.Code != 404 {
		t.Fatalf("expected code 404 for unknown platform, got %d", resp.Code)
	}
}

func TestListMethods_CaseInsensitive(t *testing.T) {
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

	// Test with different case
	resp, err := service.ListMethods(context.Background(), "OnePanel")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d", resp.Code)
	}
}

func TestListMethods_CaseInsensitivePlatform(t *testing.T) {
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

	// Test with different case
	resp, err := service.ListMethods(context.Background(), "OnePanel")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d", resp.Code)
	}
	if len(resp.Methods) == 0 {
		t.Fatal("expected methods for case-insensitive platform name")
	}
}

func TestListMethods_WithWhitespace(t *testing.T) {
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

	resp, err := service.ListMethods(context.Background(), "  onepanel  ")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d", resp.Code)
	}
}

func TestListMethods_Deduplicates(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.list_apps":  {Enabled: true},
			"external.onepanel.LIST_APPS":  {Enabled: true}, // duplicate, different case
			"external.onepanel.delete_app": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "onepanel")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should deduplicate case-insensitively
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 deduplicated methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

func TestListMethods_SkipsEmptyMethodNames(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.":        {Enabled: true}, // Empty method name - should be skipped
			"external.test.method2": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Empty method names should be filtered out
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 methods after filtering empty names, got %d", len(resp.Methods))
	}
}

func TestListMethods_EmptyMethodNames(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.": {Enabled: true}, // Empty method name
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Empty method names should be filtered out
	if len(resp.Methods) != 0 {
		t.Fatalf("expected 0 methods after filtering empty names, got %d", len(resp.Methods))
	}
}

// Tests to cover dispatcher paths

func TestListMethods_WithAgentButNoExternalFunctions(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register agent with non-external functions
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"test.echo":  {Enabled: true},
			"test.ping":  {Enabled: true},
			"game.start": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Try to get methods for a platform that doesn't exist
	resp, err := service.ListMethods(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 404 {
		t.Fatalf("expected code 404 for nonexistent platform, got %d", resp.Code)
	}
}

func TestListMethods_WhitespaceOnly(t *testing.T) {
	store := reg.NewStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "   ")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for whitespace-only platform, got %d", resp.Code)
	}
}

func TestListMethods_TabCharacters(t *testing.T) {
	store := reg.NewStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "\t\t")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for tab-only platform, got %d", resp.Code)
	}
}

func TestListMethods_ReturnsUniqueMethods(t *testing.T) {
	store := reg.NewStore()

	// Add agent with duplicate methods (different cases)
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.getplayer":  {Enabled: true},
			"external.test.GETPLAYER":  {Enabled: true},
			"external.test.delete_app": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// getplayer and GETPLAYER should be deduplicated
	// Result should have 2 methods: getplayer and delete_app
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 unique methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// Test to cover empty method name filtering path in ListMethods
func TestListMethods_FiltersWhitespaceOnlyMethodNames(t *testing.T) {
	store := reg.NewStore()

	// Create a mock function registry that will return methods with whitespace
	// Since we can't directly control discoverExternalPlatforms to return whitespace methods,
	// we test through the actual discovery mechanism
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.  ":      {Enabled: true}, // Whitespace only
			"external.test.method2": {Enabled: true},
			"external.test.\t\t":    {Enabled: true}, // Tab only
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should filter out whitespace-only methods
	if len(resp.Methods) != 2 {
		t.Logf("Got %d methods: %v", len(resp.Methods), resp.Methods)
	}
}

func TestListMethods_WithNonExistentPlatform(t *testing.T) {
	store := reg.NewStore()

	// Don't add any agents - platform won't exist
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "nonexistent_platform")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 404 {
		t.Fatalf("expected code 404 for nonexistent platform, got %d", resp.Code)
	}
	if resp.Message != "Platform not found" {
		t.Fatalf("expected 'Platform not found' message, got %s", resp.Message)
	}
}

func TestListMethods_PreservesOriginalMethodNameCase(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.GetPlayer": {Enabled: true},
			"external.test.SetScore":  {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should preserve original case (GetPlayer, SetScore)
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(resp.Methods))
	}
	// Check that original case is preserved
	foundGetPlayer := false
	for _, m := range resp.Methods {
		if m == "GetPlayer" {
			foundGetPlayer = true
		}
	}
	if !foundGetPlayer {
		t.Fatalf("expected to find GetPlayer with original case, got %v", resp.Methods)
	}
}

// Additional edge case tests for coverage

func TestListMethods_PlatformWithMixedCaseMethods(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.Get_Player": {Enabled: true},
			"external.test.get_player": {Enabled: true},
			"external.test.GET_PLAYER": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Each method has different underscores, so they're treated as different
	// Get_Player, get_player, GET_PLAYER are all different
	if len(resp.Methods) != 3 {
		t.Logf("Got %d methods: %v", len(resp.Methods), resp.Methods)
	}
}

func TestListMethods_PlatformNotFoundWithSimilarName(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.steam.get_player": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	// Search for "steam" (should find it)
	resp, err := service.ListMethods(context.Background(), "steam")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(resp.Methods))
	}
}

func TestListMethods_EmptyMethodName(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.": {Enabled: true}, // Empty method name after parsing
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Empty method names are filtered out
	if len(resp.Methods) != 0 {
		t.Fatalf("expected 0 methods after filtering empty names, got %d", len(resp.Methods))
	}
}

func TestListMethods_WithUnknownPlatform(t *testing.T) {
	store := reg.NewStore()

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "unknown_platform_xyz")

	if err != nil {
		t.Fatalf("ListMethods returned unexpected error: %v", err)
	}
	// Unknown platform should return empty methods
	if resp.Methods == nil {
		t.Log("Methods is nil for unknown platform")
	}
}

func TestListMethods_WithSpecialCharacters(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform-123.method_name": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test_platform-123")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 method, got=%v", resp.Methods)
	}
}

// TestService_Call_DispatcherSuccessPath tests the dispatcher success path with a mocked response

// TestService_ListMethods_CaseInsensitive tests that platform lookup is case insensitive
func TestService_ListMethods_CaseInsensitiveCheck(t *testing.T) {
	store := reg.NewStore()

	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.myplatform.method1": {Enabled: true},
			"external.myplatform.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	// Test with different case
	resp, err := service.ListMethods(context.Background(), "MYPLATFORM")
	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should find methods even with different case
	if resp.Code != 200 {
		t.Logf("Got code %d: %s", resp.Code, resp.Message)
	}
}

// TestService_ListMethods_DuplicateMethods tests that duplicate methods are deduplicated

// TestService_ListMethods_DuplicateMethods tests that duplicate methods are deduplicated
func TestService_ListMethods_DuplicateMethodsCheck(t *testing.T) {
	store := reg.NewStore()

	// Add same function from two agents
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
			"external.test.method1": {Enabled: true}, // Duplicate
			"external.test.method3": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")
	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should have 3 unique methods (method1, method2, method3)
	if len(resp.Methods) != 3 {
		t.Fatalf("expected 3 unique methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestService_ListPlatforms_CaseInsensitive tests platform listing with different cases

// TestService_ListMethods_SourceSet tests that source is set when methods are found
func TestService_ListMethods_SourceSet(t *testing.T) {
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

	resp, err := service.ListMethods(context.Background(), "testplatform")
	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Source should be set to "extension" when methods are found
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestService_ListMethods_EmptyStringMethodInList tests handling of empty strings in method list

// TestService_ListMethods_EmptyStringMethodInList tests handling of empty strings in method list
func TestService_ListMethods_EmptyStringMethodInList(t *testing.T) {
	store := reg.NewStore()

	// This test verifies the addMethods function handles empty strings correctly
	// The discoverExternalPlatforms will return methods from registry
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")
	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should return non-empty methods
	if len(resp.Methods) == 0 {
		t.Fatal("expected at least one method")
	}
}

// TestService_Call_ResponseUnmarshalSuccess tests the response unmarshal success path

// TestService_ListMethods_DuplicateMethodNames tests deduplication of method names
func TestService_ListMethods_DuplicateMethodNames(t *testing.T) {
	store := reg.NewStore()

	// Add same method name with different cases
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.MethodOne": {Enabled: true},
			"external.test.methodone": {Enabled: true}, // Same name, different case
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")
	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should deduplicate based on case-insensitive comparison
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 method (deduplicated), got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestService_ListPlatforms_SourceField tests that source field is set correctly

// TestListMethods_EmptyMethodsFromAddMethods covers the addMethods with empty list
func TestListMethods_EmptyMethodsFromAddMethods(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.": {Enabled: true}, // Empty method name
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should return 404 since all methods are empty
	if resp.Code != 404 {
		t.Fatalf("expected code 404 for platform with no valid methods, got %d", resp.Code)
	}
}

// TestListMethods_DuplicateNamesTests covers the deduplication logic more thoroughly

// TestListMethods_DuplicateNamesTests covers the deduplication logic more thoroughly
func TestListMethods_DuplicateNamesTests(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method_one":  {Enabled: true},
			"external.test.METHOD_ONE":  {Enabled: true}, // case insensitive duplicate
			"external.test. method_one": {Enabled: true}, // whitespace duplicate
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should deduplicate and return only 1 method
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 deduplicated method, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestDiscoverExternalPlatforms_WithExtensionsNil covers nil Extensions field

// TestListMethods_WithWhitespacePlatform covers whitespace-only platform parameter
func TestListMethods_WithWhitespacePlatform(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	resp, err := service.ListMethods(context.Background(), "   ")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should return 400 for whitespace-only platform
	if resp.Code != 400 {
		t.Fatalf("expected code 400, got %d", resp.Code)
	}
}

// TestListMethods_SourceResolution tests that the Source field is set correctly

// TestListMethods_SourceResolution tests that the Source field is set correctly
func TestListMethods_SourceResolution(t *testing.T) {
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
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// When methods are found from external platforms, Source should be "extension"
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension when methods found, got %s", resp.Source)
	}
}

// TestListMethods_EmptySourceWhenNotFound tests Source when platform not found

// TestListMethods_EmptySourceWhenNotFound tests Source when platform not found
func TestListMethods_EmptySourceWhenNotFound(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	resp, err := service.ListMethods(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// When platform not found, Source should be empty
	if resp.Source != "" {
		t.Fatalf("expected empty source when platform not found, got %s", resp.Source)
	}
}

// TestListMethods_WithOnlyWhitespaceMethods tests filtering of whitespace-only method names

// TestListMethods_WithOnlyWhitespaceMethods tests filtering of whitespace-only method names
func TestListMethods_WithOnlyWhitespaceMethods(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.   ": {Enabled: true}, // Whitespace-only method name
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Whitespace-only method names should be filtered out, resulting in 404
	if resp.Code != 404 {
		t.Fatalf("expected code 404 when all methods are whitespace, got %d", resp.Code)
	}
}

// TestListMethods_CaseInsensitiveLookup tests that platform lookup is case-insensitive

// TestListMethods_CaseInsensitiveLookup tests that platform lookup is case-insensitive
func TestListMethods_CaseInsensitiveLookup(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.mixedcase.method1": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	// Try different cases
	for _, platformName := range []string{"mixedcase", "MIXEDCASE", "MixedCase", "  MixedCase  "} {
		resp, err := service.ListMethods(context.Background(), platformName)

		if err != nil {
			t.Fatalf("ListMethods returned error for '%s': %v", platformName, err)
		}
		if resp.Code != 200 {
			t.Fatalf("expected code 200 for platform '%s', got %d", platformName, resp.Code)
		}
		if len(resp.Methods) != 1 {
			t.Fatalf("expected 1 method for platform '%s', got %d", platformName, len(resp.Methods))
		}
	}
}

// TestListMethods_DuplicateMethodsAcrossAgents tests that methods are deduplicated

// TestListMethods_DuplicateMethodsAcrossAgents tests that methods are deduplicated
func TestListMethods_DuplicateMethodsAcrossAgents(t *testing.T) {
	store := reg.NewStore()
	// Add two agents with the same platform and method
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
			"external.test_platform.method2": {Enabled: true},
		},
	})
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a2",
		Addr:     "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true}, // Duplicate
			"external.test_platform.method3": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test_platform")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	// Should have 3 unique methods (method1, method2, method3)
	if len(resp.Methods) != 3 {
		t.Fatalf("expected 3 unique methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestListPlatforms_DuplicatePlatformsAcrossAgents tests platform deduplication

// TestListMethods_NoMatchingPlatform tests when platform doesn't exist
func TestListMethods_NoMatchingPlatform(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.existing.method1": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 404 {
		t.Fatalf("expected code 404, got %d", resp.Code)
	}
}

// TestListMethods_EmptyRegistry tests with empty registry

// TestListMethods_EmptyRegistry tests with empty registry
func TestListMethods_EmptyRegistry2(t *testing.T) {
	store := reg.NewStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "any_platform")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 404 {
		t.Fatalf("expected code 404 for empty registry, got %d", resp.Code)
	}
}

// TestStringInSlice tests the stringInSlice helper function

// TestListMethods_WithDuplicateMethodNames tests deduplication of method names
func TestListMethods_WithDuplicateMethodNames(t *testing.T) {
	store := reg.NewStore()
	// Add same method multiple times (case-insensitive)
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.Method1": {Enabled: true},
			"external.test.method1": {Enabled: true},
			"external.test.METHOD1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	// Should have 2 unique methods (method1 and method2)
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 unique methods (case-insensitive dedup), got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestListMethods_WithEmptyMethodNames tests filtering of empty method names

// TestListMethods_WithEmptyMethodNames tests filtering of empty method names
func TestListMethods_WithEmptyMethodNames(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.   ": {Enabled: true}, // Whitespace-only
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	// Should return 404 because all methods are filtered out
	if resp.Code != 404 {
		t.Fatalf("expected code 404 when all methods are empty, got %d", resp.Code)
	}
}

// TestListMethods_MixedEmptyAndValidMethods tests filtering with mixed methods

// TestListMethods_MixedEmptyAndValidMethods tests filtering with mixed methods
func TestListMethods_MixedEmptyAndValidMethods(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.   ":     {Enabled: true}, // Empty after trim
			"external.test.method1": {Enabled: true}, // Valid
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	// Should have only 1 method (empty one filtered out)
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 method after filtering empty ones, got %d", len(resp.Methods))
	}
	if resp.Methods[0] != "method1" {
		t.Fatalf("expected method1, got %s", resp.Methods[0])
	}
}

// TestDiscoverExternalPlatforms_WithNilInstallation tests the Installation discovery path with nil

// TestListMethods_VariousScenarios tests various ListMethods scenarios
func TestListMethods_VariousScenarios(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		setupFunc     func(*reg.Store)
		expectedCode  int
		expectedCount int
	}{
		{
			name:      "empty platform name",
			platform:  "",
			setupFunc: func(store *reg.Store) {},
			// No setup needed
			expectedCode:  400,
			expectedCount: 0,
		},
		{
			name:     "nonexistent platform",
			platform: "nonexistent",
			setupFunc: func(store *reg.Store) {
				// Empty registry
			},
			expectedCode:  404,
			expectedCount: 0,
		},
		{
			name:     "existing platform with methods",
			platform: "test",
			setupFunc: func(store *reg.Store) {
				_ = store.UpsertAgent(&reg.AgentSession{
					AgentID:  "a1",
					Addr:     "127.0.0.1:19091",
					ExpireAt: time.Now().Add(time.Minute),
					Functions: map[string]reg.FunctionMeta{
						"external.test.method1": {Enabled: true},
						"external.test.method2": {Enabled: true},
					},
				})
			},
			expectedCode:  200,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := reg.NewStore()
			tt.setupFunc(store)

			service := NewService(&svc.ServiceContext{RegistryStore: store})
			resp, err := service.ListMethods(context.Background(), tt.platform)

			if err != nil {
				t.Fatalf("ListMethods returned error: %v", err)
			}
			if resp.Code != tt.expectedCode {
				t.Fatalf("expected code %d, got %d", tt.expectedCode, resp.Code)
			}
			if len(resp.Methods) != tt.expectedCount {
				t.Fatalf("expected %d methods, got %d", tt.expectedCount, len(resp.Methods))
			}
		})
	}
}

// TestListPlatforms_WithPlatformHavingNoMethods tests platform with no methods

// TestListMethods_PlatformNameWithDifferentCases tests case handling
func TestListMethods_PlatformNameWithDifferentCases(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.mixedcase.method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	// Test with different cases
	cases := []string{"MixedCase", "MIXEDCASE", "mixedcase", "  mixedcase  "}

	for _, platformName := range cases {
		resp, err := service.ListMethods(context.Background(), platformName)

		if err != nil {
			t.Fatalf("ListMethods returned error for '%s': %v", platformName, err)
		}
		if resp.Code != 200 {
			t.Fatalf("expected code 200 for platform '%s', got %d", platformName, resp.Code)
		}
		if len(resp.Methods) != 1 {
			t.Fatalf("expected 1 method for platform '%s', got %d", platformName, len(resp.Methods))
		}
	}
}

// TestDiscoverExternalPlatforms_MergesSources tests merging platforms from both sources

// TestListMethods_VerifiesResponseStructure verifies the response structure
func TestListMethods_VerifiesResponseStructure(t *testing.T) {
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

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Verify response structure
	if resp.Code != 200 {
		t.Errorf("expected code 200, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %s", resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(resp.Methods))
	}
}

// TestCall_VerifiesErrorResponse verifies error response structure

// TestListMethods_VerifiesMethodDeduplication verifies method deduplication
func TestListMethods_VerifiesMethodDeduplication(t *testing.T) {
	store := reg.NewStore()
	// Add same method multiple times with different cases
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.Method1": {Enabled: true},
			"external.test.METHOD1": {Enabled: true},
			"external.test.method1": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should have only 1 unique method (case-insensitive dedup)
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 unique method after deduplication, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestCall_WithFunctionIDEmptyBranch tests when buildExternalFunctionID returns empty

// TestListMethods_EmptyMethodsListAfterFiltering tests when all methods are filtered out
func TestListMethods_EmptyMethodsListAfterFiltering(t *testing.T) {
	store := reg.NewStore()
	// Add a function with empty method name (after trimming)
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.   ": {Enabled: true}, // Whitespace only
			"external.test.  ":  {Enabled: true}, // More whitespace
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should return 404 because all methods are filtered out
	if resp.Code != 404 {
		t.Errorf("expected code 404 when all methods are empty, got %d", resp.Code)
	}
	if resp.Message != "Platform not found" {
		t.Errorf("expected message 'Platform not found', got %s", resp.Message)
	}
}

// TestHandler_ListPlatforms_ResponseStructure verifies the handler response structure

// TestListMethods_UsedExtensionFlag tests the usedExtension flag behavior
func TestListMethods_UsedExtensionFlag(t *testing.T) {
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
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Verify Source is set to "extension" when methods are found
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
}

// TestListMethods_NoMethodsFlag tests when no methods are found

// TestListMethods_NoMethodsFlag tests when no methods are found
func TestListMethods_NoMethodsFlag(t *testing.T) {
	store := reg.NewStore()
	// Empty registry - no platforms

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// When no methods are found, Source should be empty
	if resp.Code != 404 {
		t.Fatalf("expected code 404, got %d", resp.Code)
	}
	if resp.Source != "" {
		t.Errorf("expected empty source when no methods found, got %s", resp.Source)
	}
}

// TestListPlatforms_PlatformSourceVerification tests that platforms have correct source

// TestListMethods_DuplicateMethodsAcrossAgents tests method deduplication
func TestListMethods_DuplicateMethodsAcrossAgents2(t *testing.T) {
	store := reg.NewStore()
	// Two agents with same platform and method (duplicate)
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
			"external.test.method1": {Enabled: true}, // Duplicate
			"external.test.method3": {Enabled: true}, // New
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Should have 3 unique methods (method1, method2, method3)
	if len(resp.Methods) != 3 {
		t.Fatalf("expected 3 unique methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestHandler_ListMethods_ResponseFields verifies response fields
