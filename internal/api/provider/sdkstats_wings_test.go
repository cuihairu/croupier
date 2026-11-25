// Coverage sweep round 17 (wt-api): closes out the 2 remaining blocks in
// service.go -- the SdkStats firstSeen <= 0 fallback wing (217-218) and the
// MetaOptions items nil -> empty slice wing (254-255, fail-soft on DB
// aggregation failure). The registration chain (registry
// carryProviderSessionHistory) only fills first == 0 with now; negative values
// pass through -- when a persisted session recovers into an illegal negative
// form, the server must still fall back to lastSeen.
package provider

import (
	"context"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestServiceSdkStats_FirstSeenNegativeFallsBackToLastSeen(t *testing.T) {
	store := reg.NewStore()
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-fs-neg",
		GameID:   "game-fs-neg",
		Env:      "dev",
		LastSeen: now,
		Providers: []reg.ProviderSession{
			{
				// Negative values are not normalized by the registration chain
				// (only zero is filled), so they pass through and the server
				// must fall back to lastSeen
				ProviderID:    "neg-provider",
				SDKLanguage:   "go",
				SDKVersion:    "1.4.0",
				FirstSeenUnix: -1,
				LastSeenUnix:  now.Unix(),
			},
		},
	}))

	s := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)
	assert.Equal(t, now.Unix(), resp.Instances[0].FirstSeenUnix,
		"firstSeen <= 0 时回退 lastSeenUnix（#44 服务端归一）")
}

func TestServiceMetaOptions_AggregateNilToEmptySlice(t *testing.T) {
	// When DB aggregation fails (provider_metadata table missing), the store
	// returns nil: the service layer must normalize it to an empty slice --
	// the dropdown options are a fail-soft contract (an aggregation fault only
	// loses options, it must not error out or surface null). The in-memory
	// aggregation path always returns a non-nil empty slice, so this wing is
	// only reachable through the DB form.
	db, err := gorm.Open(gsqlite.Open("file:provider_meta_nil_wing?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	store := reg.NewStoreWithDB(db)
	s := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := s.MetaOptions(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-meta-broken", Env: "dev"}))
	require.NoError(t, err)
	require.NotNil(t, resp.Items, "nil 聚合须归一为空切片，不得透出 null")
	assert.Empty(t, resp.Items)
}
