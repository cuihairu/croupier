package executionlog

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExecutionLog{}, &model.TaskRun{}, &model.TaskEvent{}))
	return db
}

// newTestDBSingleConn 强制单连接：:memory: 库的连接池开第二条连接会得到
// 一个全新的空库，「边写边查」的测试必须与写入共享同一连接。
func newTestDBSingleConn(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func TestWriterPersistsEntry(t *testing.T) {
	db := newTestDB(t)
	w := NewWriter(db, Config{Enabled: true, FlushInterval: 50 * time.Millisecond})
	ctx := context.Background()
	w.Run(ctx)

	w.Log(Entry{
		GameID: "g1", Env: "prod", Source: SourceInvoke,
		FunctionID: "mail.send", Actor: "alice", Status: StatusOK,
		DurationMs: 12, TraceID: "tr-1",
		Request:  map[string]interface{}{"playerId": "p1"},
		Response: map[string]interface{}{"success": true},
	})

	w.Stop()
	var items []model.ExecutionLog
	require.NoError(t, db.Find(&items).Error)
	require.Len(t, items, 1)
	assert.Equal(t, "mail.send", items[0].FunctionID)
	assert.Equal(t, "alice", items[0].Actor)
	assert.Equal(t, StatusOK, items[0].Status)
	assert.False(t, items[0].Truncated)
	var req map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(items[0].RequestPayload), &req))
	assert.Equal(t, "p1", req["playerId"])
}

func TestWriterMasksSensitiveFields(t *testing.T) {
	db := newTestDB(t)
	w := NewWriter(db, Config{Enabled: true, FlushInterval: 50 * time.Millisecond})
	w.Run(context.Background())

	w.Log(Entry{
		GameID: "g1", Env: "prod", Source: SourceInvoke,
		FunctionID: "user.create", Actor: "alice", Status: StatusOK,
		Request: map[string]interface{}{
			"username": "bob",
			"password": "super-secret",
			"nested":   map[string]interface{}{"api_key": "k-123"},
		},
	})
	w.Stop()

	var items []model.ExecutionLog
	require.NoError(t, db.Find(&items).Error)
	require.Len(t, items, 1)
	require.False(t, strings.Contains(string(items[0].RequestPayload), "super-secret"))
	require.False(t, strings.Contains(string(items[0].RequestPayload), "k-123"))
	require.True(t, strings.Contains(string(items[0].RequestPayload), "***MASKED***"))
	require.True(t, strings.Contains(string(items[0].RequestPayload), "bob"))
}

func TestWriterTruncatesOversizedPayload(t *testing.T) {
	db := newTestDB(t)
	w := NewWriter(db, Config{Enabled: true, MaxPayloadBytes: 64, FlushInterval: 50 * time.Millisecond})
	w.Run(context.Background())

	big := strings.Repeat("x", 4096)
	w.Log(Entry{
		GameID: "g1", Env: "prod", Source: SourceInvoke,
		FunctionID: "mail.send", Actor: "alice", Status: StatusOK,
		Request: map[string]interface{}{"content": big},
	})
	w.Stop()

	var items []model.ExecutionLog
	require.NoError(t, db.Find(&items).Error)
	require.Len(t, items, 1)
	assert.True(t, items[0].Truncated)
	require.True(t, strings.Contains(string(items[0].RequestPayload), "logTruncated"))
	require.Less(t, len(items[0].RequestPayload), 512)
}

func TestWriterErrorStatusEntry(t *testing.T) {
	db := newTestDB(t)
	w := NewWriter(db, Config{Enabled: true, FlushInterval: 50 * time.Millisecond})
	w.Run(context.Background())

	w.Log(Entry{
		GameID: "g1", Env: "prod", Source: SourcePage,
		FunctionID: "player.ban", Actor: "bob", Status: StatusFail,
		Response: map[string]string{"error": "target offline"},
	})
	w.Stop()

	var items []model.ExecutionLog
	require.NoError(t, db.Find(&items).Error)
	require.Len(t, items, 1)
	assert.Equal(t, StatusFail, items[0].Status)
	assert.Equal(t, SourcePage, items[0].Source)
	require.True(t, strings.Contains(string(items[0].ResponseBody), "target offline"))
}

func TestWriterQueueFullDropsWithoutBlocking(t *testing.T) {
	db := newTestDB(t)
	// 队列容量 1，消费方未启动：投递第二条开始丢弃，绝不阻塞
	w := NewWriter(db, Config{Enabled: true, QueueSize: 1, FlushInterval: time.Hour})

	var slow atomic.Int64
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			w.Log(Entry{FunctionID: "f", Status: StatusOK})
			slow.Add(1)
		}
		close(done)
	}()

	select {
	case <-done:
		assert.Greater(t, w.Dropped(), int64(0), "队列满应丢弃并计数")
	case <-time.After(5 * time.Second):
		t.Fatal("Log() blocked when queue was full")
	}
	w.Stop()
}

// fakeRouter DBRouter 内存替身：badGame 返回解析失败，badDB 库缺表导致
// 批量写入失败，其余路由到指定库。返回的 ctx 携带 per-game 库（与生产
// router.Router 的注入方式一致——writer 只消费 ctx，丢弃返回的 *gorm.DB）。
type fakeRouter struct {
	badGame string
	badDB   *gorm.DB
	good    *gorm.DB
}

func (f *fakeRouter) Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error) {
	switch gameID {
	case f.badGame:
		return ctx, nil, context.DeadlineExceeded
	case "baddb":
		return dbctx.WithDB(ctx, f.badDB), f.badDB, nil
	default:
		return dbctx.WithDB(ctx, f.good), f.good, nil
	}
}

func TestWriterRouterGroupsByGameEnv(t *testing.T) {
	good := newTestDB(t)
	badDB, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err) // 不迁移 → CreateBatch 失败分支

	w := NewWriter(good, Config{Enabled: true, FlushInterval: 50 * time.Millisecond})
	w.router = &fakeRouter{badGame: "gerr", badDB: badDB, good: good}
	w.Run(context.Background())

	w.Log(Entry{GameID: "g1", Env: "prod", Source: SourceInvoke, FunctionID: "f.ok", Status: StatusOK})
	w.Log(Entry{GameID: "g1", Env: "dev", Source: SourceInvoke, FunctionID: "f.ok", Status: StatusOK})
	w.Log(Entry{GameID: "gerr", Env: "prod", Source: SourceInvoke, FunctionID: "f.resolve-err", Status: StatusOK})
	w.Log(Entry{GameID: "baddb", Env: "prod", Source: SourceInvoke, FunctionID: "f.write-err", Status: StatusOK})
	w.Stop()

	// g1/prod 与 g1/dev 各自落库成功；gerr/baddb 两个失败组只告警不中断。
	var items []model.ExecutionLog
	require.NoError(t, good.Find(&items).Error)
	require.Len(t, items, 2)
}

func TestWriterNilReceiver(t *testing.T) {
	var w *Writer
	assert.NotPanics(t, func() {
		w.Log(Entry{FunctionID: "f"})
		assert.Equal(t, int64(0), w.Dropped())
	})
}

func TestSkipContext(t *testing.T) {
	assert.False(t, Skipped(nil))
	assert.False(t, Skipped(context.Background()))
	assert.True(t, Skipped(WithSkipContext(context.Background())))
}

func TestNormalizeJSONUnserializable(t *testing.T) {
	out := normalizeJSON(make(chan int))
	m, ok := out.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, m, "logUnserializable")
}

func TestNewWriterDefaults(t *testing.T) {
	// 全零配置：四个默认值分支全部生效
	w := NewWriter(newTestDB(t), Config{})
	assert.Equal(t, defaultQueueSize, cap(w.ch))
	assert.Equal(t, defaultBatchSize, w.batchSize)
	assert.Equal(t, defaultFlushInterval, w.flush)
	assert.Equal(t, DefaultMaxPayloadBytes, w.maxBytes)
}

func TestWriterTickerFlush(t *testing.T) {
	db := newTestDBSingleConn(t)
	w := NewWriter(db, Config{Enabled: true, FlushInterval: 10 * time.Millisecond})
	w.Run(context.Background())

	w.Log(Entry{GameID: "g1", Env: "prod", Source: SourceInvoke, FunctionID: "f.tick", Status: StatusOK})
	// 不调用 Stop：由 ticker 触发非空批次冲刷
	assert.Eventually(t, func() bool {
		var n int64
		db.Model(&model.ExecutionLog{}).Count(&n)
		return n == 1
	}, 2*time.Second, 10*time.Millisecond)
	w.Stop()
}

func TestWriterTickerFlushEmptyBatch(t *testing.T) {
	db := newTestDB(t)
	w := NewWriter(db, Config{Enabled: true, FlushInterval: 10 * time.Millisecond})
	w.Run(context.Background())
	// 无任何条目：空批次冲刷直接返回
	time.Sleep(50 * time.Millisecond)
	w.Stop()
	assert.Equal(t, int64(0), w.Dropped())
}

func TestWriterMidBatchFlush(t *testing.T) {
	db := newTestDBSingleConn(t)
	// BatchSize=1：每条入队即触发中途冲刷
	w := NewWriter(db, Config{Enabled: true, BatchSize: 1, FlushInterval: time.Hour})
	w.Run(context.Background())
	w.Log(Entry{GameID: "g1", Env: "prod", FunctionID: "f1", Status: StatusOK})
	w.Log(Entry{GameID: "g1", Env: "prod", FunctionID: "f2", Status: StatusOK})
	assert.Eventually(t, func() bool {
		var n int64
		db.Model(&model.ExecutionLog{}).Count(&n)
		return n == 2
	}, 2*time.Second, 10*time.Millisecond)
	w.Stop()
}

func TestWriterMetaWriteFailureOnStopFlush(t *testing.T) {
	// 未迁移表的库 + 单库模式：Stop 冲刷时批量写入失败只告警
	bare, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	w := NewWriter(bare, Config{Enabled: true, FlushInterval: time.Hour})
	w.Run(context.Background())
	w.Log(Entry{GameID: "g1", Env: "prod", FunctionID: "f.fail", Status: StatusOK})
	assert.NotPanics(t, func() { w.Stop() })
}

func TestWriterStopDrainsQueuedEntries(t *testing.T) {
	// Stop 冲刷路径中「队列仍有积压且凑满一批」的分支：调度竞态导致
	// 可能一次不中，多轮尝试保证覆盖。
	for i := 0; i < 40; i++ {
		db := newTestDB(t)
		w := NewWriter(db, Config{Enabled: true, BatchSize: 2, QueueSize: 4, FlushInterval: time.Hour})
		// 先积压 3 条再启动消费
		w.Log(Entry{FunctionID: "a", Status: StatusOK})
		w.Log(Entry{FunctionID: "b", Status: StatusOK})
		w.Log(Entry{FunctionID: "c", Status: StatusOK})
		w.Run(context.Background())
		w.Stop()

		var n int64
		require.NoError(t, db.Model(&model.ExecutionLog{}).Count(&n).Error)
		assert.Equal(t, int64(3), n, "iteration %d: 队列必须全部冲刷落库", i)
	}
}
