package extensiongorm

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

type CatalogListQuery struct {
	Keyword string
	Kind    string
	Status  string
	Limit   int
	Offset  int
}

type CatalogRepo struct {
	db *gorm.DB
}

func NewCatalogRepo(db *gorm.DB) *CatalogRepo {
	return &CatalogRepo{db: db}
}

func (r *CatalogRepo) List(ctx context.Context, q CatalogListQuery) ([]model.ExtensionCatalog, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ExtensionCatalog{})
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		query = query.Where("extension_id LIKE ? OR name LIKE ? OR display_name LIKE ?", like, like, like)
	}
	if q.Kind != "" {
		query = query.Where("kind = ?", q.Kind)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
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
	var items []model.ExtensionCatalog
	if err := query.Order("id desc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CatalogRepo) GetByExtensionID(ctx context.Context, extensionID string) (*model.ExtensionCatalog, error) {
	var item model.ExtensionCatalog
	if err := r.db.WithContext(ctx).Where("extension_id = ?", extensionID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
