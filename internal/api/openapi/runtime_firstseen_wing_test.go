// Coverage sweep round 17 (wt-api): close out the firstSeen <= 0 fallback wing
// of RuntimeSources in service.go (330-331). The registration chain only fills
// in now for first == 0, so negative values pass through — when an in-memory
// snapshot restores an invalid negative shape it must still fall back to
// lastSeen (#27 phase 2 normalization). The "fn-" prefix wing at 710 keeps its
// existing unreachable registration in coverage_f_test.go and is not handled here.
package openapi

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeSources_FirstSeenNegativeFallsBackToLastSeen(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "openapi-fs-neg-agent",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"players.list": {Version: "openapi-neg-1.0"},
		},
		Providers: []registry.ProviderSession{
			{
				// Negative values are not normalized by the registration chain
				// (it only fills zeros); they reach the service and must fall
				// back to lastSeen
				ProviderID:    "provider:legacy-neg",
				Version:       "openapi-neg-1.0",
				FirstSeenUnix: -1,
				LastSeenUnix:  now.Unix(),
				FunctionIDs:   []string{"players.list"},
			},
		},
		LastSeen: now,
	}))

	resp, err := service.RuntimeSources(ctx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, now.Unix(), resp.Items[0].FirstSeenUnix,
		"firstSeen <= 0 时回退 lastSeenUnix（#27② 服务端归一）")
}
