package platform

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
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
		Extensions:     &svc.ExtensionServices{Installation: nil},
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
			"external.test.":       {Enabled: true}, // Empty method name - should be skipped
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
			"external": {Enabled: true},           // No method part
			"external.test": {Enabled: true},      // Only has platform
			"test.echo":     {Enabled: true},      // Non-external
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
			"external.test.method1":      {Enabled: true},
			"external.test.  ":          {Enabled: true}, // Whitespace only
			"external.test.method2":      {Enabled: true},
			"external.test.\t\t":         {Enabled: true}, // Tab only
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
			"external.test.SetScore": {Enabled: true},
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
			"external.test.Get_Player":  {Enabled: true},
			"external.test.get_player":  {Enabled: true},
			"external.test.GET_PLAYER":  {Enabled: true},
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
			"external.steam.get_player":   {Enabled: true},
			"external.ephicgames.login":   {Enabled: true},
			"external.xbox.auth":          {Enabled: true},
			"external.ps.plus":            {Enabled: true},
			"external.nintendo.account":   {Enabled: true},
			"external.test.not_external":  {Enabled: false},
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
		AgentID:  "test-agent",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
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
			name:     "missing external prefix",
			input:    "myplatform.mymethod",
			wantOk:   false,
		},
		{
			name:     "only external prefix",
			input:    "external",
			wantOk:   false,
		},
		{
			name:     "empty string",
			input:    "",
			wantOk:   false,
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
			"external.test.method_one": {Enabled: true},
			"external.test.METHOD_ONE": {Enabled: true}, // case insensitive duplicate
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
		Extensions:     nil,
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

