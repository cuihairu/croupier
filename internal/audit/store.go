package audit

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cuihairu/croupier/internal/common/dbtype"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// AuditModel is the GORM model for audit records
type AuditModel struct {
	ID             uint        `gorm:"primaryKey;autoIncrement" json:"id"`
	AuditID        string      `gorm:"uniqueIndex;type:varchar(255)" json:"auditId"`
	Timestamp      time.Time   `gorm:"not null;index" json:"timestamp"`
	EventType      string      `gorm:"type:varchar(100);not null;index" json:"eventType"`
	Category       string      `gorm:"type:varchar(50);not null;index" json:"category"`
	Severity       string      `gorm:"type:varchar(20);not null;index" json:"severity"`
	ActorJSON      dbtype.JSON `json:"actorJson"`
	Action         string      `gorm:"type:varchar(255)" json:"action"`
	ResourceJSON   dbtype.JSON `json:"resourceJson"`
	DetailsJSON    dbtype.JSON `json:"detailsJson"`
	ChangesJSON    dbtype.JSON `json:"changesJson"`
	ContextJSON    dbtype.JSON `json:"contextJson"`
	Outcome        string      `gorm:"type:varchar(50);not null;index" json:"outcome"`
	ErrorMessage   string      `gorm:"type:text" json:"errorMessage"`
	ChainHash      string      `gorm:"type:varchar(64);not null;index" json:"chainHash"`
	ChainPrevHash  string      `gorm:"type:varchar(64)" json:"chainPrevHash"`
	ChainSequence  int64       `gorm:"not null;uniqueIndex" json:"chainSequence"`
	ChainSignerID  string      `gorm:"type:varchar(255)" json:"chainSignerId"`
	ChainSignature string      `gorm:"type:text" json:"chainSignature"`
	// Promoted first-class columns derived from Resource/Details at write
	// time (and backfilled for legacy rows). They make game/env scoping and
	// invocation analytics aggregable in plain SQL, keeping queries
	// dialect-neutral (postgres/sqlite have no portable JSON_EXTRACT).
	GameID     string    `gorm:"type:varchar(100);index" json:"gameId,omitempty"`
	Env        string    `gorm:"type:varchar(50);index" json:"env,omitempty"`
	FunctionID string    `gorm:"type:varchar(255);index" json:"functionId,omitempty"`
	DurationMs int64     `gorm:"not null;default:0" json:"durationMs,omitempty"`
	IP         string    `gorm:"type:varchar(64);index" json:"ip,omitempty"`
	ActorID    string    `gorm:"type:varchar(255);index" json:"actorId,omitempty"`
	CreatedAt  time.Time `gorm:"not null" json:"createdAt"`
}

// TableName returns the table name
func (AuditModel) TableName() string {
	return "audit_records"
}

// ToRecord converts model to domain type
func (m *AuditModel) ToRecord() (*AuditRecord, error) {
	record := &AuditRecord{
		ID:           m.AuditID,
		Timestamp:    m.Timestamp,
		EventType:    AuditEventType(m.EventType),
		Category:     AuditCategory(m.Category),
		Severity:     AuditSeverity(m.Severity),
		Action:       m.Action,
		Outcome:      m.Outcome,
		ErrorMessage: m.ErrorMessage,
		ChainInfo: ChainInfo{
			Hash:      m.ChainHash,
			PrevHash:  m.ChainPrevHash,
			Sequence:  m.ChainSequence,
			SignerID:  m.ChainSignerID,
			Signature: m.ChainSignature,
		},
	}

	if m.ActorJSON != nil {
		if err := json.Unmarshal(m.ActorJSON, &record.Actor); err != nil {
			return nil, err
		}
	}

	if m.ResourceJSON != nil {
		if err := json.Unmarshal(m.ResourceJSON, &record.Resource); err != nil {
			return nil, err
		}
	}

	if m.DetailsJSON != nil {
		if err := json.Unmarshal(m.DetailsJSON, &record.Details); err != nil {
			return nil, err
		}
	}

	if m.ChangesJSON != nil {
		if err := json.Unmarshal(m.ChangesJSON, &record.Changes); err != nil {
			return nil, err
		}
	}

	if m.ContextJSON != nil {
		if err := json.Unmarshal(m.ContextJSON, &record.Context); err != nil {
			return nil, err
		}
	}

	return record, nil
}

// FromRecord creates model from domain type
func FromRecord(r *AuditRecord) (*AuditModel, error) {
	model := &AuditModel{
		AuditID:        r.ID,
		Timestamp:      r.Timestamp,
		EventType:      string(r.EventType),
		Category:       string(r.Category),
		Severity:       string(r.Severity),
		Action:         r.Action,
		Outcome:        r.Outcome,
		ErrorMessage:   r.ErrorMessage,
		ChainHash:      r.ChainInfo.Hash,
		ChainPrevHash:  r.ChainInfo.PrevHash,
		ChainSequence:  r.ChainInfo.Sequence,
		ChainSignerID:  r.ChainInfo.SignerID,
		ChainSignature: r.ChainInfo.Signature,
		CreatedAt:      time.Now(),
	}

	if r.Actor.ID != "" {
		data, err := json.Marshal(r.Actor)
		if err != nil {
			return nil, err
		}
		model.ActorJSON = data
	}

	if r.Resource.ID != "" {
		data, err := json.Marshal(r.Resource)
		if err != nil {
			return nil, err
		}
		model.ResourceJSON = data
	}

	if r.Details != nil {
		data, err := json.Marshal(r.Details)
		if err != nil {
			return nil, err
		}
		model.DetailsJSON = data
	}

	if r.Changes != nil {
		data, err := json.Marshal(r.Changes)
		if err != nil {
			return nil, err
		}
		model.ChangesJSON = data
	}

	if r.Context.RequestID != "" {
		data, err := json.Marshal(r.Context)
		if err != nil {
			return nil, err
		}
		model.ContextJSON = data
	}

	model.GameID, model.Env, model.FunctionID, model.DurationMs = derivePromotedFields(r)
	model.IP = strings.TrimSpace(r.Actor.IPAddress)
	model.ActorID = strings.TrimSpace(r.Actor.ID)

	return model, nil
}

// derivePromotedFields extracts the queryable dimensions (game/env/
// function/duration) from a record's Resource and Details payloads so they
// can be stored as first-class columns. Explicit Resource fields win over
// Details keys; duration prefers duration_ms and falls back to elapsed_ms
// (console page.execute semantics).
func derivePromotedFields(r *AuditRecord) (gameID, env, functionID string, durationMs int64) {
	if r == nil {
		return "", "", "", 0
	}
	gameID = strings.TrimSpace(r.Resource.GameID)
	env = strings.TrimSpace(r.Resource.Environment)
	if r.Details != nil {
		if gameID == "" {
			gameID, _ = r.Details["game_id"].(string)
		}
		if env == "" {
			env, _ = r.Details["env"].(string)
		}
		if v, ok := r.Details["function_id"].(string); ok {
			functionID = v
		}
		if d, ok := detailsNumeric(r.Details["duration_ms"]); ok {
			durationMs = int64(d)
		} else if d, ok := detailsNumeric(r.Details["elapsed_ms"]); ok {
			durationMs = int64(d)
		}
	}
	if functionID == "" && r.Resource.Type == "function" {
		functionID = strings.TrimSpace(r.Resource.ID)
	}
	return gameID, env, functionID, durationMs
}

// detailsNumeric coerces JSON numbers (float64 after unmarshal) and ints.
func detailsNumeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

// SQLAuditStore implements AuditStore using SQL
type SQLAuditStore struct {
	db       *gorm.DB
	memCache *auditMemCache
}

type auditMemCache struct {
	records map[string]*AuditRecord
	latest  *AuditRecord
	mu      sync.RWMutex
}

// NewSQLAuditStore creates a new SQL audit store
func NewSQLAuditStore(db *gorm.DB) (*SQLAuditStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	// Schema note: the audit table is created by the versioned migration
	// baseline (internal/svc autoMigrate/autoMigrateMeta). Constructors must
	// not run DDL (docs/architecture/database-migration-strategy.md).

	store := &SQLAuditStore{
		db: db,
		memCache: &auditMemCache{
			records: make(map[string]*AuditRecord),
		},
	}

	// Load latest record into cache
	var model AuditModel
	if err := db.Order("chain_sequence DESC").First(&model).Error; err == nil {
		record, _ := model.ToRecord()
		store.memCache.latest = record
	}

	// Best-effort backfill of promoted columns for legacy invocation rows
	// (written before game_id/env/function_id/duration_ms were first-class
	// columns). Dialect-neutral: parse JSON payloads in Go, batched and
	// capped so startup stays fast.
	store.backfillPromotedFields()

	return store, nil
}

// backfillPromotedFields fills game_id/env/function_id/duration_ms for
// function.invoke / page.execute rows that predate the promoted columns.
func (s *SQLAuditStore) backfillPromotedFields() {
	const (
		batchSize = 500
		maxRows   = 50000
	)
	updated := 0
	for updated < maxRows {
		var rows []AuditModel
		if err := s.db.
			Where("event_type IN ? AND (function_id = '' OR function_id IS NULL)", []string{string(EventFunctionInvoke), string(EventPageExecute)}).
			Order("id").Limit(batchSize).
			Find(&rows).Error; err != nil {
			return
		}
		if len(rows) == 0 {
			return
		}
		fixed := 0
		for _, m := range rows {
			record, err := m.ToRecord()
			if err != nil {
				continue
			}
			gameID, env, functionID, durationMs := derivePromotedFields(record)
			if functionID == "" {
				// Nothing derivable; this row would match the SELECT
				// forever, so don't count it as progress.
				continue
			}
			if err := s.db.Model(&AuditModel{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
				"gameId":      gameID,
				"env":         env,
				"function_id": functionID,
				"duration_ms": durationMs,
			}).Error; err != nil {
				return
			}
			fixed++
			updated++
		}
		if len(rows) < batchSize || fixed == 0 {
			return
		}
	}
}

// Create creates an audit record
func (s *SQLAuditStore) Create(record *AuditRecord) error {
	model, err := FromRecord(record)
	if err != nil {
		return err
	}

	if err := s.db.Create(model).Error; err != nil {
		return err
	}

	// Update cache
	s.memCache.mu.Lock()
	s.memCache.records[record.ID] = record
	s.memCache.latest = record
	s.memCache.mu.Unlock()

	return nil
}

// Get gets an audit record by ID
func (s *SQLAuditStore) Get(id string) (*AuditRecord, error) {
	// Check cache first
	s.memCache.mu.RLock()
	if record, exists := s.memCache.records[id]; exists {
		s.memCache.mu.RUnlock()
		return record, nil
	}
	s.memCache.mu.RUnlock()

	var model AuditModel
	if err := s.db.Where("audit_id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuditNotFound
		}
		return nil, err
	}

	return model.ToRecord()
}

// List lists audit records with filtering and pagination
func (s *SQLAuditStore) List(filter AuditFilter, page AuditPage) ([]*AuditRecord, int, error) {
	query := s.db.Model(&AuditModel{})

	// Apply filters
	if len(filter.EventType) > 0 {
		types := make([]string, len(filter.EventType))
		for i, t := range filter.EventType {
			types[i] = string(t)
		}
		query = query.Where("event_type IN ?", types)
	}

	if len(filter.Category) > 0 {
		categories := make([]string, len(filter.Category))
		for i, c := range filter.Category {
			categories[i] = string(c)
		}
		query = query.Where("category IN ?", categories)
	}

	if len(filter.Severity) > 0 {
		severities := make([]string, len(filter.Severity))
		for i, s := range filter.Severity {
			severities[i] = string(s)
		}
		query = query.Where("severity IN ?", severities)
	}

	if filter.ActorID != "" {
		query = query.Where("json_extract(actor_json, '$.id') = ?", filter.ActorID)
	}

	if filter.ResourceID != "" {
		query = query.Where("json_extract(resource_json, '$.id') = ?", filter.ResourceID)
	}

	if filter.ResourceType != "" {
		query = query.Where("json_extract(resource_json, '$.type') = ?", filter.ResourceType)
	}

	if filter.GameID != "" {
		query = query.Where("json_extract(resource_json, '$.gameId') = ?", filter.GameID)
	}

	if filter.Environment != "" {
		query = query.Where("json_extract(resource_json, '$.environment') = ?", filter.Environment)
	}

	if filter.Outcome != "" {
		query = query.Where("outcome = ?", filter.Outcome)
	}

	if filter.StartTime != nil {
		query = query.Where("timestamp >= ?", filter.StartTime)
	}

	if filter.EndTime != nil {
		query = query.Where("timestamp <= ?", filter.EndTime)
	}

	if filter.SearchText != "" {
		search := "%" + filter.SearchText + "%"
		query = query.Where(
			"audit_id LIKE ? OR action LIKE ? OR error_message LIKE ?",
			search, search, search,
		)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if page.PageSize <= 0 {
		page.PageSize = 50
	}
	if page.PageSize > 1000 {
		page.PageSize = 1000
	}
	if page.Page <= 0 {
		page.Page = 1
	}

	offset := (page.Page - 1) * page.PageSize

	// Apply sorting
	sortBy := "timestamp"
	if page.SortBy != "" {
		switch page.SortBy {
		case "timestamp", "severity", "category":
			sortBy = page.SortBy
		}
	}

	order := "DESC"
	if !page.SortDesc {
		order = "ASC"
	}

	query = query.Order(sortBy + " " + order)

	// Execute query
	var models []AuditModel
	if err := query.Offset(offset).Limit(page.PageSize).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	records := make([]*AuditRecord, len(models))
	for i, model := range models {
		record, err := model.ToRecord()
		if err != nil {
			return nil, 0, err
		}
		records[i] = record
	}

	return records, int(total), nil
}

// Delete deletes an audit record
func (s *SQLAuditStore) Delete(id string) error {
	result := s.db.Delete(&AuditModel{}, "audit_id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	s.memCache.mu.Lock()
	delete(s.memCache.records, id)
	s.memCache.mu.Unlock()

	return nil
}

// DeleteBefore deletes records before a timestamp
func (s *SQLAuditStore) DeleteBefore(timestamp time.Time) (int64, error) {
	result := s.db.Where("timestamp < ?", timestamp).Delete(&AuditModel{})
	return result.RowsAffected, result.Error
}

// GetLatestRecord gets the latest audit record
func (s *SQLAuditStore) GetLatestRecord() (*AuditRecord, error) {
	s.memCache.mu.RLock()
	if s.memCache.latest != nil {
		record := s.memCache.latest
		s.memCache.mu.RUnlock()
		return record, nil
	}
	s.memCache.mu.RUnlock()

	var model AuditModel
	if err := s.db.Order("chain_sequence DESC").First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuditNotFound
		}
		return nil, err
	}

	return model.ToRecord()
}

// GetBySequence gets a record by chain sequence
func (s *SQLAuditStore) GetBySequence(seq int64) (*AuditRecord, error) {
	var model AuditModel
	if err := s.db.Where("chain_sequence = ?", seq).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuditNotFound
		}
		return nil, err
	}

	return model.ToRecord()
}

// GetChainRange gets records in a chain sequence range
func (s *SQLAuditStore) GetChainRange(startSeq, endSeq int64) ([]*AuditRecord, error) {
	var models []AuditModel
	if err := s.db.Where("chain_sequence >= ? AND chain_sequence <= ?", startSeq, endSeq).
		Order("chain_sequence ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	records := make([]*AuditRecord, len(models))
	for i, model := range models {
		record, err := model.ToRecord()
		if err != nil {
			return nil, err
		}
		records[i] = record
	}

	return records, nil
}

// GetStats gets audit statistics
func (s *SQLAuditStore) GetStats(startTime, endTime time.Time) (*AuditStats, error) {
	stats := &AuditStats{
		ByEventType: make(map[AuditEventType]int),
		ByCategory:  make(map[AuditCategory]int),
		BySeverity:  make(map[AuditSeverity]int),
		ByActor:     make(map[string]int),
		TopActors:   []ActorStat{},
	}

	// Total records in range
	var total int64
	s.db.Model(&AuditModel{}).Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Count(&total)
	stats.TotalRecords = total

	// Records today
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	s.db.Model(&AuditModel{}).Where("timestamp >= ?", today).Count(&todayCount)
	stats.RecordsToday = todayCount

	// Records this week
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	var weekCount int64
	s.db.Model(&AuditModel{}).Where("timestamp >= ?", weekStart).Count(&weekCount)
	stats.RecordsThisWeek = weekCount

	// By event type
	var eventTypeCounts []struct {
		EventType string
		Count     int
	}
	s.db.Model(&AuditModel{}).
		Select("event_type, count(*) as count").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("event_type").
		Scan(&eventTypeCounts)
	for _, ec := range eventTypeCounts {
		stats.ByEventType[AuditEventType(ec.EventType)] = ec.Count
	}

	// By category
	var categoryCounts []struct {
		Category string
		Count    int
	}
	s.db.Model(&AuditModel{}).
		Select("category, count(*) as count").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("category").
		Scan(&categoryCounts)
	for _, cc := range categoryCounts {
		stats.ByCategory[AuditCategory(cc.Category)] = cc.Count
	}

	// By severity
	var severityCounts []struct {
		Severity string
		Count    int
	}
	s.db.Model(&AuditModel{}).
		Select("severity, count(*) as count").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("severity").
		Scan(&severityCounts)
	for _, sc := range severityCounts {
		stats.BySeverity[AuditSeverity(sc.Severity)] = sc.Count
	}

	// By actor (from JSON)
	// This is a simplified version - in production, you'd need a separate actor table
	// or use more sophisticated JSON querying

	// Failure rate
	var failureCount int64
	s.db.Model(&AuditModel{}).
		Where("timestamp >= ? AND timestamp <= ? AND outcome = ?", startTime, endTime, "failure").
		Count(&failureCount)
	if total > 0 {
		stats.FailureRate = float64(failureCount) / float64(total) * 100
	}

	return stats, nil
}

// CountByFilter counts records matching a filter
func (s *SQLAuditStore) CountByFilter(filter AuditFilter) (int64, error) {
	query := s.db.Model(&AuditModel{})

	if len(filter.EventType) > 0 {
		types := make([]string, len(filter.EventType))
		for i, t := range filter.EventType {
			types[i] = string(t)
		}
		query = query.Where("event_type IN ?", types)
	}

	if filter.StartTime != nil {
		query = query.Where("timestamp >= ?", filter.StartTime)
	}

	if filter.EndTime != nil {
		query = query.Where("timestamp <= ?", filter.EndTime)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Export exports audit records
func (s *SQLAuditStore) Export(filter AuditFilter, format string) ([]byte, error) {
	// Get all matching records (no pagination)
	records, _, err := s.List(filter, AuditPage{PageSize: 10000})
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(records, "", "  ")
	case "jsonl":
		var buf strings.Builder
		for _, r := range records {
			data, err := json.Marshal(r)
			if err != nil {
				return nil, err
			}
			buf.WriteString(string(data) + "\n")
		}
		return []byte(buf.String()), nil
	case "csv":
		return exportCSV(records)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

func exportCSV(records []*AuditRecord) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Header
	header := []string{
		"ID", "Timestamp", "Event Type", "Category", "Severity",
		"Actor ID", "Actor Type", "Actor Name", "Actor IP",
		"Resource Type", "Resource ID", "Action",
		"Outcome", "Error Message", "Chain Hash", "Chain Sequence",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Data
	for _, r := range records {
		row := []string{
			r.ID,
			r.Timestamp.Format(time.RFC3339),
			string(r.EventType),
			string(r.Category),
			string(r.Severity),
			r.Actor.ID,
			r.Actor.Type,
			r.Actor.Name,
			r.Actor.IPAddress,
			r.Resource.Type,
			r.Resource.ID,
			r.Action,
			r.Outcome,
			r.ErrorMessage,
			r.ChainInfo.Hash,
			strconv.FormatInt(r.ChainInfo.Sequence, 10),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

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
	records, _, err := s.List(filter, AuditPage{PageSize: 100000})
	if err != nil {
		return nil, err
	}

	if format == "json" {
		return json.MarshalIndent(records, "", "  ")
	}

	return exportCSV(records)
}

// AuditWriter wraps io.Writer for audit logging
type AuditWriter struct {
	w    io.Writer
	prev []byte
	mu   sync.Mutex
	seq  int64
}

// NewAuditWriter creates a new audit writer
func NewAuditWriter(w io.Writer) *AuditWriter {
	return &AuditWriter{
		w:    w,
		prev: make([]byte, 32),
		seq:  0,
	}
}

// Write writes an audit record
func (w *AuditWriter) Write(record *AuditRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.seq++
	record.ChainInfo.Sequence = w.seq
	record.ChainInfo.PrevHash = fmt.Sprintf("%x", w.prev)

	// Calculate hash
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	hash := simpleHash(data)
	record.ChainInfo.Hash = hash

	// Write to underlying writer
	data, err = json.Marshal(record)
	if err != nil {
		return err
	}

	if _, err := w.w.Write(append(data, '\n')); err != nil {
		return err
	}

	// Update prev hash
	copy(w.prev, []byte(hash))

	return nil
}

func simpleHash(data []byte) string {
	h := uint32(5381)
	for _, b := range data {
		h = ((h << 5) + h) + uint32(b)
	}
	return fmt.Sprintf("%08x", h)
}
