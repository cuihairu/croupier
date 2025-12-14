package registry_test

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
)

func TestBuildFunctionIndex(t *testing.T) {
	tests := []struct {
		name           string
		providers      []registry.ProviderCaps
		expectedIndex  map[string]map[string]interface{}
		expectedOrder  []string // 验证覆盖顺序
	}{
		{
			name: "single provider",
			providers: []registry.ProviderCaps{
				{
					ID:      "provider1",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id":          "function1",
								"display_name": map[string]string{"en": "Function 1"},
								"summary":     map[string]string{"en": "Summary 1"},
								"tags":        []string{"tag1", "tag2"},
							},
							{
								"id":          "function2",
								"display_name": map[string]string{"en": "Function 2"},
								"summary":     map[string]string{"en": "Summary 2"},
								"tags":        []string{"tag3"},
							},
						},
					}),
				},
			},
			expectedIndex: map[string]map[string]interface{}{
				"function1": {
					"display_name": map[string]string{"en": "Function 1"},
					"summary":      map[string]string{"en": "Summary 1"},
					"tags":         []interface{}{"tag1", "tag2"},
				},
				"function2": {
					"display_name": map[string]string{"en": "Function 2"},
					"summary":      map[string]string{"en": "Summary 2"},
					"tags":         []interface{}{"tag3"},
				},
			},
		},
		{
			name: "multiple providers - no conflicts",
			providers: []registry.ProviderCaps{
				{
					ID:      "provider1",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id":          "function1",
								"display_name": map[string]string{"en": "Function 1"},
								"summary":     map[string]string{"en": "Summary 1"},
							},
						},
					}),
				},
				{
					ID:      "provider2",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id":          "function2",
								"display_name": map[string]string{"en": "Function 2"},
								"summary":     map[string]string{"en": "Summary 2"},
							},
						},
					}),
				},
			},
			expectedIndex: map[string]map[string]interface{}{
				"function1": {
					"display_name": map[string]string{"en": "Function 1"},
					"summary":      map[string]string{"en": "Summary 1"},
				},
				"function2": {
					"display_name": map[string]string{"en": "Function 2"},
					"summary":      map[string]string{"en": "Summary 2"},
				},
			},
		},
		{
			name: "multiple providers - with conflicts (last wins)",
			providers: []registry.ProviderCaps{
				{
					ID:      "provider1",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id":           "function1",
								"display_name": map[string]string{"en": "Function 1"},
								"summary":      map[string]string{"en": "Summary 1"},
								"tags":         []string{"tag1", "old"},
								"permissions": map[string]interface{}{
									"verbs": []string{"read", "invoke"},
								},
							},
						},
					}),
				},
				{
					ID:      "provider2",
					Version: "2.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id":           "function1",
								"display_name": map[string]string{"en": "Updated Function 1"},
								"summary":      map[string]string{"en": "Updated Summary 1"},
								"tags":         []string{"tag1", "new"},
								"menu": map[string]interface{}{
									"section": "Updated Section",
									"order":   999,
								},
							},
						},
					}),
				},
			},
			expectedIndex: map[string]map[string]interface{}{
				"function1": {
					"display_name": map[string]string{"en": "Updated Function 1"},
					"summary":      map[string]string{"en": "Updated Summary 1"},
					"tags":         []interface{}{"tag1", "new"},
					"menu": map[string]interface{}{
						"section": "Updated Section",
						"order":   float64(999),
					},
				},
			},
		},
		{
			name: "provider with empty manifest",
			providers: []registry.ProviderCaps{
				{
					ID:       "provider1",
					Version:  "1.0.0",
					Manifest: []byte{},
				},
				{
					ID:      "provider2",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id": "function1",
							},
						},
					}),
				},
			},
			expectedIndex: map[string]map[string]interface{}{},
		},
		{
			name: "provider with invalid manifest",
			providers: []registry.ProviderCaps{
				{
					ID:      "provider1",
					Version: "1.0.0",
					Manifest: []byte("invalid json"),
				},
				{
					ID:      "provider2",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"id": "function1",
							},
						},
					}),
				},
			},
			expectedIndex: map[string]map[string]interface{}{},
		},
		{
			name: "provider with no functions field",
			providers: []registry.ProviderCaps{
				{
					ID:      "provider1",
					Version: "1.0.0",
					Manifest: mustParseJSON(t, map[string]interface{}{
						"entities": []map[string]interface{}{},
					}),
				},
			},
			expectedIndex: map[string]map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := registry.NewStore()

			// Insert all providers
			for _, provider := range tt.providers {
				store.UpsertProviderCaps(provider)
			}

			// Build index
			index := store.BuildFunctionIndex()

			// Compare indexes
			if !equalMaps(t, index, tt.expectedIndex) {
				t.Errorf("BuildFunctionIndex() mismatch")
			}
		})
	}
}

// Helper function to parse JSON into bytes
func mustParseJSON(t *testing.T, data interface{}) []byte {
	result, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	return result
}

// Helper function to compare two maps deeply
func equalMaps(t *testing.T, a, b map[string]map[string]interface{}) bool {
	if len(a) != len(b) {
		t.Logf("Length mismatch: %d vs %d", len(a), len(b))
		return false
	}

	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			t.Logf("Key %s missing in second map", k)
			return false
		}

		if !equalInterface(t, v, bv) {
			t.Logf("Value mismatch for key %s: %+v vs %+v", k, v, bv)
			return false
		}
	}

	return true
}

// Helper function to compare two interface{} values deeply
func equalInterface(t *testing.T, a, b interface{}) bool {
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)

	if err1 != nil || err2 != nil {
		t.Logf("Marshal error: %v, %v", err1, err2)
		return false
	}

	return string(aJSON) == string(bJSON)
}