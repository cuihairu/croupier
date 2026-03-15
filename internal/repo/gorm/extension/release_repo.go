package extensiongorm

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

type ReleaseRepo struct {
	db *gorm.DB
}

func NewReleaseRepo(db *gorm.DB) *ReleaseRepo {
	return &ReleaseRepo{db: db}
}

func (r *ReleaseRepo) ListByExtensionID(ctx context.Context, extensionID string) ([]model.ExtensionRelease, error) {
	var items []model.ExtensionRelease
	if err := r.db.WithContext(ctx).Where("extension_id = ?", extensionID).Order("id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
