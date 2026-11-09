package audit

import (
	"encoding/json"
	"sync"
	"time"
)

// InMemoryAuditStore is an in-memory implementation for testing
type InMemoryAuditStore struct {
	records map[string]*AuditRecord
	bySeq   map[int64]*AuditRecord
	latest  *AuditRecord
	nextSeq int64
	mu      sync.RWMutex
}

// NewInMemoryAuditStore creates a new in-memory audit store
func NewInMemoryAuditStore() *InMemoryAuditStore {
	return &InMemoryAuditStore{
		records: make(map[string]*AuditRecord),
		bySeq:   make(map[int64]*AuditRecord),
		nextSeq: 1,
	}
}

// Create creates an audit record
func (s *InMemoryAuditStore) Create(record *AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ChainInfo.Sequence == 0 {
		record.ChainInfo.Sequence = s.nextSeq
		s.nextSeq++
	}

	s.records[record.ID] = record
	s.bySeq[record.ChainInfo.Sequence] = record
	s.latest = record

	return nil
}

// Get gets an audit record by ID
func (s *InMemoryAuditStore) Get(id string) (*AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[id]
	if !exists {
		return nil, ErrAuditNotFound
	}
	return record, nil
}

// List lists audit records
func (s *InMemoryAuditStore) List(filter AuditFilter, page AuditPage) ([]*AuditRecord, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*AuditRecord
	for _, r := range s.records {
		if !s.matchesFilter(r, filter) {
			continue
		}
		filtered = append(filtered, r)
	}

	total := len(filtered)

	// Pagination
	if page.PageSize <= 0 {
		page.PageSize = 50
	}
	if page.Page <= 0 {
		page.Page = 1
	}

	start := (page.Page - 1) * page.PageSize
	end := start + page.PageSize

	if start >= total {
		return []*AuditRecord{}, total, nil
	}
	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

func (s *InMemoryAuditStore) matchesFilter(r *AuditRecord, f AuditFilter) bool {
	if len(f.EventType) > 0 {
		found := false
		for _, t := range f.EventType {
			if r.EventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.ActorID != "" && r.Actor.ID != f.ActorID {
		return false
	}

	if f.StartTime != nil && r.Timestamp.Before(*f.StartTime) {
		return false
	}

	if f.EndTime != nil && r.Timestamp.After(*f.EndTime) {
		return false
	}

	if f.Outcome != "" && r.Outcome != f.Outcome {
		return false
	}

	return true
}

// Delete deletes an audit record
func (s *InMemoryAuditStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[id]
	if !exists {
		return ErrAuditNotFound
	}

	delete(s.records, id)
	delete(s.bySeq, record.ChainInfo.Sequence)

	return nil
}

// DeleteBefore deletes records before a timestamp
func (s *InMemoryAuditStore) DeleteBefore(timestamp time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64
	for id, r := range s.records {
		if r.Timestamp.Before(timestamp) {
			delete(s.records, id)
			delete(s.bySeq, r.ChainInfo.Sequence)
			count++
		}
	}

	return count, nil
}

// GetLatestRecord gets the latest audit record
func (s *InMemoryAuditStore) GetLatestRecord() (*AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return nil, ErrAuditNotFound
	}
	return s.latest, nil
}

// GetBySequence gets a record by sequence
func (s *InMemoryAuditStore) GetBySequence(seq int64) (*AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.bySeq[seq]
	if !exists {
		return nil, ErrAuditNotFound
	}
	return record, nil
}

// GetChainRange gets records in a sequence range
func (s *InMemoryAuditStore) GetChainRange(startSeq, endSeq int64) ([]*AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var records []*AuditRecord
	for seq := startSeq; seq <= endSeq; seq++ {
		if r, exists := s.bySeq[seq]; exists {
			records = append(records, r)
		}
	}

	return records, nil
}

// GetStats gets audit statistics
func (s *InMemoryAuditStore) GetStats(startTime, endTime time.Time) (*AuditStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &AuditStats{
		TotalRecords: int64(len(s.records)),
		ByEventType:  make(map[AuditEventType]int),
		ByCategory:   make(map[AuditCategory]int),
		BySeverity:   make(map[AuditSeverity]int),
		ByActor:      make(map[string]int),
		TopActors:    []ActorStat{},
	}

	var failureCount int64
	var totalInTimeRange int64

	for _, r := range s.records {
		if r.Timestamp.Before(startTime) || r.Timestamp.After(endTime) {
			continue
		}

		totalInTimeRange++
		stats.ByEventType[r.EventType]++
		stats.ByCategory[r.Category]++
		stats.BySeverity[r.Severity]++
		stats.ByActor[r.Actor.ID]++

		if r.Outcome == "failure" {
			failureCount++
		}
	}

	// Calculate failure rate
	if totalInTimeRange > 0 {
		stats.FailureRate = float64(failureCount) / float64(totalInTimeRange) * 100
	}

	return stats, nil
}

// CountByFilter counts records matching a filter
func (s *InMemoryAuditStore) CountByFilter(filter AuditFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, r := range s.records {
		if s.matchesFilter(r, filter) {
			count++
		}
	}

	return count, nil
}

// Export exports audit records
func (s *InMemoryAuditStore) Export(filter AuditFilter, format string) ([]byte, error) {
	// InMemoryAuditStore.List 为纯内存过滤，恒返回 nil error，
	// err 分支为死代码，已删除。
	records, _, _ := s.List(filter, AuditPage{PageSize: 100000})

	if format == "json" {
		return json.MarshalIndent(records, "", "  ")
	}

	return exportCSV(records)
}
