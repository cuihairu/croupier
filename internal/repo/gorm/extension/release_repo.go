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

func (r *ReleaseRepo) GetByExtensionIDAndVersion(ctx context.Context, extensionID, version string) (*model.ExtensionRelease, error) {
	var item model.ExtensionRelease
	if err := r.db.WithContext(ctx).Where("extension_id = ? AND version = ?", extensionID, version).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ReleaseRepo) Create(ctx context.Context, item *model.ExtensionRelease) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// DeleteByExtensionID 物理删除该扩展全部版本（catalog 移除时级联；避免软删行占用唯一检索语义）。
func (r *ReleaseRepo) DeleteByExtensionID(ctx context.Context, extensionID string) error {
	return r.db.WithContext(ctx).Unscoped().Where("extension_id = ?", extensionID).Delete(&model.ExtensionRelease{}).Error
}
