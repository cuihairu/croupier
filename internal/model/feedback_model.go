package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// FeedbackModel manages feedback entries.
type FeedbackModel struct {
	db *gorm.DB
}

// NewFeedbackModel creates helper.
func NewFeedbackModel(db *gorm.DB) *FeedbackModel {
	return &FeedbackModel{db: db}
}

// ListFeedbackOptions controls filtering.
type ListFeedbackOptions struct {
	PaginationOptions
	GameID   string
	Env      string
	Status   string
	Category string
	Keyword  string
}

// Create inserts feedback entry.
func (m *FeedbackModel) Create(ctx context.Context, feedback *Feedback) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(feedback).Error
}

// Update changes feedback fields.
func (m *FeedbackModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&Feedback{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes feedback entry.
func (m *FeedbackModel) Delete(ctx context.Context, id uint) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Delete(&Feedback{}, id).Error
}

// FindByID returns a single feedback record.
func (m *FeedbackModel) FindByID(ctx context.Context, id uint) (*Feedback, error) {
	var record Feedback
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// List fetches paginated feedback entries.
func (m *FeedbackModel) List(ctx context.Context, opts ListFeedbackOptions) ([]Feedback, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []Feedback
		total int64
	)

	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&Feedback{})
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Keyword != "" {
		like := "%" + opts.Keyword + "%"
		query = query.Where("content LIKE ?", like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FeedbackStatsOptions configures stats calculations.
type FeedbackStatsOptions struct {
	GameID string
	Days   int
}

// FeedbackStatsResult aggregates statistics for feedback.
type FeedbackStatsResult struct {
	Total      int64
	ByCategory map[string]int64
	ByStatus   map[string]int64
	AvgRating  float64
	Responded  int64
}

// Stats returns aggregate metrics for feedback entries.
func (m *FeedbackModel) Stats(ctx context.Context, opts FeedbackStatsOptions) (*FeedbackStatsResult, error) {
	query := m.statsQuery(ctx, opts)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	result := &FeedbackStatsResult{
		Total:      total,
		ByCategory: map[string]int64{},
		ByStatus:   map[string]int64{},
		AvgRating:  0,
	}

	var categories []struct {
		Category string
		Count    int64
	}
	if err := m.statsQuery(ctx, opts).
		Select("category, COUNT(*) as count").
		Group("category").
		Scan(&categories).Error; err != nil {
		return nil, err
	}
	for _, row := range categories {
		result.ByCategory[row.Category] = row.Count
	}

	var statuses []struct {
		Status string
		Count  int64
	}
	if err := m.statsQuery(ctx, opts).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statuses).Error; err != nil {
		return nil, err
	}
	for _, row := range statuses {
		result.ByStatus[row.Status] = row.Count
	}

	var avg sql.NullFloat64
	if err := m.statsQuery(ctx, opts).
		Select("AVG(rating) as avg_rating").
		Scan(&avg).Error; err != nil {
		return nil, err
	}
	if avg.Valid {
		result.AvgRating = avg.Float64
	}

	var responded int64
	if err := m.statsQuery(ctx, opts).
		Where("reply <> ''").
		Count(&responded).Error; err != nil {
		return nil, err
	}
	result.Responded = responded

	return result, nil
}

func (m *FeedbackModel) statsQuery(ctx context.Context, opts FeedbackStatsOptions) *gorm.DB {
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&Feedback{})
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Days > 0 {
		since := time.Now().Add(-time.Duration(opts.Days) * 24 * time.Hour)
		query = query.Where("created_at >= ?", since)
	}
	return query
}
