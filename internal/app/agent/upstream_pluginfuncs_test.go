package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
)

// Plugin functions declared by function adapter plugins join the agent
// register snapshot (todo.md:35 remaining wiring) so the server-side
// registry/approval/audit chain sees them like SDK functions.

func TestBuildRegisterPayload_IncludesPluginFunctions(t *testing.T) {
	store := agentlocal.NewLocalStore()
	c := &UpstreamClient{store: store}
	c.SetPluginFunctionsSource(func() []*agentv1.FunctionDescriptor {
		return []*agentv1.FunctionDescriptor{
			{
				Id:      "plugin.httpget.fetch",
				Version: "0.1.0",
				Risk:    "safe",
				Enabled: true,
			},
			{
				Id:      "plugin.httpget.head",
				Version: "0.1.0",
				Risk:    "warning",
				Enabled: false,
			},
		}
	})

	req, err := c.buildRegisterPayload(context.Background())
	require.NoError(t, err)

	byID := map[string]*agentv1.FunctionDescriptor{}
	for _, f := range req.GetFunctions() {
		byID[f.GetId()] = f
	}
	require.Len(t, byID, 2)
	require.Contains(t, byID, "plugin.httpget.fetch")
	require.Contains(t, byID, "plugin.httpget.head")
}

func TestBuildRegisterPayload_PluginIdCollisionStoreWins(t *testing.T) {
	store := agentlocal.NewLocalStore()
	store.Register("ps-1", "svc-1", "127.0.0.1:19092", "2.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "plugin.httpget.fetch", Version: "2.0.0"},
	}, nil)

	c := &UpstreamClient{store: store}
	c.SetPluginFunctionsSource(func() []*agentv1.FunctionDescriptor {
		return []*agentv1.FunctionDescriptor{{Id: "plugin.httpget.fetch", Version: "1.0.0"}}
	})

	req, err := c.buildRegisterPayload(context.Background())
	require.NoError(t, err)

	var matches []*agentv1.FunctionDescriptor
	for _, f := range req.GetFunctions() {
		if f.GetId() == "plugin.httpget.fetch" {
			matches = append(matches, f)
		}
	}
	// Exactly one entry and it is the SDK store's higher version: the
	// plugin declaration must never shadow a live provider function.
	require.Len(t, matches, 1)
	require.Equal(t, "2.0.0", matches[0].GetVersion())
}

func TestBuildRegisterPayload_NilPluginSourceUnchanged(t *testing.T) {
	store := agentlocal.NewLocalStore()
	store.Register("ps-1", "svc-1", "127.0.0.1:19092", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "game.player.get", Version: "1.0.0"},
	}, nil)

	c := &UpstreamClient{store: store}
	req, err := c.buildRegisterPayload(context.Background())
	require.NoError(t, err)
	require.Len(t, req.GetFunctions(), 1)
	require.Equal(t, "game.player.get", req.GetFunctions()[0].GetId())
}
