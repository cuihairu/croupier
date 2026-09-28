// SdkStats 实例注册时间回归（#44）：firstSeenUnix 由服务端归一透出——
// 有观测值原样透传，零值（旧快照/重启后历史缺失）回退 lastSeenUnix，
// 前端无需判零值。语义同 openapi #27②（runtime_sources_test.go 同款形态）。
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
				// 未带注册时间的实例（旧快照/持久化会话恢复）：
				// 注册链会补 now，服务端归一后须为非零且 >= lastSeen
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
