package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// BehaviorModel handles behavior analytics persistence.
type BehaviorModel struct {
	db *gorm.DB
}

// NewBehaviorModel creates a new behavior model helper.
func NewBehaviorModel(db *gorm.DB) *BehaviorModel {
	return &BehaviorModel{db: db}
}

// BehaviorEventOptions controls event queries.
type BehaviorEventOptions struct {
	PaginationOptions
	GameID    string
	Env       string
	EventType string
	StartTime time.Time
	EndTime   time.Time
}

// RecordEvent stores a new behavior event.
func (m *BehaviorModel) RecordEvent(ctx context.Context, event *BehaviorEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = NowUTC()
	}
	return m.db.WithContext(ctx).Create(event).Error
}

// ListEvents returns paginated behavior events.
func (m *BehaviorModel) ListEvents(ctx context.Context, opts BehaviorEventOptions) ([]BehaviorEvent, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []BehaviorEvent
		total int64
	)

	query := m.db.WithContext(ctx).Model(&BehaviorEvent{})

	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.EventType != "" {
		query = query.Where("event_type = ?", opts.EventType)
	}
	if !opts.StartTime.IsZero() {
		query = query.Where("occurred_at >= ?", opts.StartTime)
	}
	if !opts.EndTime.IsZero() {
		query = query.Where("occurred_at <= ?", opts.EndTime)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("occurred_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// UpsertFeatureAdoption stores aggregated adoption data.
func (m *BehaviorModel) UpsertFeatureAdoption(ctx context.Context, adoption *FeatureAdoption) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(adoption).Error
}

// ListFeatureAdoptions fetches adoption snapshots.
func (m *BehaviorModel) ListFeatureAdoptions(ctx context.Context, gameID, env string) ([]FeatureAdoption, error) {
	query := m.db.WithContext(ctx).Model(&FeatureAdoption{})
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}

	var items []FeatureAdoption
	if err := query.Order("feature ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// BehaviorDailyStat describes per-day activity metrics.
type BehaviorDailyStat struct {
	Day         time.Time
	ActiveUsers int64
	Events      int64
}

// EventTypeCount represents aggregated counts per event type.
type EventTypeCount struct {
	EventType string
	Total     int64
}

// CountDistinctUsers returns the number of unique users within the provided window.
func (m *BehaviorModel) CountDistinctUsers(ctx context.Context, gameID, env string, start, end time.Time) (int64, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end)
	var count int64
	if err := query.Distinct("user_id").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountEvents returns total events within the provided window.
func (m *BehaviorModel) CountEvents(ctx context.Context, gameID, env string, start, end time.Time) (int64, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// EventTypeCounts aggregates events by type, ordered by volume.
func (m *BehaviorModel) EventTypeCounts(ctx context.Context, gameID, env string, start, end time.Time, limit int) ([]EventTypeCount, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end).
		Select("event_type, COUNT(*) AS total").
		Group("event_type").
		Order("total DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var rows []EventTypeCount
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DailyActivity returns per-day active users and event counts between the provided range.
func (m *BehaviorModel) DailyActivity(ctx context.Context, gameID, env string, start, end time.Time) ([]BehaviorDailyStat, error) {
	type row struct {
		Day         string
		ActiveUsers int64
		Events      int64
	}

	query := m.scopedEvents(ctx, gameID, env, start, end).
		Select("DATE(occurred_at) AS day, COUNT(DISTINCT user_id) AS active_users, COUNT(*) AS events").
		Group("day").
		Order("day ASC")

	var rows []row
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make([]BehaviorDailyStat, 0, len(rows))
	for _, r := range rows {
		day, err := time.Parse("2006-01-02", r.Day)
		if err != nil {
			day = time.Time{}
		}
		stats = append(stats, BehaviorDailyStat{
			Day:         day,
			ActiveUsers: r.ActiveUsers,
			Events:      r.Events,
		})
	}
	return stats, nil
}

func (m *BehaviorModel) scopedEvents(ctx context.Context, gameID, env string, start, end time.Time) *gorm.DB {
	query := m.db.WithContext(ctx).Model(&BehaviorEvent{})
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}
	if !start.IsZero() {
		query = query.Where("occurred_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("occurred_at <= ?", end)
	}
	return query
}
