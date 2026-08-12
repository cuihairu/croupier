package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"
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

// ExistsByNameIgnoreCase checks whether a game name already exists.
func (m *GameModel) ExistsByNameIgnoreCase(ctx context.Context, name string, excludeID ...uint) (bool, error) {
	query := m.db.WithContext(ctx).Model(&Game{}).Where("LOWER(name) = LOWER(?)", name)
	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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

// ListAll returns every game ordered by updated time descending.
func (m *GameModel) ListAll(ctx context.Context) ([]Game, error) {
	var games []Game
	err := m.db.WithContext(ctx).
		Order("updated_at DESC").
		Find(&games).Error
	return games, err
}

// FindByGameID 根据 GameID 获取游戏
func (m *GameModel) FindByGameID(ctx context.Context, gameID uint) (*Game, error) {
	return m.FindOne(ctx, gameID)
}

// FindByGameIDString returns the game whose business GameID matches the given
// string (e.g. "demo").
func (m *GameModel) FindByGameIDString(ctx context.Context, gameID string) (*Game, error) {
	var game Game
	if err := m.db.WithContext(ctx).Where("game_id = ?", gameID).First(&game).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("game not found")
		}
		return nil, err
	}
	return &game, nil
}

// ============================================================================
// Game environment bindings (database-per-game routing)
// ============================================================================

// AddEnvBinding creates or updates a GameEnvBinding for (gameID, env). When
// databaseName is empty it is derived by the caller (typically the router's
// naming function) before persisting.
func (m *GameModel) AddEnvBinding(ctx context.Context, gameID, env, databaseName, description, color string) error {
	binding := GameEnvBinding{
		GameID:       gameID,
		Env:          env,
		DatabaseName: databaseName,
		Description:  description,
		Color:        color,
	}
	_, err := upsertEnvBinding(m.db.WithContext(ctx), binding)
	return err
}

// upsertEnvBinding restores a legacy soft-deleted binding when present and
// otherwise creates or updates it. Routing bindings must remain reusable
// after an environment is deleted and later recreated.
func upsertEnvBinding(db *gorm.DB, binding GameEnvBinding) (bool, error) {
	var existing GameEnvBinding
	err := db.Unscoped().Where("game_id = ? AND env = ?", binding.GameID, binding.Env).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, db.Create(&binding).Error
	}
	if err != nil {
		return false, err
	}
	updates := map[string]interface{}{
		"database_name": binding.DatabaseName,
		"description":   binding.Description,
		"color":         binding.Color,
		"deleted_at":    nil,
	}
	return false, db.Unscoped().Model(&GameEnvBinding{}).Where("id = ?", existing.ID).Updates(updates).Error
}

// FindEnvBinding returns the authoritative binding for a game environment.
func (m *GameModel) FindEnvBinding(ctx context.Context, gameID, env string) (*GameEnvBinding, error) {
	var binding GameEnvBinding
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

// RemoveEnvBinding deletes the GameEnvBinding for (gameID, env).
func (m *GameModel) RemoveEnvBinding(ctx context.Context, gameID, env string) error {
	return m.db.WithContext(ctx).
		Unscoped().
		Where("game_id = ? AND env = ?", gameID, env).
		Delete(&GameEnvBinding{}).Error
}

// RemoveAllEnvBindings permanently removes every environment binding for a
// game. Bindings are routing metadata, so retaining soft-deleted rows would
// prevent recreating an environment with the same logical name.
func (m *GameModel) RemoveAllEnvBindings(ctx context.Context, gameID string) error {
	return m.db.WithContext(ctx).
		Unscoped().
		Where("game_id = ?", gameID).
		Delete(&GameEnvBinding{}).Error
}

// UpdateEnvsAndBindings commits the legacy UI metadata and authoritative
// game_envs changes in one meta-database transaction. Callers must provide
// the complete updated JSON value plus the binding rows to create or update.
func (m *GameModel) UpdateEnvsAndBindings(
	ctx context.Context,
	gameID string,
	id uint,
	envs datatypes.JSON,
	removeEnvs []string,
	upsertBindings []GameEnvBinding,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Game{}).Where("id = ?", id).Update("envs", envs).Error; err != nil {
			return err
		}

		for _, env := range removeEnvs {
			env = strings.TrimSpace(env)
			if env == "" {
				continue
			}
			if err := tx.Unscoped().Where("game_id = ? AND env = ?", gameID, env).
				Delete(&GameEnvBinding{}).Error; err != nil {
				return err
			}
		}

		for _, binding := range upsertBindings {
			binding.GameID = gameID
			binding.Env = strings.TrimSpace(binding.Env)
			if binding.Env == "" || strings.TrimSpace(binding.DatabaseName) == "" {
				return fmt.Errorf("game environment binding requires env and database name")
			}
			if _, err := upsertEnvBinding(tx, binding); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteWithEnvBindings deletes a game and all its routing bindings as one
// transaction. The Game row remains soft-deleted while the routing metadata
// is hard-deleted so a future game may reuse the business identifier.
func (m *GameModel) DeleteWithEnvBindings(ctx context.Context, id uint, gameID string) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Game{}, id).Error; err != nil {
			return err
		}
		return tx.Unscoped().Where("game_id = ?", gameID).Delete(&GameEnvBinding{}).Error
	})
}

// BackfillEnvBindings creates missing authoritative bindings from legacy
// games.envs JSON. Existing rows, particularly database_name, are preserved.
// The operation is idempotent and is safe to execute at every startup.
func (m *GameModel) BackfillEnvBindings(
	ctx context.Context,
	databaseNameFor func(gameID, env string) string,
) (int, error) {
	if databaseNameFor == nil {
		return 0, errors.New("database name resolver is required")
	}

	created := 0
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var games []Game
		if err := tx.Find(&games).Error; err != nil {
			return err
		}
		for _, game := range games {
			gameID := strings.TrimSpace(game.GameID)
			if gameID == "" {
				continue
			}
			envs, err := game.GetEnvs()
			if err != nil {
				return fmt.Errorf("decode environments for game %q: %w", gameID, err)
			}
			for _, envMeta := range envs {
				env := strings.TrimSpace(envMeta.Env)
				if env == "" {
					continue
				}
				binding := GameEnvBinding{
					GameID:       gameID,
					Env:          env,
					DatabaseName: databaseNameFor(gameID, env),
					Description:  envMeta.Description,
					Color:        envMeta.Color,
				}
				if strings.TrimSpace(binding.DatabaseName) == "" {
					return fmt.Errorf("empty database name for game %q environment %q", gameID, env)
				}
				var existing GameEnvBinding
				err := tx.Unscoped().Where("game_id = ? AND env = ?", gameID, env).
					First(&existing).Error
				switch {
				case errors.Is(err, gorm.ErrRecordNotFound):
					if err := tx.Create(&binding).Error; err != nil {
						return err
					}
					created++
				case err != nil:
					return err
				case existing.DeletedAt.Valid:
					// A legacy JSON record still lists this environment, so restore
					// the binding while retaining its original database name.
					if err := tx.Unscoped().Model(&GameEnvBinding{}).Where("id = ?", existing.ID).
						Update("deleted_at", nil).Error; err != nil {
						return err
					}
					created++
				}
			}
		}
		return nil
	})
	return created, err
}

// ListEnvBindings returns all environment bindings for a game.
func (m *GameModel) ListEnvBindings(ctx context.Context, gameID string) ([]GameEnvBinding, error) {
	var bindings []GameEnvBinding
	err := m.db.WithContext(ctx).
		Where("game_id = ?", gameID).
		Order("env ASC").
		Find(&bindings).Error
	return bindings, err
}

// ListAllEnvBindings returns all environment bindings across all games.
func (m *GameModel) ListAllEnvBindings(ctx context.Context) ([]GameEnvBinding, error) {
	var bindings []GameEnvBinding
	err := m.db.WithContext(ctx).
		Order("game_id ASC, env ASC").
		Find(&bindings).Error
	return bindings, err
}

// LookupDatabaseName returns the physical database name for (gameID, env),
// or "" when no binding exists.
func (m *GameModel) LookupDatabaseName(ctx context.Context, gameID, env string) (string, error) {
	var binding GameEnvBinding
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return binding.DatabaseName, nil
}

// HasEnvBinding reports whether the business game/environment pair is
// registered in the authoritative game_envs table.
func (m *GameModel) HasEnvBinding(ctx context.Context, gameID, env string) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&GameEnvBinding{}).
		Where("game_id = ? AND env = ?", gameID, env).
		Count(&count).Error
	return count > 0, err
}
