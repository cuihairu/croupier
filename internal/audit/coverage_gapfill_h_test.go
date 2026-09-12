package audit

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

// isChainSequenceConflict(nil) → false（err==nil 早退分支）。
func TestIsChainSequenceConflict_NilV9H(t *testing.T) {
	assert.False(t, isChainSequenceConflict(nil))
}

// SQLAuditStore.Create：非链冲突的 DB 错误（RAISE 触发器、消息不含
// chain_sequence 关键字）原样返回，不走 ErrChainSequenceConflict 包装。
func TestSQLAuditStore_CreateGenericDBErrorV9H(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))

	require.NoError(t, db.Exec(
		"CREATE TRIGGER abort_audit_insert_h BEFORE INSERT ON audit_records "+
			"BEGIN SELECT RAISE(ABORT, 'write denied'); END").Error)

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	record := &AuditRecord{
		ID:        "cov-h-create-err",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h", Sequence: 1},
	}
	err = store.Create(record)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrChainSequenceConflict))
	assert.NotContains(t, err.Error(), ErrChainSequenceConflict.Error())
}

// retryRebuildFailStore：Create 永远报链冲突（触发 Log 重试），但
// GetLatestRecord 从第二次起报错——重试路径上 buildChainInfo 失败 →
// "failed to rebuild chain info"。
type retryRebuildFailStore struct {
	*SQLAuditStore
	latestCalls int
}

func (s *retryRebuildFailStore) GetLatestRecord() (*AuditRecord, error) {
	s.latestCalls++
	if s.latestCalls >= 2 {
		return nil, errors.New("cov-h latest record boom")
	}
	return s.SQLAuditStore.GetLatestRecord()
}

func (s *retryRebuildFailStore) Create(record *AuditRecord) error {
	return ErrChainSequenceConflict
}

func TestAuditService_LogRetryRebuildChainFailsV9H(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	svc := NewAuditService(&retryRebuildFailStore{SQLAuditStore: store}, nil)
	_, err = svc.Log(context.Background(), EventLogin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to rebuild chain info")
}
