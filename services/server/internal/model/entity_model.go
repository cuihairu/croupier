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

var _ EntityModel = (*customEntityModel)(nil)

type (
	// EntityModel 实体数据访问接口
	EntityModel interface {
		Insert(ctx context.Context, data *Entity) (sql.Result, error)
		FindOne(ctx context.Context, id int64) (*Entity, error)
		Update(ctx context.Context, data *Entity) error
		Delete(ctx context.Context, id int64) error
		List(ctx context.Context, entityType string, page, pageSize int) ([]*Entity, error)
		Count(ctx context.Context, entityType string) (int64, error)
	}

	customEntityModel struct {
		*defaultEntityModel
	}

	defaultEntityModel struct {
		sqlx.CachedConn
		table string
	}

	// Entity 实体
	Entity struct {
		Id         int64           `db:"id"`
		Type       string          `db:"type"`        // 实体类型: player, item, quest 等
		Data       json.RawMessage `db:"data"`        // JSON 数据
		ProviderId string          `db:"provider_id"` // 提供者 ID
		Status     int8            `db:"status"`      // 0:禁用 1:启用
		CreatedAt  time.Time       `db:"created_at"`
		UpdatedAt  time.Time       `db:"updated_at"`
	}
)

// NewEntityModel 创建实体 Model
func NewEntityModel(conn sqlx.SqlConn, c cache.CacheConf) EntityModel {
	return &customEntityModel{
		defaultEntityModel: &defaultEntityModel{
			CachedConn: sqlx.NewConn(conn, c),
			table:      "`entities`",
		},
	}
}

// Insert 插入实体
func (m *defaultEntityModel) Insert(ctx context.Context, data *Entity) (sql.Result, error) {
	query := fmt.Sprintf("INSERT INTO %s (type, data, provider_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", m.table)

	now := NowFunc()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = now
	}
	if data.UpdatedAt.IsZero() {
		data.UpdatedAt = now
	}

	ret, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query,
			data.Type,
			data.Data,
			data.ProviderId,
			data.Status,
			data.CreatedAt,
			data.UpdatedAt,
		)
	})

	return ret, err
}

// FindOne 根据 ID 查找实体
func (m *defaultEntityModel) FindOne(ctx context.Context, id int64) (*Entity, error) {
	cacheKey := fmt.Sprintf("cache:entity:id:%d", id)
	var resp Entity

	err := m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = ? LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, id)
	})

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// Update 更新实体
func (m *defaultEntityModel) Update(ctx context.Context, data *Entity) error {
	data.UpdatedAt = NowFunc()

	query := fmt.Sprintf("UPDATE %s SET type = ?, data = ?, provider_id = ?, status = ?, updated_at = ? WHERE id = ?", m.table)

	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query,
			data.Type,
			data.Data,
			data.ProviderId,
			data.Status,
			data.UpdatedAt,
			data.Id,
		)
	}, m.getCacheKeys(data)...)

	return err
}

// Delete 删除实体
func (m *defaultEntityModel) Delete(ctx context.Context, id int64) error {
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

// List 分页查询实体列表
func (m *defaultEntityModel) List(ctx context.Context, entityType string, page, pageSize int) ([]*Entity, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	var query string
	var args []interface{}

	if entityType != "" {
		query = fmt.Sprintf("SELECT * FROM %s WHERE type = ? ORDER BY id DESC LIMIT ? OFFSET ?", m.table)
		args = []interface{}{entityType, pageSize, offset}
	} else {
		query = fmt.Sprintf("SELECT * FROM %s ORDER BY id DESC LIMIT ? OFFSET ?", m.table)
		args = []interface{}{pageSize, offset}
	}

	var entities []*Entity
	err := m.QueryRowsNoCacheCtx(ctx, &entities, query, args...)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

// Count 统计实体总数
func (m *defaultEntityModel) Count(ctx context.Context, entityType string) (int64, error) {
	var query string
	var args []interface{}

	if entityType != "" {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE type = ?", m.table)
		args = []interface{}{entityType}
	} else {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s", m.table)
		args = []interface{}{}
	}

	var count int64
	err := m.QueryRowNoCacheCtx(ctx, &count, query, args...)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// getCacheKeys 获取缓存键列表
func (m *defaultEntityModel) getCacheKeys(data *Entity) []string {
	return []string{
		fmt.Sprintf("cache:entity:id:%d", data.Id),
	}
}
