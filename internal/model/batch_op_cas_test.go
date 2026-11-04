package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// #664：BatchOpModel.CompareAndSwapStatus 原子 draft→running，重复执行被拒。
func TestBatchOpModel_CompareAndSwapStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&BatchOp{}))
	m := NewBatchOpModel(db)
	ctx := context.Background()

	op := &BatchOp{BatchID: "b1", GameID: "g", Env: "e", Kind: "exec", PayloadJSON: "{}", Status: BatchOpStatusDraft}
	require.NoError(t, m.Create(ctx, op))

	ok, cur, err := m.CompareAndSwapStatus(ctx, op.ID, BatchOpStatusDraft, BatchOpStatusRunning)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, BatchOpStatusRunning, cur.Status)

	// 已 running，第二次从 draft 起跳失败。
	ok, cur, err = m.CompareAndSwapStatus(ctx, op.ID, BatchOpStatusDraft, BatchOpStatusRunning)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, BatchOpStatusRunning, cur.Status)
}
