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
