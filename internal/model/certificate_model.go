package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CertificateModel manages certificate data.
type CertificateModel struct {
	db *gorm.DB
}

// NewCertificateModel returns helper.
func NewCertificateModel(db *gorm.DB) *CertificateModel {
	return &CertificateModel{db: db}
}

// CertificateStatus calculates status by expiry.
func CertificateStatus(expiry time.Time) string {
	now := time.Now()
	switch {
	case expiry.IsZero():
		return "unknown"
	case expiry.Before(now):
		return "expired"
	case expiry.Sub(now) <= 30*24*time.Hour:
		return "expiring"
	default:
		return "active"
	}
}

// ListCertificatesOptions controls pagination/filtering.
type ListCertificatesOptions struct {
	Page     int
	PageSize int
	Status   string
}

// List returns certificates with pagination.
func (m *CertificateModel) List(ctx context.Context, opts ListCertificatesOptions) ([]Certificate, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	query := m.db.WithContext(ctx).Model(&Certificate{})
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var certs []Certificate
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Order("expires_at ASC").Offset(offset).Limit(opts.PageSize).Find(&certs).Error; err != nil {
		return nil, 0, err
	}
	return certs, total, nil
}

// Create stores certificate.
func (m *CertificateModel) Create(ctx context.Context, cert *Certificate) error {
	return m.db.WithContext(ctx).Create(cert).Error
}

// Update updates fields by ID.
func (m *CertificateModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Certificate{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes certificate.
func (m *CertificateModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Certificate{}, id).Error
}

// FindOne fetches certificate by ID.
func (m *CertificateModel) FindOne(ctx context.Context, id uint) (*Certificate, error) {
	var cert Certificate
	if err := m.db.WithContext(ctx).First(&cert, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("证书不存在")
		}
		return nil, err
	}
	return &cert, nil
}

// FindByDomain fetches by domain.
func (m *CertificateModel) FindByDomain(ctx context.Context, domain string) (*Certificate, error) {
	var cert Certificate
	if err := m.db.WithContext(ctx).Where("domain = ?", domain).First(&cert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("证书不存在")
		}
		return nil, err
	}
	return &cert, nil
}

// ExpiringWithin lists certificates expiring within duration.
func (m *CertificateModel) ExpiringWithin(ctx context.Context, d time.Duration) ([]Certificate, error) {
	target := time.Now().Add(d)
	var certs []Certificate
	if err := m.db.WithContext(ctx).
		Where("expires_at <= ?", target).
		Order("expires_at ASC").
		Find(&certs).Error; err != nil {
		return nil, err
	}
	return certs, nil
}

// Stats returns status counts.
func (m *CertificateModel) Stats(ctx context.Context) (map[string]int64, error) {
	result := make(map[string]int64)
	var rows []struct {
		Status string
		Count  int64
	}
	if err := m.db.WithContext(ctx).
		Model(&Certificate{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	var total int64
	for _, row := range rows {
		result[row.Status] = row.Count
		total += row.Count
	}
	result["total"] = total
	return result, nil
}

// ListAlerts returns alerts with pagination.
func (m *CertificateModel) ListAlerts(ctx context.Context, page, size int) ([]CertificateAlert, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	query := m.db.WithContext(ctx).Model(&CertificateAlert{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var alerts []CertificateAlert
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&alerts).Error; err != nil {
		return nil, 0, err
	}
	return alerts, total, nil
}

// AddAlert creates alert.
func (m *CertificateModel) AddAlert(ctx context.Context, alert *CertificateAlert) error {
	return m.db.WithContext(ctx).Create(alert).Error
}

// ListAll returns all certificates.
func (m *CertificateModel) ListAll(ctx context.Context) ([]Certificate, error) {
	var certs []Certificate
	if err := m.db.WithContext(ctx).Find(&certs).Error; err != nil {
		return nil, err
	}
	return certs, nil
}
