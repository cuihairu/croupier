package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// PlayerModel 提供玩家数据访问方法
type PlayerModel struct {
	db *gorm.DB
}

// NewPlayerModel 创建玩家模型实例
func NewPlayerModel(db *gorm.DB) *PlayerModel {
	return &PlayerModel{db: db}
}

// ListPlayersOptions 控制玩家列表查询的分页和过滤选项
type ListPlayersOptions struct {
	Page     int
	PageSize int
	GameID   string
	Search   string
	Status   *int
	Level    *int
	VIP      *int
}

// Create 创建新玩家
func (m *PlayerModel) Create(ctx context.Context, player *Player, password string) error {
	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		player.Password = string(hashedPassword)
	}
	return m.db.WithContext(ctx).Create(player).Error
}

// FindOne 根据 ID 获取玩家
func (m *PlayerModel) FindOne(ctx context.Context, id uint) (*Player, error) {
	var player Player
	if err := m.db.WithContext(ctx).First(&player, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("player not found")
		}
		return nil, err
	}
	return &player, nil
}

// FindByUsername 根据用户名获取玩家
func (m *PlayerModel) FindByUsername(ctx context.Context, username string, gameID string) (*Player, error) {
	var player Player
	query := m.db.WithContext(ctx).Where("username = ?", username)
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if err := query.First(&player).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("player not found")
		}
		return nil, err
	}
	return &player, nil
}

// Update 更新玩家
func (m *PlayerModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Player{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除玩家
func (m *PlayerModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Player{}, id).Error
}

// List 分页获取玩家列表
func (m *PlayerModel) List(ctx context.Context, opts ListPlayersOptions) ([]Player, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	var (
		players []Player
		total   int64
	)

	query := m.db.WithContext(ctx).Model(&Player{})

	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}

	if opts.Search != "" {
		query = query.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?",
			"%"+opts.Search+"%", "%"+opts.Search+"%", "%"+opts.Search+"%")
	}

	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}

	if opts.Level != nil {
		query = query.Where("level = ?", *opts.Level)
	}

	if opts.VIP != nil {
		query = query.Where("vip = ?", *opts.VIP)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&players).Error; err != nil {
		return nil, 0, err
	}

	return players, total, nil
}

// ValidatePassword 验证玩家密码
func (m *PlayerModel) ValidatePassword(ctx context.Context, username, password, gameID string) (*Player, error) {
	player, err := m.FindByUsername(ctx, username, gameID)
	if err != nil {
		return nil, err
	}

	if player.Password == "" {
		return nil, fmt.Errorf("no password set for this player")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(player.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	return player, nil
}

// UpdatePassword 更新玩家密码
func (m *PlayerModel) UpdatePassword(ctx context.Context, id uint, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	return m.db.WithContext(ctx).
		Model(&Player{}).
		Where("id = ?", id).
		Update("password", string(hashedPassword)).Error
}

// UpdateBalance 更新玩家余额
func (m *PlayerModel) UpdateBalance(ctx context.Context, id uint, amount int64, reason string) (*Player, error) {
	var player Player
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先获取当前玩家信息
		if err := tx.First(&player, id).Error; err != nil {
			return err
		}

		// 更新余额
		newBalance := player.Balance + amount
		if newBalance < 0 {
			return fmt.Errorf("insufficient balance")
		}

		return tx.Model(&player).Updates(map[string]interface{}{
			"balance":    newBalance,
			"updated_at": time.Now(),
		}).Error
	})

	if err != nil {
		return nil, err
	}

	// 重新获取更新后的玩家信息
	return m.FindOne(ctx, id)
}

// BanPlayer 封禁玩家
func (m *PlayerModel) BanPlayer(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).
		Model(&Player{}).
		Where("id = ?", id).
		Update("status", 0).Error
}

// SuspendPlayer 暂停玩家
func (m *PlayerModel) SuspendPlayer(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).
		Model(&Player{}).
		Where("id = ?", id).
		Update("status", 2).Error
}

// ActivatePlayer 激活玩家
func (m *PlayerModel) ActivatePlayer(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).
		Model(&Player{}).
		Where("id = ?", id).
		Update("status", 1).Error
}
