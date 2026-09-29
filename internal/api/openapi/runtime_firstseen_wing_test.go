// 覆盖率巡检第十七轮（wt-api）：service.go RuntimeSources 的
// firstSeen <= 0 回退翼（330-331）收口。注册链只把 first == 0 补成 now，
// 负值穿透——内存快照恢复出非法负值形态时仍须回退 lastSeen（#27② 归一）。
// 710 的 "fn-" 前缀翼维持 coverage_f_test.go 既有不可达登记，不重复处理。
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
				// 负值不被注册链归一（只补零），穿透到服务端须回退 lastSeen
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
