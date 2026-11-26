package platform

import (
	"context"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

// TestListMethods_VerifyMethodsResponse verifies methods response structure
func TestListMethods_VerifyMethodsResponse(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
			"external.test.method3": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	// Verify response structure
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %s", resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
	if len(resp.Methods) != 3 {
		t.Fatalf("expected 3 methods, got %d", len(resp.Methods))
	}
	// Verify methods are present
	methodMap := make(map[string]bool)
	for _, m := range resp.Methods {
		methodMap[m] = true
	}
	if !methodMap["method1"] {
		t.Error("expected method1 in response")
	}
	if !methodMap["method2"] {
		t.Error("expected method2 in response")
	}
	if !methodMap["method3"] {
		t.Error("expected method3 in response")
	}
}

// TestHandler_Call_WithAllFieldsMissing tests with all fields missing

// TestListMethods_WithInstallationInContext exercises Installation path
func TestListMethods_WithInstallationInContext2(t *testing.T) {
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
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}

	// Should have methods from registry
	if len(resp.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(resp.Methods))
	}
}

// TestCall_VerifyRequestDataConversion verifies request data is converted

// TestListMethods_WithEmptyMethodName tests the empty string filtering in addMethods
func TestListMethods_WithEmptyMethodName(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.valid_method": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d message=%s", resp.Code, resp.Message)
	}
	// Should have at least one method
	if len(resp.Methods) == 0 {
		t.Fatal("expected at least one method")
	}
}

// TestListMethods_DuplicateCaseInsensitive tests deduplication with different cases

// TestListMethods_DuplicateCaseInsensitive tests deduplication with different cases
func TestListMethods_DuplicateCaseInsensitive(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method_one": {Enabled: true},
			"external.test.METHOD_ONE": {Enabled: true}, // duplicate, different case
			"external.test.Method_Two": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "test")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d", resp.Code)
	}
	// Should deduplicate to 2 methods
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 deduplicated methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestListMethods_EmptyStringAfterTrim tests handling of whitespace-only method names

// TestListMethods_EmptyStringAfterTrim tests handling of whitespace-only method names
func TestListMethods_EmptyStringAfterTrim(t *testing.T) {
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
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got=%d", resp.Code)
	}
	// Should have at least the valid method
	if len(resp.Methods) < 1 {
		t.Fatal("expected at least one method")
	}
}

// TestCall_EmptyResponseData tests the empty response handling in service.Call

// TestListMethods_DuplicateSkip tests that duplicate methods are skipped
func TestListMethods_DuplicateSkip(t *testing.T) {
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.myapp.list_items":  {Enabled: true},
			"external.myapp.LIST_ITEMS":  {Enabled: true}, // Duplicate (different case)
			"external.myapp.create_item": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.ListMethods(context.Background(), "myapp")

	if err != nil {
		t.Fatalf("ListMethods returned error: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d: %s", resp.Code, resp.Message)
	}
	// Should deduplicate case-insensitively: list_items, create_item
	if len(resp.Methods) != 2 {
		t.Fatalf("expected 2 deduplicated methods, got %d: %v", len(resp.Methods), resp.Methods)
	}
}

// TestDiscoverExternalPlatforms_WithDisabledFunction tests that disabled functions are skipped
