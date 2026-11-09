package audit

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Create creates an audit record
func (s *SQLAuditStore) Create(record *AuditRecord) error {
	model, err := FromRecord(record)
	if err != nil {
		return err
	}

	if err := s.db.Create(model).Error; err != nil {
		if isChainSequenceConflict(err) {
			// 本实例 memCache 视角的链尾已过期（对端实例已写入更高
			// sequence）。失效缓存，让调用方的重试经 GetLatestRecord
			// 重查 DB 拿真实链尾重算——否则重试永远用同一旧值。
			s.memCache.mu.Lock()
			s.memCache.latest = nil
			s.memCache.mu.Unlock()
			return fmt.Errorf("%w: %v", ErrChainSequenceConflict, err)
		}
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

	// 时间参数归一 UTC：落库恒为 UTC，sqlite 序列化后逐字节比较（见 GetStats 注）。
	if filter.StartTime != nil {
		query = query.Where("timestamp >= ?", filter.StartTime.UTC())
	}

	if filter.EndTime != nil {
		query = query.Where("timestamp <= ?", filter.EndTime.UTC())
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
	result := s.db.Where("timestamp < ?", timestamp.UTC()).Delete(&AuditModel{})
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
	// 审计行落库恒为 UTC（NewAuditRecord 用 time.Now().UTC()），sqlite 把
	// time.Time 序列化成带偏移的字符串逐字节比较，参数必须同样归一到 UTC，
	// 否则非 UTC 时区的服务器上时间窗统计全错（postgres 走真实时间类型，不受影响）。
	startTime = startTime.UTC()
	endTime = endTime.UTC()
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
	today := time.Now().Truncate(24 * time.Hour).UTC()
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
		query = query.Where("timestamp >= ?", filter.StartTime.UTC())
	}

	if filter.EndTime != nil {
		query = query.Where("timestamp <= ?", filter.EndTime.UTC())
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
			// records 全部经 DB 反序列化而来，Details 等动态字段均为合法 JSON
			// 产物（无 NaN/func 值），Marshal 恒成功，err 分支为死代码，已删除。
			data, _ := json.Marshal(r)
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
	// csv.Writer 的错误仅透传底层 writer 错误，而 strings.Builder.Write 恒返回
	// nil，故 Write/Flush 的 err 分支均为死代码，已删除。
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Header
	header := []string{
		"ID", "Timestamp", "Event Type", "Category", "Severity",
		"Actor ID", "Actor Type", "Actor Name", "Actor IP",
		"Resource Type", "Resource ID", "Action",
		"Outcome", "Error Message", "Chain Hash", "Chain Sequence",
	}
	_ = writer.Write(header)

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
		_ = writer.Write(row)
	}

	writer.Flush()

	return []byte(buf.String()), nil
}
