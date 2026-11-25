// SdkStats instance metadata regression: metadata passes through to the
// instance details, plus server-side metaKey/metaValue substring filtering (the
// value condition also matches the key name; key+value must hit the same KV
// pair).
package provider

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMetadataStatsStore() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-meta",
		GameID:  "game-meta",
		Env:     "dev",
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "game-demo",
				SDKLanguage: "go",
				SDKVersion:  "1.4.0",
				Metadata:    map[string]string{"serverId": "s1", "pod": "game-7c4d"},
			},
			{
				ProviderID:  "prom-adapter",
				SDKLanguage: "js",
				SDKVersion:  "1.5.0",
				Metadata:    map[string]string{"serverId": "s9"},
			},
			{
				ProviderID:  "bare-adapter",
				SDKLanguage: "python",
				SDKVersion:  "0.1.0",
			},
		},
	}); err != nil {
		panic(err)
	}
	return store
}

func findInstanceItem(t *testing.T, items []SdkInstanceItem, providerID string) SdkInstanceItem {
	t.Helper()
	for _, item := range items {
		if item.ProviderID == providerID {
			return item
		}
	}
	t.Fatalf("instance %q not found", providerID)
	return SdkInstanceItem{}
}

func TestServiceSdkStats_MetadataPassthrough(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 3)

	demo := findInstanceItem(t, resp.Instances, "game-demo")
	assert.Equal(t, map[string]string{"serverId": "s1", "pod": "game-7c4d"}, demo.Metadata)
	// An instance without metadata has a nil field (wire omitempty does not
	// emit an empty map)
	assert.Nil(t, findInstanceItem(t, resp.Instances, "bare-adapter").Metadata)
	assert.Equal(t, 3, resp.TotalInstances)
}

func TestServiceSdkStats_MetaKeyFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})

	// metaKey=serverid (case-insensitive) -> the two instances with serverId
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "ServerID "})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"game-demo", "prom-adapter"}, providerIDs(resp.Instances))

	// metaKey=pod -> game-demo only
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "pod"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))

	// metaKey with no hit -> empty result
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "region"})
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
	assert.Equal(t, 0, resp.TotalInstances)
}

func TestServiceSdkStats_MetaValueFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})

	// Pasting just the value s1 -> game-demo
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{MetaValue: "s1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))

	// The value condition also matches the key name: pasting "pod" still hits
	// the instance that has a pod key
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaValue: "POD"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))

	// A whole k=v pair as the value also matches (isomorphic with the frontend
	// haystack)
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaValue: "serverId=s9"})
	require.NoError(t, err)
	assert.Equal(t, []string{"prom-adapter"}, providerIDs(resp.Instances))
}

func TestServiceSdkStats_MetaKeyAndValueMustHitSamePair(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})

	// Both key and value exist but are spread across two KV pairs -> no hit
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "serverId", MetaValue: "game-7c4d"})
	require.NoError(t, err)
	assert.Empty(t, providerIDs(resp.Instances))

	// The same KV pair hits both conditions -> game-demo
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "serverId", MetaValue: "s1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))
}

func TestMatchMetadata_SubstringSemantics(t *testing.T) {
	// Empty conditions are always true; an instance without metadata is always
	// false against any condition
	assert.True(t, matchMetadata(nil, "", ""))
	assert.False(t, matchMetadata(nil, "k", ""))
	assert.False(t, matchMetadata(map[string]string{}, "", "v"))

	md := map[string]string{"serverId": "s1"}
	assert.True(t, matchMetadata(md, "server", "1"))
	assert.True(t, matchMetadata(md, "SERVERID", "S1"))
	assert.False(t, matchMetadata(md, "serverId", "s2"))
}

func providerIDs(items []SdkInstanceItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ProviderID)
	}
	return out
}
