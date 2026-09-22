package model_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openContractRemovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))
	return db
}

func seedRemovalContract(t *testing.T, db *gorm.DB, functionID string) *model.FunctionContract {
	t.Helper()
	contract := &model.FunctionContract{
		GameID:     "demo-game",
		Env:        "development",
		FunctionID: functionID,
		Version:    "1.0.0",
		Enabled:    true,
	}
	require.NoError(t, db.Create(contract).Error)
	return contract
}

func TestFunctionContractRemovalPendingLifecycle(t *testing.T) {
	db := openContractRemovalTestDB(t)
	ctx := context.Background()
	m := model.NewFunctionContractModel(db)
	seedRemovalContract(t, db, "mail.send")
	seedRemovalContract(t, db, "mail.recv")

	// 打标：仅目标函数受影响；重复打标刷新时间戳且幂等成功。
	marked, err := m.MarkRemovalPending(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.True(t, marked)
	firstAt := func() *time.Time {
		row, err := m.FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
		require.NoError(t, err)
		return row.RemovalPendingAt
	}()
	require.NotNil(t, firstAt)

	time.Sleep(10 * time.Millisecond)
	marked, err = m.MarkRemovalPending(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.True(t, marked)
	secondAt := func() *time.Time {
		row, err := m.FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
		require.NoError(t, err)
		return row.RemovalPendingAt
	}()
	require.NotNil(t, secondAt)
	assert.True(t, secondAt.After(*firstAt), "重复摘除应刷新 pending 时间戳")

	// 不存在的函数打标：无行受影响。
	marked, err = m.MarkRemovalPending(ctx, "demo-game", "development", "ghost")
	require.NoError(t, err)
	assert.False(t, marked)

	// 清除：只清 pending 行，无 pending 时是 no-op。
	require.NoError(t, m.ClearRemovalPending(ctx, "demo-game", "development", "mail.send"))
	row, err := m.FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.Nil(t, row.RemovalPendingAt, "清除后 pending 标记应为 NULL")
	require.NoError(t, m.ClearRemovalPending(ctx, "demo-game", "development", "mail.send"))

	// 候选列表：只含 pending 行；limit 截断最旧优先。
	marked, err = m.MarkRemovalPending(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.True(t, marked)
	expired, err := m.ListExpiredPendingRemovals(ctx, time.Now().Add(time.Minute), 0)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, "mail.send", expired[0].FunctionID)
	none, err := m.ListExpiredPendingRemovals(ctx, time.Now().Add(-time.Minute), 0)
	require.NoError(t, err)
	assert.Empty(t, none, "未到期的不应出现在候选列表")

	// 批量上限（R7）：limit=0（不限量）全量返回，limit 按最旧优先截断。
	seedRemovalContract(t, db, "mail.forward")
	require.NoError(t, db.Model(&model.FunctionContract{}).
		Where("function_id = ?", "mail.forward").
		UpdateColumn("removal_pending_at", time.Now().Add(-2*time.Minute)).Error)
	limited, err := m.ListExpiredPendingRemovals(ctx, time.Now().Add(time.Minute), 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	assert.Equal(t, "mail.forward", limited[0].FunctionID, "先到期的先出队")

	// 条件删除：原子认领只命中 pending 行，清除后删除落空。
	deleted, err := m.DeleteIfRemovalPending(ctx, "demo-game", "development", "mail.recv")
	require.NoError(t, err)
	assert.False(t, deleted, "非 pending 行不应被条件删除命中")
	deleted, err = m.DeleteIfRemovalPending(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.True(t, deleted)
	deleted, err = m.DeleteIfRemovalPending(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.False(t, deleted, "已删除行不可被二次认领")
}

// R4：打标/清标必须走 UpdateColumn（不触碰 updated_at）——computeDigest
// 整行 json.Marshal 含 updated_at，gorm Update 的自动触碰会让宽限内首轮
// 提案重建误判「契约已变化」，把 accepted 提案打回 pending（digest 幽灵
// churn）。pending 标记本身 json:"-"，不进 digest 序列。
func TestFunctionContractRemovalPendingDoesNotTouchUpdatedAtOrDigest(t *testing.T) {
	db := openContractRemovalTestDB(t)
	ctx := context.Background()
	m := model.NewFunctionContractModel(db)
	seedRemovalContract(t, db, "mail.send")

	rowAt := func() *model.FunctionContract {
		row, err := m.FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
		require.NoError(t, err)
		return row
	}
	digestOf := func(row *model.FunctionContract) string {
		b, err := json.Marshal(row)
		require.NoError(t, err)
		return string(b)
	}

	before := rowAt()
	beforeDigest := digestOf(before)
	marked, err := m.MarkRemovalPending(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	require.True(t, marked)
	mid := rowAt()
	assert.NotNil(t, mid.RemovalPendingAt)
	assert.Equal(t, before.UpdatedAt, mid.UpdatedAt, "打标不得触碰 updated_at")
	assert.Equal(t, beforeDigest, digestOf(mid), "打标不得改变契约 digest")

	require.NoError(t, m.ClearRemovalPending(ctx, "demo-game", "development", "mail.send"))
	after := rowAt()
	assert.Nil(t, after.RemovalPendingAt)
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt, "清标不得触碰 updated_at")
	assert.Equal(t, beforeDigest, digestOf(after), "清标不得改变契约 digest")
}
