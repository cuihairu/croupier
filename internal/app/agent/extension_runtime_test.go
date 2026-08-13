package agent

import (
	"testing"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func TestExtensionRuntimeApplyPayloadAndSnapshot(t *testing.T) {
	rt := NewExtensionRuntime()
	payload := &extensionsync.AgentSyncPayload{
		AgentID:     "agent-1",
		GeneratedAt: 100,
		Version:     "v1",
		Installations: []extensionsync.AgentInstallationPayload{
			{
				InstallationID:  1,
				InstallationKey: "ins-1",
				ExtensionID:     "official.analytics",
				ReleaseVersion:  "1.0.0",
				Enabled:         true,
				ScopeType:       "system",
				ScopeID:         "global",
				TargetType:      "agent_group",
				TargetID:        "default",
				ConfigJSON:      `{"enabled":true,"retry":3}`,
				SecretRefsJSON:  `{"token":"secret.analytics.token"}`,
				Bindings: []extensionsync.AgentBindingPayload{
					{
						BindingType: "function",
						BindingKey:  "analytics.query",
						TargetRef:   "agent:default",
						SpecJSON:    `{"timeout_ms":1000}`,
						Status:      "active",
					},
				},
			},
		},
	}

	res, err := rt.ApplyPayload(payload)
	if err != nil {
		t.Fatalf("apply payload failed: %v", err)
	}
	if res.Applied != 1 || res.Removed != 0 || res.Failed != 0 {
		t.Fatalf("unexpected apply result: %+v", res)
	}

	snap := rt.Snapshot()
	if snap.AgentID != "agent-1" || snap.Version != "v1" {
		t.Fatalf("unexpected snapshot meta: %+v", snap)
	}
	if len(snap.Installations) != 1 {
		t.Fatalf("unexpected installation count: %d", len(snap.Installations))
	}
	inst := snap.Installations[0]
	if inst.Config["retry"] != float64(3) {
		t.Fatalf("unexpected config decode: %+v", inst.Config)
	}
	if inst.SecretRefs["token"] != "secret.analytics.token" {
		t.Fatalf("unexpected secret refs decode: %+v", inst.SecretRefs)
	}
	if len(inst.Bindings) != 1 || inst.Bindings[0].Spec["timeout_ms"] != float64(1000) {
		t.Fatalf("unexpected bindings decode: %+v", inst.Bindings)
	}
}

func TestExtensionRuntimeReconcileRemovesMissing(t *testing.T) {
	rt := NewExtensionRuntime()
	_, err := rt.ApplyPayload(&extensionsync.AgentSyncPayload{
		AgentID:     "agent-1",
		GeneratedAt: 100,
		Version:     "v1",
		Installations: []extensionsync.AgentInstallationPayload{
			{InstallationID: 1, ExtensionID: "a", ConfigJSON: `{}`},
			{InstallationID: 2, ExtensionID: "b", ConfigJSON: `{}`},
		},
	})
	if err != nil {
		t.Fatalf("seed apply failed: %v", err)
	}

	res, err := rt.Reconcile(&extensionsync.AgentSyncPayload{
		AgentID:       "agent-1",
		GeneratedAt:   101,
		Version:       "v2",
		Installations: []extensionsync.AgentInstallationPayload{{InstallationID: 2, ExtensionID: "b", ConfigJSON: `{}`}},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if res.Applied != 1 || res.Removed != 1 {
		t.Fatalf("unexpected reconcile result: %+v", res)
	}
	snap := rt.Snapshot()
	if len(snap.Installations) != 1 || snap.Installations[0].InstallationID != 2 {
		t.Fatalf("unexpected snapshot after reconcile: %+v", snap.Installations)
	}
}

func TestExtensionRuntimeReload(t *testing.T) {
	rt := NewExtensionRuntime()
	if _, err := rt.Reload(); err == nil {
		t.Fatalf("expected reload error when no previous payload")
	}
	snap0 := rt.Snapshot()
	if snap0.LastApplyStatus != "unknown" {
		t.Fatalf("expected unknown initial status, got %s", snap0.LastApplyStatus)
	}
	_, err := rt.ApplyPayload(&extensionsync.AgentSyncPayload{
		AgentID:       "agent-1",
		GeneratedAt:   100,
		Version:       "v1",
		Installations: []extensionsync.AgentInstallationPayload{{InstallationID: 1, ExtensionID: "a", ConfigJSON: `{}`}},
	})
	if err != nil {
		t.Fatalf("seed apply failed: %v", err)
	}
	res, err := rt.Reload()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if res.Applied != 1 || res.Failed != 0 {
		t.Fatalf("unexpected reload result: %+v", res)
	}
	snap := rt.Snapshot()
	if snap.LastApplyStatus != "ok" {
		t.Fatalf("expected ok status after reload, got %s", snap.LastApplyStatus)
	}
}

func TestAppApplyExtensionSyncPayloadJSON(t *testing.T) {
	app := New("", "agent-1")
	res, err := app.ApplyExtensionSyncPayloadJSON([]byte(`{
		"agent_id":"agent-1",
		"generated_at":123,
		"version":"v1",
		"installations":[
			{
				"installationId":7,
				"extensionId":"official.analytics",
				"releaseVersion":"1.2.0",
				"configJson":"{}",
				"bindings":[
					{"bindingType":"function","bindingKey":"analytics.query","specJson":"{\"operation\":\"read\",\"category\":\"analytics\"}"},
					{"bindingType":"capability","bindingKey":"ops.alert","specJson":"{\"operations\":[\"list\",\"silence\"]}"}
				]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("apply payload json failed: %v", err)
	}
	if res.Applied != 1 {
		t.Fatalf("unexpected apply result: %+v", res)
	}
	snap := app.ExtensionRuntime().Snapshot()
	if len(snap.Installations) != 1 || snap.Installations[0].InstallationID != 7 {
		t.Fatalf("unexpected app runtime snapshot: %+v", snap.Installations)
	}
	data := app.Store().List()
	if _, ok := data["analytics.query"]; !ok {
		t.Fatalf("expected discovered function analytics.query in local store")
	}
	if _, ok := data["ops.alert.list"]; !ok {
		t.Fatalf("expected discovered capability operation ops.alert.list in local store")
	}
	if _, ok := data["ops.alert.silence"]; !ok {
		t.Fatalf("expected discovered capability operation ops.alert.silence in local store")
	}
}

func TestDiscoverExtensionFunctionsOperationBinding(t *testing.T) {
	out := discoverExtensionFunctions(RuntimeInstallation{
		ExtensionID:    "official.ops",
		ReleaseVersion: "1.0.0",
		Bindings: []RuntimeBinding{
			{
				BindingType: "operation",
				BindingKey:  "ops.node",
				Spec: map[string]any{
					"operation": "restart",
				},
			},
		},
	})
	if len(out) != 1 {
		t.Fatalf("expected one discovered function, got %d", len(out))
	}
	if out[0].GetId() != "ops.node.restart" {
		t.Fatalf("unexpected operation function id: %s", out[0].GetId())
	}
}

func TestDiscoverExternalPlatformFunctions(t *testing.T) {
	item := RuntimeInstallation{
		ExtensionID:    "official.external-platform",
		ReleaseVersion: "1.0.0",
		Bindings: []RuntimeBinding{
			{
				BindingType: "provider",
				BindingKey:  "onepanel",
				Spec: map[string]any{
					"provider":   "onepanel",
					"operations": []any{"list_apps", "install_app"},
				},
			},
		},
	}
	funcs := discoverExtensionFunctions(item)
	ids := map[string]bool{}
	byID := map[string]*sdkv1.ProviderFunctionDescriptor{}
	for _, f := range funcs {
		ids[f.GetId()] = true
		byID[f.GetId()] = f
	}
	if !ids["external.onepanel.list_apps"] {
		t.Fatalf("expected discovered external function external.onepanel.list_apps")
	}
	if !ids["external.onepanel.install_app"] {
		t.Fatalf("expected discovered external function external.onepanel.install_app")
	}
	desc := byID["external.onepanel.install_app"]
	if desc == nil {
		t.Fatalf("expected descriptor for external.onepanel.install_app")
	}
	if desc.GetResource() != "onepanel" {
		t.Fatalf("expected resource=onepanel, got %s", desc.GetResource())
	}
	if desc.GetOperation() != "install_app" {
		t.Fatalf("expected operation=install_app, got %s", desc.GetOperation())
	}
	hasCapabilityTag := false
	for _, tag := range desc.GetTags() {
		if tag == "capability:external.onepanel" {
			hasCapabilityTag = true
			break
		}
	}
	if !hasCapabilityTag {
		t.Fatalf("expected capability tag in descriptor tags: %+v", desc.GetTags())
	}
}

func TestExtensionRuntimeRecordError(t *testing.T) {
	rt := NewExtensionRuntime()
	rt.RecordError(assertErr("boom"))
	snap := rt.Snapshot()
	if snap.LastApplyStatus != "error" {
		t.Fatalf("expected error status, got %s", snap.LastApplyStatus)
	}
	if snap.LastError == "" {
		t.Fatalf("expected last error message")
	}
	if snap.LastErrorAt <= 0 {
		t.Fatalf("expected last error timestamp")
	}
}

func TestBuildExtensionProviderEntries(t *testing.T) {
	snap := ExtensionRuntimeSnapshot{
		Installations: []RuntimeInstallation{
			{
				ExtensionID: "official.external-platform",
				Bindings: []RuntimeBinding{
					{
						BindingType: "provider",
						BindingKey:  "onepanel",
						Spec: map[string]any{
							"provider":   "onepanel",
							"type":       "openapi",
							"base_url":   "http://127.0.0.1:8080",
							"operations": []any{"install_app", "list_app"},
						},
					},
				},
			},
		},
	}
	entries := buildExtensionProviderEntries(snap)
	entry, ok := entries["onepanel"]
	if !ok {
		t.Fatalf("expected onepanel provider entry")
	}
	if entry.Type != "openapi" {
		t.Fatalf("unexpected provider type: %s", entry.Type)
	}
	if entry.Config["base_url"] != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected provider config base_url: %+v", entry.Config)
	}
	methods, ok := entry.Config["methods"].([]map[string]any)
	if !ok || len(methods) != 2 {
		t.Fatalf("expected generated methods from operations, got: %#v", entry.Config["methods"])
	}
}

func TestResolveFunctionDriver_ExternalFunctionUsesOpenAPI(t *testing.T) {
	driver := resolveFunctionDriver(RuntimeInstallation{
		Bindings: []RuntimeBinding{
			{
				BindingType: "provider",
				BindingKey:  "onepanel",
			},
		},
	}, "external.onepanel.install_app")
	if driver != "openapi-driver" {
		t.Fatalf("expected openapi-driver, got %s", driver)
	}
}

func TestAppApplyExternalPlatformBindingsRegistersFunctions(t *testing.T) {
	app := New("", "agent-ext-platform")
	_, err := app.ApplyExtensionSyncPayloadJSON([]byte(`{
		"agent_id":"agent-ext-platform",
		"generated_at":123,
		"version":"v1",
		"installations":[
			{
				"installationId":88,
				"extensionId":"official.external-platform",
				"releaseVersion":"1.0.0",
				"configJson":"{}",
				"bindings":[
					{
						"bindingType":"provider",
						"bindingKey":"acmeops",
						"specJson":"{\"provider\":\"acmeops\",\"operations\":[\"invoke\",\"sync_status\"]}"
					}
				]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("apply payload json failed: %v", err)
	}
	store := app.Store().List()
	if _, ok := store["external.acmeops.invoke"]; !ok {
		t.Fatalf("expected registered function external.acmeops.invoke")
	}
	if _, ok := store["external.acmeops.sync_status"]; !ok {
		t.Fatalf("expected registered function external.acmeops.sync_status")
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
