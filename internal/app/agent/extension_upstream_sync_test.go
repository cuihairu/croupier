package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	controlnng "github.com/cuihairu/croupier/internal/nng"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestExtensionRuntimeFunctionsReportedToUpstream(t *testing.T) {
	t.Parallel()

	controlAddr := reserveTCPAddr(t)
	registryStore := reg.NewStore()
	controlServer := controlnng.NewServer(controlAddr, registryStore)
	if err := controlServer.Start(); err != nil {
		t.Fatalf("start control server failed: %v", err)
	}
	defer func() {
		if err := controlServer.Stop(); err != nil {
			t.Fatalf("stop control server failed: %v", err)
		}
	}()

	app := New(controlAddr, "agent-ext-1")
	if _, err := app.ApplyExtensionSyncPayload(&extensionsync.AgentSyncPayload{
		AgentID:     "agent-ext-1",
		GeneratedAt: time.Now().Unix(),
		Version:     "v1",
		Installations: []extensionsync.AgentInstallationPayload{
			{
				InstallationID: 1,
				ExtensionID:    "official.analytics",
				ReleaseVersion: "1.0.0",
				ConfigJSON:     `{}`,
				Bindings: []extensionsync.AgentBindingPayload{
					{BindingType: "function", BindingKey: "analytics.query"},
					{BindingType: "capability", BindingKey: "ops.alert", SpecJSON: `{"operations":["list"]}`},
				},
			},
		},
	}); err != nil {
		t.Fatalf("apply extension payload failed: %v", err)
	}
	app.WithUpstreamMetadata(UpstreamMetadata{
		GameID:  "g1",
		Env:     "dev",
		Version: "1.0.0",
		RPCAddr: "127.0.0.1:19091",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.upstream.dialServer(ctx); err != nil {
		t.Fatalf("dial upstream failed: %v", err)
	}
	defer app.upstream.Stop()

	registryStore.Mu().RLock()
	defer registryStore.Mu().RUnlock()
	agent := registryStore.AgentsUnsafe()["agent-ext-1"]
	if agent == nil {
		t.Fatalf("agent session not found in registry")
	}
	if _, ok := agent.Functions["analytics.query"]; !ok {
		t.Fatalf("analytics.query not reported, got functions=%s", functionKeys(agent.Functions))
	}
	if _, ok := agent.Functions["ops.alert.list"]; !ok {
		t.Fatalf("ops.alert.list not reported, got functions=%s", functionKeys(agent.Functions))
	}
}

func functionKeys(m map[string]reg.FunctionMeta) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return fmt.Sprint(out)
}
