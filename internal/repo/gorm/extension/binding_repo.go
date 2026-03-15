package extensiongorm

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

type BindingRepo struct {
	db *gorm.DB
}

func NewBindingRepo(db *gorm.DB) *BindingRepo {
	return &BindingRepo{db: db}
}

func (r *BindingRepo) ReplaceForInstallation(ctx context.Context, installationID uint, bindings []model.ExtensionRuntimeBinding) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Where("installation_id = ?", installationID).Delete(&model.ExtensionRuntimeBinding{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for i := range bindings {
		bindings[i].InstallationID = installationID
		if err := tx.Create(&bindings[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *BindingRepo) ListByInstallationID(ctx context.Context, installationID uint) ([]model.ExtensionRuntimeBinding, error) {
	var items []model.ExtensionRuntimeBinding
	if err := r.db.WithContext(ctx).Where("installation_id = ?", installationID).Order("id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
