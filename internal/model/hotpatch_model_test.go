package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var hotpatchSeq int

func setupHotpatchDB(t *testing.T) *gorm.DB {
	t.Helper()
	hotpatchSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:hp%d?mode=memory&cache=shared", hotpatchSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Hotpatch{}))
	return db
}

func seedHotpatch(t *testing.T, db *gorm.DB, status, version string) *Hotpatch {
	t.Helper()
	hp := &Hotpatch{
		GameID: "demo", Env: "prod", Framework: "skynet",
		Status: status, PackageKey: "key-" + version, BugID: 1,
	}
	require.NoError(t, db.Create(hp).Error)
	return hp
}

func TestHotpatchModelCRUDAndTransition(t *testing.T) {
	db := setupHotpatchDB(t)
	m := NewHotpatchModel(db)
	ctx := context.Background()

	hp := seedHotpatch(t, db, "draft", "1.0.0")

	// FindOne
	got, err := m.FindOne(ctx, hp.ID)
	require.NoError(t, err)
	assert.Equal(t, "draft", got.Status)

	// Update
	require.NoError(t, m.Update(ctx, hp.ID, map[string]interface{}{"target_selector": "[]"}))

	// Transition 合法链：draft → approved → rolling
	upgraded, err := m.Transition(ctx, hp.ID, "approved", nil)
	require.NoError(t, err)
	assert.Equal(t, "approved", upgraded.Status)

	upgraded, err = m.Transition(ctx, hp.ID, "rolling", nil)
	require.NoError(t, err)
	assert.Equal(t, "rolling", upgraded.Status)

	// 非法迁移
	_, err = m.Transition(ctx, hp.ID, "draft", nil)
	assert.Error(t, err)

	// List
	seedHotpatch(t, db, "testing", "1.1.0")
	list, total, err := m.List(ctx, HotpatchQueryOptions{GameID: "demo", Env: "prod", PaginationOptions: PaginationOptions{Page: 1, PageSize: 10}})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)

	// CanHotpatchTransition 状态机
	assert.True(t, CanHotpatchTransition("draft", "approved"))
	assert.False(t, CanHotpatchTransition("draft", "applied"))
	assert.True(t, CanHotpatchTransition("rolling", "rolling"))
}

// 设计债回归：HotpatchSeedHex 的 crypto/rand.Read 回退分支在 Go≥1.24
// 不可达已删除——返回值必须恒为 16 位 hex（8 字节），且两次调用不同。
func TestHotpatchSeedHex(t *testing.T) {
	s1 := HotpatchSeedHex()
	s2 := HotpatchSeedHex()
	assert.NotEmpty(t, s1)
	assert.Len(t, s1, 16, "8 字节种子编码后为 16 位 hex")
	assert.Regexp(t, `^[0-9a-f]{16}$`, s1, "必须是纯小写 hex")
	assert.NotEqual(t, s1, s2, "每次随机")
}
