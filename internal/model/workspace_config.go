package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WorkspaceConfig stores workspace UI configuration as a JSON blob.
type WorkspaceConfig struct {
	gorm.Model
	ObjectKey   string         `gorm:"size:128;uniqueIndex"`
	Title       string         `gorm:"size:256"`
	Published   bool           `gorm:"default:false"`
	PublishedAt *time.Time     `gorm:"index"`
	PublishedBy string         `gorm:"size:128"`
	MenuOrder   int            `gorm:"default:0"`
	Config      datatypes.JSON `gorm:"type:json"` // full JSON blob
}

func (WorkspaceConfig) TableName() string {
	return "workspace_configs"
}

// WorkspaceConfigModel wraps data access for workspace configs.
type WorkspaceConfigModel struct {
	db *gorm.DB
}

func NewWorkspaceConfigModel(db *gorm.DB) *WorkspaceConfigModel {
	return &WorkspaceConfigModel{db: db}
}

// Upsert creates or fully replaces a workspace config by objectKey.
func (m *WorkspaceConfigModel) Upsert(ctx context.Context, cfg *WorkspaceConfig) error {
	var existing WorkspaceConfig
	err := m.db.WithContext(ctx).Where("object_key = ?", cfg.ObjectKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return m.db.WithContext(ctx).Create(cfg).Error
	}
	if err != nil {
		return err
	}
	cfg.ID = existing.ID
	cfg.CreatedAt = existing.CreatedAt
	return m.db.WithContext(ctx).Save(cfg).Error
}

// FindByObjectKey fetches a workspace config by objectKey.
func (m *WorkspaceConfigModel) FindByObjectKey(ctx context.Context, objectKey string) (*WorkspaceConfig, error) {
	var cfg WorkspaceConfig
	if err := m.db.WithContext(ctx).Where("object_key = ?", objectKey).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Delete removes a workspace config by objectKey.
func (m *WorkspaceConfigModel) Delete(ctx context.Context, objectKey string) error {
	return m.db.WithContext(ctx).Where("object_key = ?", objectKey).Delete(&WorkspaceConfig{}).Error
}

// ListAll returns all workspace configs ordered by menu_order.
func (m *WorkspaceConfigModel) ListAll(ctx context.Context) ([]WorkspaceConfig, error) {
	var items []WorkspaceConfig
	if err := m.db.WithContext(ctx).Order("menu_order ASC, created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListPublished returns all published workspace configs ordered by menu_order.
func (m *WorkspaceConfigModel) ListPublished(ctx context.Context) ([]WorkspaceConfig, error) {
	var items []WorkspaceConfig
	if err := m.db.WithContext(ctx).
		Where("published = ?", true).
		Order("menu_order ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// SetPublished updates the published state of a workspace config.
func (m *WorkspaceConfigModel) SetPublished(ctx context.Context, objectKey string, published bool, publishedBy string) error {
	updates := map[string]interface{}{
		"published": published,
	}
	if published {
		now := time.Now()
		updates["published_at"] = now
		updates["published_by"] = publishedBy
	} else {
		updates["published_at"] = nil
		updates["published_by"] = ""
	}
	return m.db.WithContext(ctx).
		Model(&WorkspaceConfig{}).
		Where("object_key = ?", objectKey).
		Updates(updates).Error
}
