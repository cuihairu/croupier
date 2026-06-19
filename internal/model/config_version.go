package model

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// ConfigVersion stores versioned configuration values for arbitrary keys.
type ConfigVersion struct {
	gorm.Model
	Key       string `gorm:"size:128;index:idx_config_key_version,priority:1"`
	Version   int    `gorm:"index:idx_config_key_version,priority:2"`
	Value     string `gorm:"type:text"`
	Format    string `gorm:"size:16"`
	GameID    string `gorm:"size:64"`
	Env       string `gorm:"size:64"`
	Message   string `gorm:"size:255"`
	CreatedBy string `gorm:"size:64"`
}

func (ConfigVersion) TableName() string { return "config_versions" }

// ConfigVersionModel provides helpers for managing configuration history.
type ConfigVersionModel struct {
	db *gorm.DB
}

func NewConfigVersionModel(db *gorm.DB) *ConfigVersionModel {
	return &ConfigVersionModel{db: db}
}

// Create inserts a new config version, automatically incrementing the version number.
func (m *ConfigVersionModel) Create(ctx context.Context, key, value, createdBy string) (*ConfigVersion, error) {
	payload := ConfigVersionPayload{
		Key:     key,
		Content: value,
	}
	return m.CreateWithMeta(ctx, payload, createdBy)
}

// List returns all versions for the given key ordered from newest to oldest.
func (m *ConfigVersionModel) List(ctx context.Context, key string) ([]ConfigVersion, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return []ConfigVersion{}, nil
	}
	var records []ConfigVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("key = ?", key).
		Order("version DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// Find returns the requested key/version pair.
func (m *ConfigVersionModel) Find(ctx context.Context, key string, version int) (*ConfigVersion, error) {
	key = strings.TrimSpace(key)
	if key == "" || version <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var record ConfigVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("key = ? AND version = ?", key, version).
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

type ConfigVersionPayload struct {
	Key         string
	Content     string
	Format      string
	GameID      string
	Env         string
	Message     string
	BaseVersion int
}

// CreateWithMeta inserts a new config version with metadata and optimistic locking support.
func (m *ConfigVersionModel) CreateWithMeta(ctx context.Context, payload ConfigVersionPayload, createdBy string) (*ConfigVersion, error) {
	key := strings.TrimSpace(payload.Key)
	if key == "" {
		return nil, errors.New("config key required")
	}
	createdBy = strings.TrimSpace(createdBy)

	var latest ConfigVersion
	var version int
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("key = ?", key).
		Order("version DESC").
		Take(&latest).Error
	switch {
	case err == nil:
		if payload.BaseVersion > 0 && latest.Version != payload.BaseVersion {
			return nil, errors.New("config has been updated by another user")
		}
		version = latest.Version + 1
	case errors.Is(err, gorm.ErrRecordNotFound):
		if payload.BaseVersion > 0 {
			return nil, errors.New("config base version mismatch")
		}
		version = 1
	default:
		return nil, err
	}

	record := &ConfigVersion{
		Key:       key,
		Version:   version,
		Value:     payload.Content,
		Format:    strings.TrimSpace(payload.Format),
		GameID:    strings.TrimSpace(payload.GameID),
		Env:       strings.TrimSpace(payload.Env),
		Message:   strings.TrimSpace(payload.Message),
		CreatedBy: createdBy,
	}
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// ListLatest fetches the latest version per key with optional filters.
func (m *ConfigVersionModel) ListLatest(ctx context.Context, opts ConfigListOptions) ([]ConfigVersion, error) {
	// 先应用过滤条件到子查询
	sub := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&ConfigVersion{}).
		Select("key, MAX(version) AS version")

	if v := strings.TrimSpace(opts.GameID); v != "" {
		sub = sub.Where("game_id = ?", v)
	}
	if v := strings.TrimSpace(opts.Env); v != "" {
		sub = sub.Where("env = ?", v)
	}
	if v := strings.TrimSpace(opts.Format); v != "" {
		sub = sub.Where("format = ?", v)
	}
	if v := strings.TrimSpace(opts.IDLike); v != "" {
		sub = sub.Where("key LIKE ?", "%"+v+"%")
	}

	sub = sub.Group("key")

	// 查询完整记录
	var records []ConfigVersion
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Table("config_versions").
		Joins("JOIN (?) AS latest ON config_versions.key = latest.key AND config_versions.version = latest.version", sub)

	if err := query.Order("config_versions.updated_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

type ConfigListOptions struct {
	GameID string
	Env    string
	Format string
	IDLike string
}

// FindLatest returns the newest version for the given key.
func (m *ConfigVersionModel) FindLatest(ctx context.Context, key string) (*ConfigVersion, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("config key required")
	}
	var record ConfigVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("key = ?", key).
		Order("version DESC").
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
