package provider

import (
	"encoding/json"
	"testing"
	"time"

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
		caps := reg.ProviderCaps{
			ID:       "test-provider",
			Version:  "1.0.0",
			Lang:     "go",
			SDK:      "test-sdk",
			Manifest: []byte(`{}`), // Required for UpsertProviderCaps
		}
		store.UpsertProviderCaps(caps)

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
		caps := reg.ProviderCaps{
			ID:       "test-provider",
			Manifest: []byte(`{}`), // Required for UpsertProviderCaps
		}
		store.UpsertProviderCaps(caps)

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
		caps := reg.ProviderCaps{
			ID:       "test-provider",
			Manifest: []byte(`{}`), // Required for UpsertProviderCaps
		}
		store.UpsertProviderCaps(caps)

		err := deleteProviderCaps(store, "test-provider")
		if err != nil {
			t.Errorf("deleteProviderCaps() error = %v", err)
		}

		// Verify deletion
		_, ok := store.GetProviderCaps("test-provider")
		if ok {
			t.Error("deleteProviderCaps() should have removed the provider")
		}
	})
}

// TestDecodeManifest tests the decodeManifest helper
func TestDecodeManifest(t *testing.T) {
	t.Run("empty manifest returns empty map", func(t *testing.T) {
		result, err := decodeManifest(nil)
		if err != nil {
			t.Errorf("decodeManifest(nil) error = %v", err)
		}
		if len(result) != 0 {
			t.Errorf("decodeManifest(nil) should return empty map, got %v", result)
		}
	})

	t.Run("empty byte slice returns empty map", func(t *testing.T) {
		result, err := decodeManifest([]byte{})
		if err != nil {
			t.Errorf("decodeManifest([]) error = %v", err)
		}
		if len(result) != 0 {
			t.Errorf("decodeManifest([]) should return empty map, got %v", result)
		}
	})

	t.Run("valid JSON is decoded", func(t *testing.T) {
		manifest := []byte(`{"key": "value", "number": 123}`)
		result, err := decodeManifest(manifest)
		if err != nil {
			t.Errorf("decodeManifest() error = %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("decodeManifest() key = %v, want %v", result["key"], "value")
		}
		if result["number"] != float64(123) {
			t.Errorf("decodeManifest() number = %v, want %v", result["number"], 123)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		manifest := []byte(`not valid json`)
		_, err := decodeManifest(manifest)
		if err == nil {
			t.Error("decodeManifest() should return error for invalid JSON")
		}
	})
}

// TestManifestArray tests the manifestArray helper
func TestManifestArray(t *testing.T) {
	t.Run("nil map returns nil", func(t *testing.T) {
		result := manifestArray(nil, "key")
		if result != nil {
			t.Errorf("manifestArray(nil, ...) = %v, want nil", result)
		}
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		m := map[string]interface{}{"other": []interface{}{}}
		result := manifestArray(m, "missing")
		if result != nil {
			t.Errorf("manifestArray() for missing key = %v, want nil", result)
		}
	})

	t.Run("non-array value returns nil", func(t *testing.T) {
		m := map[string]interface{}{"key": "string value"}
		result := manifestArray(m, "key")
		if result != nil {
			t.Errorf("manifestArray() for non-array = %v, want nil", result)
		}
	})

	t.Run("array value is returned", func(t *testing.T) {
		m := map[string]interface{}{"items": []interface{}{"a", "b", "c"}}
		result := manifestArray(m, "items")
		if len(result) != 3 {
			t.Errorf("manifestArray() length = %d, want 3", len(result))
		}
	})
}

// TestManifestEntities tests the manifestEntities helper
func TestManifestEntities(t *testing.T) {
	t.Run("nil map returns nil", func(t *testing.T) {
		result := manifestEntities(nil)
		if result != nil {
			t.Errorf("manifestEntities(nil) = %v, want nil", result)
		}
	})

	t.Run("missing entities key returns nil", func(t *testing.T) {
		m := map[string]interface{}{"other": []interface{}{}}
		result := manifestEntities(m)
		if result != nil {
			t.Errorf("manifestEntities() for missing key = %v, want nil", result)
		}
	})

	t.Run("entities are extracted", func(t *testing.T) {
		m := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"id": "entity1"},
				map[string]interface{}{"id": "entity2"},
			},
		}
		result := manifestEntities(m)
		if len(result) != 2 {
			t.Errorf("manifestEntities() length = %d, want 2", len(result))
		}
		if result[0]["id"] != "entity1" {
			t.Errorf("manifestEntities()[0].id = %v, want %v", result[0]["id"], "entity1")
		}
	})

	t.Run("non-map items are skipped", func(t *testing.T) {
		m := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"id": "entity1"},
				"not a map",
				123,
			},
		}
		result := manifestEntities(m)
		if len(result) != 1 {
			t.Errorf("manifestEntities() should skip non-map items, got length = %d", len(result))
		}
	})
}

// TestManifestFunctions tests the manifestFunctions helper
func TestManifestFunctions(t *testing.T) {
	t.Run("nil map returns nil", func(t *testing.T) {
		result := manifestFunctions(nil)
		if result != nil {
			t.Errorf("manifestFunctions(nil) = %v, want nil", result)
		}
	})

	t.Run("functions are extracted", func(t *testing.T) {
		m := map[string]interface{}{
			"functions": []interface{}{
				map[string]interface{}{"id": "func1", "version": "1.0.0"},
				map[string]interface{}{"id": "func2", "version": "2.0.0"},
			},
		}
		result := manifestFunctions(m)
		if len(result) != 2 {
			t.Errorf("manifestFunctions() length = %d, want 2", len(result))
		}
		if result[0]["id"] != "func1" {
			t.Errorf("manifestFunctions()[0].id = %v, want %v", result[0]["id"], "func1")
		}
	})
}

// TestBuildProviderMeta tests the buildProviderMeta helper
func TestBuildProviderMeta(t *testing.T) {
	t.Run("basic meta is built", func(t *testing.T) {
		now := time.Now()
		caps := reg.ProviderCaps{
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

	t.Run("manifest is included when requested", func(t *testing.T) {
		manifest := map[string]interface{}{
			"functions": []interface{}{
				map[string]interface{}{"id": "func1"},
			},
		}
		manifestBytes, _ := json.Marshal(manifest)

		caps := reg.ProviderCaps{
			ID:       "test-provider",
			Manifest: manifestBytes,
		}

		result := buildProviderMeta(caps, true)

		if result["manifest"] == nil {
			t.Error("buildProviderMeta() should include manifest when requested")
		}
		if result["functions"] != 1 {
			t.Errorf("buildProviderMeta() functions count = %v, want 1", result["functions"])
		}
	})

	t.Run("manifest not included when not requested", func(t *testing.T) {
		manifest := map[string]interface{}{"key": "value"}
		manifestBytes, _ := json.Marshal(manifest)

		caps := reg.ProviderCaps{
			ID:       "test-provider",
			Manifest: manifestBytes,
		}

		result := buildProviderMeta(caps, false)

		if result["manifest"] != nil {
			t.Error("buildProviderMeta() should not include manifest when not requested")
		}
	})

	t.Run("invalid manifest error is reported", func(t *testing.T) {
		caps := reg.ProviderCaps{
			ID:       "test-provider",
			Manifest: []byte("invalid json"),
		}

		result := buildProviderMeta(caps, true)

		if result["manifestError"] == nil {
			t.Error("buildProviderMeta() should report manifest error")
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

	t.Run("entities are aggregated from multiple providers", func(t *testing.T) {
		store := reg.NewStore()

		manifest1, _ := json.Marshal(map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"id": "entity1"},
			},
		})
		manifest2, _ := json.Marshal(map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"id": "entity2"},
				map[string]interface{}{"id": "entity3"},
			},
		})

		store.UpsertProviderCaps(reg.ProviderCaps{ID: "provider1", Manifest: manifest1})
		store.UpsertProviderCaps(reg.ProviderCaps{ID: "provider2", Manifest: manifest2})

		result := aggregateEntities(store)

		if len(result) != 3 {
			t.Errorf("aggregateEntities() length = %d, want 3", len(result))
		}

		// Check that provider_id is added
		hasProviderID := false
		for _, entity := range result {
			if entity["provider_id"] != nil {
				hasProviderID = true
				break
			}
		}
		if !hasProviderID {
			t.Error("aggregateEntities() should add provider_id to each entity")
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

	t.Run("invalid manifest returns error", func(t *testing.T) {
		store := reg.NewStore()
		store.UpsertProviderCaps(reg.ProviderCaps{
			ID:       "test-provider",
			Manifest: []byte("invalid json"),
		})

		_, err := aggregateEntitiesForProvider(store, "test-provider")
		if err == nil {
			t.Error("aggregateEntitiesForProvider() should return error for invalid manifest")
		}
	})

	t.Run("entities are returned with provider_id", func(t *testing.T) {
		store := reg.NewStore()
		manifest, _ := json.Marshal(map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"id": "entity1", "name": "Entity One"},
			},
		})
		store.UpsertProviderCaps(reg.ProviderCaps{ID: "test-provider", Manifest: manifest})

		result, err := aggregateEntitiesForProvider(store, "test-provider")
		if err != nil {
			t.Errorf("aggregateEntitiesForProvider() error = %v", err)
		}
		if len(result) != 1 {
			t.Errorf("aggregateEntitiesForProvider() length = %d, want 1", len(result))
		}
		if result[0]["provider_id"] != "test-provider" {
			t.Errorf("aggregateEntitiesForProvider()[0].provider_id = %v, want %v", result[0]["provider_id"], "test-provider")
		}
		if result[0]["id"] != "entity1" {
			t.Errorf("aggregateEntitiesForProvider()[0].id = %v, want %v", result[0]["id"], "entity1")
		}
	})
}

// TestRefreshProviderTimestamp tests the refreshProviderTimestamp helper
func TestRefreshProviderTimestamp(t *testing.T) {
	t.Run("nil store does nothing", func(t *testing.T) {
		caps := reg.ProviderCaps{ID: "test"}
		// Should not panic
		refreshProviderTimestamp(nil, caps)
	})

	t.Run("timestamp is updated", func(t *testing.T) {
		store := reg.NewStore()
		oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		caps := reg.ProviderCaps{
			ID:        "test-provider",
			UpdatedAt: oldTime,
			Manifest:  []byte(`{}`), // Required for UpsertProviderCaps
		}
		store.UpsertProviderCaps(caps)

		// Wait a bit to ensure timestamp changes
		time.Sleep(time.Millisecond)

		refreshProviderTimestamp(store, caps)

		updated, ok := store.GetProviderCaps("test-provider")
		if !ok {
			t.Fatal("Provider should exist after refresh")
		}
		if !updated.UpdatedAt.After(oldTime) {
			t.Error("refreshProviderTimestamp() should update the timestamp")
		}
	})
}
