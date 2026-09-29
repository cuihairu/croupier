// 覆盖率巡检第十七轮（wt-api）：service.go 残余 2 块收口——
// SdkStats 的 firstSeen <= 0 回退翼（217-218）与 MetaOptions 的
// items nil → 空切片翼（254-255，DB 聚合失败 fail-soft）。
// 注册链（registry carryProviderSessionHistory）只把 first == 0 补成 now，
// 负值穿透——持久化会话恢复出非法负值形态时，服务端仍须回退 lastSeen。
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
				// 负值不被注册链归一（只补零），穿透到服务端须回退 lastSeen
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
	// DB 聚合失败（provider_metadata 缺表）时 store 返回 nil：
	// 服务层须归一为空切片——下拉选项是 fail-soft 契约（聚合故障只损失
	// 选项、不报错不透出 null）。内存聚合路径恒返回非 nil 空切片，
	// 该翼只能经 DB 形态触达。
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
