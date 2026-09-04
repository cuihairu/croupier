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

	"github.com/cuihairu/croupier/internal/model"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExecutionLog{}, &model.TaskRun{}, &model.TaskEvent{}))
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
