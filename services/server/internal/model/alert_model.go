package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// AlertModel provides CRUD operations for alerts and silences.
type AlertModel struct {
	db *gorm.DB
}

// NewAlertModel creates a new alert model helper.
func NewAlertModel(db *gorm.DB) *AlertModel {
	return &AlertModel{db: db}
}

// ListAlertsOptions controls filtering/pagination.
type ListAlertsOptions struct {
	PaginationOptions
	Level  string
	Status string
	Source string
}

// Create inserts a new alert.
func (m *AlertModel) Create(ctx context.Context, alert *Alert) error {
	return m.db.WithContext(ctx).Create(alert).Error
}

// UpdateStatus updates alert status.
func (m *AlertModel) UpdateStatus(ctx context.Context, id uint, status string) error {
	return m.db.WithContext(ctx).
		Model(&Alert{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// List returns paginated alerts.
func (m *AlertModel) List(ctx context.Context, opts ListAlertsOptions) ([]Alert, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []Alert
		total int64
	)

	query := m.db.WithContext(ctx).Model(&Alert{})

	if opts.Level != "" {
		query = query.Where("level = ?", opts.Level)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Source != "" {
		query = query.Where("source = ?", opts.Source)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateSilence records a new silence window.
func (m *AlertModel) CreateSilence(ctx context.Context, silence *AlertSilence) error {
	if silence.ExpiresAt.IsZero() && silence.DurationMinute > 0 {
		silence.ExpiresAt = NowUTC().Add(time.Duration(silence.DurationMinute) * time.Minute)
	}
	return m.db.WithContext(ctx).Create(silence).Error
}

// ListSilencesOptions filters silence entries.
type ListSilencesOptions struct {
	ActiveOnly bool
}

// ListSilences returns alert silences.
func (m *AlertModel) ListSilences(ctx context.Context, opts ListSilencesOptions) ([]AlertSilence, error) {
	query := m.db.WithContext(ctx).Model(&AlertSilence{})
	if opts.ActiveOnly {
		query = query.Where("expires_at > ?", NowUTC())
	}

	var silences []AlertSilence
	if err := query.Order("created_at DESC").Find(&silences).Error; err != nil {
		return nil, err
	}
	return silences, nil
}

// PruneExpiredSilences removes expired silences.
func (m *AlertModel) PruneExpiredSilences(ctx context.Context) error {
	return m.db.WithContext(ctx).
		Where("expires_at <= ?", NowUTC()).
		Delete(&AlertSilence{}).Error
}
