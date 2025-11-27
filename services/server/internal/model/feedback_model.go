package model

import (
	"context"

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
	GameID  string
	Env     string
	Status  string
	Keyword string
}

// Create inserts feedback entry.
func (m *FeedbackModel) Create(ctx context.Context, feedback *Feedback) error {
	return m.db.WithContext(ctx).Create(feedback).Error
}

// Update changes feedback fields.
func (m *FeedbackModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Feedback{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes feedback entry.
func (m *FeedbackModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Feedback{}, id).Error
}

// List fetches paginated feedback entries.
func (m *FeedbackModel) List(ctx context.Context, opts ListFeedbackOptions) ([]Feedback, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []Feedback
		total int64
	)

	query := m.db.WithContext(ctx).Model(&Feedback{})
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
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
