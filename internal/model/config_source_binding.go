package model

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// Config source types for the online config explorer. The explorer is a
// read-first viewer over each project's *existing* config center; croupier
// does not define the project's config workflow, it adapts to it.
// See docs/research/config-workflows-analysis.md.
const (
	ConfigSourceTypeGit      = "git"      // Git 仓库（只读）
	ConfigSourceTypeRedis    = "redis"    // Redis 配置总线（skynet 惯例，可写）
	ConfigSourceTypeNacos    = "nacos"    // Nacos 配置中心（可写）
	ConfigSourceTypeDB       = "db"       // 数据库直管（只读快照）
	ConfigSourceTypeCroupier = "croupier" // Croupier ConfigVersion（可写=新版本）
)

// ValidConfigSourceTypes is the closed set of source types.
var ValidConfigSourceTypes = map[string]struct{}{
	ConfigSourceTypeGit: {}, ConfigSourceTypeRedis: {}, ConfigSourceTypeNacos: {},
	ConfigSourceTypeDB: {}, ConfigSourceTypeCroupier: {},
}

// WritableConfigSourceTypes reports whether a source type supports emergency
// edit (写回配置中心本身；Git 永远只读——改 Git 就该走项目组的 MR 流程).
func WritableConfigSourceType(t string) bool {
	switch t {
	case ConfigSourceTypeRedis, ConfigSourceTypeNacos, ConfigSourceTypeCroupier:
		return true
	default:
		return false
	}
}

// ConfigSourceBinding binds one (game_id, env) to an external config center
// for online browsing. Config is a JSON object carrying connection params
// (credentials included; API 响应必须脱敏).
type ConfigSourceBinding struct {
	gorm.Model
	GameID string `gorm:"size:64;index:idx_cfg_src_game_env,priority:1;not null" json:"gameId"`
	Env    string `gorm:"size:64;index:idx_cfg_src_game_env,priority:2;not null" json:"env"`
	Name   string `gorm:"size:64;not null" json:"name"`
	Type   string `gorm:"size:16;not null" json:"type"`
	Config string `gorm:"type:text" json:"config"`
}

func (ConfigSourceBinding) TableName() string { return "config_source_bindings" }

// NormalizeConfigSourceType trims/validates a source type.
func NormalizeConfigSourceType(t string) (string, bool) {
	t = strings.ToLower(strings.TrimSpace(t))
	if _, ok := ValidConfigSourceTypes[t]; ok {
		return t, true
	}
	return "", false
}

// ConfigSourceBindingModel provides CRUD helpers for source bindings.
type ConfigSourceBindingModel struct {
	db *gorm.DB
}

func NewConfigSourceBindingModel(db *gorm.DB) *ConfigSourceBindingModel {
	return &ConfigSourceBindingModel{db: db}
}

// Create inserts a binding.
func (m *ConfigSourceBindingModel) Create(ctx context.Context, b *ConfigSourceBinding) error {
	if b == nil {
		return errors.New("binding required")
	}
	b.GameID = strings.TrimSpace(b.GameID)
	b.Env = strings.TrimSpace(b.Env)
	b.Name = strings.TrimSpace(b.Name)
	t, ok := NormalizeConfigSourceType(b.Type)
	if !ok {
		return errors.New("invalid config source type")
	}
	b.Type = t
	if b.GameID == "" || b.Env == "" || b.Name == "" {
		return errors.New("gameId/env/name required")
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(b).Error
}

// Update persists editable fields of an existing binding.
func (m *ConfigSourceBindingModel) Update(ctx context.Context, b *ConfigSourceBinding) error {
	if b == nil || b.ID == 0 {
		return errors.New("binding id required")
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&ConfigSourceBinding{}).
		Where("id = ?", b.ID).
		Updates(map[string]interface{}{
			"name":   strings.TrimSpace(b.Name),
			"type":   b.Type,
			"config": b.Config,
		}).Error
}

// Delete removes a binding by id.
func (m *ConfigSourceBindingModel) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return errors.New("binding id required")
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("id = ?", id).Delete(&ConfigSourceBinding{}).Error
}

// Get returns one binding by id.
func (m *ConfigSourceBindingModel) Get(ctx context.Context, id uint) (*ConfigSourceBinding, error) {
	if id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var b ConfigSourceBinding
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("id = ?", id).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// ListByScope lists bindings for one game environment.
func (m *ConfigSourceBindingModel) ListByScope(ctx context.Context, gameID, env string) ([]ConfigSourceBinding, error) {
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	var out []ConfigSourceBinding
	q := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	if gameID != "" {
		q = q.Where("game_id = ?", gameID)
	}
	if env != "" {
		q = q.Where("env = ?", env)
	}
	if err := q.Order("id ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
