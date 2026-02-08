package provider

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// TestEnsureRegistryStore tests the ensureRegistryStore helper
func TestEnsureRegistryStore(t *testing.T) {
	t.Run("nil store returns error", func(t *testing.T) {
		_, err := ensureRegistryStore(nil)
		if err == nil {
			t.Error("ensureRegistryStore(nil) should return error")
		}
	})

	t.Run("valid store returns store", func(t *testing.T) {
		store := reg.NewStore()
		result, err := ensureRegistryStore(store)
		if err != nil {
			t.Errorf("ensureRegistryStore() error = %v", err)
		}
		if result != store {
			t.Error("ensureRegistryStore() should return the same store")
		}
	})
}

// TestGetProviderCaps tests the getProviderCaps helper
func TestGetProviderCaps(t *testing.T) {
	t.Run("nil store returns error", func(t *testing.T) {
		_, err := getProviderCaps(nil, "test-id")
		if err == nil {
			t.Error("getProviderCaps(nil, ...) should return error")
		}
	})

	t.Run("empty ID returns error", func(t *testing.T) {
		store := reg.NewStore()
		_, err := getProviderCaps(store, "")
		if err == nil {
			t.Error("getProviderCaps(..., '') should return error")
		}
	})

	t.Run("whitespace ID returns error", func(t *testing.T) {
		store := reg.NewStore()
		_, err := getProviderCaps(store, "   ")
		if err == nil {
			t.Error("getProviderCaps(..., '   ') should return error")
		}
	})

	t.Run("non-existent provider returns error", func(t *testing.T) {
		store := reg.NewStore()
		_, err := getProviderCaps(store, "non-existent")
		if err == nil {
			t.Error("getProviderCaps() should return error for non-existent provider")
		}
	})

	t.Run("existing provider returns caps", func(t *testing.T) {
		store := reg.NewStore()
		openapiDoc := createTestOpenAPIDoc()
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			Version:    "1.0.0",
			Lang:       "go",
			SDK:        "test-sdk",
			OpenAPIDoc: openapiDoc,
		}
		store.UpsertOpenAPIProvider(caps)

		result, err := getProviderCaps(store, "test-provider")
		if err != nil {
			t.Errorf("getProviderCaps() error = %v", err)
		}
		if result.ID != "test-provider" {
			t.Errorf("getProviderCaps() ID = %v, want %v", result.ID, "test-provider")
		}
	})

	t.Run("ID with spaces is trimmed", func(t *testing.T) {
		store := reg.NewStore()
		openapiDoc := createTestOpenAPIDoc()
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			OpenAPIDoc: openapiDoc,
		}
		store.UpsertOpenAPIProvider(caps)

		result, err := getProviderCaps(store, "  test-provider  ")
		if err != nil {
			t.Errorf("getProviderCaps() should trim spaces, got error = %v", err)
		}
		if result.ID != "test-provider" {
			t.Errorf("getProviderCaps() ID = %v, want %v", result.ID, "test-provider")
		}
	})
}

// TestDeleteProviderCaps tests the deleteProviderCaps helper
func TestDeleteProviderCaps(t *testing.T) {
	t.Run("nil store returns error", func(t *testing.T) {
		err := deleteProviderCaps(nil, "test-id")
		if err == nil {
			t.Error("deleteProviderCaps(nil, ...) should return error")
		}
	})

	t.Run("non-existent provider returns error", func(t *testing.T) {
		store := reg.NewStore()
		err := deleteProviderCaps(store, "non-existent")
		if err == nil {
			t.Error("deleteProviderCaps() should return error for non-existent provider")
		}
	})

	t.Run("existing provider is deleted", func(t *testing.T) {
		store := reg.NewStore()
		openapiDoc := createTestOpenAPIDoc()
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			OpenAPIDoc: openapiDoc,
		}
		store.UpsertOpenAPIProvider(caps)

		err := deleteProviderCaps(store, "test-provider")
		if err != nil {
			t.Errorf("deleteProviderCaps() error = %v", err)
		}

		// Verify deletion
		_, err = store.GetOpenAPIProvider("test-provider")
		if err == nil {
			t.Error("deleteProviderCaps() should have removed the provider")
		}
	})
}

// TestDecodeOpenAPIDoc tests the decodeOpenAPIDoc helper
func TestDecodeOpenAPIDoc(t *testing.T) {
	t.Run("empty doc returns nil", func(t *testing.T) {
		result, err := decodeOpenAPIDoc(nil)
		if err != nil {
			t.Errorf("decodeOpenAPIDoc(nil) error = %v", err)
		}
		if result != nil {
			t.Errorf("decodeOpenAPIDoc(nil) should return nil, got %v", result)
		}
	})

	t.Run("empty byte slice returns nil", func(t *testing.T) {
		result, err := decodeOpenAPIDoc([]byte{})
		if err != nil {
			t.Errorf("decodeOpenAPIDoc([]) error = %v", err)
		}
		if result != nil {
			t.Errorf("decodeOpenAPIDoc([]) should return nil, got %v", result)
		}
	})

	t.Run("valid OpenAPI JSON is decoded", func(t *testing.T) {
		doc := createTestOpenAPIDoc()
		result, err := decodeOpenAPIDoc(doc)
		if err != nil {
			t.Errorf("decodeOpenAPIDoc() error = %v", err)
		}
		if result == nil {
			t.Error("decodeOpenAPIDoc() should return non-nil result")
		}
		if result.OpenAPI != "3.0.3" {
			t.Errorf("decodeOpenAPIDoc() openapi version = %v, want 3.0.3", result.OpenAPI)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		doc := []byte(`not valid json`)
		_, err := decodeOpenAPIDoc(doc)
		if err == nil {
			t.Error("decodeOpenAPIDoc() should return error for invalid JSON")
		}
	})
}

// TestOpenAPIDocFunctions tests the openAPIDocFunctions helper
func TestOpenAPIDocFunctions(t *testing.T) {
	t.Run("nil doc returns nil", func(t *testing.T) {
		result := openAPIDocFunctions(nil)
		if result != nil {
			t.Errorf("openAPIDocFunctions(nil) = %v, want nil", result)
		}
	})

	t.Run("functions are extracted from OpenAPI doc", func(t *testing.T) {
		docBytes := createTestOpenAPIDoc()
		doc, err := decodeOpenAPIDoc(docBytes)
		if err != nil {
			t.Fatalf("decodeOpenAPIDoc() error = %v", err)
		}
		result := openAPIDocFunctions(doc)
		if len(result) == 0 {
			t.Error("openAPIDocFunctions() should extract functions")
		}

		// Check that first function has expected fields
		if len(result) > 0 {
			fn := result[0]
			if fn["operationId"] == nil {
				t.Error("openAPIDocFunctions() should include operationId")
			}
			if fn["method"] == nil {
				t.Error("openAPIDocFunctions() should include method")
			}
			if fn["path"] == nil {
				t.Error("openAPIDocFunctions() should include path")
			}
		}
	})
}

// TestOpenAPIDocEntities tests the openAPIDocEntities helper
func TestOpenAPIDocEntities(t *testing.T) {
	t.Run("nil doc returns nil", func(t *testing.T) {
		result := openAPIDocEntities(nil)
		if result != nil {
			t.Errorf("openAPIDocEntities(nil) = %v, want nil", result)
		}
	})

	t.Run("entities are extracted from x-entity extensions", func(t *testing.T) {
		doc := &openapi3.T{
			OpenAPI: "3.0.3",
			Paths: openapi3.NewPaths(),
		}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Post: &openapi3.Operation{
				OperationID: "test.function",
				Extensions: map[string]interface{}{
					"x-entity": "Player",
				},
			},
		})
		result := openAPIDocEntities(doc)
		if len(result) != 1 {
			t.Errorf("openAPIDocEntities() length = %d, want 1", len(result))
		}
		if len(result) > 0 && result[0]["name"] != "Player" {
			t.Errorf("openAPIDocEntities()[0].name = %v, want Player", result[0]["name"])
		}
	})
}

// TestBuildProviderMeta tests the buildProviderMeta helper
func TestBuildProviderMeta(t *testing.T) {
	t.Run("basic meta is built", func(t *testing.T) {
		now := time.Now()
		caps := reg.OpenAPIProviderCaps{
			ID:        "test-provider",
			Version:   "1.0.0",
			Lang:      "go",
			SDK:       "test-sdk",
			UpdatedAt: now,
		}

		result := buildProviderMeta(caps, false)

		if result["id"] != "test-provider" {
			t.Errorf("buildProviderMeta() id = %v, want %v", result["id"], "test-provider")
		}
		if result["version"] != "1.0.0" {
			t.Errorf("buildProviderMeta() version = %v, want %v", result["version"], "1.0.0")
		}
		if result["lang"] != "go" {
			t.Errorf("buildProviderMeta() lang = %v, want %v", result["lang"], "go")
		}
		if result["sdk"] != "test-sdk" {
			t.Errorf("buildProviderMeta() sdk = %v, want %v", result["sdk"], "test-sdk")
		}
	})

	t.Run("OpenAPI doc is included when requested", func(t *testing.T) {
		openapiDoc := createTestOpenAPIDoc()
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			OpenAPIDoc: openapiDoc,
		}

		result := buildProviderMeta(caps, true)

		if result["openapi"] == nil {
			t.Error("buildProviderMeta() should include openapi when requested")
		}
		if result["functions"] == nil {
			t.Error("buildProviderMeta() should include functions count")
		}
	})

	t.Run("doc not included when not requested", func(t *testing.T) {
		openapiDoc := createTestOpenAPIDoc()
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			OpenAPIDoc: openapiDoc,
		}

		result := buildProviderMeta(caps, false)

		if result["openapi"] != nil {
			t.Error("buildProviderMeta() should not include openapi when not requested")
		}
	})

	t.Run("invalid doc error is reported", func(t *testing.T) {
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			OpenAPIDoc: []byte("invalid json"),
		}

		result := buildProviderMeta(caps, true)

		if result["docError"] == nil {
			t.Error("buildProviderMeta() should report doc error")
		}
	})
}

// TestAggregateEntities tests the aggregateEntities helper
func TestAggregateEntities(t *testing.T) {
	t.Run("nil store returns nil", func(t *testing.T) {
		result := aggregateEntities(nil)
		if result != nil {
			t.Errorf("aggregateEntities(nil) = %v, want nil", result)
		}
	})

	t.Run("empty store returns empty slice", func(t *testing.T) {
		store := reg.NewStore()
		result := aggregateEntities(store)
		if result == nil || len(result) != 0 {
			t.Errorf("aggregateEntities() should return empty slice for empty store")
		}
	})
}

// TestAggregateEntitiesForProvider tests the aggregateEntitiesForProvider helper
func TestAggregateEntitiesForProvider(t *testing.T) {
	t.Run("non-existent provider returns error", func(t *testing.T) {
		store := reg.NewStore()
		_, err := aggregateEntitiesForProvider(store, "non-existent")
		if err == nil {
			t.Error("aggregateEntitiesForProvider() should return error for non-existent provider")
		}
	})
}

// TestRefreshProviderTimestamp tests the refreshProviderTimestamp helper
func TestRefreshProviderTimestamp(t *testing.T) {
	t.Run("nil store does nothing", func(t *testing.T) {
		caps := reg.OpenAPIProviderCaps{ID: "test"}
		// Should not panic
		refreshProviderTimestamp(nil, caps)
	})

	t.Run("timestamp is updated", func(t *testing.T) {
		store := reg.NewStore()
		oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		openapiDoc := createTestOpenAPIDoc()
		caps := reg.OpenAPIProviderCaps{
			ID:         "test-provider",
			UpdatedAt:  oldTime,
			OpenAPIDoc: openapiDoc,
		}
		store.UpsertOpenAPIProvider(caps)

		// Wait a bit to ensure timestamp changes
		time.Sleep(time.Millisecond)

		refreshProviderTimestamp(store, caps)

		providers := store.ListOpenAPIProviders()
		if len(providers) == 0 {
			t.Fatal("Provider should exist after refresh")
		}
		if !providers[0].UpdatedAt.After(oldTime) {
			t.Error("refreshProviderTimestamp() should update the timestamp")
		}
	})
}

// Helper function to create a test OpenAPI 3.0.3 document
func createTestOpenAPIDoc() []byte {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Test Provider",
			Version:     "1.0.0",
			Description: "Test OpenAPI document",
		},
		Paths: openapi3.NewPaths(),
	}
	doc.Paths.Set("/test/function", &openapi3.PathItem{
		Post: &openapi3.Operation{
			OperationID: "test.function",
			Summary:     "Test function",
			Extensions: map[string]interface{}{
				"x-category":  "test",
				"x-risk":      "safe",
				"x-entity":    "TestEntity",
				"x-operation": "read",
			},
		},
	})

	data, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return data
}
