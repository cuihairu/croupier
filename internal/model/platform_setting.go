package model

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// PlatformSetting is one L3 (database) override of a platform configuration
// key. Layering semantics: L1 code default ← L2 config file/env ← L3 this
// table (highest). Deleting a row = "follow the config file again".
// See docs/architecture/config-layering.md.
type PlatformSetting struct {
	ID        uint   `gorm:"primaryKey"`
	Key       string `gorm:"size:128;uniqueIndex"`
	Value     string `gorm:"type:text"`
	UpdatedBy string `gorm:"size:64"`
	UpdatedAt time.Time
}

func (PlatformSetting) TableName() string { return "platform_settings" }

// PlatformSettingModel provides CRUD for L3 overrides.
type PlatformSettingModel struct {
	db *gorm.DB
}

// NewPlatformSettingModel creates a helper.
func NewPlatformSettingModel(db *gorm.DB) *PlatformSettingModel {
	return &PlatformSettingModel{db: db}
}

// List returns all overrides (key → raw JSON value).
func (m *PlatformSettingModel) List(ctx context.Context) (map[string]json.RawMessage, error) {
	var rows []PlatformSetting
	if err := m.db.WithContext(ctx).Order("key").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(rows))
	for _, r := range rows {
		out[r.Key] = json.RawMessage(r.Value)
	}
	return out, nil
}

// Get returns one override; ok=false when absent.
func (m *PlatformSettingModel) Get(ctx context.Context, key string) (json.RawMessage, bool, error) {
	var row PlatformSetting
	err := m.db.WithContext(ctx).Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(row.Value), true, nil
}

// Set upserts an override.
func (m *PlatformSettingModel) Set(ctx context.Context, key string, value json.RawMessage, updatedBy string) error {
	v := strings.TrimSpace(string(value))
	if v == "" {
		return errors.New("platform setting value is required")
	}
	row := PlatformSetting{Key: key, Value: v, UpdatedBy: updatedBy, UpdatedAt: time.Now()}
	return m.db.WithContext(ctx).
		Where("key = ?", key).
		Assign(row).
		FirstOrCreate(&row).Error
}

// Clear removes an override (= follow the config file again).
func (m *PlatformSettingModel) Clear(ctx context.Context, key string) error {
	res := m.db.WithContext(ctx).Where("key = ?", key).Delete(&PlatformSetting{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
