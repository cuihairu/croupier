package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// GameModel 提供游戏数据访问方法
type GameModel struct {
	db *gorm.DB
}

// NewGameModel 创建游戏模型实例
func NewGameModel(db *gorm.DB) *GameModel {
	return &GameModel{db: db}
}

// ListGamesOptions 控制游戏列表查询的分页和过滤选项
type ListGamesOptions struct {
	Page     int
	PageSize int
	Status   string
	Search   string
}

// Create 创建新游戏
func (m *GameModel) Create(ctx context.Context, game *Game) error {
	return m.db.WithContext(ctx).Create(game).Error
}

// FindOne 根据 ID 获取游戏
func (m *GameModel) FindOne(ctx context.Context, id uint) (*Game, error) {
	var game Game
	if err := m.db.WithContext(ctx).First(&game, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("game not found")
		}
		return nil, err
	}
	return &game, nil
}

// FindByName 根据名称获取游戏
func (m *GameModel) FindByName(ctx context.Context, name string) (*Game, error) {
	var game Game
	if err := m.db.WithContext(ctx).Where("name = ?", name).First(&game).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("game not found")
		}
		return nil, err
	}
	return &game, nil
}

// Update 更新游戏
func (m *GameModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Game{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除游戏
func (m *GameModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Game{}, id).Error
}

// List 分页获取游戏列表
func (m *GameModel) List(ctx context.Context, opts ListGamesOptions) ([]Game, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	var (
		games []Game
		total int64
	)

	query := m.db.WithContext(ctx).Model(&Game{})

	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	if opts.Search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?",
			"%"+opts.Search+"%", "%"+opts.Search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&games).Error; err != nil {
		return nil, 0, err
	}

	return games, total, nil
}

// UpdateStatus 更新游戏状态
func (m *GameModel) UpdateStatus(ctx context.Context, id uint, status string) error {
	return m.db.WithContext(ctx).
		Model(&Game{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ToggleEnabled 切换游戏启用状态
func (m *GameModel) ToggleEnabled(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).
		Model(&Game{}).
		Where("id = ?", id).
		Update("enabled", gorm.Expr("NOT enabled")).Error
}
