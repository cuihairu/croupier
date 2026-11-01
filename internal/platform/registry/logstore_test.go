package registry

import (
	"context"
	"strconv"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func newLogStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AgentLogEntryRecord{}))
	return db
}

func TestLogStore_IngestAndSearch(t *testing.T) {
	db := newLogStoreDB(t)
	s := NewLogStore()
	s.SetDB(db)

	persisted := s.Ingest("agent-1", []*opsv1.LogEntry{
		{File: "/var/log/app.log", Level: "info", Message: "hello", Seq: 1},
		{File: "/var/log/app.log", Level: "error", Message: "boom", Seq: 2},
		{File: "/var/log/other.log", Level: "warn", Message: "x", Seq: 1},
	})
	assert.Equal(t, 3, persisted)

	rows, err := s.Search(context.Background(), LogSearchQuery{AgentID: "agent-1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, rows, 3)
	// 同一 ReceivedAt 下按 seq desc；断言整体集合而非依赖插入次序。
	byKey := map[string]string{}
	for _, r := range rows {
		byKey[r.File+"#"+itoa(r.Seq)] = r.Level
	}
	assert.Equal(t, "info", byKey["/var/log/app.log#1"])
	assert.Equal(t, "error", byKey["/var/log/app.log#2"])
	assert.Equal(t, "warn", byKey["/var/log/other.log#1"])

	// Filter by level
	rows, err = s.Search(context.Background(), LogSearchQuery{AgentID: "agent-1", Level: "error", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "boom", rows[0].Message)

	// Filter by file
	rows, err = s.Search(context.Background(), LogSearchQuery{AgentID: "agent-1", File: "/var/log/other.log", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "warn", rows[0].Level)
}

func TestLogStore_DedupBySeq(t *testing.T) {
	db := newLogStoreDB(t)
	s := NewLogStore()
	s.SetDB(db)

	s.Ingest("a", []*opsv1.LogEntry{{File: "f", Seq: 1, Message: "one"}})
	// retransmit seq 1 -> dropped
	persisted := s.Ingest("a", []*opsv1.LogEntry{{File: "f", Seq: 1, Message: "one"}})
	assert.Equal(t, 0, persisted)
	// new seq -> persisted
	persisted = s.Ingest("a", []*opsv1.LogEntry{{File: "f", Seq: 2, Message: "two"}})
	assert.Equal(t, 1, persisted)

	rows, _ := s.Search(context.Background(), LogSearchQuery{AgentID: "a", Limit: 10})
	assert.Len(t, rows, 2)
}

func TestLogStore_SeparateAgentNoDedupInterference(t *testing.T) {
	db := newLogStoreDB(t)
	s := NewLogStore()
	s.SetDB(db)
	s.Ingest("a", []*opsv1.LogEntry{{File: "f", Seq: 1, Message: "a1"}})
	s.Ingest("b", []*opsv1.LogEntry{{File: "f", Seq: 1, Message: "b1"}})
	rows, _ := s.Search(context.Background(), LogSearchQuery{AgentID: "b", Limit: 10})
	assert.Len(t, rows, 1)
	assert.Equal(t, "b1", rows[0].Message)
}

func TestLogStore_Prune(t *testing.T) {
	db := newLogStoreDB(t)
	s := NewLogStore()
	s.SetDB(db)
	s.Ingest("a", []*opsv1.LogEntry{{File: "f", Seq: 1, Message: "old"}})
	// Backdate received_at directly
	require.NoError(t, db.Model(&AgentLogEntryRecord{}).Where("seq = 1").Update("received_at", time.Now().Add(-30*24*time.Hour)).Error)
	deleted := s.Prune(7 * 24 * time.Hour)
	assert.Equal(t, int64(1), deleted)
}
