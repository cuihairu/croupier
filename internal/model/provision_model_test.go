package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// #661：ProvisionRunModel.CompareAndSwapState 原子推进 pending→applying，
// 并发续行只有第一个通过（RowsAffected 校验），第二个返回 ok=false。
func TestProvisionRunModel_CompareAndSwapState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ProvisionRun{}))
	m := NewProvisionRunModel(db)
	ctx := context.Background()

	run := &ProvisionRun{RunID: "prov-cas", Provider: "aws", Region: "r", GameID: "g", Env: "e", Policy: "ticket", Count: 1, State: "pending"}
	require.NoError(t, m.Create(ctx, run))

	// 首次 CAS 成功。
	ok, got, err := m.CompareAndSwapState(ctx, "prov-cas", "pending", "applying")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "applying", got.State)

	// 第二次 CAS（from 已非 pending）失败，当前行为 applying。
	ok, cur, err := m.CompareAndSwapState(ctx, "prov-cas", "pending", "applying")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "applying", cur.State)

	// 不存在的 run 返回 ok=false 且无错误。
	ok, _, err = m.CompareAndSwapState(ctx, "nope", "pending", "applying")
	require.NoError(t, err)
	assert.False(t, ok)
}
