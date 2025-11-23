package games

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/database"
	"gorm.io/gorm"
)

// EnhancedRepo 使用新的DatabaseManager的增强版仓库
type EnhancedRepo struct {
	manager *database.Manager
}

// NewEnhancedRepo 创建新的增强版仓库
func NewEnhancedRepo(manager *database.Manager) *EnhancedRepo {
	return &EnhancedRepo{
		manager: manager,
	}
}

// AutoMigrate 运行数据库迁移
func (r *EnhancedRepo) AutoMigrate() error {
	return r.manager.GetDB().AutoMigrate(&Game{}, &GameEnv{})
}

// Create 创建游戏记录（带重试机制）
func (r *EnhancedRepo) Create(ctx context.Context, g *Game) error {
	return r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.Create(g).Error
	})
}

// Update 更新游戏记录（带重试机制）
func (r *EnhancedRepo) Update(ctx context.Context, g *Game) error {
	return r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.Save(g).Error
	})
}

// Delete 删除游戏记录（带重试机制）
func (r *EnhancedRepo) Delete(ctx context.Context, id uint) error {
	return r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.Delete(&Game{}, id).Error
	})
}

// Get 获取单个游戏记录（带重试机制）
func (r *EnhancedRepo) Get(ctx context.Context, id uint) (*Game, error) {
	var g Game
	err := r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.First(&g, id).Error
	})

	if err != nil {
		return nil, err
	}
	return &g, nil
}

// List 分页查询游戏列表
func (r *EnhancedRepo) List(ctx context.Context, opts *database.ListOptions) (*database.PaginatedResult[*Game], error) {
	// 构建搜索条件
	searchCondition := func(db *gorm.DB) *gorm.DB {
		if opts.Search != "" {
			return db.Where("name LIKE ? OR description LIKE ?",
				"%"+opts.Search+"%", "%"+opts.Search+"%")
		}
		return db
	}

	return database.ListGeneric[*Game](r.manager.GetDB(), ctx, opts, searchCondition)
}

// ListEnvs 获取游戏的环境列表
func (r *EnhancedRepo) ListEnvs(ctx context.Context, gameID uint) ([]string, error) {
	g, err := r.Get(ctx, gameID)
	if err != nil {
		return nil, err
	}

	envs := g.GetEnvList()
	// normalize unique lower-case preserve original order
	seen := map[string]struct{}{}
	out := make([]string, 0, len(envs))
	for _, e := range envs {
		t := strings.TrimSpace(e)
		if t == "" {
			continue
		}
		if _, exists := seen[t]; !exists {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out, nil
}

// GetStats 获取仓库统计信息
func (r *EnhancedRepo) GetStats() *database.PoolStats {
	return r.manager.GetStats()
}

// Ping 检查数据库连接
func (r *EnhancedRepo) Ping(ctx context.Context) error {
	return r.manager.Ping(ctx)
}

// HealthCheck 执行健康检查
func (r *EnhancedRepo) HealthCheck(ctx context.Context) error {
	// 执行简单查询来验证数据库连接
	var count int64
	err := r.manager.WithContext(ctx).
		Model(&Game{}).
		Count(&count).
		Error

	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// BulkCreate 批量创建游戏记录
func (r *EnhancedRepo) BulkCreate(ctx context.Context, games []*Game) error {
	return r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.CreateInBatches(games, 100).Error
	})
}

// GetByName 根据名称获取游戏
func (r *EnhancedRepo) GetByName(ctx context.Context, name string) (*Game, error) {
	var g Game
	err := r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.Where("name = ?", name).First(&g).Error
	})

	if err != nil {
		return nil, err
	}
	return &g, nil
}

// GetActiveGames 获取活跃游戏列表
func (r *EnhancedRepo) GetActiveGames(ctx context.Context, opts *database.ListOptions) (*database.PaginatedResult[*Game], error) {
	// 构建搜索条件
	searchCondition := func(db *gorm.DB) *gorm.DB {
		db = db.Where("enabled = ?", true)
		if opts.Search != "" {
			return db.Where("name LIKE ? OR description LIKE ?",
				"%"+opts.Search+"%", "%"+opts.Search+"%")
		}
		return db
	}

	return database.ListGeneric[*Game](r.manager.GetDB(), ctx, opts, searchCondition)
}

// UpdateLastActivity 更新游戏最后活动时间
func (r *EnhancedRepo) UpdateLastActivity(ctx context.Context, gameID uint) error {
	return r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.Model(&Game{}).
			Where("id = ?", gameID).
			Update("last_activity_at", gorm.Expr("CURRENT_TIMESTAMP")).
			Error
	})
}

// GetGameCount 获取游戏总数
func (r *EnhancedRepo) GetGameCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.manager.WithRetry(ctx, func(db *gorm.DB) error {
		return db.Model(&Game{}).Count(&count).Error
	})

	return count, err
}

// Close 关闭数据库连接
func (r *EnhancedRepo) Close() error {
	return r.manager.Close()
}