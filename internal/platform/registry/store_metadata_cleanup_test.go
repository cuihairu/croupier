package registry

// #11 回归（过期会话清理路径）：cleanupExpiredSessions 是包私有方法，
// 且本用例需要直接建 provider_metadata 表（外部测试包建表走 model，包内
// 不能 import model——用包内匿名 row struct 建表，列集即镜像本体）。

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStore_cleanupExpiredSessionsPurgesProviderMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta-cleanup.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AgentSessionDB{}, &providerMetadataRow{}))

	store := NewStoreWithDB(db)
	expired := time.Now().Add(-time.Minute)
	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-a",
		Env:       "dev",
		ExpireAt:  expired,
		Providers: []ProviderSession{{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}}},
	}))

	require.NotEmpty(t, store.ProviderMetaOptions("game-a", "dev"))

	store.cleanupExpiredSessions()

	items := store.ProviderMetaOptions("game-a", "dev")
	assert.Empty(t, metaOptionValuesForTest(items, "serverId"), "expired session metadata rows must be purged")
}

func metaOptionValuesForTest(items []ProviderMetaKeyOption, key string) []ProviderMetaValueOption {
	for _, item := range items {
		if item.Key == key {
			return item.Values
		}
	}
	return nil
}
