// #38 regression: SdkStats filters instances by the game scope injected by
// middleware. SDKs are bound to a game, and the page only sees the (game, env)
// selected in the top bar; when the scope is missing the full set is kept
// (existing behavior of direct service callers / internal consumers).
package provider

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newScopedStatsStore() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-a",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []reg.ProviderSession{
			{ProviderID: "a-go", SDKLanguage: "go", SDKVersion: "1.4.0"},
			{ProviderID: "a-js", SDKLanguage: "js", SDKVersion: "1.5.0"},
		},
	}); err != nil {
		panic(err)
	}
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-b",
		GameID:  "game-b",
		Env:     "prod",
		Providers: []reg.ProviderSession{
			{ProviderID: "b-py", SDKLanguage: "python", SDKVersion: "0.9.0"},
		},
	}); err != nil {
		panic(err)
	}
	return store
}

func sdkLanguageNames(resp *SdkStatsResponse) []string {
	out := make([]string, 0, len(resp.Languages))
	for _, item := range resp.Languages {
		out = append(out, item.Language)
	}
	return out
}

func TestServiceSdkStats_ScopeFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newScopedStatsStore()})

	// No scope -> full set (historical behavior; other direct service callers
	// are unaffected)
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 3, resp.TotalInstances)

	// scope=game-a/dev -> only that game's instances; the total and language
	// aggregates narrow accordingly
	resp, err = s.SdkStats(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "dev"}),
		&SdkStatsRequest{},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"a-go", "a-js"}, providerIDs(resp.Instances))
	assert.Equal(t, 2, resp.TotalInstances)
	assert.ElementsMatch(t, []string{"go", "js"}, sdkLanguageNames(resp))

	// Same game but a different env -> filtered out too (game-a has no prod
	// sessions)
	resp, err = s.SdkStats(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"}),
		&SdkStatsRequest{},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
	assert.Equal(t, 0, resp.TotalInstances)
}

func TestServiceSdkStats_ScopeFilterWithMetaCondition(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newScopedStatsStore()})

	// The scope filter stacks with the metaKey/metaValue filter: game-b has
	// no serverId=s1
	resp, err := s.SdkStats(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"}),
		&SdkStatsRequest{MetaKey: "serverId", MetaValue: "s1"},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

// #2/#11 regression: MetaOptions aggregates deduplicated key/value pairs of
// instance metadata under the game scope injected by middleware (dropdown
// options are supplied by the server); when the scope is missing the full set
// is kept.
func TestServiceMetaOptions_ScopeFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newScopedMetaStore()})

	resp, err := s.MetaOptions(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "dev"}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "serverId", resp.Items[0].Key)
	require.Len(t, resp.Items[0].Values, 1)
	assert.Equal(t, "s-a", resp.Items[0].Values[0].Value)
	assert.Equal(t, 1, resp.Items[0].Values[0].Count)

	// No scope -> full aggregation (one value per game).
	resp, err = s.MetaOptions(context.Background())
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Len(t, resp.Items[0].Values, 2)
}

func newScopedMetaStore() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-a",
		GameID:    "game-a",
		Env:       "dev",
		Providers: []reg.ProviderSession{{ProviderID: "a-go", Metadata: map[string]string{"serverId": "s-a"}}},
	}); err != nil {
		panic(err)
	}
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-b",
		GameID:    "game-b",
		Env:       "prod",
		Providers: []reg.ProviderSession{{ProviderID: "b-py", Metadata: map[string]string{"serverId": "s-b"}}},
	}); err != nil {
		panic(err)
	}
	return store
}
