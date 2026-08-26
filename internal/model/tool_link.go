package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ToolLink is one registered internal tool (Jenkins/GitLab/Grafana/Wiki...)
// shown in the dev toolbox. See docs/research/tool-registry-design.md.
type ToolLink struct {
	gorm.Model
	Name        string `gorm:"size:128"`
	URL         string `gorm:"size:512"`
	Description string `gorm:"size:255"`
	Category    string `gorm:"size:32;index"`
	Icon        string `gorm:"size:512"`
	GameID      string `gorm:"size:64;index:idx_tool_scope,priority:1"`
	Env         string `gorm:"size:64;index:idx_tool_scope,priority:2"`
	Enabled     bool   `gorm:"default:true"`
	Sort        int    `gorm:"default:0"`
	CreatedBy   string `gorm:"size:64"`
	UpdatedAt   time.Time
}

func (ToolLink) TableName() string { return "tool_links" }

// Tool categories (closed set; the frontend icon/group mapping depends on it).
const (
	ToolCategoryCI       = "ci"
	ToolCategoryRepo     = "repo"
	ToolCategoryMonitor  = "monitor"
	ToolCategoryDocs     = "docs"
	ToolCategoryArtifact = "artifact"
	ToolCategoryOther    = "other"
)

// ValidToolCategories is the closed set of tool categories.
var ValidToolCategories = map[string]struct{}{
	ToolCategoryCI: {}, ToolCategoryRepo: {}, ToolCategoryMonitor: {},
	ToolCategoryDocs: {}, ToolCategoryArtifact: {}, ToolCategoryOther: {},
}

// ValidateToolLink checks name/url/category invariants.
func ValidateToolLink(tool *ToolLink) error {
	tool.Name = strings.TrimSpace(tool.Name)
	tool.URL = strings.TrimSpace(tool.URL)
	if tool.Name == "" {
		return errors.New("tool name is required")
	}
	if !strings.HasPrefix(tool.URL, "http://") && !strings.HasPrefix(tool.URL, "https://") {
		return errors.New("tool url must start with http:// or https://")
	}
	if tool.Category == "" {
		tool.Category = ToolCategoryOther
	}
	if _, ok := ValidToolCategories[tool.Category]; !ok {
		return errors.New("invalid tool category: " + tool.Category)
	}
	return nil
}

// ToolLinkModel provides CRUD for the toolbox.
type ToolLinkModel struct {
	db *gorm.DB
}

// NewToolLinkModel creates a helper.
func NewToolLinkModel(db *gorm.DB) *ToolLinkModel {
	return &ToolLinkModel{db: db}
}

// ToolQueryOptions controls listing. Empty GameID/Env means global tools only
// plus everything scoped to the given pair.
type ToolQueryOptions struct {
	GameID string
	Env    string
}

// List returns enabled tools visible in the given scope (global rows have
// empty game/env).
func (m *ToolLinkModel) List(ctx context.Context, opts ToolQueryOptions) ([]ToolLink, error) {
	var items []ToolLink
	query := m.db.WithContext(ctx).Where("enabled = ?", true)
	if opts.GameID != "" && opts.Env != "" {
		query = query.Where(
			"(game_id = '' OR game_id IS NULL) OR (game_id = ? AND env = ?)",
			opts.GameID, opts.Env,
		)
	} else {
		query = query.Where("(game_id = '' OR game_id IS NULL)")
	}
	err := query.Order("category ASC, sort DESC, updated_at DESC").Find(&items).Error
	return items, err
}

// ListAll returns every tool including disabled ones (admin view).
func (m *ToolLinkModel) ListAll(ctx context.Context) ([]ToolLink, error) {
	var items []ToolLink
	err := m.db.WithContext(ctx).Order("category ASC, sort DESC, updated_at DESC").Find(&items).Error
	return items, err
}

// Create inserts a tool.
func (m *ToolLinkModel) Create(ctx context.Context, tool *ToolLink) error {
	return m.db.WithContext(ctx).Create(tool).Error
}

// Update applies a partial update map.
func (m *ToolLinkModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&ToolLink{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a tool.
func (m *ToolLinkModel) Delete(ctx context.Context, id uint) error {
	res := m.db.WithContext(ctx).Delete(&ToolLink{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
