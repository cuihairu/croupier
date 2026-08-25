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
	Tag      string
	Visible  *bool
	// OrderByHelpful orders by helpful ratio first (content governance view).
	OrderByHelpful bool
}

// Create inserts a new FAQ.
func (m *FAQModel) Create(ctx context.Context, faq *FAQ) error {
	return m.db.WithContext(ctx).Create(faq).Error
}

// SlugExists reports whether a non-empty slug is already taken by another
// FAQ entry (service-layer uniqueness check; see model comment).
func (m *FAQModel) SlugExists(ctx context.Context, slug string, excludeID uint) (bool, error) {
	if slug == "" {
		return false, nil
	}
	var count int64
	err := m.db.WithContext(ctx).Model(&FAQ{}).
		Where("slug = ? AND id <> ?", slug, excludeID).
		Count(&count).Error
	return count > 0, err
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
	if opts.Tag != "" {
		// Tags are stored as a JSON string array; substring match is the
		// portable filter across the three supported dialects.
		query = query.Where("tags LIKE ?", `%"`+opts.Tag+`"%`)
	}
	if opts.Visible != nil {
		query = query.Where("visible = ?", *opts.Visible)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "sort DESC, updated_at DESC"
	if opts.OrderByHelpful {
		order = "(helpful_count + unhelpful_count) DESC, helpful_count DESC, updated_at DESC"
	}
	if err := query.Order(order).
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Vote records a helpful/unhelpful vote on one FAQ entry. Columns are
// incremented atomically so concurrent player votes never lose updates.
func (m *FAQModel) Vote(ctx context.Context, id uint, helpful bool) error {
	column := "unhelpful_count"
	if helpful {
		column = "helpful_count"
	}
	res := m.db.WithContext(ctx).Model(&FAQ{}).
		Where("id = ?", id).
		Update(column, gorm.Expr(column+" + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
