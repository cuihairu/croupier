package model

import (
	"context"

	"gorm.io/gorm"
)

// FAQModel manages FAQ data.
type FAQModel struct {
	db *gorm.DB
}

// NewFAQModel creates a helper.
func NewFAQModel(db *gorm.DB) *FAQModel {
	return &FAQModel{db: db}
}

// ListFAQOptions controls listing.
type ListFAQOptions struct {
	PaginationOptions
	Category string
	Keyword  string
	Visible  *bool
}

// Create inserts a new FAQ.
func (m *FAQModel) Create(ctx context.Context, faq *FAQ) error {
	return m.db.WithContext(ctx).Create(faq).Error
}

// Update applies updates to FAQ.
func (m *FAQModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&FAQ{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a FAQ.
func (m *FAQModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&FAQ{}, id).Error
}

// FindOne loads a FAQ.
func (m *FAQModel) FindOne(ctx context.Context, id uint) (*FAQ, error) {
	var faq FAQ
	if err := m.db.WithContext(ctx).First(&faq, id).Error; err != nil {
		return nil, err
	}
	return &faq, nil
}

// List returns paginated FAQs.
func (m *FAQModel) List(ctx context.Context, opts ListFAQOptions) ([]FAQ, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []FAQ
		total int64
	)

	query := m.db.WithContext(ctx).Model(&FAQ{})
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Keyword != "" {
		like := "%" + opts.Keyword + "%"
		query = query.Where("question LIKE ? OR answer LIKE ?", like, like)
	}
	if opts.Visible != nil {
		query = query.Where("visible = ?", *opts.Visible)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("sort DESC, updated_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpsertCategory stores category metadata.
func (m *FAQModel) UpsertCategory(ctx context.Context, category *FAQCategory) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(category).Error
}

// ListCategories fetches FAQ categories.
type FAQCategoryStat struct {
	Name  string
	Count int
}

func (m *FAQModel) ListCategories(ctx context.Context) ([]FAQCategoryStat, error) {
	type aggResult struct {
		Name  string
		Count int64
	}

	var rows []aggResult
	if err := m.db.WithContext(ctx).
		Model(&FAQ{}).
		Select("category AS name, COUNT(*) AS count").
		Group("category").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make([]FAQCategoryStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, FAQCategoryStat{
			Name:  row.Name,
			Count: int(row.Count),
		})
	}
	return stats, nil
}
