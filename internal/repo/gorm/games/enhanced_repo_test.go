package games

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *database.Manager {
	// 使用内存SQLite进行测试
	config := &database.Config{
		MaxOpenConns:        5,
		MaxIdleConns:        2,
		ConnMaxLifetime:     1 * time.Minute,
		ConnMaxIdleTime:     30 * time.Second,
		HealthCheckInterval: 5 * time.Second,
		RetryAttempts:       2,
		RetryDelay:          10 * time.Millisecond,
	}

	manager, err := database.NewManager(":memory:", config)
	require.NoError(t, err)

	// 运行迁移
	repo := NewEnhancedRepo(manager)
	err = repo.AutoMigrate()
	require.NoError(t, err)

	return manager
}

func TestEnhancedRepo_Create(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	game := &Game{
		Name:        "test-game",
		Description: "A test game",
		Status:      "dev",
	}

	err := repo.Create(ctx, game)
	assert.NoError(t, err)
	assert.NotZero(t, game.ID)
	assert.NotZero(t, game.CreatedAt)
	assert.NotZero(t, game.UpdatedAt)
}

func TestEnhancedRepo_Get(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 创建测试数据
	game := &Game{Name: "test-game"}
	err := repo.Create(ctx, game)
	require.NoError(t, err)

	// 获取数据
	fetched, err := repo.Get(ctx, game.ID)
	assert.NoError(t, err)
	assert.Equal(t, game.ID, fetched.ID)
	assert.Equal(t, game.Name, fetched.Name)
}

func TestEnhancedRepo_List(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 创建测试数据
	for i := 0; i < 25; i++ {
		game := &Game{Name: fmt.Sprintf("game-%d", i)}
		err := repo.Create(ctx, game)
		require.NoError(t, err)
	}

	// 测试分页
	t.Run("first_page", func(t *testing.T) {
		opts := &database.ListOptions{
			Page:     1,
			PageSize: 10,
			Sort:     "id",
			Order:    "asc",
		}

		result, err := repo.List(ctx, opts)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 10)
		assert.Equal(t, int64(25), result.Pagination.Total)
		assert.Equal(t, 3, result.Pagination.TotalPages)
		assert.True(t, result.Pagination.HasNext)
		assert.False(t, result.Pagination.HasPrev)
	})

	// 测试搜索
	t.Run("search", func(t *testing.T) {
		opts := &database.ListOptions{
			Page:     1,
			PageSize: 10,
			Search:   "game-1",
			Sort:     "id",
			Order:    "asc",
		}

		result, err := repo.List(ctx, opts)
		assert.NoError(t, err)
		// 应该找到包含"game-1"的游戏（game-1, game-10, game-11, ..., game-19）
		assert.True(t, len(result.Items) > 0)
	})
}

func TestEnhancedRepo_Update(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 创建测试数据
	game := &Game{Name: "test-game", Status: "dev"}
	err := repo.Create(ctx, game)
	require.NoError(t, err)

	// 更新数据
	game.Status = "test"
	err = repo.Update(ctx, game)
	assert.NoError(t, err)

	// 验证更新
	fetched, err := repo.Get(ctx, game.ID)
	assert.NoError(t, err)
	assert.Equal(t, "test", fetched.Status)
}

func TestEnhancedRepo_Delete(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 创建测试数据
	game := &Game{Name: "test-game"}
	err := repo.Create(ctx, game)
	require.NoError(t, err)

	// 删除数据
	err = repo.Delete(ctx, game.ID)
	assert.NoError(t, err)

	// 验证删除
	_, err = repo.Get(ctx, game.ID)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestEnhancedRepo_BulkCreate(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 创建批量测试数据
	games := make([]*Game, 50)
	for i := 0; i < 50; i++ {
		games[i] = &Game{
			Name: fmt.Sprintf("bulk-game-%d", i),
		}
	}

	// 批量创建
	err := repo.BulkCreate(ctx, games)
	assert.NoError(t, err)

	// 验证创建结果
	opts := &database.ListOptions{
		Page:     1,
		PageSize: 100,
	}

	result, err := repo.List(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, int64(50), result.Pagination.Total)
}

func TestEnhancedRepo_GetByName(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 创建测试数据
	game := &Game{Name: "unique-game-name"}
	err := repo.Create(ctx, game)
	require.NoError(t, err)

	// 根据名称获取
	fetched, err := repo.GetByName(ctx, "unique-game-name")
	assert.NoError(t, err)
	assert.Equal(t, game.ID, fetched.ID)

	// 测试不存在的名称
	_, err = repo.GetByName(ctx, "non-existent")
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestEnhancedRepo_GetStats(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)

	stats := repo.GetStats()
	assert.NotNil(t, stats)
	assert.True(t, stats.MaxOpenConnections > 0)
}

func TestEnhancedRepo_Ping(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	err := repo.Ping(ctx)
	assert.NoError(t, err)
}

func TestEnhancedRepo_HealthCheck(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	err := repo.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestEnhancedRepo_GetGameCount(t *testing.T) {
	manager := setupTestDB(t)
	defer manager.Close()

	repo := NewEnhancedRepo(manager)
	ctx := context.Background()

	// 初始计数
	count, err := repo.GetGameCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 添加一些游戏
	for i := 0; i < 5; i++ {
		game := &Game{Name: fmt.Sprintf("count-game-%d", i)}
		err := repo.Create(ctx, game)
		require.NoError(t, err)
	}

	// 再次计数
	count, err = repo.GetGameCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}