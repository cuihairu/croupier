package extensiongorm

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

type InstallationListQuery struct {
	ExtensionID string
	ScopeType   string
	ScopeID     string
	TargetType  string
	TargetID    string
	Status      string
	Enabled     *bool
	Limit       int
	Offset      int
}

type InstallationRepo struct {
	db *gorm.DB
}

func NewInstallationRepo(db *gorm.DB) *InstallationRepo {
	return &InstallationRepo{db: db}
}

func (r *InstallationRepo) Create(ctx context.Context, item *model.ExtensionInstallation) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *InstallationRepo) Save(ctx context.Context, item *model.ExtensionInstallation) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *InstallationRepo) GetByID(ctx context.Context, id uint) (*model.ExtensionInstallation, error) {
	var item model.ExtensionInstallation
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *InstallationRepo) List(ctx context.Context, q InstallationListQuery) ([]model.ExtensionInstallation, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ExtensionInstallation{})
	if q.ExtensionID != "" {
		query = query.Where("extension_id = ?", q.ExtensionID)
	}
	if q.ScopeType != "" {
		query = query.Where("scope_type = ?", q.ScopeType)
	}
	if q.ScopeID != "" {
		query = query.Where("scope_id = ?", q.ScopeID)
	}
	if q.TargetType != "" {
		query = query.Where("target_type = ?", q.TargetType)
	}
	if q.TargetID != "" {
		query = query.Where("target_id = ?", q.TargetID)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Enabled != nil {
		query = query.Where("enabled = ?", *q.Enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}
	if q.Offset > 0 {
		query = query.Offset(q.Offset)
	}
	var items []model.ExtensionInstallation
	if err := query.Order("id desc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
