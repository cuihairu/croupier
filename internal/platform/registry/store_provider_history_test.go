package registry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/registry"
)

// findSnapshot 按 providerID 取快照。
func findSnapshot(t *testing.T, snapshots []registry.ProviderSessionSnapshot, providerID string) registry.ProviderSessionSnapshot {
	t.Helper()
	for _, s := range snapshots {
		if s.ProviderID == providerID {
			return s
		}
	}
	t.Fatalf("provider %q not found in %d snapshots", providerID, len(snapshots))
	return registry.ProviderSessionSnapshot{}
}

// #27②：provider 首次注册时服务端打点导入时间（FirstSeenUnix），版本高
// 水位（VersionHWM）取当次 Version。均为内存态（会话过期/重启即失）。
func TestProviderSessionHistory_FirstRegistrationStampsFirstSeenAndHWM(t *testing.T) {
	s := registry.NewStore()
	before := time.Now().Unix()
	require.NoError(t, s.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Providers: []registry.ProviderSession{{
			ProviderID: "provider:players",
			Version:    "1.2.0",
		}},
	}))
	after := time.Now().Unix()

	snapshots := s.ProviderSessionSnapshots()
	require.Len(t, snapshots, 1)
	snap := findSnapshot(t, snapshots, "provider:players")
	assert.GreaterOrEqual(t, snap.FirstSeenUnix, before)
	assert.LessOrEqual(t, snap.FirstSeenUnix, after)
	assert.Equal(t, "1.2.0", snap.VersionHWM)
}

// #27②：重复注册（心跳/重连）必须继承历史——FirstSeenUnix 不被新观测
// 冲掉；VersionHWM 走高不回退（低版本注册后 HWM 仍为此前最高值），当次
// Version 如实反映最新注册。
func TestProviderSessionHistory_ReregistrationCarriesFirstSeenAndHWM(t *testing.T) {
	s := registry.NewStore()
	require.NoError(t, s.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Providers: []registry.ProviderSession{{
			ProviderID: "provider:players",
			Version:    "1.2.0",
		}},
	}))
	firstSeen := findSnapshot(t, s.ProviderSessionSnapshots(), "provider:players").FirstSeenUnix
	require.Greater(t, firstSeen, int64(0))

	// 重连：回退版本 1.1.0，且为 provider:ops 新增会话
	require.NoError(t, s.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Providers: []registry.ProviderSession{
			{ProviderID: "provider:players", Version: "1.1.0"},
			{ProviderID: "provider:ops", Version: "0.9.0"},
		},
	}))

	snapshots := s.ProviderSessionSnapshots()
	require.Len(t, snapshots, 2)
	players := findSnapshot(t, snapshots, "provider:players")
	assert.Equal(t, firstSeen, players.FirstSeenUnix, "FirstSeenUnix 必须跨注册继承")
	assert.Equal(t, "1.1.0", players.Version, "当次 Version 反映最新注册")
	assert.Equal(t, "1.2.0", players.VersionHWM, "VersionHWM 走高不回退")

	// 再次注册升到 1.3.0：HWM 抬升，FirstSeen 仍不变
	require.NoError(t, s.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Providers: []registry.ProviderSession{
			{ProviderID: "provider:players", Version: "1.3.0"},
		},
	}))
	players = findSnapshot(t, s.ProviderSessionSnapshots(), "provider:players")
	assert.Equal(t, firstSeen, players.FirstSeenUnix)
	assert.Equal(t, "1.3.0", players.VersionHWM)
}

// #27②：wire 侧若显式携带更早的 FirstSeenUnix（未来 SDK 上报真实首见
// 时间），以更早者为准；不可解析版本（unknown/垃圾）永不入选 HWM，全部
// 不可解析时保持空（展示层回退 Version）。
func TestProviderSessionHistory_EarlierFirstSeenWinsAndUnparseableNeverHWM(t *testing.T) {
	s := registry.NewStore()
	early := time.Now().Add(-24 * time.Hour).Unix()
	require.NoError(t, s.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Providers: []registry.ProviderSession{{
			ProviderID:    "provider:players",
			Version:       "unknown",
			FirstSeenUnix: early,
		}},
	}))

	snap := findSnapshot(t, s.ProviderSessionSnapshots(), "provider:players")
	assert.Equal(t, early, snap.FirstSeenUnix, "显式更早的首见时间优先于服务端打点")
	assert.Empty(t, snap.VersionHWM, "不可解析版本不进高水位")
	assert.Equal(t, "unknown", snap.Version)
}
