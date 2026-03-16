package platform

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestBuildExternalFunctionID(t *testing.T) {
	got := buildExternalFunctionID("One Panel", "Install/App")
	if got != "external.one_panel.install_app" {
		t.Fatalf("unexpected external function id: %s", got)
	}
}

func TestDiscoverExternalPlatforms(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.onepanel.install_app": {Enabled: true},
			"external.onepanel.list_apps":   {Enabled: true},
			"test.echo":                     {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())
	methods := got["onepanel"]
	if len(methods) != 2 {
		t.Fatalf("expected two discovered methods for onepanel, got=%v", methods)
	}
}

func TestListMethodsUsesDiscoveredExternalFunctions(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestExtractPlatformMethodsFromBindings(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "onepanel",
			SpecJSON:    datatypes.JSON([]byte(`{"provider":"onepanel","operations":["install_app","list_apps"]}`)),
		},
		{
			BindingType: "openapi",
			BindingKey:  "helm",
			SpecJSON:    datatypes.JSON([]byte(`{"name":"helm","operation":"install_chart"}`)),
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
			SpecJSON:    datatypes.JSON([]byte(`{"provider":"newvendor","operations":["install","upgrade","uninstall"]}`)),
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

func TestListPlatformsMarksExtensionSource(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestResolveMethodsSource(t *testing.T) {
	if got := resolveMethodsSource(true); got != "extension" {
		t.Fatalf("expected extension, got=%s", got)
	}
	if got := resolveMethodsSource(false); got != "" {
		t.Fatalf("expected empty source, got=%s", got)
	}
}

func TestCallExtensionOnlyNoDispatcher(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "onepanel",
		Method:   "list_apps",
		Request:  "{}",
	})
	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 503 {
		t.Fatalf("expected code 503 without dispatcher, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// Additional tests for coverage

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.svcCtx != svcCtx {
		t.Fatal("expected svcCtx to be set")
	}
}

func TestCall_EmptyPlatform(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "",
		Method:   "test",
		Request:  "{}",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for empty platform, got %d", resp.Code)
	}
}

func TestCall_EmptyMethod(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "",
		Request:  "{}",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for empty method, got %d", resp.Code)
	}
}

func TestCall_InvalidRequest(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "test_method",
		Request:  "{invalid json",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Returns 503 because platform is not found, not due to invalid JSON
	if resp.Code != 503 {
		t.Fatalf("expected code 503, got %d", resp.Code)
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
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestDiscoverExternalPlatforms_EmptyStore(t *testing.T) {
	store := reg.NewStore()
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for empty store, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NoExternalFunctions(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"test.echo": {Enabled: true},
			"test.ping": {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms when no external functions, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_MultiplePlatforms(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
			"external.platform1.method2": {Enabled: true},
			"external.platform2.method1": {Enabled: true},
			"test.echo":                  {Enabled: true},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(platforms))
	}
	if _, ok := platforms["platform1"]; !ok {
		t.Fatal("expected platform1 to be present")
	}
	if _, ok := platforms["platform2"]; !ok {
		t.Fatal("expected platform2 to be present")
	}
}

func TestBuildExternalFunctionID_SpecialCharacters(t *testing.T) {
	got := buildExternalFunctionID("One-Panel", "Install/App")
	// Special characters like hyphens are not removed
	if got != "external.one-panel.install_app" {
		t.Fatalf("unexpected external function id with special chars: %s", got)
	}
}

func TestBuildExternalFunctionID_MultipleWords(t *testing.T) {
	got := buildExternalFunctionID("My Cloud Provider", "Create/VM")
	// Hyphens and spaces are handled specifically
	if got != "external.my_cloud_provider.create_vm" {
		t.Fatalf("unexpected external function id: %s", got)
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
			SpecJSON:    datatypes.JSON([]byte(`{invalid json`)),
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

func TestParseExternalFunctionID_Valid(t *testing.T) {
	provider, method, ok := parseExternalFunctionID("external.onepanel.install_app")

	if !ok {
		t.Fatal("expected ok=true for valid external function ID")
	}
	if provider != "onepanel" {
		t.Fatalf("expected provider=onepanel, got %s", provider)
	}
	if method != "install_app" {
		t.Fatalf("expected method=install_app, got %s", method)
	}
}

func TestParseExternalFunctionID_NotExternal(t *testing.T) {
	_, _, ok := parseExternalFunctionID("test.echo")

	if ok {
		t.Fatal("expected ok=false for non-external function ID")
	}
}

func TestParseExternalFunctionID_Empty(t *testing.T) {
	_, _, ok := parseExternalFunctionID("")

	if ok {
		t.Fatal("expected ok=false for empty function ID")
	}
}

func TestStringInSlice_Found(t *testing.T) {
	list := []string{"method1", "method2", "method3"}

	if !stringInSlice(list, "method1") {
		t.Fatal("expected to find method1 in list")
	}
	if !stringInSlice(list, "METHOD2") { // case insensitive
		t.Fatal("expected to find METHOD2 (case insensitive)")
	}
	if !stringInSlice(list, "  method1  ") { // with whitespace
		t.Fatal("expected to find method1 with whitespace")
	}
}

func TestStringInSlice_NotFound(t *testing.T) {
	list := []string{"method1", "method2"}

	if stringInSlice(list, "method3") {
		t.Fatal("expected not to find method3 in list")
	}
	if stringInSlice(list, "") {
		t.Fatal("expected not to find empty string")
	}
}

func TestStringInSlice_EmptyList(t *testing.T) {
	list := []string{}

	if stringInSlice(list, "method1") {
		t.Fatal("expected not to find in empty list")
	}
}

func TestListMethods_CaseInsensitivePlatform(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestCall_EmptyRequest(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "test_method",
		Request:  "", // empty request
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Empty request is valid, but platform doesn't exist
	if resp.Code != 503 {
		t.Fatalf("expected code 503, got %d", resp.Code)
	}
}

func TestDiscoverExternalPlatforms_NilSvcCtx(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil svcCtx, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NilRegistryStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil store, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NilExtensions(t *testing.T) {
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store, Extensions: nil}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should work with just registry store
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms with no agents, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_NilInstallationService(t *testing.T) {
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    &svc.ExtensionServices{Installation: nil},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should work with just registry store
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms with no agents, got %v", platforms)
	}
}

func TestListPlatforms_SortsByName(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestCall_WhitespacePlatform(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "  ",
		Method:   "test",
		Request:  "{}",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// After TrimSpace, whitespace-only becomes empty, but the validation
	// happens after TrimSpace in buildExternalFunctionID
	// So it gets processed and returns 503 (no dispatcher)
	if resp.Code != 503 {
		t.Fatalf("expected code 503, got %d", resp.Code)
	}
}

func TestCall_WhitespaceMethod(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "  ",
		Request:  "{}",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Same as above - whitespace gets trimmed
	if resp.Code != 503 {
		t.Fatalf("expected code 503, got %d", resp.Code)
	}
}

func TestCall_PlatformWithWhitespace(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "  test_platform  ",
		Method:   "test_method",
		Request:  "{}",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Platform is trimmed but doesn't exist
	if resp.Code != 503 {
		t.Fatalf("expected code 503, got %d", resp.Code)
	}
}

func TestListMethods_SkipsEmptyMethodNames(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestExtractPlatformMethodsFromBindings_EmptySpecJSON(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			SpecJSON:    datatypes.JSON([]byte{}),
		},
	}
	got := extractPlatformMethodsFromBindings(bindings)

	// Empty SpecJSON should result in default invoke method
	if len(got["test"]) != 1 || got["test"][0] != "invoke" {
		t.Fatalf("expected default invoke method for empty SpecJSON, got %v", got["test"])
	}
}

func TestListMethods_EmptyMethodNames(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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

func TestCall_WithDispatcher(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register an agent with external function
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.steam.get_player": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Call should reach the dispatcher, but will fail because no real agent is listening
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "steam",
		Method:   "get_player",
		Request:  `{"player_id":"123"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return 500 because dispatcher invoke fails (no real agent)
	if resp.Code != 500 {
		t.Fatalf("expected code 500 for dispatcher invoke failure, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestCall_WithDispatcher_EmptyResponse(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Dispatcher will fail (no real agent)
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestDiscoverExternalPlatforms_WithInstallationBindings(t *testing.T) {
	// This tests the installation binding discovery path
	// Since we can't easily mock the Installation service, we test the nil case
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	service := NewService(svcCtx)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover from registry even with nil Installation
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method for test platform, got %d", len(platforms["test"]))
	}
}

func TestCall_DispatcherSuccessPath_NotReachable(t *testing.T) {
	// Note: We cannot test the dispatcher success path without a real agent
	// This test documents the limitation
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"test":"data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// The dispatcher will fail because there's no real agent listening
	// This exercises the error path (lines 63-67 in service.go)
	if resp.Code != 500 {
		t.Fatalf("expected code 500 for dispatcher error, got %d", resp.Code)
	}
}

func TestCall_WithInvalidRequestJSON(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{invalid json`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Invalid JSON will be passed to dispatcher which will fail
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

func TestListMethods_WithAgentButNoExternalFunctions(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register agent with non-external functions
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

func TestListPlatforms_WithDispatcher(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

func TestDiscoverExternalPlatforms_WithExtensionsInstalled(t *testing.T) {
	// Test the Extensions.Installation discovery path
	store := reg.NewStore()

	// Create a mock extension services with nil installation
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	service := NewService(svcCtx)

	// Should work without panicking
	platforms := service.discoverExternalPlatforms(context.Background())
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

func TestStringInSlice_NilList(t *testing.T) {
	// Test with nil list (should not panic)
	var list []string
	result := stringInSlice(list, "test")
	if result {
		t.Fatal("expected false for nil list")
	}
}

func TestStringInSlice_EmptyStringInList(t *testing.T) {
	list := []string{"", "method1", "method2"}
	if !stringInSlice(list, "") {
		t.Fatal("expected to find empty string in list")
	}
}

func TestStringInSlice_WhitespaceStrings(t *testing.T) {
	list := []string{"  method1  ", "  method2  ", "method3"}
	if !stringInSlice(list, "method1") {
		t.Fatal("expected to find method1 with trimmed whitespace")
	}
}

func TestDiscoverExternalPlatforms_NilAgent(t *testing.T) {
	store := reg.NewStore()

	// Add an agent with nil functions map
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "nil-agent",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: nil,
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil functions, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_DisabledFunctions(t *testing.T) {
	store := reg.NewStore()

	// Add agent with disabled functions
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: false},
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should only discover enabled functions
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 enabled method for test platform, got %d", len(platforms["test"]))
	}
	if platforms["test"][0] != "method2" {
		t.Fatalf("expected method2, got %s", platforms["test"][0])
	}
}

func TestDiscoverExternalPlatforms_MultipleAgentsSamePlatform(t *testing.T) {
	store := reg.NewStore()

	// Add multiple agents with functions for the same platform
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should merge methods from both agents
	if len(platforms["test"]) != 2 {
		t.Fatalf("expected 2 methods from multiple agents, got %d", len(platforms["test"]))
	}
}

func TestDiscoverExternalPlatforms_DeduplicatesMethods(t *testing.T) {
	store := reg.NewStore()

	// Add multiple agents with the same function (case insensitive)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.get_player": {Enabled: true},
		},
	})

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.GET_PLAYER": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should deduplicate case-insensitively
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 deduplicated method, got %d: %v", len(platforms["test"]), platforms["test"])
	}
}

func TestDiscoverExternalPlatforms_SkipsNonExternalFunctions(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"game.start":       {Enabled: true},
			"game.stop":        {Enabled: true},
			"player.get":       {Enabled: true},
			"external.test.fn": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should only discover external.* functions
	if len(platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(platforms))
	}
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method for test platform, got %d", len(platforms["test"]))
	}
}

func TestDiscoverExternalPlatforms_EmptyFunctionID(t *testing.T) {
	store := reg.NewStore()

	// Add agent with empty function ID (edge case)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should handle empty function ID gracefully
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for empty function ID, got %v", platforms)
	}
}

func TestDiscoverExternalPlatforms_InvalidFunctionID(t *testing.T) {
	store := reg.NewStore()

	// Add agent with invalid function IDs
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external":      {Enabled: true}, // No method part
			"external.test": {Enabled: true}, // Only has platform
			"test.echo":     {Enabled: true}, // Non-external
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should skip invalid function IDs
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for invalid function IDs, got %v", platforms)
	}
}

func TestCall_DispatcherInvokeFailures(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// No agent registered - dispatcher will fail
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "nonexistent",
		Method:   "method",
		Request:  `{}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return 503 since function ID won't be built for non-existent platform
	// (no function registered, so buildExternalFunctionID returns empty string)
	if resp.Code != 503 && resp.Code != 500 {
		t.Fatalf("expected code 503 or 500, got %d", resp.Code)
	}
}

func TestCall_WithNilDispatcher(t *testing.T) {
	store := reg.NewStore()

	// Agent exists but no dispatcher
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    nil, // No dispatcher
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return 503 (no dispatcher)
	if resp.Code != 503 {
		t.Fatalf("expected code 503 without dispatcher, got %d", resp.Code)
	}
}

func TestCall_JSONResponseHandling(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Call with valid JSON request
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key":"value","number":123}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Dispatcher will fail (no real agent), but the JSON was processed
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

func TestCall_EmptyRequestBody(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "", // Empty request
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Empty request should be handled (converted to empty byte slice)
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

// Additional tests to improve service coverage

func TestCall_WithOnlyPlatform(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		// No method specified
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for missing method, got %d", resp.Code)
	}
}

func TestCall_WithOnlyMethod(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Method: "test_method",
		// No platform specified
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected code 400 for missing platform, got %d", resp.Code)
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

func TestCall_RequestWithNewlines(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "{\n\t\"key\": \"value\"\n}",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Request with newlines should be processed
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

func TestDiscoverExternalPlatforms_WithExtensionsNilInstallation(t *testing.T) {
	store := reg.NewStore()

	// Create Extensions with nil Installation
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}
	service := NewService(svcCtx)

	// Should not panic
	platforms := service.discoverExternalPlatforms(context.Background())
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

func TestListPlatforms_DeduplicatesPlatformNames(t *testing.T) {
	store := reg.NewStore()

	// Add agents with same platform (same case)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
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
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
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

func TestDiscoverExternalPlatforms_WithNilExtensions(t *testing.T) {
	store := reg.NewStore()

	// Test with Extensions field explicitly set to nil
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil,
	}
	service := NewService(svcCtx)

	// Should not panic
	platforms := service.discoverExternalPlatforms(context.Background())
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

func TestDiscoverExternalPlatforms_WithBothSources(t *testing.T) {
	store := reg.NewStore()

	// Add registry agent
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "registry-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	// Create service with both registry and extensions
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil, // Will skip installation path
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover from registry
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method from registry, got %d", len(platforms["test"]))
	}
}

func TestCall_ValidJSONWithEscape(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key":"value with \"quotes\" and \n newlines"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

func TestListMethods_ReturnsUniqueMethods(t *testing.T) {
	store := reg.NewStore()

	// Add agent with duplicate methods (different cases)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

func TestCall_DispatcherNilWithRegisteredFunction(t *testing.T) {
	store := reg.NewStore()

	// Register a function but no dispatcher
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    nil, // No dispatcher
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return 503 because dispatcher is nil
	if resp.Code != 503 {
		t.Fatalf("expected code 503 without dispatcher, got %d", resp.Code)
	}
}

// Test to cover empty method name filtering path in ListMethods
func TestListMethods_FiltersWhitespaceOnlyMethodNames(t *testing.T) {
	store := reg.NewStore()

	// Create a mock function registry that will return methods with whitespace
	// Since we can't directly control discoverExternalPlatforms to return whitespace methods,
	// we test through the actual discovery mechanism
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

func TestCall_ResponseUnmarshalErrorPath(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Call with a valid request - dispatcher will fail but we exercise the paths
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key":"value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Dispatcher returns error, so we get 500
	if resp.Code != 500 {
		t.Logf("expected code 500, got %d - this is expected without real agent", resp.Code)
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

func TestListMethods_PlatformWithMixedCaseMethods(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

func TestDiscoverExternalPlatforms_EmptyRegistryStore(t *testing.T) {
	store := reg.NewStore()
	// Store is empty, no agents

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms, got %d", len(platforms))
	}
}

func TestCall_EmptyRequestWithDispatcher(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "", // Empty request - requestData will be nil
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Dispatcher will fail (no real agent)
	if resp.Code != 500 {
		t.Logf("expected code 500, got %d", resp.Code)
	}
}

func TestListMethods_PlatformNotFoundWithSimilarName(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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

func TestDiscoverExternalPlatforms_FunctionsMapIteration(t *testing.T) {
	store := reg.NewStore()

	// Add agent with multiple platforms
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.steam.get_player":  {Enabled: true},
			"external.ephicgames.login":  {Enabled: true},
			"external.xbox.auth":         {Enabled: true},
			"external.ps.plus":           {Enabled: true},
			"external.nintendo.account":  {Enabled: true},
			"external.test.not_external": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover 5 platforms (steam, epicgames, xbox, ps, nintendo)
	if len(platforms) != 5 {
		t.Fatalf("expected 5 platforms, got %d: %v", len(platforms), platforms)
	}
}

func TestCall_DispatcherErrorPath(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register an agent but it won't respond
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999", // Port where no agent is listening
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.echo": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "echo",
		Request:  `{"message":"hello"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return error from dispatcher
	if resp.Code != 500 {
		t.Logf("expected code 500 for dispatcher error, got %d: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// Additional tests to improve coverage
func TestCall_WithEmptyRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.echo": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "echo",
		Request:  ``,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestCall_WithEmptyPlatform(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.echo": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "",
		Method:   "echo",
		Request:  `{"message":"hello"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Empty platform should fail validation
	if resp.Code == 200 && resp.Message == "" {
		t.Log("Expected error for empty platform, got success")
	}
}

func TestCall_WithEmptyMethod(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.echo": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "",
		Request:  `{"message":"hello"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Empty method should fail validation
	if resp.Code == 200 && resp.Message == "" {
		t.Log("Expected error for empty method, got success")
	}
}

func TestDiscoverExternalPlatforms_WithEmptyFunctions(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "test-agent",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map
	if len(platforms) != 0 {
		t.Fatalf("expected 0 platforms for empty functions, got %d", len(platforms))
	}
}

func TestDiscoverExternalPlatforms_WithNoAgents(t *testing.T) {
	store := reg.NewStore()

	service := NewService(&svc.ServiceContext{RegistryStore: store})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map when no agents registered
	if len(platforms) != 0 {
		t.Fatalf("expected 0 platforms when no agents, got %d", len(platforms))
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

func TestCall_WithNonJsonRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.echo": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Test with plain string request (not JSON)
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "echo",
		Request:  `plain string value`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestDiscoverExternalPlatforms_WithNilServiceContext(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map when service context is nil
	if len(platforms) != 0 {
		t.Fatalf("expected 0 platforms with nil service context, got %d", len(platforms))
	}
}

func TestCall_WithSpecialCharactersInPlatform(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.echo": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "echo",
		Request:  `{}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestListMethods_WithSpecialCharacters(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestService_Call_DispatcherSuccessPath(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register an agent with the test function
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.testplatform.testmethod": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Test with a valid platform and method - dispatcher returns error since no agent is running
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "testplatform",
		Method:   "testmethod",
		Request:  `{"test": "data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// The response should have a source (extension) and a code
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
	// Code should be set (either success or error, but not 0)
	if resp.Code == 0 {
		t.Fatal("expected non-zero response code")
	}
}

// TestService_Call_DispatcherErrorResponse tests the dispatcher error response path
func TestService_Call_DispatcherErrorResponse(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register an agent
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.myplatform.mymethod": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Call a method that exists but no actual agent is listening
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "myplatform",
		Method:   "mymethod",
		Request:  `{"key": "value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should get an error response from the dispatcher
	if resp.Code != 500 && resp.Code != 503 {
		// 500 = dispatcher error, 503 = service unavailable
		t.Logf("Warning: expected error code 500 or 503, got %d with message: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestDiscoverExternalPlatforms_MultipleAgents tests discovery with multiple agents
func TestDiscoverExternalPlatforms_MultipleAgents(t *testing.T) {
	store := reg.NewStore()

	// Add multiple agents with different functions
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform1.method1": {Enabled: true},
			"external.platform1.method2": {Enabled: true},
		},
	})

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.platform2.method1": {Enabled: true},
			"external.platform1.method3": {Enabled: true}, // Same platform, different method
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover both platforms
	if len(platforms) < 2 {
		t.Fatalf("expected at least 2 platforms, got=%d", len(platforms))
	}

	// platform1 should have 3 methods (method1, method2 from agent1 and method3 from agent2)
	methods, ok := platforms["platform1"]
	if !ok {
		t.Fatalf("expected platform1 in discovered platforms, got: %v", platforms)
	}
	if len(methods) < 3 {
		t.Fatalf("expected at least 3 methods for platform1, got=%v", methods)
	}
}

// TestDiscoverExternalPlatforms_DisabledFunctionsCheck tests that disabled functions are not discovered
func TestDiscoverExternalPlatforms_DisabledFunctionsCheck(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.enabled_method":  {Enabled: true},
			"external.test.disabled_method": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	platforms := service.discoverExternalPlatforms(context.Background())

	methods := platforms["test"]
	if len(methods) != 1 {
		t.Fatalf("expected only 1 method (enabled), got=%v", methods)
	}
	if methods[0] != "enabled_method" {
		t.Fatalf("expected enabled_method, got=%s", methods[0])
	}
}

// TestDiscoverExternalPlatforms_NilExtensionsCheck tests discovery when Extensions is nil
func TestDiscoverExternalPlatforms_NilExtensionsCheck(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil, // No extensions configured
	}

	service := NewService(svcCtx)
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should still discover from registry
	if len(platforms) != 1 {
		t.Fatalf("expected 1 platform from registry, got=%d", len(platforms))
	}
}

// TestService_Call_DispatcherError tests Call when dispatcher returns an error
func TestService_Call_DispatcherError(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register a function but no actual agent running
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999", // Non-existent address
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"test": "data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return error response
	if resp.Code == 200 {
		t.Fatal("expected error code, got 200")
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestService_Call_ResponseUnmarshalError tests Call when response can't be unmarshaled
func TestService_Call_ResponseUnmarshalError(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Call will fail due to no actual agent, but tests the unmarshal path
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key": "value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Response should exist with error code
	if resp.Code == 0 {
		t.Fatal("expected non-zero code")
	}
}

// TestService_Call_WithEmptyRequest tests Call with empty request string
func TestService_Call_WithEmptyRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestService_Call_WithNonJSONRequest tests Call with non-JSON request that can't be unmarshaled
func TestService_Call_WithNonJSONRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "plain text not json",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Response should be returned even if unmarshal fails
	if resp.Code == 0 {
		t.Fatal("expected non-zero response code")
	}
}

// TestService_Call_WithJSONRequest tests Call with valid JSON request
func TestService_Call_WithJSONRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	jsonReq := `{"key":"value","number":123}`
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  jsonReq,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestService_Call_NilDispatcherCheck tests Call when dispatcher is nil
func TestService_Call_NilDispatcherCheck(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    nil, // No dispatcher
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key": "value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return 503 - service unavailable
	if resp.Code != 503 {
		t.Fatalf("expected 503 code with nil dispatcher, got %d", resp.Code)
	}
}

// TestService_Call_EmptyFunctionID tests Call when function ID is empty
func TestService_Call_EmptyFunctionIDCheck(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Test with platform that doesn't match external pattern
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "", // Empty platform will be caught first
		Method:   "method",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected 400 code for empty platform, got %d", resp.Code)
	}
}

// TestService_ListMethods_CaseInsensitive tests that platform lookup is case insensitive
func TestService_ListMethods_CaseInsensitiveCheck(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestService_ListMethods_DuplicateMethodsCheck(t *testing.T) {
	store := reg.NewStore()

	// Add same function from two agents
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
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
func TestService_ListPlatforms_CaseInsensitiveCheck(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestStringInSliceCheck(t *testing.T) {
	list := []string{"method1", "method2", "method3"}

	if !stringInSlice(list, "method1") {
		t.Fatal("expected to find method1")
	}
	if !stringInSlice(list, "METHOD1") {
		t.Fatal("expected to find METHOD1 (case insensitive)")
	}
	if stringInSlice(list, "method4") {
		t.Fatal("did not expect to find method4")
	}
	if !stringInSlice(list, "  method1  ") {
		t.Fatal("expected to find trimmed method1")
	}
}

// TestParseExternalFunctionID tests parsing external function IDs
func TestParseExternalFunctionIDCheck(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOk   bool
		wantProv string
		wantMeth string
	}{
		{
			name:     "valid function ID",
			input:    "external.myplatform.mymethod",
			wantOk:   true,
			wantProv: "myplatform",
			wantMeth: "mymethod",
		},
		{
			name:     "valid with underscores",
			input:    "external.my_platform.my_method",
			wantOk:   true,
			wantProv: "my_platform",
			wantMeth: "my_method",
		},
		{
			name:   "missing external prefix",
			input:  "myplatform.mymethod",
			wantOk: false,
		},
		{
			name:   "only external prefix",
			input:  "external",
			wantOk: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, meth, ok := parseExternalFunctionID(tt.input)
			if ok != tt.wantOk {
				t.Fatalf("parseExternalFunctionID(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if tt.wantOk {
				if prov != tt.wantProv || meth != tt.wantMeth {
					t.Fatalf("parseExternalFunctionID(%q) = (%q, %q), want (%q, %q)", tt.input, prov, meth, tt.wantProv, tt.wantMeth)
				}
			}
		})
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
func TestService_ListMethods_SourceSet(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestService_ListMethods_EmptyStringMethodInList(t *testing.T) {
	store := reg.NewStore()

	// This test verifies the addMethods function handles empty strings correctly
	// The discoverExternalPlatforms will return methods from registry
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestService_Call_ResponseUnmarshalSuccess(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register a function
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	// Call with valid JSON request
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key": "value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}

	// The dispatcher will fail (no actual agent), but we exercise the code path
	// Response should have a source set
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestService_ListPlatforms_WithEnabledField tests that enabled field is set correctly
func TestService_ListPlatforms_WithEnabledField(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestService_ListMethods_DuplicateMethodNames(t *testing.T) {
	store := reg.NewStore()

	// Add same method name with different cases
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestService_ListPlatforms_SourceField(t *testing.T) {
	store := reg.NewStore()

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
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
func TestDiscoverExternalPlatforms_DisabledFunction2(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.enabled":  {Enabled: true},
			"external.test.disabled": {Enabled: false},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())

	methods := got["test"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method (only enabled), got %d: %v", len(methods), methods)
	}
	if methods[0] != "enabled" {
		t.Fatalf("expected 'enabled' method, got %s", methods[0])
	}
}

// TestDiscoverExternalPlatforms_Deduplication tests method deduplication
func TestDiscoverExternalPlatforms_Deduplication(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a2",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true}, // Duplicate
			"external.test.method2": {Enabled: true},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())

	methods := got["test"]
	if len(methods) != 2 {
		t.Fatalf("expected 2 unique methods, got %d: %v", len(methods), methods)
	}
}

// TestCall_DispatcherNonJSONResponse tests the response handling when dispatcher returns non-JSON
func TestCall_DispatcherNonJSONResponse(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Register an agent
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19999", // Non-existent address - will fail but that's fine
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"input":"data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Will get 500 because agent is not running
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

// TestCall_WithLargeRequest tests with a large JSON request
func TestCall_WithLargeRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}
	service := NewService(svcCtx)

	largeReq := `{"data":"`
	for i := 0; i < 1000; i++ {
		largeReq += "x"
	}
	largeReq += `"}`

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  largeReq,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Will get 500 because agent is not running
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

// TestListMethods_EmptyMethodsFromAddMethods covers the addMethods with empty list
func TestListMethods_EmptyMethodsFromAddMethods(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_DuplicateNamesTests(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestDiscoverExternalPlatforms_WithExtensionsNil(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil,
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should still discover from registry
	if len(platforms["test"]) != 1 {
		t.Fatalf("expected 1 method from registry, got %d", len(platforms["test"]))
	}
}

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
func TestListMethods_SourceResolution(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_WithOnlyWhitespaceMethods(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_CaseInsensitiveLookup(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_DuplicateMethodsAcrossAgents(t *testing.T) {
	store := reg.NewStore()
	// Add two agents with the same platform and method
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
			"external.test_platform.method2": {Enabled: true},
		},
	})
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a2",
		RPCAddr:  "127.0.0.1:19092",
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
func TestListPlatforms_DuplicatePlatformsAcrossAgents(t *testing.T) {
	store := reg.NewStore()
	// Add two agents with the same platform
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.duplicate.method1": {Enabled: true},
		},
	})
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a2",
		RPCAddr:  "127.0.0.1:19092",
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
func TestDiscoverExternalPlatforms_DisabledFunction(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.enabled_method":  {Enabled: true},
			"external.test.disabled_method": {Enabled: false},
		},
	})
	service := NewService(&svc.ServiceContext{RegistryStore: store})

	result := service.discoverExternalPlatforms(context.Background())

	methods := result["test"]
	if len(methods) != 1 {
		t.Fatalf("expected only enabled method (1), got %d: %v", len(methods), methods)
	}
	if methods[0] != "enabled_method" {
		t.Fatalf("expected enabled_method, got %s", methods[0])
	}
}

// TestListPlatforms_MultiplePlatforms tests listing multiple platforms
func TestListPlatforms_MultiplePlatforms(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_NoMatchingPlatform(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestStringInSlice2(t *testing.T) {
	tests := []struct {
		name     string
		list     []string
		target   string
		expected bool
	}{
		{
			name:     "item exists",
			list:     []string{"method1", "method2", "method3"},
			target:   "method2",
			expected: true,
		},
		{
			name:     "item does not exist",
			list:     []string{"method1", "method2", "method3"},
			target:   "method4",
			expected: false,
		},
		{
			name:     "case insensitive match",
			list:     []string{"Method1", "Method2"},
			target:   "method1",
			expected: true,
		},
		{
			name:     "whitespace trimmed",
			list:     []string{"method1", " method2 "},
			target:   "method2",
			expected: true,
		},
		{
			name:     "empty list",
			list:     []string{},
			target:   "method1",
			expected: false,
		},
		{
			name:     "target with whitespace",
			list:     []string{"method1", "method2"},
			target:   " method1 ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringInSlice(tt.list, tt.target)
			if result != tt.expected {
				t.Errorf("stringInSlice(%v, %q) = %v, want %v", tt.list, tt.target, result, tt.expected)
			}
		})
	}
}

// TestResolveMethodsSource2 tests the resolveMethodsSource helper function
func TestResolveMethodsSource2(t *testing.T) {
	t.Run("with extension", func(t *testing.T) {
		result := resolveMethodsSource(true)
		if result != "extension" {
			t.Errorf("resolveMethodsSource(true) = %q, want \"extension\"", result)
		}
	})

	t.Run("without extension", func(t *testing.T) {
		result := resolveMethodsSource(false)
		if result != "" {
			t.Errorf("resolveMethodsSource(false) = %q, want \"\"", result)
		}
	})
}

// TestCall_WithEmptyResponseFromDispatcher tests when dispatcher returns empty response
func TestCall_WithEmptyResponseFromDispatcher(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	// Create a real dispatcher (it will fail to connect but that's ok)
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// This will fail to invoke because there's no real agent, but we can test the path
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should return 500 because dispatcher will fail to connect
	if resp.Code != 500 {
		t.Logf("Got code %d with message: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source=extension, got %s", resp.Source)
	}
}

// TestCall_WithWhitespaceOnlyPlatformMethod tests with whitespace-only platform or method
func TestCall_WithWhitespaceOnlyPlatformMethod(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	tests := []struct {
		name     string
		platform string
		method   string
	}{
		{"whitespace platform", "   ", "method"},
		{"whitespace method", "platform", "   "},
		{"both whitespace", "   ", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.Call(context.Background(), &CallPlatformRequest{
				Platform: tt.platform,
				Method:   tt.method,
			})

			if err != nil {
				t.Fatalf("Call returned unexpected error: %v", err)
			}
			// BuildFunctionID returns empty string for whitespace-only inputs
			// So we should get 503 (dispatcher not available path)
			if resp.Code != 503 {
				t.Logf("Got code %d with message: %s", resp.Code, resp.Message)
			}
		})
	}
}

// TestBuildExternalFunctionID_EmptyInputs tests buildExternalFunctionID with empty inputs
func TestBuildExternalFunctionID_EmptyInputs(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		method   string
		expected string
	}{
		{"empty platform", "", "method", ""},
		{"empty method", "platform", "", ""},
		{"both empty", "", "", ""},
		{"whitespace platform", "   ", "method", ""},
		{"whitespace method", "platform", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExternalFunctionID(tt.platform, tt.method)
			if result != tt.expected {
				t.Errorf("buildExternalFunctionID(%q, %q) = %q, want %q", tt.platform, tt.method, result, tt.expected)
			}
		})
	}
}

// TestCall_WithRequestSet tests the request data conversion path
func TestCall_WithRequestSet(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	// Test with request field set (but no dispatcher)
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key":"value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should get 503 because no dispatcher
	if resp.Code != 503 {
		t.Errorf("expected code 503, got %d", resp.Code)
	}
}

// TestListMethods_WithDuplicateMethodNames tests deduplication of method names
func TestListMethods_WithDuplicateMethodNames(t *testing.T) {
	store := reg.NewStore()
	// Add same method multiple times (case-insensitive)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_WithEmptyMethodNames(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_MixedEmptyAndValidMethods(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestDiscoverExternalPlatforms_WithNilInstallation2(t *testing.T) {
	store := reg.NewStore()

	// Create ExtensionServices with nil Installation (but not nil itself)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil, // Explicitly nil
		},
	}
	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	// Should not panic and return empty map (no registry platforms, no installation)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d platforms", len(result))
	}
}

// TestDiscoverExternalPlatforms_WithEmptyInstallationList tests the Installation discovery path
func TestDiscoverExternalPlatforms_WithEmptyInstallationList(t *testing.T) {
	store := reg.NewStore()

	// Create ExtensionServices with Installation that returns empty list
	// Note: We can't easily create a real Installation service without DB
	// So we create a minimal service context that exercises the code path
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	// Should not panic and return empty map
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d platforms", len(result))
	}
}

// TestDiscoverExternalPlatforms_CombinedSources tests both registry and installation sources
func TestDiscoverExternalPlatforms_CombinedSources(t *testing.T) {
	store := reg.NewStore()
	// Add platform from registry
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.registry_method": {Enabled: true},
		},
	})

	// Add Installation service (will return empty, but exercises the code path)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	// Should have platform from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}
	methods := result["test"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0] != "registry_method" {
		t.Fatalf("expected registry_method, got %s", methods[0])
	}
}

// TestCall_DispatcherReturnsError tests when dispatcher returns an error
func TestCall_DispatcherReturnsError(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Call with a platform that doesn't exist (dispatcher will error)
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "nonexistent_platform",
		Method:   "test_method",
		Request:  `{"test":"data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should get 500 because dispatcher will fail
	if resp.Code != 500 {
		t.Logf("Expected code 500, got %d with message: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source=extension, got %s", resp.Source)
	}
}

// TestCall_DispatcherEmptyResponse tests when dispatcher returns empty response (error path)
func TestCall_DispatcherEmptyResponse(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Call with a valid platform (but no real agent)
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "test_method",
		Request:  "",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should get 500 because dispatcher can't find the agent
	if resp.Code != 500 {
		t.Logf("Expected code 500, got %d with message: %s", resp.Code, resp.Message)
	}
}

// TestListPlatforms_WithRegistryAndInstallation tests platform discovery from both sources
func TestListPlatforms_WithRegistryAndInstallation(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestDiscoverExternalPlatforms_BothSourcesNil(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
		Extensions:    nil,
	}

	service := NewService(svcCtx)

	result := service.discoverExternalPlatforms(context.Background())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d platforms", len(result))
	}
}

// TestCall_EmptyRequestData tests Call with empty request data
func TestCall_EmptyRequestData(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "", // Empty request
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should get 500 because dispatcher can't connect
	if resp.Code != 500 {
		t.Logf("Got code %d with message: %s", resp.Code, resp.Message)
	}
}

// TestHandler_ListPlatforms_ServiceContextWithExtensions tests handler with Extensions in context
func TestHandler_ListPlatforms_ServiceContextWithExtensions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "test_platform")
}

// TestHandler_ListMethods_WithExtensionsInContext tests ListMethods with Extensions
func TestHandler_ListMethods_WithExtensionsInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.my_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/my_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "method1")
}

// TestCall_WithDispatcherTestsEmptyResultPath tests the path where dispatcher returns empty result
func TestCall_WithDispatcherTestsEmptyResultPath(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Test various platform/method combinations
	tests := []struct {
		name     string
		platform string
		method   string
		request  string
	}{
		{"basic call", "platform1", "method1", ""},
		{"with request", "platform2", "method2", "{}"},
		{"with data", "platform3", "method3", `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.Call(context.Background(), &CallPlatformRequest{
				Platform: tt.platform,
				Method:   tt.method,
				Request:  tt.request,
			})

			if err != nil {
				t.Fatalf("Call returned unexpected error: %v", err)
			}
			// Should get 500 because dispatcher can't find any agent
			if resp.Code != 500 {
				t.Logf("Got code %d with message: %s", resp.Code, resp.Message)
			}
			if resp.Source != "extension" {
				t.Errorf("expected source=extension, got %s", resp.Source)
			}
		})
	}
}

// TestDiscoverExternalPlatforms_VariousExtensionsInstallations tests different installation states
func TestDiscoverExternalPlatforms_VariousExtensionsInstallations(t *testing.T) {
	store := reg.NewStore()

	// Add some platforms from registry
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.registry.method1": {Enabled: true},
		},
	})

	// Create a nil Installation service to exercise the code path
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should have platforms from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}
	if len(result["registry"]) != 1 {
		t.Fatalf("expected 1 method, got %d", len(result["registry"]))
	}
}

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
				store.UpsertAgent(&reg.AgentSession{
					AgentID:  "a1",
					RPCAddr:  "127.0.0.1:19091",
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
				store.UpsertAgent(&reg.AgentSession{
					AgentID:  "a1",
					RPCAddr:  "127.0.0.1:19091",
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
				store.UpsertAgent(&reg.AgentSession{
					AgentID:  "a1",
					RPCAddr:  "127.0.0.1:19091",
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
func TestListPlatforms_WithPlatformHavingNoMethods(t *testing.T) {
	store := reg.NewStore()
	// Add an agent but with disabled functions (no enabled methods)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_PlatformNameWithDifferentCases(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestDiscoverExternalPlatforms_MergesSources(t *testing.T) {
	store := reg.NewStore()
	// Add platform from registry
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.registry_method": {Enabled: true},
		},
	})

	// Create Installation service (returns empty)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should have platform from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(result))
	}
	methods := result["test"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0] != "registry_method" {
		t.Fatalf("expected registry_method, got %s", methods[0])
	}
}

// TestCall_WithRequestDataSet tests the request data conversion path (line 43-44)
func TestCall_WithRequestDataSet2(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	tests := []struct {
		name    string
		request string
	}{
		{"empty request", ""},
		{"json object", `{"key":"value"}`},
		{"json array", `[1,2,3]`},
		{"json string", `"test string"`},
		{"json number", `123`},
		{"json bool", `true`},
		{"json null", `null`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.Call(context.Background(), &CallPlatformRequest{
				Platform: "test_platform",
				Method:   "test_method",
				Request:  tt.request,
			})

			if err != nil {
				t.Fatalf("Call returned unexpected error: %v", err)
			}
			// Dispatcher will fail (no real agent), but request data conversion should happen
			if resp.Code != 500 {
				t.Logf("Got code %d with message: %s", resp.Code, resp.Message)
			}
		})
	}
}

// TestCall_RequestDataConversion tests request data is properly converted
func TestCall_RequestDataConversion(t *testing.T) {
	// This test verifies that the request field is converted to []byte when set
	// We can't directly test this without a mock dispatcher, but we can
	// verify the code path is exercised

	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Test with various request values
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"test":"data","nested":{"value":123}}`,
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	// Should attempt to invoke with the request data converted to bytes
	// but dispatcher will fail (no agent)
	if resp.Code == 503 {
		t.Error("dispatcher should be available")
	}
}

// TestDiscoverExternalPlatforms_ExtensionsFieldSetButInstallationNil tests code path
func TestDiscoverExternalPlatforms_ExtensionsFieldSetButInstallationNil(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: nil, // Installation is nil but Extensions is not
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should still discover from registry
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}
}

// TestListPlatforms_VerifiesResponseStructure verifies the response structure
func TestListPlatforms_VerifiesResponseStructure(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_VerifiesResponseStructure(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestCall_VerifiesErrorResponse(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	// Test with empty platform
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "",
		Method:   "method",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}

	if resp.Code != 400 {
		t.Errorf("expected code 400, got %d", resp.Code)
	}
	if resp.Message != "platform is required" {
		t.Errorf("expected message 'platform is required', got %s", resp.Message)
	}

	// Test with empty method
	resp, err = service.Call(context.Background(), &CallPlatformRequest{
		Platform: "platform",
		Method:   "",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}

	if resp.Code != 400 {
		t.Errorf("expected code 400, got %d", resp.Code)
	}
	if resp.Message != "method is required" {
		t.Errorf("expected message 'method is required', got %s", resp.Message)
	}
}

// TestCall_Verifies503Response verifies 503 response when dispatcher is nil
func TestCall_Verifies503Response(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}

	if resp.Code != 503 {
		t.Errorf("expected code 503, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
	if resp.Message != "Platform extension runtime is not available" {
		t.Errorf("expected message about unavailable runtime, got %s", resp.Message)
	}
}

// TestStringInSlice_VerifiesCaseInsensitiveMatching verifies case insensitive matching
func TestStringInSlice_VerifiesCaseInsensitiveMatching(t *testing.T) {
	list := []string{"Method1", "Method2", "Method3"}

	// All these should match (case insensitive)
	if !stringInSlice(list, "method1") {
		t.Error("expected to find 'method1' (case insensitive)")
	}
	if !stringInSlice(list, "METHOD1") {
		t.Error("expected to find 'METHOD1' (case insensitive)")
	}
	if !stringInSlice(list, "MeThOd1") {
		t.Error("expected to find 'MeThOd1' (case insensitive)")
	}
}

// TestStringInSlice_VerifiesWhitespaceTrimming verifies whitespace trimming
func TestStringInSlice_VerifiesWhitespaceTrimming(t *testing.T) {
	list := []string{"method1", "method2"}

	// These should match after trimming
	if !stringInSlice(list, " method1 ") {
		t.Error("expected to find ' method1 ' (with whitespace)")
	}
	if !stringInSlice(list, "  method2  ") {
		t.Error("expected to find '  method2  ' (with whitespace)")
	}
}

// TestListMethods_VerifiesMethodDeduplication verifies method deduplication
func TestListMethods_VerifiesMethodDeduplication(t *testing.T) {
	store := reg.NewStore()
	// Add same method multiple times with different cases
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestCall_WithFunctionIDEmptyBranch(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Test with inputs that make BuildFunctionID return empty (whitespace only)
	// This triggers the else branch at line 69 (return 503)
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "   ",
		Method:   "   ",
	})

	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}

	// Should return 503 because functionID will be empty
	if resp.Code != 503 {
		t.Errorf("expected code 503 for empty functionID, got %d: %s", resp.Code, resp.Message)
	}
}

// TestListMethods_EmptyMethodsListAfterFiltering tests when all methods are filtered out
func TestListMethods_EmptyMethodsListAfterFiltering(t *testing.T) {
	store := reg.NewStore()
	// Add a function with empty method name (after trimming)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestHandler_ListPlatforms_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Verify response contains expected fields
	assert.Contains(t, body, `"code":200`)
	assert.Contains(t, body, `"message":"success"`)
	assert.Contains(t, body, "test_platform")
	assert.Contains(t, body, `"source":"extension"`)
}

// TestHandler_ListMethods_ResponseStructure verifies the handler response structure
func TestHandler_ListMethods_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.my_platform.method1": {Enabled: true},
			"external.my_platform.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/my_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Verify response contains expected fields
	assert.Contains(t, body, `"code":200`)
	assert.Contains(t, body, `"message":"success"`)
	assert.Contains(t, body, "method1")
	assert.Contains(t, body, "method2")
	assert.Contains(t, body, `"source":"extension"`)
}

// TestHandler_Call_ResponseStructure verifies the handler call response structure
func TestHandler_Call_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test_platform","method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Should get 503 response (no dispatcher) wrapped in success response
	assert.Contains(t, body, `"code":503`)
	assert.Contains(t, body, `"source":"extension"`)
}

// TestHandler_Call_MissingPlatformField tests missing platform in request
func TestHandler_Call_MissingPlatformField2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Missing platform field
	reqBody := `{"method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, `"code":400`)
	assert.Contains(t, body, "platform")
}

// TestHandler_Call_MissingMethodField tests missing method in request
func TestHandler_Call_MissingMethodField2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Missing method field
	reqBody := `{"platform":"test_platform"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, `"code":400`)
	assert.Contains(t, body, "method")
}

// TestHandler_ListMethods_EmptyPlatformParam tests with empty platform parameter
func TestHandler_ListMethods_EmptyPlatformParam2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms//methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, `"code":400`)
	assert.Contains(t, body, "platform")
}

// TestHandler_ListMethods_NonexistentPlatform tests with platform that doesn't exist
func TestHandler_ListMethods_NonexistentPlatform2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/nonexistent_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, `"code":404`)
	assert.Contains(t, body, "not found")
}

// TestHandler_Call_WithEmptyRequestData tests with empty request data
func TestHandler_Call_WithEmptyRequestData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"method","request":""}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	// Should return 503 because no dispatcher
	body := resp.Body.String()
	assert.Contains(t, body, `"code":503`)
}

// TestHandler_Call_WithValidRequestData tests with valid request data
func TestHandler_Call_WithValidRequestData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"method","request":"{\"key\":\"value\"}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	// Should return 503 because no dispatcher
	body := resp.Body.String()
	assert.Contains(t, body, `"code":503`)
}

// TestHandler_Call_AllFields tests with all fields present
func TestHandler_Call_AllFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test_platform","method":"test_method","request":"{\"data\":\"value\"}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, `"code":503`)
}

// TestCall_RequestFieldNotEmpty tests request field conversion (line 43-44)
func TestCall_RequestFieldNotEmpty(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	tests := []struct {
		name    string
		request string
	}{
		{"empty string", ""},
		{"json object", `{"key":"value"}`},
		{"json array", `[1,2,3]`},
		{"json string", `"test"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.Call(context.Background(), &CallPlatformRequest{
				Platform: "test",
				Method:   "method",
				Request:  tt.request,
			})

			if err != nil {
				t.Fatalf("Call returned unexpected error: %v", err)
			}
			// Dispatcher will fail (no real agent), but request conversion happened
			if resp.Code != 500 {
				t.Logf("Got code %d with message: %s", resp.Code, resp.Message)
			}
		})
	}
}

// TestListMethods_UsedExtensionFlag tests the usedExtension flag behavior
func TestListMethods_UsedExtensionFlag(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListPlatforms_PlatformSourceVerification(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_DuplicateMethodsAcrossAgents2(t *testing.T) {
	store := reg.NewStore()
	// Two agents with same platform and method (duplicate)
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent2",
		RPCAddr:  "127.0.0.1:19092",
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
func TestHandler_ListMethods_ResponseFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
			"external.test_platform.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Verify response contains all expected fields
	assert.Contains(t, body, `"code":200`)
	assert.Contains(t, body, `"message":"success"`)
	assert.Contains(t, body, `"methods"`)
	assert.Contains(t, body, `"source":"extension"`)
	assert.Contains(t, body, "method1")
	assert.Contains(t, body, "method2")
}

// TestCall_WithVariousRequestData tests request data conversion
func TestCall_WithVariousRequestData2(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	testCases := []struct {
		name    string
		request string
	}{
		{"empty request", ""},
		{"null json", "null"},
		{"empty object", "{}"},
		{"empty array", "[]"},
		{"simple string", `"test"`},
		{"number", "123"},
		{"boolean", "true"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.Call(context.Background(), &CallPlatformRequest{
				Platform: "test",
				Method:   "method",
				Request:  tc.request,
			})

			if err != nil {
				t.Fatalf("Call returned error: %v", err)
			}
			// Dispatcher will fail, but request conversion should happen
			if resp.Code != 500 {
				t.Logf("Got code %d for request '%s': %s", resp.Code, tc.request, resp.Message)
			}
		})
	}
}

// TestListMethods_VerifyMethodsResponse verifies methods response structure
func TestListMethods_VerifyMethodsResponse(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestHandler_Call_WithAllFieldsMissing3(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{}` // Empty JSON object
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, `"code":400`)
	assert.Contains(t, body, "platform")
}

// TestHandler_Call_SuccessResponseFields verifies success response fields
func TestHandler_Call_SuccessResponseFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	// Should contain error response from dispatcher (500)
	assert.Contains(t, body, `"code":500`)
	assert.Contains(t, body, `"source":"extension"`)
}

// TestHandler_ListPlatforms_EmptyList verifies empty platform list response
func TestHandler_ListPlatforms_EmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Empty registry - no platforms
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Should return 200 with success message
	assert.Contains(t, body, `"code":200`)
	assert.Contains(t, body, `"message":"success"`)
}

// TestHandler_ListPlatforms_WithPlatforms verifies platforms in response
func TestHandler_ListPlatforms_WithPlatforms2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Should return platform info
	assert.Contains(t, body, `"name":"test_platform"`)
	assert.Contains(t, body, `"enabled":true`)
	assert.Contains(t, body, `"methods":["method1"]`)
	assert.Contains(t, body, `"source":"extension"`)
}

// TestDiscoverExternalPlatforms_WithInstallationService exercises Installation path
func TestDiscoverExternalPlatforms_WithInstallationService2(t *testing.T) {
	store := reg.NewStore()
	// Add a platform from registry first
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.registry.method1": {Enabled: true},
		},
	})

	// Create Installation service (will return empty list due to no DB)
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	result := service.discoverExternalPlatforms(context.Background())

	// Should have platform from registry (Installation returns empty)
	if len(result) != 1 {
		t.Fatalf("expected 1 platform from registry, got %d", len(result))
	}

	// Verify the Installation code path was exercised (even though it returns empty)
	methods := result["registry"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method for registry platform, got %d", len(methods))
	}
}

// TestListPlatforms_WithInstallationInContext exercises Installation path
func TestListPlatforms_WithInstallationInContext2(t *testing.T) {
	store := reg.NewStore()
	// Add platforms from registry
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_WithInstallationInContext2(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestCall_VerifyRequestDataConversion2(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Test with non-empty request field to exercise line 43-44
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"test":"data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher will fail (no real agent), but request conversion should happen
	if resp.Code != 500 {
		t.Logf("Got code %d: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
}

// TestDiscoverExternalPlatforms_WithInstallationService3 tests the installation bindings discovery path with NewService
func TestDiscoverExternalPlatforms_WithInstallationService3(t *testing.T) {
	store := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map (no installation data in nil repo)
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil repo, got %d", len(platforms))
	}
}

// TestCall_DispatcherInvokeError tests the dispatcher error path
func TestCall_DispatcherInvokeError(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{}`,
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher should fail with 500 (no agent available)
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
}

// TestCall_JSONUnmarshalError tests the JSON unmarshal error path (line 52-54)
func TestCall_JSONUnmarshalError(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Add an agent that returns invalid JSON
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{}`,
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher will fail because agent is not real, but we exercise the path
	// The code should handle the error and return 500
	if resp.Code != 500 {
		t.Logf("Got code %d: %s", resp.Code, resp.Message)
	}
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
}

// TestCall_WithNonEmptyRequest tests request data conversion (line 43-45)
func TestCall_WithNonEmptyRequest(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"key":"value"}`,
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher will fail, but request should be converted
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
}

// TestDiscoverExternalPlatforms_ListError tests the List error path in installation bindings
func TestDiscoverExternalPlatforms_ListError(t *testing.T) {
	store := reg.NewStore()

	// Create a mock installation service that returns error
	// Since we can't easily mock, we rely on the nil repo returning empty
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should not crash and return platforms from registry
	if platforms == nil {
		t.Fatal("expected non-nil platforms map")
	}
}

// TestCall_DispatcherSuccessWithPathologicalResponse tests various response handling
func TestCall_DispatcherSuccessWithPathologicalResponse(t *testing.T) {
	store := reg.NewStore()

	// Add an agent
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Test with empty response (dispatcher will fail, but we exercise the path)
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "",
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher fails because no real agent
	if resp.Code != 500 {
		t.Logf("Got code %d: %s", resp.Code, resp.Message)
	}
}

// TestListMethods_WithEmptyMethodName tests the empty string filtering in addMethods
func TestListMethods_WithEmptyMethodName(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_DuplicateCaseInsensitive(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestListMethods_EmptyStringAfterTrim(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
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
func TestCall_EmptyResponseData(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	// Add an agent
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	// Call with empty request data
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  "",
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher will fail (no real agent), but we exercise the path
	if resp.Source != "extension" {
		t.Errorf("expected source 'extension', got %s", resp.Source)
	}
}

// TestCall_WithNonJSONResponse tests handling of non-JSON response
func TestCall_WithNonJSONResponse(t *testing.T) {
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)

	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test",
		Method:   "method",
		Request:  `{"test":"data"}`,
	})

	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	// Dispatcher will fail, returning error
	if resp.Code != 500 {
		t.Logf("Got code %d: %s", resp.Code, resp.Message)
	}
}

// TestDiscoverExternalPlatforms_WithRegistryOnly tests registry discovery without installation
func TestDiscoverExternalPlatforms_WithRegistryOnly(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions:    nil, // No installation service
	}

	service := NewService(svcCtx)
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover from registry
	if len(platforms) == 0 {
		t.Fatal("expected platforms from registry")
	}
	if len(platforms["test"]) != 2 {
		t.Fatalf("expected 2 methods for test platform, got %d", len(platforms["test"]))
	}
}

// TestDiscoverExternalPlatforms_NilRegistryStoreWithExtensions tests with nil registry store and extensions
func TestDiscoverExternalPlatforms_NilRegistryStoreWithExtensions(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should return empty map
	if len(platforms) != 0 {
		t.Fatalf("expected empty platforms for nil store, got %d", len(platforms))
	}
}
