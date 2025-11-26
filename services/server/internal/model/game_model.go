package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GameModel = (*customGameModel)(nil)

type (
	// GameModel 游戏数据访问接口
	GameModel interface {
		Insert(ctx context.Context, data *Game) (sql.Result, error)
		FindOne(ctx context.Context, id int64) (*Game, error)
		FindOneByGameId(ctx context.Context, gameId string) (*Game, error)
		Update(ctx context.Context, data *Game) error
		Delete(ctx context.Context, id int64) error
		List(ctx context.Context, page, pageSize int) ([]*Game, error)
		Count(ctx context.Context) (int64, error)
	}

	customGameModel struct {
		*defaultGameModel
	}

	defaultGameModel struct {
		sqlx.CachedConn
		table string
	}

	// Game 游戏
	Game struct {
		Id          int64           `db:"id"`
		GameId      string          `db:"game_id"`      // 游戏 ID（唯一标识）
		Name        string          `db:"name"`         // 游戏名称
		Description string          `db:"description"`  // 游戏描述
		Config      json.RawMessage `db:"config"`       // 游戏配置（JSON）
		Envs        json.RawMessage `db:"envs"`         // 游戏环境列表（JSON）
		Status      int8            `db:"status"`       // 0:禁用 1:启用
		CreatedAt   time.Time       `db:"created_at"`
		UpdatedAt   time.Time       `db:"updated_at"`
	}
)

// NewGameModel 创建游戏 Model
func NewGameModel(conn sqlx.SqlConn, c cache.CacheConf) GameModel {
	return &customGameModel{
		defaultGameModel: &defaultGameModel{
			CachedConn: sqlx.NewConn(conn, c),
			table:      "`games`",
		},
	}
}

// Insert 插入游戏
func (m *defaultGameModel) Insert(ctx context.Context, data *Game) (sql.Result, error) {
	query := fmt.Sprintf("INSERT INTO %s (game_id, name, description, config, envs, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", m.table)

	now := NowFunc()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = now
	}
	if data.UpdatedAt.IsZero() {
		data.UpdatedAt = now
	}

	ret, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query,
			data.GameId,
			data.Name,
			data.Description,
			data.Config,
			data.Envs,
			data.Status,
			data.CreatedAt,
			data.UpdatedAt,
		)
	})

	return ret, err
}

// FindOne 根据 ID 查找游戏
func (m *defaultGameModel) FindOne(ctx context.Context, id int64) (*Game, error) {
	cacheKey := fmt.Sprintf("cache:game:id:%d", id)
	var resp Game

	err := m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = ? LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, id)
	})

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// FindOneByGameId 根据 game_id 查找游戏
func (m *defaultGameModel) FindOneByGameId(ctx context.Context, gameId string) (*Game, error) {
	cacheKey := fmt.Sprintf("cache:game:game_id:%s", gameId)
	var resp Game

	err := m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE game_id = ? LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, gameId)
	})

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// Update 更新游戏
func (m *defaultGameModel) Update(ctx context.Context, data *Game) error {
	data.UpdatedAt = NowFunc()

	query := fmt.Sprintf("UPDATE %s SET game_id = ?, name = ?, description = ?, config = ?, envs = ?, status = ?, updated_at = ? WHERE id = ?", m.table)

	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query,
			data.GameId,
			data.Name,
			data.Description,
			data.Config,
			data.Envs,
			data.Status,
			data.UpdatedAt,
			data.Id,
		)
	}, m.getCacheKeys(data)...)

	return err
}

// Delete 删除游戏
func (m *defaultGameModel) Delete(ctx context.Context, id int64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", m.table)
	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query, id)
	}, m.getCacheKeys(data)...)

	return err
}

// List 分页查询游戏列表
func (m *defaultGameModel) List(ctx context.Context, page, pageSize int) ([]*Game, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY id DESC LIMIT ? OFFSET ?", m.table)

	var games []*Game
	err := m.QueryRowsNoCacheCtx(ctx, &games, query, pageSize, offset)
	if err != nil {
		return nil, err
	}

	return games, nil
}

// Count 统计游戏总数
func (m *defaultGameModel) Count(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", m.table)

	var count int64
	err := m.QueryRowNoCacheCtx(ctx, &count, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// getCacheKeys 获取缓存键列表
func (m *defaultGameModel) getCacheKeys(data *Game) []string {
	return []string{
		fmt.Sprintf("cache:game:id:%d", data.Id),
		fmt.Sprintf("cache:game:game_id:%s", data.GameId),
	}
}
