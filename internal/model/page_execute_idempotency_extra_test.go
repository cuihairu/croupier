package model

// page_execute_idempotency 错误臂补测：Begin 的占位写入失败/冲突后回读失败/
// 超窗回收更新失败臂，Complete 的空结果/更新失败/无占位行臂，Find 的非
// NotFound 错误臂，DeleteForTest 清理路径。
//
// 诚实跳过：
//   - Begin 的「冲突后行读不到」防御臂（149-150）：唯一索引冲突即行存在，
//     读不到需行在插入与回读窗口内被并发清扫，公开路径不可达。
//   - purgeOlderThan 的 CAS 竞争失败臂（91-92）：需两 goroutine 在 Load 与
//     CAS 之间竞争同一原子槽，属真实并发臂，单线程测试不可确定复现。

import (
	"context"
	"errors"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newIdemTestDB(t *testing.T) (*gorm.DB, *PageExecuteIdempotencyModel, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&PageExecuteIdempotency{}))
	return db, NewPageExecuteIdempotencyModel(db), context.Background()
}

var errIdemFailpoint = errors.New("failpoint: idem store down")

// regIdemFailpoint 按表内第 n 次指定操作注入错误；update/create 走 Before
// （核心回调见 Error 即短路，写入不落地），query 走 After（调用方以 Error 为准）。
func regIdemFailpoint(t *testing.T, db *gorm.DB, op string, n int) {
	t.Helper()
	var hits int64
	name := "test:idem_" + op
	reg := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "page_execute_idempotencies" {
			return
		}
		hits++
		if hits == int64(n) {
			_ = tx.AddError(errIdemFailpoint)
		}
	}
	switch op {
	case "query":
		require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg))
	case "update":
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, reg))
	case "create":
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, reg))
	default:
		t.Fatalf("unknown failpoint op %q", op)
	}
}

func seedIdemRow(t *testing.T, db *gorm.DB, key, hash, status string, startedAt time.Time) *PageExecuteIdempotency {
	t.Helper()
	row := &PageExecuteIdempotency{
		GameID: "demo-game", Env: "development", PageKey: "p1", BindingID: "b1",
		IdempotencyKey: key, RequestHash: hash, Status: status, StartedAt: startedAt,
		CreatedAt: startedAt, UpdatedAt: startedAt, Actor: "alice", FunctionID: "player.query",
	}
	require.NoError(t, db.Create(row).Error)
	return row
}

// Begin：占位插入失败（134）、冲突后回读失败（145）、超窗回收更新失败（163）。
func TestPageExecuteIdempotencyBeginErrorArms(t *testing.T) {
	db, m, ctx := newIdemTestDB(t)
	const scope, env = "demo-game", "development"

	// 134：首次占位插入被注入错误。
	regIdemFailpoint(t, db, "create", 1)
	_, _, err := m.Begin(ctx, scope, env, "p1", "b1", "k-begin-err", "hash-a", "alice", "player.query")
	require.ErrorIs(t, err, errIdemFailpoint)

	// 145：同 key 冲突转入回读，回读被注入错误（重注册清零计数）。
	seedIdemRow(t, db, "k-conflict", "hash-a", PageExecuteIdempotencyRunning, time.Now().UTC())
	regIdemFailpoint(t, db, "query", 1)
	_, _, err = m.Begin(ctx, scope, env, "p1", "b1", "k-conflict", "hash-a", "alice", "player.query")
	require.ErrorIs(t, err, errIdemFailpoint)

	// 163：超窗 running 行的回收条件更新被注入错误。
	regIdemFailpoint(t, db, "update", 1)
	stale := time.Now().UTC().Add(-2 * idempotencyStaleWindow)
	seedIdemRow(t, db, "k-stale", "hash-a", PageExecuteIdempotencyRunning, stale)
	_, _, err = m.Begin(ctx, scope, env, "p1", "b1", "k-stale", "hash-a", "alice", "player.query")
	require.ErrorIs(t, err, errIdemFailpoint)
}

// Complete：空结果拒绝（182）、回写失败（194）、无占位行（197）。
func TestPageExecuteIdempotencyCompleteErrorArms(t *testing.T) {
	db, m, ctx := newIdemTestDB(t)
	const scope, env = "demo-game", "development"

	// 182：空结果拒绝。
	require.ErrorContains(t, m.Complete(ctx, scope, env, "p1", "b1", "k-empty", nil), "empty result")

	// 194：回写被注入错误。
	seedIdemRow(t, db, "k-complete-err", "hash-a", PageExecuteIdempotencyRunning, time.Now().UTC())
	regIdemFailpoint(t, db, "update", 1)
	require.ErrorIs(t, m.Complete(ctx, scope, env, "p1", "b1", "k-complete-err", JSON(`{"ok":true}`)), errIdemFailpoint)

	// 197：作用域内无占位行 → 影响行数 0。
	require.ErrorContains(t, m.Complete(ctx, scope, env, "p1", "b1", "k-missing", JSON(`{"ok":true}`)), "no claim row")
}

// Find：非 NotFound 查询错误透传（216）。
func TestPageExecuteIdempotencyFindError(t *testing.T) {
	db, m, ctx := newIdemTestDB(t)
	regIdemFailpoint(t, db, "query", 1)
	_, err := m.Find(ctx, "demo-game", "development", "p1", "b1", "k-find-err")
	require.ErrorIs(t, err, errIdemFailpoint)
}

// DeleteForTest 按键清理占位行（223-226）。
func TestPageExecuteIdempotencyDeleteForTest(t *testing.T) {
	_, m, ctx := newIdemTestDB(t)
	const scope, env = "demo-game", "development"
	_, owner, err := m.Begin(ctx, scope, env, "p1", "b1", "k-del", "hash-a", "alice", "player.query")
	require.NoError(t, err)
	require.True(t, owner)

	require.NoError(t, m.DeleteForTest(ctx, "k-del"))
	found, err := m.Find(ctx, scope, env, "p1", "b1", "k-del")
	require.NoError(t, err)
	assert.Nil(t, found)
}
