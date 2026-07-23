package model

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// PageSpec stores the draft state of a page.
type PageSpec struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	PageKey            string         `gorm:"size:128;uniqueIndex" json:"pageKey"`
	Type               string         `gorm:"size:32" json:"type"` // entity/operation/task/report
	ResourceKey        string         `gorm:"size:128;index" json:"resourceKey,omitempty"`
	TitleJSON          string         `gorm:"type:json" json:"-"` // LocalizedText
	DescriptionJSON    string         `gorm:"type:json" json:"-"` // LocalizedText
	CategoryKey        string         `gorm:"size:64;index" json:"categoryKey"`
	CategoryLabelsJSON string         `gorm:"type:json" json:"-"` // LocalizedText
	CategoryOrder      int            `gorm:"default:0" json:"categoryOrder"`
	Order              int            `gorm:"default:0" json:"order"`
	Icon               string         `gorm:"size:64" json:"icon,omitempty"`
	SchemaJSON         string         `gorm:"type:json" json:"-"`                    // FormilySchema
	BindingsJSON       string         `gorm:"type:json" json:"-"`                    // []PageFunctionBinding
	MetadataJSON       string         `gorm:"type:json" json:"-"`                    // map[string]json.RawMessage
	Status             string         `gorm:"size:32;default:'draft'" json:"status"` // draft/published/archived
	PublishedActive    bool           `gorm:"default:false;index" json:"publishedActive"`
	DraftVersion       int            `gorm:"default:1" json:"draftVersion"`
	PublishedVersion   int            `gorm:"default:0" json:"publishedVersion"`
	UpdatedBy          string         `gorm:"size:128" json:"updatedBy,omitempty"`
}

func (PageSpec) TableName() string {
	return "page_specs"
}

// GetTitle returns the parsed title LocalizedText.
func (p *PageSpec) GetTitle() map[string]string {
	var title map[string]string
	if p.TitleJSON != "" {
		json.Unmarshal([]byte(p.TitleJSON), &title)
	}
	return title
}

// SetTitle sets the title from LocalizedText.
func (p *PageSpec) SetTitle(title map[string]string) error {
	b, err := json.Marshal(title)
	if err != nil {
		return err
	}
	p.TitleJSON = string(b)
	return nil
}

// GetCategoryLabels returns the parsed category labels.
func (p *PageSpec) GetCategoryLabels() map[string]string {
	var labels map[string]string
	if p.CategoryLabelsJSON != "" {
		json.Unmarshal([]byte(p.CategoryLabelsJSON), &labels)
	}
	return labels
}

// SetCategoryLabels sets the category labels.
func (p *PageSpec) SetCategoryLabels(labels map[string]string) error {
	b, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	p.CategoryLabelsJSON = string(b)
	return nil
}

// GetSchema returns the parsed Formily schema.
func (p *PageSpec) GetSchema() json.RawMessage {
	return json.RawMessage(p.SchemaJSON)
}

// SetSchema sets the Formily schema.
func (p *PageSpec) SetSchema(schema json.RawMessage) {
	p.SchemaJSON = string(schema)
}

// GetBindings returns the parsed bindings.
func (p *PageSpec) GetBindings() []PageFunctionBindingBinding {
	var bindings []PageFunctionBindingBinding
	if p.BindingsJSON != "" {
		json.Unmarshal([]byte(p.BindingsJSON), &bindings)
	}
	return bindings
}

// SetBindings sets the bindings.
func (p *PageSpec) SetBindings(bindings []PageFunctionBindingBinding) error {
	b, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	p.BindingsJSON = string(b)
	return nil
}

// PageFunctionBindingBinding is the GORM model for page function bindings.
type PageFunctionBindingBinding struct {
	FunctionID string `json:"functionId"`
	Role       string `json:"role"`
}

// PublishedPageSpec stores an immutable snapshot of a published page.
type PublishedPageSpec struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time  `json:"createdAt"`
	PageKey       string     `gorm:"size:128;index" json:"pageKey"`
	Version       int        `gorm:"index" json:"version"`
	SpecJSON      string     `gorm:"type:json" json:"-"` // Full PageSpec JSON
	Active        bool       `gorm:"default:true;index" json:"active"`
	PublishedAt   time.Time  `json:"publishedAt"`
	UnpublishedAt *time.Time `json:"unpublishedAt,omitempty"`
	PublishedBy   string     `gorm:"size:128" json:"publishedBy,omitempty"`

	// Unique constraint on (page_key, version)
}

func (PublishedPageSpec) TableName() string {
	return "published_page_specs"
}

// PageVersion stores version history for page specs.
type PageVersion struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	PageKey   string    `gorm:"size:128;index" json:"pageKey"`
	Version   int       `gorm:"index" json:"version"`
	SpecJSON  string    `gorm:"type:json" json:"-"`    // Full PageSpec JSON
	Status    string    `gorm:"size:32" json:"status"` // draft/published
	Message   string    `gorm:"size:512" json:"message,omitempty"`
	CreatedBy string    `gorm:"size:128" json:"createdBy,omitempty"`
}

func (PageVersion) TableName() string {
	return "page_versions"
}

// PageSpecModel provides data access for page specs.
type PageSpecModel struct {
	db *gorm.DB
}

// NewPageSpecModel creates a new PageSpecModel.
func NewPageSpecModel(db *gorm.DB) *PageSpecModel {
	return &PageSpecModel{db: db}
}

// FindByPageKey returns a page spec by page key.
func (m *PageSpecModel) FindByPageKey(ctx context.Context, pageKey string) (*PageSpec, error) {
	var ps PageSpec
	if err := m.db.WithContext(ctx).Where("page_key = ?", pageKey).First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// ListAll returns all page specs ordered by category and order.
func (m *PageSpecModel) ListAll(ctx context.Context) ([]PageSpec, error) {
	var items []PageSpec
	if err := m.db.WithContext(ctx).
		Order("category_order ASC, `order` ASC, page_key ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByStatus returns page specs filtered by status.
func (m *PageSpecModel) ListByStatus(ctx context.Context, status string) ([]PageSpec, error) {
	var items []PageSpec
	if err := m.db.WithContext(ctx).
		Where("status = ?", status).
		Order("category_order ASC, `order` ASC, page_key ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Upsert creates or updates a page spec.
func (m *PageSpecModel) Upsert(ctx context.Context, ps *PageSpec) error {
	var existing PageSpec
	err := m.db.WithContext(ctx).Where("page_key = ?", ps.PageKey).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return m.db.WithContext(ctx).Create(ps).Error
	}
	if err != nil {
		return err
	}
	ps.ID = existing.ID
	ps.CreatedAt = existing.CreatedAt
	return m.db.WithContext(ctx).Save(ps).Error
}

// Delete removes a page spec by page key.
func (m *PageSpecModel) Delete(ctx context.Context, pageKey string) error {
	return m.db.WithContext(ctx).Where("page_key = ?", pageKey).Delete(&PageSpec{}).Error
}

// PublishedPageSpecModel provides data access for published page specs.
type PublishedPageSpecModel struct {
	db *gorm.DB
}

// NewPublishedPageSpecModel creates a new PublishedPageSpecModel.
func NewPublishedPageSpecModel(db *gorm.DB) *PublishedPageSpecModel {
	return &PublishedPageSpecModel{db: db}
}

// FindByPageKeyAndVersion returns a published page spec.
func (m *PublishedPageSpecModel) FindByPageKeyAndVersion(ctx context.Context, pageKey string, version int) (*PublishedPageSpec, error) {
	var ps PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("page_key = ? AND version = ?", pageKey, version).
		First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// FindLatestByPageKey returns the latest published version of a page.
func (m *PublishedPageSpecModel) FindLatestByPageKey(ctx context.Context, pageKey string) (*PublishedPageSpec, error) {
	var ps PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("page_key = ? AND active = ?", pageKey, true).
		Order("version DESC").
		First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// ListAll returns all published page specs.
func (m *PublishedPageSpecModel) ListAll(ctx context.Context) ([]PublishedPageSpec, error) {
	var items []PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Order("page_key ASC, version DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListLatestActive returns the latest active published version of each page.
func (m *PublishedPageSpecModel) ListLatestActive(ctx context.Context) ([]PublishedPageSpec, error) {
	var items []PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("active = ? AND id IN (SELECT MAX(id) FROM published_page_specs WHERE active = ? GROUP BY page_key)", true, true).
		Order("page_key ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeactivatePage marks all published snapshots of a page inactive.
func (m *PublishedPageSpecModel) DeactivatePage(ctx context.Context, pageKey string, at time.Time) error {
	return m.db.WithContext(ctx).
		Model(&PublishedPageSpec{}).
		Where("page_key = ? AND active = ?", pageKey, true).
		Select("active", "unpublished_at").
		Updates(PublishedPageSpec{
			Active:        false,
			UnpublishedAt: &at,
		}).Error
}

// Create inserts a new published page spec.
func (m *PublishedPageSpecModel) Create(ctx context.Context, ps *PublishedPageSpec) error {
	return m.db.WithContext(ctx).Create(ps).Error
}

// PageVersionModel provides data access for page versions.
type PageVersionModel struct {
	db *gorm.DB
}

// NewPageVersionModel creates a new PageVersionModel.
func NewPageVersionModel(db *gorm.DB) *PageVersionModel {
	return &PageVersionModel{db: db}
}

// ListByPageKey returns version history for a page.
func (m *PageVersionModel) ListByPageKey(ctx context.Context, pageKey string) ([]PageVersion, error) {
	var items []PageVersion
	if err := m.db.WithContext(ctx).
		Where("page_key = ?", pageKey).
		Order("version DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Create inserts a new page version record.
func (m *PageVersionModel) Create(ctx context.Context, pv *PageVersion) error {
	return m.db.WithContext(ctx).Create(pv).Error
}

// GetNextVersion returns the next version number for a page.
func (m *PageVersionModel) GetNextVersion(ctx context.Context, pageKey string) (int, error) {
	var maxVersion int
	err := m.db.WithContext(ctx).
		Model(&PageVersion{}).
		Where("page_key = ?", pageKey).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	return maxVersion + 1, err
}
