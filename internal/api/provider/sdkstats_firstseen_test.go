// SdkStats instance registration-time regression (#44): firstSeenUnix is
// normalized by the server before being surfaced -- an observed value passes
// through unchanged, a zero value (old snapshots / history lost after restart)
// falls back to lastSeenUnix, and the frontend need not check for zero.
// Same semantics as openapi #27② (same shape as runtime_sources_test.go).
package provider

import (
	"context"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceSdkStats_FirstSeenUnixNormalized(t *testing.T) {
	store := reg.NewStore()
	now := time.Now()
	firstSeen := now.Add(-2 * time.Hour).Unix()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-fs",
		GameID:   "game-fs",
		Env:      "dev",
		LastSeen: now,
		Providers: []reg.ProviderSession{
			{
				ProviderID:    "veteran-provider",
				SDKLanguage:   "go",
				SDKVersion:    "1.4.0",
				FirstSeenUnix: firstSeen,
				LastSeenUnix:  now.Unix(),
			},
			{
				// Instance without a registration time (old snapshot /
				// persisted session recovery): the registration chain fills in
				// now, and after server-side normalization it must be non-zero
				// and >= lastSeen
				ProviderID:   "fresh-provider",
				SDKLanguage:  "js",
				SDKVersion:   "1.5.0",
				LastSeenUnix: now.Unix(),
			},
		},
	}))

	s := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 2)

	byID := map[string]SdkInstanceItem{}
	for _, item := range resp.Instances {
		byID[item.ProviderID] = item
	}

	veteran := byID["veteran-provider"]
	assert.Equal(t, firstSeen, veteran.FirstSeenUnix, "有观测值原样透传")
	assert.Equal(t, now.Unix(), veteran.LastSeenUnix)

	fresh := byID["fresh-provider"]
	assert.GreaterOrEqual(t, fresh.FirstSeenUnix, fresh.LastSeenUnix,
		"零值归一后注册时间不为零（注册链补 now / 服务端回退 lastSeen）")
}
