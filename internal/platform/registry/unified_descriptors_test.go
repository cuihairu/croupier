package registry_test

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/require"
)

func TestBuildUnifiedDescriptors_FunctionsDedupAndLastWins(t *testing.T) {
	store := registry.NewStore()
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:      "p1",
		Version: "1.0.0",
		Manifest: mustParseJSON(t, map[string]interface{}{
			"provider": map[string]interface{}{"id": "p1", "version": "1.0.0"},
			"functions": []map[string]interface{}{
				{"id": "f1", "display_name": map[string]string{"en": "old"}},
			},
		}),
	})
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:      "p2",
		Version: "2.0.0",
		Manifest: mustParseJSON(t, map[string]interface{}{
			"provider": map[string]interface{}{"id": "p2", "version": "2.0.0"},
			"functions": []map[string]interface{}{
				{"id": "f1", "display_name": map[string]string{"en": "new"}},
			},
		}),
	})

	unified := store.BuildUnifiedDescriptors()
	functions, ok := unified["functions"].([]interface{})
	require.True(t, ok)
	require.Len(t, functions, 1)

	fn, ok := functions[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "f1", fn["id"])

	dn, ok := fn["display_name"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "new", dn["en"])
}

func TestBuildUnifiedDescriptors_StableSortOrder(t *testing.T) {
	store := registry.NewStore()
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:      "p1",
		Version: "1.0.0",
		Manifest: mustParseJSON(t, map[string]interface{}{
			"provider": map[string]interface{}{"id": "p1", "version": "1.0.0"},
			"functions": []map[string]interface{}{
				{"id": "b"},
				{"id": "a"},
			},
			"entities": []map[string]interface{}{
				{"id": "e2"},
				{"id": "e1"},
			},
		}),
	})

	unified := store.BuildUnifiedDescriptors()

	functions := unified["functions"].([]interface{})
	require.Len(t, functions, 2)
	require.Equal(t, "a", functions[0].(map[string]interface{})["id"])
	require.Equal(t, "b", functions[1].(map[string]interface{})["id"])

	entities := unified["entities"].([]interface{})
	require.Len(t, entities, 2)
	require.Equal(t, "e1", entities[0].(map[string]interface{})["id"])
	require.Equal(t, "e2", entities[1].(map[string]interface{})["id"])
}

func TestBuildUnifiedDescriptors_ProviderMetaIncludesUpdatedAt(t *testing.T) {
	store := registry.NewStore()
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:      "p1",
		Version: "1.0.0",
		Lang:    "go",
		SDK:     "sdk@1",
		Manifest: mustParseJSON(t, map[string]interface{}{
			"provider": map[string]interface{}{"id": "p1", "version": "1.0.0"},
		}),
	})

	unified := store.BuildUnifiedDescriptors()
	providers := unified["providers"].(map[string]interface{})
	prov := providers["p1"].(map[string]interface{})
	require.Equal(t, "p1", prov["id"])
	require.Equal(t, "1.0.0", prov["version"])
	require.NotNil(t, prov["updated_at"])
	_, ok := prov["updated_at"].(time.Time)
	require.True(t, ok)
}
