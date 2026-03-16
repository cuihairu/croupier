package platform

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
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
