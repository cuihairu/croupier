package registry

import (
	"context"
	"sync"
	"time"

	"github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"gorm.io/gorm"
)

// LogRetention is how long application log entries are retained (OPS-005).
const LogRetention = 7 * 24 * time.Hour

// LogSearchQuery filters the log search API.
type LogSearchQuery struct {
	AgentID string
	File    string
	Level   string
	// Since/Until are inclusive bounds on ReceivedAt (zero = unbounded).
	Since time.Time
	Until time.Time
	Limit int
	// Offset supports pagination.
	Offset int
}

// AgentLogEntryRecord is the persisted form of one agent log line. Defined in
// this package (not model) because model imports registry for other types and
// importing model here would create an import cycle.
type AgentLogEntryRecord struct {
	ID         uint   `gorm:"primaryKey"`
	AgentID    string `gorm:"index:idx_agent_log_search;index:idx_agent_log_dedup,unique;not null"`
	File       string `gorm:"index:idx_agent_log_search;index:idx_agent_log_dedup,unique;size:512;not null"`
	Seq        int64  `gorm:"index:idx_agent_log_dedup,unique;not null"`
	Level      string `gorm:"size:16;index"`
	Message    string `gorm:"type:text;not null"`
	TsUnixMs   int64
	ReceivedAt time.Time `gorm:"index:idx_agent_log_search;not null"`
}

// TableName returns the storage table name.
func (AgentLogEntryRecord) TableName() string { return "agent_log_entries" }

// LogStore persists and queries agent application log entries (OPS-005).
// It is a thin DB-backed store: logs are not held in memory (unlike metrics
// rings) because they are high-volume, and the search API always queries the
// database with an indexed (agent_id, file, received_at) lookup.
type LogStore struct {
	mu sync.RWMutex
	db *gorm.DB
	// lastSeq tracks the last persisted (agent, file) sequence for dedup.
	lastSeq map[string]int64
}

// NewLogStore creates a log store (no DB until SetDB).
func NewLogStore() *LogStore {
	return &LogStore{lastSeq: make(map[string]int64)}
}

// SetDB attaches the database.
func (s *LogStore) SetDB(db *gorm.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db
}

// Ingest persists a batch of log entries from one agent, deduplicating by
// (agent, file, seq). Returns the count actually persisted.
func (s *LogStore) Ingest(agentID string, entries []*opsv1.LogEntry) int {
	if s == nil || agentID == "" || len(entries) == 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return 0
	}
	now := time.Now()
	var rows []AgentLogEntryRecord
	persisted := 0
	for _, e := range entries {
		if e == nil {
			continue
		}
		key := agentID + "\x00" + e.File
		if e.Seq <= s.lastSeq[key] {
			// Retransmit: drop.
			continue
		}
		s.lastSeq[key] = e.Seq
		rows = append(rows, AgentLogEntryRecord{
			AgentID:    agentID,
			File:       e.File,
			Seq:        e.Seq,
			Level:      e.Level,
			Message:    e.Message,
			TsUnixMs:   e.TsUnixMs,
			ReceivedAt: now,
		})
		persisted++
	}
	if len(rows) > 0 {
		s.db.Create(&rows)
	}
	return persisted
}

// Search returns log entries matching the query, newest first (then by seq
// descending). An empty result is not an error.
func (s *LogStore) Search(ctx context.Context, q LogSearchQuery) ([]AgentLogEntryRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx := s.db.WithContext(ctx).Model(&AgentLogEntryRecord{})
	if q.AgentID != "" {
		tx = tx.Where("agent_id = ?", q.AgentID)
	}
	if q.File != "" {
		tx = tx.Where("file = ?", q.File)
	}
	if q.Level != "" {
		tx = tx.Where("level = ?", q.Level)
	}
	if !q.Since.IsZero() {
		tx = tx.Where("received_at >= ?", q.Since)
	}
	if !q.Until.IsZero() {
		tx = tx.Where("received_at <= ?", q.Until)
	}
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	var rows []AgentLogEntryRecord
	err := tx.Order("received_at DESC, seq DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, err
}

// Prune deletes rows older than the retention window. Returns rows deleted.
func (s *LogStore) Prune(olderThan time.Duration) int64 {
	if s == nil || s.db == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	res := s.db.Where("received_at < ?", cutoff).Delete(&AgentLogEntryRecord{})
	return res.RowsAffected
}

// StartCleanupRoutine periodically prunes old log entries.
func (s *LogStore) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Prune(LogRetention)
			}
		}
	}()
}
