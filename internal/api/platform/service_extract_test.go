package platform

import (
	"github.com/cuihairu/croupier/internal/model"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestExtractPlatformMethodsFromBindings(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "onepanel",
			SpecJSON:    model.JSON([]byte(`{"provider":"onepanel","operations":["install_app","list_apps"]}`)),
		},
		{
			BindingType: "openapi",
			BindingKey:  "helm",
			SpecJSON:    model.JSON([]byte(`{"name":"helm","operation":"install_chart"}`)),
		},
		{
			BindingType: "function",
			BindingKey:  "external.k8s.restart_deploy",
		},
	}
	got := extractPlatformMethodsFromBindings(bindings)
	if len(got["onepanel"]) != 2 {
		t.Fatalf("expected onepanel methods parsed, got=%v", got["onepanel"])
	}
	if len(got["helm"]) != 1 || got["helm"][0] != "install_chart" {
		t.Fatalf("expected helm install_chart parsed, got=%v", got["helm"])
	}
	if len(got["k8s"]) != 1 || got["k8s"][0] != "restart_deploy" {
		t.Fatalf("expected k8s restart_deploy parsed from function binding, got=%v", got["k8s"])
	}
}

func TestExtractPlatformMethodsFromBindings_NewProviderNoCoreChange(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "newvendor",
			SpecJSON:    model.JSON([]byte(`{"provider":"newvendor","operations":["install","upgrade","uninstall"]}`)),
		},
	}
	got := extractPlatformMethodsFromBindings(bindings)
	methods := got["newvendor"]
	if len(methods) != 3 {
		t.Fatalf("expected 3 methods from extension binding, got=%v", methods)
	}
	if methods[0] != "install" || methods[1] != "upgrade" || methods[2] != "uninstall" {
		t.Fatalf("unexpected methods order/content: %v", methods)
	}
}

func TestExtractPlatformMethodsFromBindings_Empty(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{}
	got := extractPlatformMethodsFromBindings(bindings)

	if len(got) != 0 {
		t.Fatalf("expected empty methods for empty bindings, got %v", got)
	}
}

func TestExtractPlatformMethodsFromBindings_InvalidJSON(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			SpecJSON:    model.JSON([]byte(`{invalid json`)),
		},
	}
	got := extractPlatformMethodsFromBindings(bindings)

	// Invalid JSON results in default "invoke" method
	if len(got["test"]) != 1 || got["test"][0] != "invoke" {
		t.Fatalf("expected default invoke method for invalid JSON, got %v", got["test"])
	}
}

func TestExtractPlatformMethodsFromBindings_FunctionBinding(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "function",
			BindingKey:  "external.test.platform_method",
		},
	}
	got := extractPlatformMethodsFromBindings(bindings)

	if len(got["test"]) != 1 || got["test"][0] != "platform_method" {
		t.Fatalf("expected test.platform_method parsed, got %v", got["test"])
	}
}

func TestExtractPlatformMethodsFromBindings_EmptySpecJSON(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			SpecJSON:    model.JSON([]byte{}),
		},
	}
	got := extractPlatformMethodsFromBindings(bindings)

	// Empty SpecJSON should result in default invoke method
	if len(got["test"]) != 1 || got["test"][0] != "invoke" {
		t.Fatalf("expected default invoke method for empty SpecJSON, got %v", got["test"])
	}
}

// TestExtractPlatformMethodsFromBindingsDetailed tests extracting platform methods from bindings
func TestExtractPlatformMethodsFromBindingsDetailed(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "function",
			BindingKey:  "external.testplatform.create",
			SpecJSON:    []byte(`{"operation":"create"}`),
		},
		{
			BindingType: "function",
			BindingKey:  "external.testplatform.delete",
			SpecJSON:    []byte(`{}`),
		},
		{
			BindingType: "provider",
			BindingKey:  "otherplatform",
			SpecJSON:    []byte(`{"operations":["list","get"]}`),
		},
	}

	result := extractPlatformMethodsFromBindings(bindings)

	// Should extract methods from bindings
	if len(result) == 0 {
		t.Fatal("expected to extract some platform methods")
	}

	t.Logf("Extracted platforms: %v", result)
}

// TestService_ListMethods_SourceSet tests that source is set when methods are found
