package model

import (
	"context"

	"gorm.io/gorm"
)

// RateLimitModel manages rate limit configs.
type RateLimitModel struct {
	db *gorm.DB
}

// NewRateLimitModel creates helper.
func NewRateLimitModel(db *gorm.DB) *RateLimitModel {
	return &RateLimitModel{db: db}
}

// Upsert stores a rate limit entry.
func (m *RateLimitModel) Upsert(ctx context.Context, rl *RateLimit) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(rl).Error
}

// FindByID loads a rate limit by numeric primary key.
func (m *RateLimitModel) FindByID(ctx context.Context, id uint) (*RateLimit, error) {
	var limit RateLimit
	if err := m.db.WithContext(ctx).First(&limit, id).Error; err != nil {
		return nil, err
	}
	return &limit, nil
}

// FindByKey loads a rate limit via RateLimitID.
func (m *RateLimitModel) FindByKey(ctx context.Context, key string) (*RateLimit, error) {
	var limit RateLimit
	if err := m.db.WithContext(ctx).
		Where("rate_limit_id = ?", key).
		First(&limit).Error; err != nil {
		return nil, err
	}
	return &limit, nil
}

// Delete removes a rate limit.
func (m *RateLimitModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&RateLimit{}, id).Error
}

// DeleteByKey removes a rate limit by RateLimitID.
func (m *RateLimitModel) DeleteByKey(ctx context.Context, key string) error {
	result := m.db.WithContext(ctx).
		Where("rate_limit_id = ?", key).
		Delete(&RateLimit{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// List returns rate limits filtered by resource.
func (m *RateLimitModel) List(ctx context.Context, resource string) ([]RateLimit, error) {
	query := m.db.WithContext(ctx).Model(&RateLimit{})
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}

	var limits []RateLimit
	if err := query.Order("updated_at DESC").Find(&limits).Error; err != nil {
		return nil, err
	}
	return limits, nil
}
