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

func TestHotpatchSeedHex(t *testing.T) {
	assert.NotEmpty(t, HotpatchSeedHex())
	assert.NotEmpty(t, HotpatchSeedHex())
}
