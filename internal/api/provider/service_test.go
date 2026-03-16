package provider

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
)

// Test helpers

func createMockStore() *registry.Store {
	store := registry.NewStore()

	// Add some test providers with OpenAPI docs
	openAPIDoc := createTestOpenAPIDoc()

	store.UpsertOpenAPIProvider(registry.OpenAPIProviderCaps{
		ID:         "test-provider-1",
		Version:    "1.0.0",
		Lang:       "go",
		SDK:        "croupier-go",
		UpdatedAt:  time.Now().Add(-1 * time.Hour),
		OpenAPIDoc: openAPIDoc,
	})

	store.UpsertOpenAPIProvider(registry.OpenAPIProviderCaps{
		ID:         "test-provider-2",
		Version:    "2.0.0",
		Lang:       "python",
		SDK:        "croupier-python",
		UpdatedAt:  time.Now().Add(-2 * time.Hour),
		OpenAPIDoc: openAPIDoc,
	})

	return store
}

func createTestOpenAPIDoc() []byte {
	paths := openapi3.NewPaths()
	paths.Set("/users", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "listUsers",
			Summary:     "List all users",
			Extensions: map[string]interface{}{
				"x-category":  "users",
				"x-risk":      "low",
				"x-entity":    "user",
				"x-operation": "list",
			},
		},
		Post: &openapi3.Operation{
			OperationID: "createUser",
			Summary:     "Create a new user",
			Extensions: map[string]interface{}{
				"x-category":  "users",
				"x-risk":      "medium",
				"x-entity":    "user",
				"x-operation": "create",
			},
		},
	})
	paths.Set("/posts", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "listPosts",
			Summary:     "List all posts",
			Extensions: map[string]interface{}{
				"x-category": "posts",
				"x-entity":   "post",
			},
		},
	})

	doc := openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: paths,
		Extensions: map[string]interface{}{
			"x-entities": []interface{}{
				map[string]interface{}{
					"name":        "user",
					"description": "User entity",
				},
				map[string]interface{}{
					"name":        "post",
					"description": "Post entity",
				},
			},
		},
	}

	data, _ := json.Marshal(doc)
	return data
}

func createInvalidOpenAPIDoc() []byte {
	return []byte("{ invalid json }")
}

// Tests for helper functions

func TestEnsureRegistryStore_Nil(t *testing.T) {
	t.Parallel()

	_, err := ensureRegistryStore(nil)
	if err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

func TestGetProviderCaps_NilStore(t *testing.T) {
	t.Parallel()

	_, err := getProviderCaps(nil, "test-id")
	if err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

func TestGetProviderCaps_EmptyID(t *testing.T) {
	t.Parallel()

	store := registry.NewStore()
	_, err := getProviderCaps(store, "   ")
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestGetProviderCaps_NotFound(t *testing.T) {
	t.Parallel()

	store := registry.NewStore()
	_, err := getProviderCaps(store, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent provider, got nil")
	}
}

func TestGetProviderCaps_Success(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	caps, err := getProviderCaps(store, "test-provider-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caps.ID != "test-provider-1" {
		t.Fatalf("expected ID 'test-provider-1', got '%s'", caps.ID)
	}
	if caps.Version != "1.0.0" {
		t.Fatalf("expected Version '1.0.0', got '%s'", caps.Version)
	}
}

func TestDeleteProviderCaps_NilStore(t *testing.T) {
	t.Parallel()

	err := deleteProviderCaps(nil, "test-id")
	if err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

func TestDeleteProviderCaps_Success(t *testing.T) {
	t.Parallel()

	store := createMockStore()

	// Verify provider exists
	_, err := store.GetOpenAPIProvider("test-provider-1")
	if err != nil {
		t.Fatalf("provider should exist: %v", err)
	}

	// Delete it
	err = deleteProviderCaps(store, "test-provider-1")
	if err != nil {
		t.Fatalf("unexpected error deleting provider: %v", err)
	}

	// Verify it's gone
	_, err = store.GetOpenAPIProvider("test-provider-1")
	if err == nil {
		t.Fatal("provider should be deleted")
	}
}

func TestDecodeOpenAPIDoc_Empty(t *testing.T) {
	t.Parallel()

	doc, err := decodeOpenAPIDoc([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Fatal("expected nil doc for empty input")
	}
}

func TestDecodeOpenAPIDoc_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeOpenAPIDoc(createInvalidOpenAPIDoc())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestDecodeOpenAPIDoc_Success(t *testing.T) {
	t.Parallel()

	doc, err := decodeOpenAPIDoc(createTestOpenAPIDoc())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}
	if doc.Info.Title != "Test API" {
		t.Fatalf("expected title 'Test API', got '%s'", doc.Info.Title)
	}
}

func TestOpenAPIDocEntities_NilDoc(t *testing.T) {
	t.Parallel()

	entities := openAPIDocEntities(nil)
	if entities != nil {
		t.Fatal("expected nil for nil doc")
	}
}

func TestOpenAPIDocEntities_WithXEntities(t *testing.T) {
	t.Parallel()

	doc, _ := decodeOpenAPIDoc(createTestOpenAPIDoc())
	entities := openAPIDocEntities(doc)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}

	// Check x-entities extension
	foundUser := false
	for _, e := range entities {
		if name, ok := e["name"].(string); ok && name == "user" {
			foundUser = true
			break
		}
	}
	if !foundUser {
		t.Fatal("expected to find 'user' entity")
	}
}

func TestOpenAPIDocEntities_WithXEntity(t *testing.T) {
	t.Parallel()

	// Create doc with x-entity in operations but no x-entities
	paths := openapi3.NewPaths()
	paths.Set("/items", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "listItems",
			Extensions: map[string]interface{}{
				"x-entity": "item",
			},
		},
	})

	doc := &openapi3.T{
		Paths: paths,
	}

	entities := openAPIDocEntities(doc)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if entities[0]["name"] != "item" {
		t.Fatalf("expected entity name 'item', got '%v'", entities[0]["name"])
	}
}

func TestOpenAPIDocEntities_EmptyDoc(t *testing.T) {
	t.Parallel()

	paths := openapi3.NewPaths()
	doc := &openapi3.T{
		Paths: paths,
	}
	entities := openAPIDocEntities(doc)

	if len(entities) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(entities))
	}
}

func TestOpenAPIDocFunctions_NilDoc(t *testing.T) {
	t.Parallel()

	functions := openAPIDocFunctions(nil)
	if functions != nil {
		t.Fatal("expected nil for nil doc")
	}
}

func TestOpenAPIDocFunctions_Success(t *testing.T) {
	t.Parallel()

	doc, _ := decodeOpenAPIDoc(createTestOpenAPIDoc())
	functions := openAPIDocFunctions(doc)

	if len(functions) != 3 {
		t.Fatalf("expected 3 functions, got %d", len(functions))
	}

	// Check listUsers function
	foundListUsers := false
	for _, fn := range functions {
		if fn["operationId"] == "listUsers" {
			foundListUsers = true
			if fn["method"] != "GET" {
				t.Fatalf("expected method GET, got '%v'", fn["method"])
			}
			if fn["path"] != "/users" {
				t.Fatalf("expected path '/users', got '%v'", fn["path"])
			}
			if fn["category"] != "users" {
				t.Fatalf("expected category 'users', got '%v'", fn["category"])
			}
			if fn["entity"] != "user" {
				t.Fatalf("expected entity 'user', got '%v'", fn["entity"])
			}
			break
		}
	}
	if !foundListUsers {
		t.Fatal("expected to find 'listUsers' function")
	}
}

func TestOpenAPIDocFunctions_EmptyDoc(t *testing.T) {
	t.Parallel()

	paths := openapi3.NewPaths()
	doc := &openapi3.T{
		Paths: paths,
	}
	functions := openAPIDocFunctions(doc)

	if len(functions) != 0 {
		t.Fatalf("expected 0 functions, got %d", len(functions))
	}
}

func TestOpenAPIDocFunctions_WithNilOperation(t *testing.T) {
	t.Parallel()

	paths := openapi3.NewPaths()
	paths.Set("/test", &openapi3.PathItem{
		Get: nil,
		Post: &openapi3.Operation{
			OperationID: "testOp",
		},
	})

	doc := &openapi3.T{
		Paths: paths,
	}
	functions := openAPIDocFunctions(doc)

	if len(functions) != 1 {
		t.Fatalf("expected 1 function (nil should be skipped), got %d", len(functions))
	}
}

func TestBuildProviderMeta_WithDoc(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	caps, _ := store.GetOpenAPIProvider("test-provider-1")

	meta := buildProviderMeta(*caps, true)

	if meta["id"] != "test-provider-1" {
		t.Fatalf("expected id 'test-provider-1', got '%v'", meta["id"])
	}
	if meta["version"] != "1.0.0" {
		t.Fatalf("expected version '1.0.0', got '%v'", meta["version"])
	}
	if meta["functions"] != 3 {
		t.Fatalf("expected 3 functions, got %v", meta["functions"])
	}
	if meta["entities"] != 2 {
		t.Fatalf("expected 2 entities, got %v", meta["entities"])
	}
	if meta["openapi"] == nil {
		t.Fatal("expected openapi field to be set")
	}
}

func TestBuildProviderMeta_WithoutDoc(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	caps, _ := store.GetOpenAPIProvider("test-provider-1")

	meta := buildProviderMeta(*caps, false)

	if meta["openapi"] != nil {
		t.Fatal("expected openapi field to be nil when includeDoc=false")
	}
}

func TestBuildProviderMeta_InvalidDoc(t *testing.T) {
	t.Parallel()

	caps := registry.OpenAPIProviderCaps{
		ID:         "test",
		Version:    "1.0",
		OpenAPIDoc: createInvalidOpenAPIDoc(),
	}

	meta := buildProviderMeta(caps, true)

	if meta["docError"] == nil {
		t.Fatal("expected docError for invalid doc")
	}
	if meta["functions"] != nil {
		t.Fatal("expected nil functions for invalid doc")
	}
}

func TestAggregateEntities_NilStore(t *testing.T) {
	t.Parallel()

	entities := aggregateEntities(nil)
	if entities != nil {
		t.Fatal("expected nil for nil store")
	}
}

func TestAggregateEntities_Success(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	entities := aggregateEntities(store)

	if len(entities) == 0 {
		t.Fatal("expected entities from store")
	}

	// Check that provider_id is included
	for _, e := range entities {
		if e["provider_id"] == nil {
			t.Fatal("expected provider_id in each entity")
		}
	}
}

func TestAggregateEntitiesForProvider_Success(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	entities, err := aggregateEntitiesForProvider(store, "test-provider-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) == 0 {
		t.Fatal("expected entities for provider")
	}

	// Check provider_id
	for _, e := range entities {
		if e["provider_id"] != "test-provider-1" {
			t.Fatalf("expected provider_id 'test-provider-1', got '%v'", e["provider_id"])
		}
	}
}

func TestAggregateEntitiesForProvider_InvalidProvider(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	_, err := aggregateEntitiesForProvider(store, "non-existent")

	if err == nil {
		t.Fatal("expected error for non-existent provider")
	}
}

func TestRefreshProviderTimestamp_NilStore(t *testing.T) {
	t.Parallel()

	// Should not panic
	refreshProviderTimestamp(nil, registry.OpenAPIProviderCaps{})
}

func TestRefreshProviderTimestamp_Success(t *testing.T) {
	t.Parallel()

	store := createMockStore()
	caps, _ := store.GetOpenAPIProvider("test-provider-1")
	oldTime := caps.UpdatedAt

	// Wait a bit to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	refreshProviderTimestamp(store, *caps)

	// Get updated caps
	updatedCaps, _ := store.GetOpenAPIProvider("test-provider-1")
	if !updatedCaps.UpdatedAt.After(oldTime) {
		t.Fatal("expected timestamp to be updated")
	}
}

// Tests for service methods with mock store

func TestService_List_WithProviders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.List(ctx, &ProvidersListRequest{
		Page:     1,
		PageSize: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	if resp.Message != "OK" {
		t.Fatalf("expected message 'OK', got '%s'", resp.Message)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(items))
	}
}

func TestService_Capabilities_WithProviders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Capabilities(ctx, &ProvidersCapabilitiesRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(items))
	}

	// Check that openapi field is included
	for _, item := range items {
		if item["openapi"] == nil {
			t.Fatal("expected openapi field in capabilities")
		}
	}
}

func TestService_Descriptors_WithProviders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Descriptors(ctx, &ProvidersDescriptorsRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	manifests := data["provider_manifests"].(map[string]interface{})
	if len(manifests) != 2 {
		t.Fatalf("expected 2 provider manifests, got %d", len(manifests))
	}

	// Check structure of manifests
	for providerID, manifest := range manifests {
		m := manifest.(map[string]interface{})
		if m["id"] != providerID {
			t.Fatal("manifest id should match provider id")
		}
		if m["functions"] == nil || m["entities"] == nil {
			t.Fatal("expected functions and entities in manifest")
		}
		if m["openapi"] == nil {
			t.Fatal("expected openapi field in manifest")
		}
	}
}

func TestService_Detail_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Detail(ctx, &ProviderDetailRequest{
		ID: "test-provider-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	if data["id"] != "test-provider-1" {
		t.Fatalf("expected id 'test-provider-1', got '%v'", data["id"])
	}
	if data["openapi"] == nil {
		t.Fatal("expected openapi field in detail")
	}
}

func TestService_Entities_AllProviders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Entities(ctx, &ProvidersEntitiesRequest{
		ID: "",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	if len(items) == 0 {
		t.Fatal("expected entities")
	}
}

func TestService_Entities_Wildcard(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Entities(ctx, &ProvidersEntitiesRequest{
		ID: "*",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	if len(items) == 0 {
		t.Fatal("expected entities")
	}
}

func TestService_Entities_SingleProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Entities(ctx, &ProvidersEntitiesRequest{
		ID: "test-provider-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	if len(items) == 0 {
		t.Fatal("expected entities")
	}

	// Verify all entities have the correct provider_id
	for _, item := range items {
		if item["provider_id"] != "test-provider-1" {
			t.Fatalf("expected provider_id 'test-provider-1', got '%v'", item["provider_id"])
		}
	}
}

func TestService_Delete_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Delete(ctx, &ProviderActionRequest{
		ID: "test-provider-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	if data["id"] != "test-provider-1" {
		t.Fatalf("expected id 'test-provider-1', got '%v'", data["id"])
	}

	// Verify provider is deleted
	_, err = store.GetOpenAPIProvider("test-provider-1")
	if err == nil {
		t.Fatal("provider should be deleted")
	}
}

func TestService_Reload_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := createMockStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := service.Reload(ctx, &ProviderActionRequest{
		ID: "test-provider-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}

	data := resp.Data.(map[string]interface{})
	if data["id"] != "test-provider-1" {
		t.Fatalf("expected id 'test-provider-1', got '%v'", data["id"])
	}
	if data["updatedAt"] == nil {
		t.Fatal("expected updatedAt field")
	}
}
