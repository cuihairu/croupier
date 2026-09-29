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

// GetByExtensionIDs fetches catalog rows by a set of extension ids (for list assembly to avoid N+1 queries); missing ids are simply absent.
func (r *CatalogRepo) GetByExtensionIDs(ctx context.Context, extensionIDs []string) ([]model.ExtensionCatalog, error) {
	if len(extensionIDs) == 0 {
		return nil, nil
	}
	var items []model.ExtensionCatalog
	if err := r.db.WithContext(ctx).Where("extension_id IN ?", extensionIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CatalogRepo) Create(ctx context.Context, item *model.ExtensionCatalog) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// UpdateByExtensionID updates specified columns by extension_id (updates only when the row exists; returns ErrRecordNotFound when the row does not exist).
func (r *CatalogRepo) UpdateByExtensionID(ctx context.Context, extensionID string, updates map[string]any) error {
	res := r.db.WithContext(ctx).Model(&model.ExtensionCatalog{}).Where("extension_id = ?", extensionID).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteByExtensionID physically removes the catalog row (soft-deleted rows would continue to occupy the extension_id unique index, causing re-registration conflicts).
func (r *CatalogRepo) DeleteByExtensionID(ctx context.Context, extensionID string) error {
	res := r.db.WithContext(ctx).Unscoped().Where("extension_id = ?", extensionID).Delete(&model.ExtensionCatalog{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
