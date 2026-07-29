package model

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PageSpec stores the draft state of a page.
type PageSpec struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	GameID             string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_page_specs_scope_key,priority:1;index:idx_page_specs_scope,priority:1" json:"gameId"`
	Env                string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_page_specs_scope_key,priority:2;index:idx_page_specs_scope,priority:2" json:"env"`
	PageKey            string         `gorm:"size:128;not null;uniqueIndex:uidx_page_specs_scope_key,priority:3" json:"pageKey"`
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
	DraftRevision      int            `gorm:"default:1" json:"draftRevision"`
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
	ID            string                  `json:"id"`
	FunctionID    string                  `json:"functionId"`
	Usage         string                  `json:"usage"`
	InputMapping  json.RawMessage         `json:"inputMapping,omitempty"`
	OutputMapping json.RawMessage         `json:"outputMapping,omitempty"`
	Execution     BindingExecutionBinding `json:"execution"`
}

type BindingExecutionBinding struct {
	Mode           string `json:"mode"`
	RequireConfirm bool   `json:"requireConfirm,omitempty"`
}

// PublishedPageSpec stores an immutable snapshot of a published page.
type PublishedPageSpec struct {
	ID                    uint       `gorm:"primarykey" json:"id"`
	CreatedAt             time.Time  `json:"createdAt"`
	GameID                string     `gorm:"size:64;not null;default:'';uniqueIndex:uidx_published_page_specs_scope_version,priority:1;index:idx_published_page_specs_scope,priority:1" json:"gameId"`
	Env                   string     `gorm:"size:64;not null;default:'';uniqueIndex:uidx_published_page_specs_scope_version,priority:2;index:idx_published_page_specs_scope,priority:2" json:"env"`
	PageKey               string     `gorm:"size:128;not null;uniqueIndex:uidx_published_page_specs_scope_version,priority:3" json:"pageKey"`
	Version               int        `gorm:"not null;uniqueIndex:uidx_published_page_specs_scope_version,priority:4;index" json:"version"`
	SpecJSON              string     `gorm:"type:json" json:"-"` // Full PageSpec JSON
	BindingContractsJSON  string     `gorm:"type:json" json:"-"`
	RendererSchemaVersion string     `gorm:"size:32;not null" json:"rendererSchemaVersion"`
	Active                bool       `gorm:"default:true;index" json:"active"`
	PublishedAt           time.Time  `json:"publishedAt"`
	UnpublishedAt         *time.Time `json:"unpublishedAt,omitempty"`
	PublishedBy           string     `gorm:"size:128" json:"publishedBy,omitempty"`
}

func (PublishedPageSpec) TableName() string {
	return "published_page_specs"
}

// PageVersion stores version history for page specs.
type PageVersion struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	GameID    string    `gorm:"size:64;not null;default:'';index:idx_page_versions_scope_key,priority:1;uniqueIndex:uidx_page_versions_scope_key_version,priority:1" json:"gameId"`
	Env       string    `gorm:"size:64;not null;default:'';index:idx_page_versions_scope_key,priority:2;uniqueIndex:uidx_page_versions_scope_key_version,priority:2" json:"env"`
	PageKey   string    `gorm:"size:128;not null;index:idx_page_versions_scope_key,priority:3;uniqueIndex:uidx_page_versions_scope_key_version,priority:3" json:"pageKey"`
	Version   int       `gorm:"index;uniqueIndex:uidx_page_versions_scope_key_version,priority:4" json:"version"`
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

// FindByScopeAndPageKey returns a page spec by PageIdentity.
func (m *PageSpecModel) FindByScopeAndPageKey(ctx context.Context, gameID, env, pageKey string) (*PageSpec, error) {
	var ps PageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ?", gameID, env, pageKey).
		First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// ListByScope returns all page specs in a scope ordered by category and order.
func (m *PageSpecModel) ListByScope(ctx context.Context, gameID, env string) ([]PageSpec, error) {
	var items []PageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("category_order ASC, \"order\" ASC, page_key ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByScopeAndStatus returns page specs filtered by status in a scope.
func (m *PageSpecModel) ListByScopeAndStatus(ctx context.Context, gameID, env, status string) ([]PageSpec, error) {
	var items []PageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND status = ?", gameID, env, status).
		Order("category_order ASC, \"order\" ASC, page_key ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Upsert creates or updates a page spec by PageIdentity.
func (m *PageSpecModel) Upsert(ctx context.Context, ps *PageSpec) error {
	var existing PageSpec
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ?", ps.GameID, ps.Env, ps.PageKey).
		First(&existing).Error
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

// Delete removes a page spec by PageIdentity.
func (m *PageSpecModel) Delete(ctx context.Context, gameID, env, pageKey string) error {
	return m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ?", gameID, env, pageKey).
		Delete(&PageSpec{}).Error
}

// PublishedPageSpecModel provides data access for published page specs.
type PublishedPageSpecModel struct {
	db *gorm.DB
}

// NewPublishedPageSpecModel creates a new PublishedPageSpecModel.
func NewPublishedPageSpecModel(db *gorm.DB) *PublishedPageSpecModel {
	return &PublishedPageSpecModel{db: db}
}

// FindByScopePageKeyAndVersion returns a published page spec.
func (m *PublishedPageSpecModel) FindByScopePageKeyAndVersion(ctx context.Context, gameID, env, pageKey string, version int) (*PublishedPageSpec, error) {
	var ps PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ? AND version = ?", gameID, env, pageKey, version).
		First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// FindLatestByScopeAndPageKey returns the active published version of a page.
func (m *PublishedPageSpecModel) FindLatestByScopeAndPageKey(ctx context.Context, gameID, env, pageKey string) (*PublishedPageSpec, error) {
	var ps PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ? AND active = ?", gameID, env, pageKey, true).
		Order("version DESC").
		First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// ListByScope returns all published page specs in a scope.
func (m *PublishedPageSpecModel) ListByScope(ctx context.Context, gameID, env string) ([]PublishedPageSpec, error) {
	var items []PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("page_key ASC, version DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListLatestActiveByScope returns active published pages for a scope.
func (m *PublishedPageSpecModel) ListLatestActiveByScope(ctx context.Context, gameID, env string) ([]PublishedPageSpec, error) {
	var items []PublishedPageSpec
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND active = ?", gameID, env, true).
		Order("page_key ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeactivatePage marks all published snapshots of a scoped page inactive.
func (m *PublishedPageSpecModel) DeactivatePage(ctx context.Context, gameID, env, pageKey string, at time.Time) error {
	return m.db.WithContext(ctx).
		Model(&PublishedPageSpec{}).
		Where("game_id = ? AND env = ? AND page_key = ? AND active = ?", gameID, env, pageKey, true).
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

// ListByScopeAndPageKey returns version history for a scoped page.
func (m *PageVersionModel) ListByScopeAndPageKey(ctx context.Context, gameID, env, pageKey string) ([]PageVersion, error) {
	var items []PageVersion
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ?", gameID, env, pageKey).
		Order("version DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// UpsertByScopePageKeyVersion stores one logical snapshot per PageIdentity version.
func (m *PageVersionModel) UpsertByScopePageKeyVersion(ctx context.Context, pv *PageVersion) error {
	return m.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "game_id"},
			{Name: "env"},
			{Name: "page_key"},
			{Name: "version"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"spec_json",
			"status",
			"message",
			"created_by",
		}),
	}).Create(pv).Error
}

// GetNextVersion returns the next version number for a scoped page.
func (m *PageVersionModel) GetNextVersion(ctx context.Context, gameID, env, pageKey string) (int, error) {
	var maxVersion int
	err := m.db.WithContext(ctx).
		Model(&PageVersion{}).
		Where("game_id = ? AND env = ? AND page_key = ?", gameID, env, pageKey).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	return maxVersion + 1, err
}
