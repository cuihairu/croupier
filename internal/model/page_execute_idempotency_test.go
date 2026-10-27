package model

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestPageExecuteIdempotencyBegin 覆盖服务端幂等占位的四种判定：新占位、
// 窗口内重复占位（结果未知）、已完成重放、同 key 键误复用；以及超窗遗留
// running 行的回收与键误复用不被回收的边界。
func TestPageExecuteIdempotencyBegin(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PageExecuteIdempotency{}))
	ctx := context.Background()
	m := NewPageExecuteIdempotencyModel(db)

	// 唯一索引是幂等判定的物理前提（Begin 的插入冲突即同 key 已占位）——
	// 索引 tag 一旦漂移（列集与索引名不匹配）会让 Begin 恒判「新占位」，
	// 服务端幂等静默失效、重复执行无法被拦。故连「同键第二行必须报错」一并
	// 断言，防止 tag 改了索引没建。
	if !db.Migrator().HasIndex(&PageExecuteIdempotency{}, "idx_page_execute_idem_scope_key") {
		t.Fatal("unique index idx_page_execute_idem_scope_key missing: idempotency relies on the physical unique key")
	}
	dupRow := PageExecuteIdempotency{
		GameID: "dup", Env: "e", PageKey: "p", BindingID: "b",
		IdempotencyKey: "k", RequestHash: "h", Status: PageExecuteIdempotencyRunning,
		StartedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&dupRow).Error)
	dupRow.ID = 0
	require.Error(t, db.Create(&dupRow).Error, "同键同域第二行必须撞唯一索引")
	require.NoError(t, db.Where("game_id = ?", "dup").Delete(&PageExecuteIdempotency{}).Error)

	const scope = "demo-game"
	const idemKey = "exec-key-1"

	// 1) 新占位：获得执行权。
	rec, owner, err := m.Begin(ctx, scope, "development", "p1", "b1", idemKey, "hash-a", "alice", "player.query")
	require.NoError(t, err)
	assert.True(t, owner, "首次占位必须获得执行权")
	assert.Nil(t, rec)

	// 2) 同 key 同内容、窗口内仍 running：判结果未知，不获得执行权。
	rec, owner, err = m.Begin(ctx, scope, "development", "p1", "b1", idemKey, "hash-a", "alice", "player.query")
	require.NoError(t, err)
	assert.False(t, owner)
	require.NotNil(t, rec)
	assert.Equal(t, PageExecuteIdempotencyRunning, rec.Status)
	assert.Equal(t, "hash-a", rec.RequestHash)
	assert.Equal(t, "player.query", rec.FunctionID)

	// 3) 同 key 不同内容：键误复用（调用方判 409 idempotency_key_reused）。
	rec, owner, err = m.Begin(ctx, scope, "development", "p1", "b1", idemKey, "hash-b", "alice", "player.query")
	require.NoError(t, err)
	assert.False(t, owner)
	require.NotNil(t, rec)
	assert.Equal(t, "hash-a", rec.RequestHash, "返回现存记录供调用方比对指纹")

	// 4) 不同 scope（其他游戏/页面）复用同名字面键互不影响。
	_, owner, err = m.Begin(ctx, "other-game", "development", "p1", "b1", idemKey, "hash-a", "alice", "player.query")
	require.NoError(t, err)
	assert.True(t, owner, "幂等键按 (game, env, page, binding, key) 作用域隔离")

	// 5) 完成占位后可重放。
	require.NoError(t, m.Complete(ctx, scope, "development", "p1", "b1", idemKey, JSON(`{"kind":"sync","data":{"ok":true}}`)))
	_, owner, err = m.Begin(ctx, scope, "development", "p1", "b1", idemKey, "hash-a", "alice", "player.query")
	require.NoError(t, err)
	assert.False(t, owner, "已完成记录不再获得执行权")
	found, err := m.Find(ctx, scope, "development", "p1", "b1", idemKey)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, PageExecuteIdempotencyCompleted, found.Status)
	assert.JSONEq(t, `{"kind":"sync","data":{"ok":true}}`, string(found.Result))
	require.NotNil(t, found.CompletedAt)
	assert.WithinDuration(t, time.Now().UTC(), *found.CompletedAt, 5*time.Second)

	// 6) 已完成记录的键误复用同样不重放、不回收。
	_, owner, err = m.Begin(ctx, scope, "development", "p1", "b1", idemKey, "hash-other", "alice", "player.query")
	require.NoError(t, err)
	assert.False(t, owner)

	// 7) 无记录：Find 返回 (nil, nil)。
	missing, err := m.Find(ctx, scope, "development", "p1", "b1", "no-such-key")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

// TestPageExecuteIdempotencyStaleReclaim 覆盖占位窗口到期后的遗留行回收：
// 进程崩溃/连接中断留下的 running 行可被同 key 同内容请求接管；同 key 但
// 请求内容不同的请求不得劫持该执行。
func TestPageExecuteIdempotencyStaleReclaim(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PageExecuteIdempotency{}))
	ctx := context.Background()
	m := NewPageExecuteIdempotencyModel(db)

	const idemKey = "exec-key-stale"
	_, owner, err := m.Begin(ctx, "demo-game", "development", "p1", "b1", idemKey, "hash-a", "alice", "player.query")
	require.NoError(t, err)
	assert.True(t, owner)

	// 把遗留行回拨到占位窗口之外，模拟持有连接后崩溃的实例。
	stale := time.Now().UTC().Add(-2 * idempotencyStaleWindow)
	require.NoError(t, db.Model(&PageExecuteIdempotency{}).
		Where("idempotency_key = ?", idemKey).
		UpdateColumns(map[string]interface{}{"started_at": stale}).Error)

	// 同 key 但请求内容不同：不得接管（否则会重跑一次他人请求）。
	_, owner, err = m.Begin(ctx, "demo-game", "development", "p1", "b1", idemKey, "hash-b", "bob", "player.query")
	require.NoError(t, err)
	assert.False(t, owner)
	kept, err := m.Find(ctx, "demo-game", "development", "p1", "b1", idemKey)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "hash-a", kept.RequestHash, "键误复用不得改写占位行的请求指纹")
	assert.WithinDuration(t, stale, kept.StartedAt, 10*time.Second, "键误复用不得刷新占位时间")

	// 同 key 同内容：接管成功。
	_, reclaimed, err := m.Begin(ctx, "demo-game", "development", "p1", "b1", idemKey, "hash-a", "bob", "player.query")
	require.NoError(t, err)
	assert.True(t, reclaimed, "超窗遗留行可由同内容请求回收重跑")
	// 回收后同一 key 再次请求回到窗口内 running（新的占位开始）。
	rec, owner, err := m.Begin(ctx, "demo-game", "development", "p1", "b1", idemKey, "hash-a", "bob", "player.query")
	require.NoError(t, err)
	assert.False(t, owner)
	require.NotNil(t, rec)
	assert.WithinDuration(t, time.Now().UTC(), rec.StartedAt, 5*time.Second)
}
