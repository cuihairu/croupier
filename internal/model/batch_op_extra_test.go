package model

// batch_op_model 补测：Find 的命中/未命中臂、List 的 limit 归一与查询臂、
// UpdateStatus 的 id 校验/字段合并/纯状态三臂，以及 CompareAndSwapStatus 的
// 更新失败/回读失败/行已删除三错误臂。
//
// 诚实跳过：
//   - UpdateStatus 的 len(fields) > math.MaxInt-1 防御臂（91-92）：64 位平台
//     上 map 长度不可能超过 MaxInt-1，不可达。

import (
	"context"
	"errors"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBatchOpTestDB(t *testing.T) (*BatchOpModel, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&BatchOp{}))
	return NewBatchOpModel(db), context.Background()
}

func seedBatchOp(t *testing.T, m *BatchOpModel, ctx context.Context, batchID, status string) *BatchOp {
	t.Helper()
	op := &BatchOp{BatchID: batchID, GameID: "g1", Env: "e1", Kind: BatchOpKindExec, PayloadJSON: "{}", Status: status}
	require.NoError(t, m.Create(ctx, op))
	return op
}

// Find：命中返回行、未命中报 ErrRecordNotFound（67-72）。
func TestBatchOpModel_FindArms(t *testing.T) {
	m, ctx := newBatchOpTestDB(t)
	seedBatchOp(t, m, ctx, "b-find", BatchOpStatusDraft)

	found, err := m.Find(ctx, "g1", "e1", "b-find")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "b-find", found.BatchID)

	// 作用域隔离：跨 game 查不到。
	_, err = m.Find(ctx, "other", "e1", "b-find")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// List：limit<=0 / limit>100 归一为 50，合法 limit 透传（76-81）。
func TestBatchOpModel_ListArms(t *testing.T) {
	m, ctx := newBatchOpTestDB(t)
	for _, id := range []string{"b1", "b2", "b3"} {
		seedBatchOp(t, m, ctx, id, BatchOpStatusDraft)
	}

	// 非法 limit 归一 50：3 条全回。
	out, err := m.List(ctx, "g1", "e1", 0)
	require.NoError(t, err)
	assert.Len(t, out, 3)

	out, err = m.List(ctx, "g1", "e1", 500)
	require.NoError(t, err)
	assert.Len(t, out, 3)

	// 合法 limit=2：只回 2 条（created_at DESC）。
	out, err = m.List(ctx, "g1", "e1", 2)
	require.NoError(t, err)
	assert.Len(t, out, 2)
}

// UpdateStatus：id=0 拒绝、带字段合并、纯状态三条路径（85-101）。
func TestBatchOpModel_UpdateStatusArms(t *testing.T) {
	m, ctx := newBatchOpTestDB(t)
	op := seedBatchOp(t, m, ctx, "b-upd", BatchOpStatusDraft)

	// id=0 → 校验错误（85-87）
	require.Error(t, m.UpdateStatus(ctx, 0, BatchOpStatusRunning, nil))

	// 带字段：status + 计数一并落库（88-99）
	require.NoError(t, m.UpdateStatus(ctx, op.ID, BatchOpStatusRunning, map[string]any{
		"targets": 5, "succeeded": 3, "failed": 1, "skipped": 1,
	}))
	cur, err := m.Find(ctx, "g1", "e1", "b-upd")
	require.NoError(t, err)
	assert.Equal(t, BatchOpStatusRunning, cur.Status)
	assert.Equal(t, 5, cur.Targets)
	assert.Equal(t, 3, cur.Succeeded)

	// 纯状态（100-101）
	require.NoError(t, m.UpdateStatus(ctx, op.ID, BatchOpStatusCompleted, nil))
	cur, err = m.Find(ctx, "g1", "e1", "b-upd")
	require.NoError(t, err)
	assert.Equal(t, BatchOpStatusCompleted, cur.Status)
}

// CompareAndSwapStatus 错误臂：更新失败（110）、回读失败（118）、
// 行已删除返回 (false, nil, nil)（116）、CAS 成功后回读失败（124）。
func TestBatchOpModel_CompareAndSwapErrorArms(t *testing.T) {
	// 116：CAS 不命中且行不存在 → (false, nil, nil)。
	m, ctx := newBatchOpTestDB(t)
	ok, cur, err := m.CompareAndSwapStatus(ctx, 9999, BatchOpStatusDraft, BatchOpStatusRunning)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, cur)

	// 110/118/124：注册 Before 注错钩子按第 n 次命中注入。
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&BatchOp{}))
	m2 := NewBatchOpModel(db)
	op := seedBatchOp(t, m2, ctx, "b-cas", BatchOpStatusDraft)

	errBoom := errors.New("failpoint: batch op store down")
	regFailpoint := func(t *testing.T, op string, n int) {
		t.Helper()
		var hits int64
		name := "test:batchop_" + op
		reg := func(tx *gorm.DB) {
			if tx.Statement == nil || tx.Statement.Table != "batch_ops" {
				return
			}
			hits++
			if hits == int64(n) {
				_ = tx.AddError(errBoom)
			}
		}
		switch op {
		case "update":
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, reg))
		case "query":
			require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg))
		}
	}

	// 110：CAS 更新被注入错误。
	regFailpoint(t, "update", 1)
	_, _, err = m2.CompareAndSwapStatus(ctx, op.ID, BatchOpStatusDraft, BatchOpStatusRunning)
	require.ErrorIs(t, err, errBoom)

	// 118：CAS 不命中后的回读被注入非 NotFound 错误。
	regFailpoint(t, "query", 1)
	_, _, err = m2.CompareAndSwapStatus(ctx, op.ID, BatchOpStatusDryRun, BatchOpStatusRunning)
	require.ErrorIs(t, err, errBoom)

	// 124：CAS 命中后的回读被注入错误。
	regFailpoint(t, "query", 1)
	ok, _, err = m2.CompareAndSwapStatus(ctx, op.ID, BatchOpStatusDraft, BatchOpStatusRunning)
	require.ErrorIs(t, err, errBoom)
	assert.False(t, ok)
}
