package platform

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

type testLegacyGateway struct {
	callFn func(ctx context.Context, platform, method string, request []byte) ([]byte, error)
}

func (g *testLegacyGateway) Call(ctx context.Context, platform, method string, request []byte) ([]byte, error) {
	if g.callFn == nil {
		return nil, errors.New("call not implemented")
	}
	return g.callFn(ctx, platform, method, request)
}

func (g *testLegacyGateway) ListProviders() []legacyPlatformProviderInfo { return nil }

func (g *testLegacyGateway) ListMethods(platform string) ([]string, bool) { return nil, false }

func (g *testLegacyGateway) Reload(ctx context.Context) error { return nil }

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
			SpecJSON:    `{"provider":"onepanel","operations":["install_app","list_apps"]}`,
		},
		{
			BindingType: "openapi",
			BindingKey:  "helm",
			SpecJSON:    `{"name":"helm","operation":"install_chart"}`,
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
	if got := resolveMethodsSource(true, true); got != "mixed" {
		t.Fatalf("expected mixed, got=%s", got)
	}
	if got := resolveMethodsSource(true, false); got != "extension" {
		t.Fatalf("expected extension, got=%s", got)
	}
	if got := resolveMethodsSource(false, true); got != "legacy" {
		t.Fatalf("expected legacy, got=%s", got)
	}
}

func TestIsPlatformExtensionOnly(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "true")
	if !isPlatformExtensionOnly() {
		t.Fatalf("expected extension-only mode true")
	}
}

func TestIsPlatformLegacyDisabled(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "1")
	if !isPlatformLegacyDisabled() {
		t.Fatalf("expected legacy disabled=true")
	}
}

func TestAllowLegacyFallbackAfterExtensionError(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "")
	if !allowLegacyFallbackAfterExtensionError() {
		t.Fatalf("expected default true when env empty")
	}
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "false")
	if allowLegacyFallbackAfterExtensionError() {
		t.Fatalf("expected false when env=false")
	}
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "true")
	if !allowLegacyFallbackAfterExtensionError() {
		t.Fatalf("expected true when env=true")
	}
}

func TestCallExtensionOnlyNoDispatcher(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "true")
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
		t.Fatalf("expected code 503 in extension-only mode without dispatcher, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestCallLegacyDisabledNoDispatcher(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "true")
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
		t.Fatalf("expected code 503 in legacy-disabled mode without dispatcher, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension, got %s", resp.Source)
	}
}

func TestCallStrictNoFallbackOnExtensionError(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "false")

	store := reg.NewStore()
	service := NewService(&svc.ServiceContext{
		RegistryStore:  store,
		Dispatcher:     dispatch.NewDispatcher(store),
		PlatformLoader: nil,
	})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "onepanel",
		Method:   "list_apps",
		Request:  "{}",
	})
	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 500 {
		t.Fatalf("expected code 500 when strict no-fallback is enabled, got %d", resp.Code)
	}
	if resp.Source != "extension" {
		t.Fatalf("expected source=extension in strict mode, got %s", resp.Source)
	}
}

func TestReloadConfigExtensionOnly(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "true")
	service := NewService(&svc.ServiceContext{})
	resp, err := service.ReloadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReloadConfig returned unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true in extension-only mode")
	}
}

func TestReloadConfigLegacyDisabled(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "yes")
	service := NewService(&svc.ServiceContext{})
	resp, err := service.ReloadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReloadConfig returned unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true in legacy-disabled mode")
	}
}

func TestCallLegacyResponseNoFallbackFlag(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "true")

	svcObj := NewService(&svc.ServiceContext{Dispatcher: nil})
	svcObj.legacy = &testLegacyGateway{
		callFn: func(ctx context.Context, platform, method string, request []byte) ([]byte, error) {
			return []byte(`{"ok":true}`), nil
		},
	}
	resp, err := svcObj.Call(context.Background(), &CallPlatformRequest{
		Platform: "onepanel",
		Method:   "list_apps",
		Request:  "{}",
	})
	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 200 || resp.Source != "legacy" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Fallback {
		t.Fatalf("expected fallback=false when extension invoke not attempted")
	}
	if resp.FallbackReason != "" {
		t.Fatalf("expected empty fallback reason, got %s", resp.FallbackReason)
	}
}

func TestCallLegacyResponseWithFallbackFlag(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "true")

	store := reg.NewStore()
	svcObj := NewService(&svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatch.NewDispatcher(store),
	})
	svcObj.legacy = &testLegacyGateway{
		callFn: func(ctx context.Context, platform, method string, request []byte) ([]byte, error) {
			return []byte(`{"ok":true}`), nil
		},
	}
	resp, err := svcObj.Call(context.Background(), &CallPlatformRequest{
		Platform: "onepanel",
		Method:   "list_apps",
		Request:  "{}",
	})
	if err != nil {
		t.Fatalf("Call returned unexpected error: %v", err)
	}
	if resp.Code != 200 || resp.Source != "legacy" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !resp.Fallback {
		t.Fatalf("expected fallback=true when extension invoke failed then legacy succeeded")
	}
	if resp.FallbackReason != "extension_error" {
		t.Fatalf("unexpected fallback reason: %s", resp.FallbackReason)
	}
	raw, _ := json.Marshal(resp)
	if len(raw) == 0 {
		t.Fatalf("expected marshalable response")
	}
}
