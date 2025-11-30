package model

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ConfigVersion stores versioned configuration values for arbitrary keys.
type ConfigVersion struct {
	gorm.Model
	Key       string `gorm:"size:128;index:idx_config_key_version,priority:1"`
	Version   int    `gorm:"index:idx_config_key_version,priority:2"`
	Value     string `gorm:"type:text"`
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
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("config key required")
	}
	createdBy = strings.TrimSpace(createdBy)

	var latest ConfigVersion
	var version int
	err := m.db.WithContext(ctx).
		Where("key = ?", key).
		Order("version DESC").
		Take(&latest).Error
	switch {
	case err == nil:
		version = latest.Version + 1
	case errors.Is(err, gorm.ErrRecordNotFound):
		version = 1
	default:
		return nil, err
	}

	record := &ConfigVersion{
		Key:       key,
		Version:   version,
		Value:     value,
		CreatedBy: createdBy,
	}
	if err := m.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// List returns all versions for the given key ordered from newest to oldest.
func (m *ConfigVersionModel) List(ctx context.Context, key string) ([]ConfigVersion, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return []ConfigVersion{}, nil
	}
	var records []ConfigVersion
	if err := m.db.WithContext(ctx).
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
	if err := m.db.WithContext(ctx).
		Where("key = ? AND version = ?", key, version).
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
