package externalfunc

import "testing"

func TestParseProviderBinding_WithConfigMap(t *testing.T) {
	spec := map[string]any{
		"provider":   "One Panel",
		"type":       "openapi",
		"operations": []any{"Install/App", "status"},
		"enabled":    true,
		"config": map[string]any{
			"base_url": "https://example.com",
		},
	}
	got, ok := ParseProviderBinding("", spec)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if got.Provider != "one_panel" {
		t.Fatalf("unexpected provider: %s", got.Provider)
	}
	if got.Type != "openapi" {
		t.Fatalf("unexpected type: %s", got.Type)
	}
	if len(got.Operations) != 2 || got.Operations[0] != "install_app" || got.Operations[1] != "status" {
		t.Fatalf("unexpected operations: %#v", got.Operations)
	}
	if !got.Enabled {
		t.Fatalf("expected enabled=true")
	}
	if got.Config["base_url"] != "https://example.com" {
		t.Fatalf("unexpected config: %#v", got.Config)
	}
}

func TestParseProviderBinding_FallbackAndDefaults(t *testing.T) {
	spec := map[string]any{
		"name":    "Test Provider",
		"enabled": "0",
		"url":     "https://api.example.com",
	}
	got, ok := ParseProviderBinding("", spec)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if got.Provider != "test_provider" {
		t.Fatalf("unexpected provider: %s", got.Provider)
	}
	if got.Type != "openapi" {
		t.Fatalf("unexpected type: %s", got.Type)
	}
	if len(got.Operations) != 1 || got.Operations[0] != "invoke" {
		t.Fatalf("unexpected operations: %#v", got.Operations)
	}
	if got.Enabled {
		t.Fatalf("expected enabled=false")
	}
	if got.Config["url"] != "https://api.example.com" {
		t.Fatalf("unexpected config fallback: %#v", got.Config)
	}
}

func TestParseProviderBinding_UsesBindingKeyAlias(t *testing.T) {
	spec := map[string]any{
		"operation": "invoke",
	}
	got, ok := ParseProviderBinding("my-provider", spec)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if got.Provider != "my-provider" {
		t.Fatalf("unexpected provider: %s", got.Provider)
	}
}

func TestParseProviderBinding_EmptyProvider(t *testing.T) {
	got, ok := ParseProviderBinding(" ", map[string]any{
		"enabled": true,
	})
	if ok {
		t.Fatalf("expected parse failure, got: %#v", got)
	}
}
